// internal/manager/check.go
package manager

import (
    "context"
    "fmt"

    "zockimate/pkg/utils"
    "zockimate/internal/types"
    "zockimate/internal/types/options"
    "github.com/docker/docker/client"
)

// CheckContainer vérifie si une mise à jour est disponible pour un conteneur
func (cm *ContainerManager) CheckContainer(ctx context.Context, name string, opts options.CheckOptions) (types.CheckResult, error) {

    cm.lock.Lock()
    defer cm.lock.Unlock()

    result := types.CheckResult{}
    name = utils.CleanContainerName(name)
    cm.logger.Debugf("Starting check process for container: %s", name)

    // Inspecter le conteneur
    ctn, err := cm.docker.InspectContainer(ctx, name)
    if err != nil {
        if client.IsErrNotFound(err) {
            return result, fmt.Errorf("container does not exist: %w", err)
        }
        return result, fmt.Errorf("failed to inspect container: %w", err)
    }

    // Vérifier si le conteneur doit être géré
    if !cm.config.NoFilter && !utils.IsContainerEnabled(ctn.Config.Labels) {
        return result, fmt.Errorf("container not enabled for management")
    }

    // Vérifier si le conteneur doit être en cours d'exécution
    if !cm.config.All && !ctn.State.Running {
        return result, fmt.Errorf("container not running (use --all to include stopped containers)")
    }

    // Obtenir la référence de l'image actuelle
    currentImage, err := cm.docker.GetImageInfo(ctx, ctn.Image)
    if err != nil {
        return result, err
    }
    result.CurrentImage = currentImage

    // Déterminer l'image à vérifier pour la mise à jour
    updateRef := ctn.Config.Labels["zockimate.original_image"]
    if updateRef == "" {
        updateRef = ctn.Config.Image
    }

    // Créer un contexte avec timeout pour le pull
    pullCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
    defer cancel()

    // Pull de la dernière version
    if err := cm.docker.PullImage(pullCtx, updateRef); err != nil {
        return result, fmt.Errorf("failed to pull update image: %w", err)
    }

    // Obtenir les infos de la nouvelle image
    latestImage, err := cm.docker.GetImageInfo(ctx, updateRef)
    if err != nil {
        return result, err
    }
    result.UpdateImage = latestImage

    // Vérifier la compatibilité des architectures
    if currentImage.Platform != latestImage.Platform {
        return result, fmt.Errorf("architecture mismatch: current=%s, latest=%s",
            currentImage.Platform, latestImage.Platform)
    }

    // Comparer les images
	result.NeedsUpdate = false
	if currentImage.IsExactReference() && latestImage.IsExactReference() {
		// On devrait d'abord vérifier si les architectures correspondent
		if currentImage.Platform != latestImage.Platform {
			return result, fmt.Errorf("architecture mismatch: current=%s, latest=%s", 
				currentImage.Platform, latestImage.Platform)
		}
        // Si on a des références exactes, on peut comparer directement
        if currentImage.RepoDigest != "" && latestImage.RepoDigest != "" {
            result.NeedsUpdate = currentImage.RepoDigest != latestImage.RepoDigest
        } else {
            result.NeedsUpdate = currentImage.ID != latestImage.ID
        }
    } else {
        // Sinon on compare les IDs après pull
        result.NeedsUpdate = currentImage.ID != latestImage.ID
    }

    // Nettoyer l'image téléchargée si demandé
    if opts.Cleanup && result.NeedsUpdate {
        cm.logger.Debugf("Starting cleanup image : %s", name)
        if err := cm.docker.RemoveImage(ctx, latestImage.ID); err != nil {
            cm.logger.Warnf("Failed to cleanup image %s: %v", latestImage.ID, err)
        }
    }

    if result.NeedsUpdate {
        cm.logger.Debugf("Update available for %s: %s -> %s",
            name,
            utils.ShortenID(currentImage.ID),
            utils.ShortenID(latestImage.ID))

        if cm.notify != nil && opts.Notify {
            cm.notifyf(
                "Update Available",
                "Container %s has an update available.\nCurrent: %s\nLatest: %s",
                name,
                currentImage.String(),
                latestImage.String(),
            )
        }
    } else {
        cm.logger.Debugf("No update needed for %s", name)
    }

    return result, nil
}

// GetContainers retourne la liste des conteneurs à gérer
func (cm *ContainerManager) GetContainers(ctx context.Context) ([]string, error) {
    containers, err := cm.docker.ListContainers(ctx, cm.config.All)
    if err != nil {
        return nil, fmt.Errorf("failed to list containers: %w", err)
    }

    var managed []string
    for _, ctn := range containers {
        name := utils.CleanContainerName(ctn.Names[0])
        if cm.config.NoFilter || utils.IsContainerEnabled(ctn.Labels) {
            managed = append(managed, name)
        }
    }

    return managed, nil
}
