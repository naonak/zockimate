// internal/manager/update.go
package manager

import (

    "context"
    "encoding/json"
    "fmt"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/network"
    "github.com/docker/docker/client"

    "zockimate/pkg/utils"
    "zockimate/internal/types"
    "zockimate/internal/types/options"
)

func (cm *ContainerManager) UpdateContainer(ctx context.Context, name string, opts options.UpdateOptions) (*types.UpdateResult, error) {
    result := &types.UpdateResult{ContainerName: name}

    name = utils.CleanContainerName(name)
    cm.logger.Debugf("Starting update process for container: %s", name)

    if opts.DryRun {
        cm.logger.Debugf("Dry run: would update container %s", name)
        return result, nil
    }

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

    // Vérifier si le conteneur doit être en cours d'exécution
    if !cm.config.All && !ctn.State.Running {
        result.Error = fmt.Errorf("container not running (use --all to include stopped containers)")
        return result, nil
    }

    // Vérifier les mises à jour disponibles
    checkResult, err := cm.CheckContainer(ctx, name, options.NewCheckOptions(options.WithCheckCleanup(false)))
    if err != nil {
        result.Error = fmt.Errorf("failed to check for updates: %w", err)
        return result, nil
    }

    result.OldImage = checkResult.CurrentImage
    result.NewImage = checkResult.UpdateImage
    result.NeedsUpdate = checkResult.NeedsUpdate

    if !checkResult.NeedsUpdate && !opts.Force {
        cm.logger.Debugf("No update needed for container %s", name)
        return result, nil
    }

    cm.logger.Debugf("Create snapshot for container: %s", name)

    // Créer un snapshot de sécurité avant le rollback
    safetySnapshot, err := cm.CreateSnapshot(ctx, name, options.NewSnapshotOptions(
        options.WithSnapshotMessage("Pre-update snapshot"),
        options.WithSnapshotDryRun(false),
        options.WithSnapshotForce(false),
        options.WithSnapshotNoCleanup(true),
    ))

    if err != nil {
        return result, fmt.Errorf("failed to create pre-update snapshot: %w", err)
    }

    cm.lock.Lock()
    defer cm.lock.Unlock()

    // Récupérer la configuration actuelle
    containerConfig, hostConfig, networkConfig, err := cm.docker.GetContainerConfigs(ctn)
    if err != nil {
        return result, fmt.Errorf("failed to get container configurations: %w", err)
    }

    var config container.Config
    if err := json.Unmarshal(containerConfig, &config); err != nil {
        return result, fmt.Errorf("failed to unmarshal config: %w", err)
    }

    var hostCfg container.HostConfig
    if err := json.Unmarshal(hostConfig, &hostCfg); err != nil {
        return result, fmt.Errorf("failed to unmarshal host config: %w", err)
    }

    var netConfig network.NetworkingConfig
    if err := json.Unmarshal(networkConfig, &netConfig); err != nil {
        return result, fmt.Errorf("failed to unmarshal network config: %w", err)
    }

    // Préserver ou mettre à jour les labels importants
    if config.Labels == nil {
        config.Labels = make(map[string]string)
    }

    // Si on avait une image originale, la préserver
    if originalImage, ok := ctn.Config.Labels["zockimate.original_image"]; ok {
        config.Labels["zockimate.original_image"] = originalImage
        config.Image = originalImage // Utiliser l'image d'origine pour l'update
    }

    // Si snapshot_id, le supprimer
    if _, ok := ctn.Config.Labels["zockimate.snapshot_id"]; ok {
        delete(config.Labels, "zockimate.snapshot_id")
    }    

    // Créer le nouveau conteneur
    cm.logger.Debugf("Creating new container with image: %s", config.Image)
    if err := cm.docker.RecreateContainer(ctx, name, &config, &hostCfg, &netConfig); err != nil {
        return result, fmt.Errorf("failed to recreate container: %w", err)
    }    

    // Attendre que le conteneur soit prêt
    timeout := utils.GetTimeout(ctn.Config.Labels, opts.Timeout)
    cm.logger.Debugf("Waiting for container %s to be ready (timeout: %s)", name, timeout)
    
    // Modifier la gestion des erreurs pour WaitForContainer
    if err := cm.docker.WaitForContainer(ctx, name, timeout); err != nil {
        cm.logger.Error("Container failed to become ready, initiating rollback")
        result.RollbackNeeded = true
        
        cm.lock.Unlock()
    
        rollbackResult, rollbackErr := cm.RollbackContainer(ctx, name, options.RollbackOptions{
            SnapshotID: safetySnapshot.ID,
            Image:     true,
            Data:      true,
            Config:    true,
            Force:     true,
        })
    
        if rollbackErr != nil || !rollbackResult.Success {
            result.Error = fmt.Errorf("update failed and rollback failed: %v (original error: %v)", 
                rollbackResult.Error, err)
            return result, nil
        }
    
        result.Error = fmt.Errorf("update failed (rolled back to previous version: %d): %v", 
            rollbackResult.SnapshotID, err)
        return result, nil
    }

    result.Success = true

    cm.logger.Debugf("Successfully updated container %s to image %s",
        name, utils.ShortenID(checkResult.UpdateImage.ID))

    return result, nil
}