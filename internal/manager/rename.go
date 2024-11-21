// internal/manager/check.go
package manager

import (
    "context"
    "fmt"

    "zockimate/pkg/utils"
    "zockimate/internal/types/options"
    "github.com/docker/docker/client"
    "zockimate/internal/types"
)

// internal/manager/rename.go
func (cm *ContainerManager) RenameContainer(ctx context.Context, oldName, newName string, opts options.RenameOptions) (*types.RenameResult, error) {
    cm.lock.Lock()
    defer cm.lock.Unlock()

    result := &types.RenameResult{
        OldName: oldName,
        NewName: newName,
    }

    oldName = utils.CleanContainerName(oldName)
    newName = utils.CleanContainerName(newName)

    if !opts.DbOnly {
        // Vérifier si le nouveau nom existe déjà dans Docker 
        if _, err := cm.docker.InspectContainer(ctx, newName); err == nil {
            result.Error = fmt.Errorf("container with name %s already exists in Docker", newName)
            return result, nil
        }

        // Inspecter le conteneur source
        ctn, err := cm.docker.InspectContainer(ctx, oldName)
        if client.IsErrNotFound(err) {
            result.Error = fmt.Errorf("source container %s does not exist", oldName)
            return result, nil
        } else if err != nil {
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

        // Renommer dans Docker
        if err := cm.docker.ContainerRename(ctx, oldName, newName); err != nil {
            result.Error = fmt.Errorf("failed to rename container in Docker: %w", err)
            return result, nil
        }
        result.DockerRenamed = true
        cm.logger.Debugf("Container renamed in Docker from %s to %s", oldName, newName)
    }

    // Update database
    affected, err := cm.db.RenameContainer(oldName, newName)
    if err != nil {
        if !opts.DbOnly && result.DockerRenamed {
            // Try to revert Docker rename if needed
            if revertErr := cm.docker.ContainerRename(ctx, newName, oldName); revertErr != nil {
                result.Error = fmt.Errorf("failed to update database and revert Docker rename failed: %v (original error: %v)", 
                    revertErr, err)
                return result, nil
            }
        }
        result.Error = fmt.Errorf("failed to update container name in database: %w", err)
        return result, nil
    }

    result.EntriesRenamed = affected
    result.Success = true

    if affected > 0 {
        cm.logger.Debugf("Updated %d database entries from %s to %s", affected, oldName, newName)
    } else {
        cm.logger.Warnf("No database entries found for container %s", oldName)
    }

    return result, nil
}
