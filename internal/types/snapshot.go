// internal/types/snapshot.go

package types

import (
    "encoding/json"
    "time"
    "zockimate/pkg/utils"
)

// ContainerSnapshot représente une sauvegarde d'un conteneur à un instant T
type ContainerSnapshot struct {
    ID              int64           `json:"id"`
    ContainerName   string          `json:"container_name"`
    ImageRef        ImageReference  `json:"image_ref"`
    Config          []byte          `json:"config"`         // Configuration Docker sérialisée
    HostConfig      []byte          `json:"host_config"`    // Configuration Host sérialisée
    NetworkConfig   []byte          `json:"network_config"` // Configuration réseau sérialisée
    ZFSSnapshot     string          `json:"zfs_snapshot,omitempty"`
    Status          string          `json:"status"`
    Message         string          `json:"message"`
    CreatedAt       time.Time       `json:"created_at"`
}

// SnapshotMetadata contient les métadonnées d'un snapshot pour l'historique
type SnapshotMetadata struct {
    ID            int64     `json:"id"`
    ContainerName string    `json:"container_name"`
    ImageTag      string    `json:"image_tag"`
    ImageID       string    `json:"image_id"`
    RepoDigest    string    `json:"repo_digest,omitempty"`
    Status        string    `json:"status"`
    Message       string    `json:"message"`
    CreatedAt     time.Time `json:"created_at"`
}

// UnmarshalJSON pour les deux types
func (s *ContainerSnapshot) UnmarshalJSON(data []byte) error {
    type Alias ContainerSnapshot
    aux := &struct {
        CreatedAt string `json:"created_at"`
        *Alias
    }{
        Alias: (*Alias)(s),
    }
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }

    t, err := utils.ParseTime(aux.CreatedAt)
    if err != nil {
        return err
    }
    s.CreatedAt = t
    return nil
}

func (s *SnapshotMetadata) UnmarshalJSON(data []byte) error {
    type Alias SnapshotMetadata
    aux := &struct {
        CreatedAt string `json:"created_at"`
        *Alias
    }{
        Alias: (*Alias)(s),
    }
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }

    t, err := utils.ParseTime(aux.CreatedAt)
    if err != nil {
        return err
    }
    s.CreatedAt = t
    return nil
}