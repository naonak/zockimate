// internal/manager/check.go
package manager

import (
    "context"
    "fmt"

    "zockimate/pkg/utils"
    "zockimate/internal/types/options"
    "github.com/docker/docker/client"
)


func (cm *ContainerManager) RenameContainer(ctx context.Context, oldName, newName string, opts options.RenameOptions) error {
    cm.lock.Lock()
    defer cm.lock.Unlock()

    oldName = utils.CleanContainerName(oldName)
    newName = utils.CleanContainerName(newName)

    if !opts.DbOnly {
		// Vérifier si le nouveau nom existe déjà dans Docker 
		if _, err := cm.docker.InspectContainer(ctx, newName); err == nil {
			return fmt.Errorf("container with name %s already exists in Docker", newName)
		}

        // Inspecter le conteneur source
        ctn, err := cm.docker.InspectContainer(ctx, oldName)
        if client.IsErrNotFound(err) {
            return fmt.Errorf("source container %s does not exist", oldName)
        } else if err != nil {
            return fmt.Errorf("failed to inspect container: %w", err)
        }

        // Vérifier si le conteneur doit être géré
        if !cm.config.NoFilter && !utils.IsContainerEnabled(ctn.Config.Labels) {
            return fmt.Errorf("container not enabled for management")
        }

        // Vérifier si le conteneur doit être en cours d'exécution
        if !cm.config.All && !ctn.State.Running {
            return fmt.Errorf("container not running (use --all to include stopped containers)")
        }

        // Renommer dans Docker
        if err := cm.docker.ContainerRename(ctx, oldName, newName); err != nil {
            return fmt.Errorf("failed to rename container in Docker: %w", err)
        }
        cm.logger.Infof("Container renamed in Docker from %s to %s", oldName, newName)
    }

    // Update database
    affected, err := cm.db.RenameContainer(oldName, newName)
    if err != nil {
        if !opts.DbOnly {
            // Try to revert Docker rename if needed
            if revertErr := cm.docker.ContainerRename(ctx, newName, oldName); revertErr != nil {
                return fmt.Errorf("failed to update database and revert Docker rename failed: %v (original error: %v)", 
                    revertErr, err)
            }
        }
        return fmt.Errorf("failed to update container name in database: %w", err)
    }

    if affected > 0 {
        cm.logger.Infof("Updated %d database entries from %s to %s", affected, oldName, newName)
    } else {
        cm.logger.Warnf("No database entries found for container %s", oldName)
    }

    return nil
}