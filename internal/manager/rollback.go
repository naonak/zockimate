package manager

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/network"

    "zockimate/internal/types"
    "zockimate/pkg/utils"
)

func (cm *ContainerManager) RollbackContainer(ctx context.Context, name string, opts types.RollbackOptions) error {

    name = utils.CleanContainerName(name)
    cm.logger.Infof("Rolling back container %s to snapshot %d", name, opts.SnapshotID)

    // Récupérer le snapshot
    snapshot, err := cm.db.GetSnapshot(name, opts.SnapshotID)
    if err != nil {
        return fmt.Errorf("failed to get snapshot: %w", err)
    }

    // Créer un snapshot de sécurité avant le rollback
    safetySnapshot, err := cm.CreateSnapshot(ctx, name, types.NewSnapshotOptions(
        types.WithMessage(fmt.Sprintf("Auto-save before rollback to snapshot %d", snapshot.ID)),
        types.WithDryRun(false),
        types.WithForce(false),
        types.WithNoCleanup(true),
    ))

    if err != nil {
        return fmt.Errorf("failed to create safety snapshot: %w", err)
    }

    cm.lock.Lock()
    defer cm.lock.Unlock()

    var rollbackErr error
    defer func() {
        if rollbackErr != nil {
            cm.logger.Error("Rollback failed, attempting to restore from safety snapshot")
            cm.lock.Unlock()
            if err := cm.RollbackContainer(ctx, name, types.RollbackOptions{
                SnapshotID: safetySnapshot.ID,
                Image:     true,
                Data:      true,
                Config:    true,
                Force:     true,
            }); err != nil {
                cm.logger.Errorf("Failed to restore safety snapshot: %v", err)
            }
        }
    }()

    var config container.Config
    var hostConfig container.HostConfig
    var networkConfig network.NetworkingConfig

    if err := json.Unmarshal(snapshot.Config, &config); err != nil {
        rollbackErr = fmt.Errorf("failed to unmarshal config: %w", err)
        return rollbackErr
    }

    if err := json.Unmarshal(snapshot.HostConfig, &hostConfig); err != nil {
        rollbackErr = fmt.Errorf("failed to unmarshal host config: %w", err)
        return rollbackErr
    }

    if err := json.Unmarshal(snapshot.NetworkConfig, &networkConfig); err != nil {
        rollbackErr = fmt.Errorf("failed to unmarshal network config: %w", err)
        return rollbackErr
    }

    // Mettre à jour les labels pour le rollback
    if config.Labels == nil {
        config.Labels = make(map[string]string)
    }

    // Garder une trace du snapshot utilisé
    config.Labels["zockimate.snapshot_id"] = fmt.Sprintf("%d", snapshot.ID)

    // Si on doit restaurer l'image
    if opts.Image {
        // Vérifier si on peut garantir la version exacte
        if !opts.Force && !snapshot.ImageRef.IsExactReference() {
            rollbackErr = fmt.Errorf(
                "cannot guarantee exact image version for rollback (use --force to override)")
            return rollbackErr
        }

        // Utiliser la meilleure référence disponible
        imageRef := snapshot.ImageRef.BestReference()

        // Pull de l'image si nécessaire
        if err := cm.docker.PullImage(ctx, imageRef); err != nil {
            rollbackErr = fmt.Errorf("failed to pull rollback image: %w", err)
            return rollbackErr
        }

        // Mettre à jour la configuration
        config.Image = imageRef

        // Préserver l'image originale dans les labels
        config.Labels["zockimate.original_image"] = snapshot.ImageRef.Original
    }

    // Restaurer les données si demandé
    if opts.Data && snapshot.ZFSSnapshot != "" {
        if err := cm.zfs.RollbackSnapshot(snapshot.ZFSSnapshot); err != nil {
            rollbackErr = fmt.Errorf("failed to rollback ZFS snapshot: %w", err)
            return rollbackErr
        }
    }
    
    // Si on restaure l'image
    if opts.Image {
        config.Image = snapshot.ImageRef.BestReference()
        if snapshot.ImageRef.Original != "" {
            config.Labels["zockimate.original_image"] = snapshot.ImageRef.Original
        }
    }

    // Recréer le conteneur avec les pointeurs corrects
    if err := cm.docker.RecreateContainer(ctx, name, &config, &hostConfig, &networkConfig); err != nil {
        rollbackErr = fmt.Errorf("failed to recreate container: %w", err)
        return rollbackErr
    }
    
    // Attendre que le conteneur soit prêt
    timeout := utils.GetTimeout(config.Labels, opts.Timeout)
    cm.logger.Debugf("Waiting for container %s to be ready (timeout: %s)", name, timeout)

    if err := cm.docker.WaitForContainer(ctx, name, timeout); err != nil {
        rollbackErr = fmt.Errorf("container failed to become ready after rollback: %w", err)
        return rollbackErr
    }

    cm.logger.Infof("Successfully rolled back container %s to snapshot %d", name, snapshot.ID)

    if cm.notify != nil {
        cm.notifyf(
            "Rollback Successful",
            "Container %s successfully rolled back to snapshot %d (Image: %s)",
            name, snapshot.ID, snapshot.ImageRef.String(),
        )
    }

    return nil
}
