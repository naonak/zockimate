// internal/manager/update.go
package manager

import (

    "context"
    "encoding/json"
    "fmt"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/network"

    "zockimate/pkg/utils"
    "zockimate/internal/types"
)

// UpdateContainer met à jour un conteneur
func (cm *ContainerManager) UpdateContainer(ctx context.Context, name string, opts types.UpdateOptions) error {

    name = utils.CleanContainerName(name)
    cm.logger.Infof("Starting update process for container: %s", name)

    if opts.DryRun {
        cm.logger.Infof("Dry run: would update container %s", name)
        return nil
    }

    cm.logger.Infof("CheckContainer for container: %s", name)

    // Vérifier les mises à jour disponibles
    checkResult, err := cm.CheckContainer(ctx, name, types.CheckOptions{
        Force: opts.Force,
        Timeout:   types.DefaultCheckTimeout,
        Cleanup: false,
    })

    cm.logger.Infof("CheckContainer ok for container: %s", name)

    if err != nil {
        return fmt.Errorf("failed to check for updates: %w", err)
    }

    if !checkResult.NeedsUpdate && !opts.Force {
        cm.logger.Infof("No update needed for container %s", name)
        return nil
    }

    cm.logger.Infof("Create snapshot for container: %s", name)

    // Créer un snapshot de sécurité avant le rollback
    safetySnapshot, err := cm.CreateSnapshot(ctx, name, types.NewSnapshotOptions(
        types.WithMessage("Pre-update snapshot"),
        types.WithDryRun(false),
        types.WithForce(false),
        types.WithNoCleanup(true),
    ))

    if err != nil {
        return fmt.Errorf("failed to create pre-update snapshot: %w", err)
    }

    cm.lock.Lock()
    defer cm.lock.Unlock()

    // Inspecter le conteneur actuel
    ctn, err := cm.docker.InspectContainer(ctx, name)
    if err != nil {
        return fmt.Errorf("failed to inspect container: %w", err)
    }

    // Récupérer la configuration actuelle
    containerConfig, hostConfig, networkConfig, err := cm.docker.GetContainerConfigs(ctn)
    if err != nil {
        return fmt.Errorf("failed to get container configurations: %w", err)
    }

    var config container.Config
    if err := json.Unmarshal(containerConfig, &config); err != nil {
        return fmt.Errorf("failed to unmarshal config: %w", err)
    }

    var hostCfg container.HostConfig
    if err := json.Unmarshal(hostConfig, &hostCfg); err != nil {
        return fmt.Errorf("failed to unmarshal host config: %w", err)
    }

    var netConfig network.NetworkingConfig
    if err := json.Unmarshal(networkConfig, &netConfig); err != nil {
        return fmt.Errorf("failed to unmarshal network config: %w", err)
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
        return fmt.Errorf("failed to recreate container: %w", err)
    }    

    // Attendre que le conteneur soit prêt
    timeout := utils.GetTimeout(ctn.Config.Labels, opts.Timeout)
    cm.logger.Debugf("Waiting for container %s to be ready (timeout: %s)", name, timeout)
    
    if err := cm.docker.WaitForContainer(ctx, name, timeout); err != nil {
        cm.logger.Error("Container failed to become ready, initiating rollback")
        
        cm.lock.Unlock()

        if rollbackErr := cm.RollbackContainer(ctx, name, types.RollbackOptions{
            SnapshotID: safetySnapshot.ID,
            Image:     true,
            Data:      true,
            Config:    true,
            Force:     true,
        }); rollbackErr != nil {
            return fmt.Errorf("update failed and rollback failed: %v (original error: %v)", 
                rollbackErr, err)
        }
        return fmt.Errorf("update failed, rolled back to previous version: %v", err)
    }

    cm.logger.Infof("Successfully updated container %s to image %s",
        name, utils.ShortenID(checkResult.UpdateImage.ID))

    if cm.notify != nil {
        cm.notifyf(
            "Update Successful",
            "Container %s successfully updated from %s to %s",
            name,
            checkResult.CurrentImage.String(),
            checkResult.UpdateImage.String(),
        )
    }

    return nil
}