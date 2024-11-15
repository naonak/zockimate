// internal/docker/client.go
package docker

import (
    "context"
    "fmt"
    "io"
    "time"
    "encoding/json"

    "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/network"
    "github.com/docker/docker/client"
    "github.com/sirupsen/logrus"
    
    zTypes "zockimate/internal/types"
)

// Client encapsule le client Docker avec des fonctionnalités supplémentaires
type Client struct {
    cli    *client.Client
    logger *logrus.Logger
}

// NewClient crée une nouvelle instance du client Docker
func NewClient(logger *logrus.Logger) (*Client, error) {
    logger.Debug("Creating new Docker client...")

    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, fmt.Errorf("failed to create Docker client: %w", err)
    }

    // Test connection
    ctx := context.Background()
    if _, err := cli.Ping(ctx); err != nil {
        cli.Close()
        return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
    }

    logger.Debug("Successfully connected to Docker daemon")

    return &Client{
        cli:    cli,
        logger: logger,
    }, nil
}
// Close ferme le client Docker
func (c *Client) Close() error {
    return c.cli.Close()
}

// PullImage télécharge une image avec retry

func (c *Client) PullImage(ctx context.Context, ref string) error {
    c.logger.Debugf("Starting pull for image: %s", ref)

    reader, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
    if err != nil {
        c.logger.Debugf("Pull failed with error: %v", err)
        return fmt.Errorf("pull failed: %w", err)
    }
    defer reader.Close()

    _, err = io.Copy(io.Discard, reader)
    if err != nil {
        return fmt.Errorf("error reading pull response: %w", err)
    }

    c.logger.Debug("Pull completed successfully")
    return nil
}

// GetImageInfo récupère les informations complètes d'une image
func (c *Client) GetImageInfo(ctx context.Context, ref string) (*zTypes.ImageReference, error) {
    inspect, _, err := c.cli.ImageInspectWithRaw(ctx, ref)
    if err != nil {
        return nil, fmt.Errorf("failed to inspect image: %w", err)
    }

    imgRef := &zTypes.ImageReference{
        ID:       inspect.ID,
        Platform: fmt.Sprintf("%s/%s", inspect.Architecture, inspect.Os),
    }

    if len(inspect.RepoDigests) > 0 {
        imgRef.RepoDigest = inspect.RepoDigests[0]
    }
    if len(inspect.RepoTags) > 0 {
        imgRef.Tag = inspect.RepoTags[0]
    }

    return imgRef, nil
}

// RecreateContainer recrée un conteneur avec la nouvelle configuration
func (c *Client) RecreateContainer(ctx context.Context, name string, config *container.Config, 
    hostConfig *container.HostConfig, networkConfig *network.NetworkingConfig) error {
    
    // Vérifier si le conteneur existe et le stopper/supprimer
    if _, err := c.cli.ContainerInspect(ctx, name); err == nil {
        timeout := 30 // secondes
        if err := c.cli.ContainerStop(ctx, name, container.StopOptions{
            Timeout: &timeout,
        }); err != nil {
            return fmt.Errorf("failed to stop container: %w", err)
        }

        if err := c.cli.ContainerRemove(ctx, name, container.RemoveOptions{
            Force: true,
        }); err != nil {
            return fmt.Errorf("failed to remove container: %w", err)
        }
    }

    // Créer le nouveau conteneur
    resp, err := c.cli.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, name)
    if err != nil {
        return fmt.Errorf("failed to create container: %w", err)
    }

    // Démarrer le conteneur
    if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
        return fmt.Errorf("failed to start container: %w", err)
    }

    return nil
}

// WaitForContainer attend que le conteneur soit prêt
func (c *Client) WaitForContainer(ctx context.Context, name string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("timeout waiting for container to be ready")
        case <-ticker.C:
            container, err := c.cli.ContainerInspect(ctx, name)
            if err != nil {
                return err
            }

            if container.State.Health != nil {
                if container.State.Health.Status == "healthy" {
                    return nil
                }
            } else if container.State.Running {
                return nil
            }
        }
    }
}

// ListContainers liste les conteneurs selon les critères
func (c *Client) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
    opts := container.ListOptions{
        All: all,
    }
    return c.cli.ContainerList(ctx, opts)
}

// InspectContainer inspecte un conteneur avec gestion des erreurs
func (c *Client) InspectContainer(ctx context.Context, name string) (types.ContainerJSON, error) {
    ctn, err := c.cli.ContainerInspect(ctx, name)
    if err != nil {
        if client.IsErrNotFound(err) {
            return types.ContainerJSON{}, fmt.Errorf("container not found: %s", name)
        }
        return types.ContainerJSON{}, fmt.Errorf("failed to inspect container: %w", err)
    }
    return ctn, nil
}

// GetContainerConfigs extrait et sérialise les configurations d'un conteneur
func (c *Client) GetContainerConfigs(ctn types.ContainerJSON) ([]byte, []byte, []byte, error) {
    configJSON, err := json.Marshal(ctn.Config)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("failed to marshal container config: %w", err)
    }

    hostConfigJSON, err := json.Marshal(ctn.HostConfig)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("failed to marshal host config: %w", err)
    }

    networkConfigJSON, err := json.Marshal(&network.NetworkingConfig{
        EndpointsConfig: ctn.NetworkSettings.Networks,
    })
    if err != nil {
        return nil, nil, nil, fmt.Errorf("failed to marshal network config: %w", err)
    }

    return configJSON, hostConfigJSON, networkConfigJSON, nil
}

// UnmarshalConfigs désérialise les configurations d'un conteneur
func (c *Client) UnmarshalConfigs(configJSON, hostConfigJSON, networkConfigJSON []byte) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error) {
    var config container.Config
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return nil, nil, nil, fmt.Errorf("failed to unmarshal container config: %w", err)
    }

    var hostConfig container.HostConfig
    if err := json.Unmarshal(hostConfigJSON, &hostConfig); err != nil {
        return nil, nil, nil, fmt.Errorf("failed to unmarshal host config: %w", err)
    }

    var networkConfig network.NetworkingConfig
    if err := json.Unmarshal(networkConfigJSON, &networkConfig); err != nil {
        return nil, nil, nil, fmt.Errorf("failed to unmarshal network config: %w", err)
    }

    return &config, &hostConfig, &networkConfig, nil
}

// RemoveImage supprime une image avec options
func (c *Client) RemoveImage(ctx context.Context, imageID string) error {
    _, err := c.cli.ImageRemove(ctx, imageID, image.RemoveOptions{
        Force:         false,
        PruneChildren: true,
    })
    if err != nil {
        if !client.IsErrNotFound(err) {
            return fmt.Errorf("failed to remove image: %w", err)
        }
        // Ignorer si l'image n'existe pas
        c.logger.Debugf("Image %s already removed", imageID)
    }
    return nil
}