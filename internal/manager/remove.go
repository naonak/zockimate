package manager

import (
    "context"
    "fmt"

    "zockimate/pkg/utils"
    "zockimate/internal/types/options"
)

func (cm *ContainerManager) RemoveContainer(ctx context.Context, name string, opts options.RemoveOptions) error {
    cm.lock.Lock()
    defer cm.lock.Unlock()

    name = utils.CleanContainerName(name)
    cm.logger.Infof("Starting remove process for container: %s", name)

    if opts.DryRun {
        cm.logger.Infof("Dry run: would remove container %s", name)
        return nil
    }

    // Vérifier si le conteneur existe dans Docker
    _, err := cm.docker.InspectContainer(ctx, name)
    containerExists := err == nil

    if containerExists {
        if !opts.Force && !opts.WithContainer {
            return fmt.Errorf("container %s still exists in Docker. Use --force or --with-container to remove anyway", name)
        }

        if opts.WithContainer {
            // Arrêter et supprimer le conteneur
            if err := cm.docker.RemoveContainer(ctx, name, true); err != nil {
                return fmt.Errorf("failed to remove Docker container: %w", err)
            }
            cm.logger.Infof("Removed Docker container: %s", name)
        }
    }

    // Supprimer les entrées de la base de données
    deleted, err := cm.db.RemoveEntries(name, opts)
    if err != nil {
        return fmt.Errorf("failed to remove database entries: %w", err)
    }

    if deleted > 0 {
        cm.logger.Infof("Removed %d database entries for container %s", deleted, name)
    } else {
        cm.logger.Warnf("No database entries found for container %s", name)
    }

    return nil
}