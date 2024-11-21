// internal/manager/manager.go
package manager

import (
    "fmt"
    "sync"
    "time"
    "context"
    
    "zockimate/pkg/utils"
    "zockimate/internal/config"
    "zockimate/internal/docker"
    "zockimate/internal/storage/database"
    "zockimate/internal/storage/zfs"
    "zockimate/internal/notify"
    "zockimate/internal/types"
    "zockimate/internal/types/options"
    "github.com/sirupsen/logrus"
)

// ContainerManager coordonne toutes les opérations sur les conteneurs
type ContainerManager struct {
    docker  *docker.Client
    db      *database.Database
    zfs     *zfs.ZFSManager
    notify  *notify.AppriseClient
    config  *config.Config
    logger  *logrus.Logger
    lock    sync.RWMutex
}

// NewContainerManager crée une nouvelle instance du manager
func NewContainerManager(cfg *config.Config) (*ContainerManager, error) {
    logger := cfg.Logger
    if logger == nil {
        logger = logrus.New()
        logger.SetLevel(logrus.InfoLevel)
    }

    // Initialiser le client Docker
    dockerClient, err := docker.NewClient(logger)
    if err != nil {
        return nil, fmt.Errorf("failed to create Docker client: %w", err)
    }

    // Initialiser la base de données
    db, err := database.NewDatabase(cfg.DbPath, logger)
    if err != nil {
        dockerClient.Close()
        return nil, fmt.Errorf("failed to initialize database: %w", err)
    }

    // Initialiser le gestionnaire ZFS
    zfsManager := zfs.NewZFSManager(logger)

    // Initialiser le client Apprise si configuré
    var notifier *notify.AppriseClient
    if cfg.AppriseURL != "" {
        var err error
        notifier, err = notify.NewAppriseClient(cfg.AppriseURL, logger)
        if err != nil {
            logger.Warnf("Failed to initialize Apprise notifications: %v", err)
        }
    }

    return &ContainerManager{
        docker:  dockerClient,
        db:      db,
        zfs:     zfsManager,
        notify:  notifier,
        config:  cfg,
        logger:  logger,
    }, nil
}

// Close libère les ressources
func (cm *ContainerManager) Close() error {
    var errs []error
    
    if err := cm.docker.Close(); err != nil {
        errs = append(errs, fmt.Errorf("failed to close Docker client: %w", err))
    }
    if err := cm.db.Close(); err != nil {
        errs = append(errs, fmt.Errorf("failed to close database: %w", err))
    }
    if cm.notify != nil {
        if err := cm.notify.Close(); err != nil {
            errs = append(errs, fmt.Errorf("failed to close Apprise client: %w", err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("errors closing manager: %v", errs)
    }
    return nil
}

// notifyf envoie une notification si configuré
func (cm *ContainerManager) notifyf(title, format string, args ...interface{}) {
    if cm.notify != nil {
        message := fmt.Sprintf(format, args...)
        if err := cm.notify.SendNotification(title, message, nil); err != nil {
            cm.logger.Warnf("Failed to send notification: %v", err)
        }
    }
}

// GetHistory récupère l'historique des snapshots
func (cm *ContainerManager) GetHistory(opts options.HistoryOptions) ([]types.SnapshotMetadata, error) {
    cm.lock.RLock()
    defer cm.lock.RUnlock()

    return cm.db.GetHistory(opts)
}

func (cm *ContainerManager) CreateSnapshot(ctx context.Context, name string, opts options.SnapshotOptions) (*types.ContainerSnapshot, error) {
    cm.lock.Lock()
    defer cm.lock.Unlock()

    name = utils.CleanContainerName(name)
    cm.logger.Debugf("Creating snapshot for container %s: %s", name, opts.Message)

    // Si dry-run, simuler seulement
    if opts.DryRun {
        cm.logger.Debugf("Dry run: would create snapshot for container %s", name)
        return nil, nil
    }

    // Inspecter le conteneur
    ctn, err := cm.docker.InspectContainer(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("failed to inspect container: %w", err)
    }

    // Vérifier si le conteneur est en cours d'exécution sauf si force
    if !opts.Force && !ctn.State.Running {
        return nil, fmt.Errorf("container is not running (use --force to snapshot anyway)")
    }

    // Obtenir les références de l'image
    imageRef, err := cm.docker.GetImageInfo(ctx, ctn.Image)
    if err != nil {
        return nil, fmt.Errorf("failed to get image info: %w", err)
    }

    // Si une image originale est définie, la préserver
    if originalImage, ok := ctn.Config.Labels["zockimate.original_image"]; ok {
        imageRef.Original = originalImage
    } else {
        // Sinon utiliser l'image actuelle
        imageRef.Original = ctn.Config.Image
    }

    // Créer le snapshot ZFS si configuré
    var zfsSnapshot string
    if dataset := utils.GetZFSDataset(ctn.Config.Labels); dataset != "" {
        snapshot, err := cm.zfs.CreateSnapshot(dataset)
        if err != nil {
            return nil, fmt.Errorf("failed to create ZFS snapshot: %w", err)
        }
        zfsSnapshot = snapshot
    }

    // Obtenir les configurations
    config, hostConfig, networkConfig, err := cm.docker.GetContainerConfigs(ctn)
    if err != nil {
        if zfsSnapshot != "" {
            cm.zfs.DeleteSnapshot(zfsSnapshot)
        }
        return nil, fmt.Errorf("failed to get container configs: %w", err)
    }

    // Créer le snapshot
    snapshot := &types.ContainerSnapshot{
        ContainerName:  name,
        ImageRef:      *imageRef,
        Config:        config,
        HostConfig:    hostConfig,
        NetworkConfig: networkConfig,
        ZFSSnapshot:   zfsSnapshot,
        Status:        "snapshot",
        Message:       opts.Message,
        CreatedAt:     time.Now().UTC(),
    }

    // Sauvegarder dans la base de données
    if err := cm.db.SaveSnapshot(snapshot); err != nil {
        if zfsSnapshot != "" {
            cm.zfs.DeleteSnapshot(zfsSnapshot)
        }
        return nil, fmt.Errorf("failed to save snapshot: %w", err)
    }

    // Nettoyer les anciens snapshots sauf si NoCleanup
    if !opts.NoCleanup {
        if err := cm.db.CleanupSnapshots(name, cm.config.Retention); err != nil {
            cm.logger.Warnf("Failed to cleanup old snapshots: %v", err)
        }
    }

    cm.logger.Debugf("Successfully created snapshot %d for container %s", snapshot.ID, name)

    return snapshot, nil
}