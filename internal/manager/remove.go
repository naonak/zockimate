package manager

import (
    "context"
    "fmt"

    "zockimate/pkg/utils"
    "zockimate/internal/types"
    "zockimate/internal/types/options"
)

// internal/manager/remove.go
func (cm *ContainerManager) RemoveContainer(ctx context.Context, name string, opts options.RemoveOptions) (*types.RemoveResult, error) {
    cm.lock.Lock()
    defer cm.lock.Unlock()

    result := &types.RemoveResult{ContainerName: name}
    name = utils.CleanContainerName(name)
    cm.logger.Debugf("Starting remove process for container: %s", name)

    if opts.DryRun {
        cm.logger.Debugf("Dry run: would remove container %s", name)
        return result, nil
    }

    // Vérifier si le conteneur existe dans Docker
    _, err := cm.docker.InspectContainer(ctx, name)
    containerExists := err == nil

    if containerExists {
        if !opts.Force && !opts.WithContainer {
            result.Error = fmt.Errorf("container %s still exists in Docker. Use --force or --with-container to remove anyway", name)
            return result, nil
        }

        if opts.WithContainer {
            if err := cm.docker.RemoveContainer(ctx, name, true); err != nil {
                result.Error = fmt.Errorf("failed to remove Docker container: %w", err)
                return result, nil
            }
            result.ContainerRemoved = true
            cm.logger.Debugf("Removed Docker container: %s", name)
        }
    }

    // Supprimer les entrées de la base de données
    deleted, err := cm.db.RemoveEntries(name, opts)
    if err != nil {
        result.Error = fmt.Errorf("failed to remove database entries: %w", err)
        return result, nil
    }

    result.EntriesDeleted = deleted
    result.Success = true

    if deleted > 0 {
        cm.logger.Debugf("Removed %d database entries for container %s", deleted, name)
    } else {
        cm.logger.Warnf("No database entries found for container %s", name)
    }

    return result, nil
}