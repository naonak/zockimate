package manager

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/network"
    "github.com/docker/docker/client"

    "zockimate/internal/types"
    "zockimate/internal/types/options"
    "zockimate/pkg/utils"
)

// internal/manager/rollback.go
func (cm *ContainerManager) RollbackContainer(ctx context.Context, name string, opts options.RollbackOptions) (*types.RollbackResult, error) {
    result := &types.RollbackResult{
        ContainerName:   name,
        SnapshotID:     opts.SnapshotID,
        ImageRollback:  opts.Image,
        DataRollback:   opts.Data,
        ConfigRollback: opts.Config,
    }

    name = utils.CleanContainerName(name)
    cm.logger.Debugf("Rolling back container %s to snapshot %d", name, opts.SnapshotID)

    // Inspecter le conteneur
    ctn, err := cm.docker.InspectContainer(ctx, name)
    if err != nil {
        if client.IsErrNotFound(err) {
            result.Error = fmt.Errorf("container does not exist: %w", err)
            return result, nil
        }
        result.Error = fmt.Errorf("failed to inspect container: %w", err)
        return result, nil
    }

    // Vérifier si le conteneur doit être géré
    if !cm.config.NoFilter && !utils.IsContainerEnabled(ctn.Config.Labels) {
        result.Error = fmt.Errorf("container not enabled for management")
        return result, nil
    }

    // Récupérer le snapshot
    snapshot, err := cm.db.GetSnapshot(name, opts.SnapshotID)
    if err != nil {
        result.Error = fmt.Errorf("failed to get snapshot: %w", err)
        return result, nil
    }

    // Créer un snapshot de sécurité
    safetySnapshot, err := cm.CreateSnapshot(ctx, name, options.NewSnapshotOptions(
        options.WithSnapshotMessage(fmt.Sprintf("Auto-save before rollback to snapshot %d", snapshot.ID)),
        options.WithSnapshotDryRun(false),
        options.WithSnapshotForce(false),
        options.WithSnapshotNoCleanup(true),
    ))

    if err != nil {
        result.Error = fmt.Errorf("failed to create safety snapshot: %w", err)
        return result, nil
    }

    result.SafetySnapshot = safetySnapshot.ID

    cm.lock.Lock()
    defer cm.lock.Unlock()
    
    defer func() {
        if result.Error != nil {
            cm.logger.Error("Rollback failed, attempting to restore from safety snapshot")
            cm.lock.Unlock()
            
            var err error // Déclarer err localement
            var safetyResult *types.RollbackResult
            safetyResult, err = cm.RollbackContainer(ctx, name, options.RollbackOptions{
                SnapshotID: safetySnapshot.ID,
                Image:     true,
                Data:      true,
                Config:    true,
                Force:     true,
            })
            
            if err != nil || !safetyResult.Success {
                result.Error = fmt.Errorf("failed to restore safety snapshot: %v (original error: %v)", 
                    safetyResult.Error, result.Error)
            }
        }
    }()

    var config container.Config
    var hostConfig container.HostConfig
    var networkConfig network.NetworkingConfig

    if err := json.Unmarshal(snapshot.Config, &config); err != nil {
        result.Error = fmt.Errorf("failed to unmarshal config: %w", err)
        return result, result.Error
    }

    if err := json.Unmarshal(snapshot.HostConfig, &hostConfig); err != nil {
        result.Error = fmt.Errorf("failed to unmarshal host config: %w", err)
        return result, result.Error
    }

    if err := json.Unmarshal(snapshot.NetworkConfig, &networkConfig); err != nil {
        result.Error = fmt.Errorf("failed to unmarshal network config: %w", err)
        return result, result.Error
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
            result.Error = fmt.Errorf(
                "cannot guarantee exact image version for rollback (use --force to override)")
            return result, result.Error
        }

        // Utiliser la meilleure référence disponible
        imageRef := snapshot.ImageRef.BestReference()

        // Pull de l'image si nécessaire
        if err := cm.docker.PullImage(ctx, imageRef); err != nil {
            result.Error = fmt.Errorf("failed to pull rollback image: %w", err)
            return result, result.Error
        }

        // Mettre à jour la configuration
        config.Image = imageRef

        // Préserver l'image originale dans les labels
        config.Labels["zockimate.original_image"] = snapshot.ImageRef.Original
    }

    // Restaurer les données si demandé
    if opts.Data && snapshot.ZFSSnapshot != "" {
        if err := cm.zfs.RollbackSnapshot(snapshot.ZFSSnapshot); err != nil {
            result.Error = fmt.Errorf("failed to rollback ZFS snapshot: %w", err)
            return result, result.Error
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
        result.Error = fmt.Errorf("failed to recreate container: %w", err)
        return result, result.Error
    }
    
    // Attendre que le conteneur soit prêt
    timeout := utils.GetTimeout(config.Labels, opts.Timeout)
    cm.logger.Debugf("Waiting for container %s to be ready (timeout: %s)", name, timeout)

    if err := cm.docker.WaitForContainer(ctx, name, timeout); err != nil {
        result.Error = fmt.Errorf("container failed to become ready after rollback: %w", err)
        return result, result.Error
    }

    result.Success = true

    cm.logger.Debugf("Successfully rolled back container %s to snapshot %d", name, snapshot.ID)

    if cm.notify != nil {
        cm.notifyf(
            "Rollback Successful",
            "Container %s successfully rolled back to snapshot %d (Image: %s)",
            name, snapshot.ID, snapshot.ImageRef.String(),
        )
    }

    return result, nil
}