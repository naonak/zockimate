// internal/types/image.go

package types

import (
    "fmt"
)

// ImageReference représente une référence complète à une image Docker
type ImageReference struct {
    ID          string   // ID local de l'image
    RepoDigest  string   // Digest du repository (sha256)
    Tag         string   // Tag de l'image
    Original    string   // Référence originale (avant rollback)
    Platform    string   // Architecture/OS
}

// String retourne une représentation lisible de l'ImageReference
func (ir *ImageReference) String() string {
    if ir.RepoDigest != "" {
        return fmt.Sprintf("%s (%s)", ir.RepoDigest, ir.ID[:12])
    }
    if ir.Tag != "" {
        return fmt.Sprintf("%s (%s)", ir.Tag, ir.ID[:12])
    }
    return ir.ID[:12]
}

// BestReference retourne la meilleure référence disponible pour cette image
func (ir *ImageReference) BestReference() string {
    if ir.RepoDigest != "" {
        return ir.RepoDigest
    }
    if ir.Tag != "" {
        return ir.Tag
    }
    return ir.ID
}

// IsExactReference indique si on peut garantir la version exacte de l'image
func (ir *ImageReference) IsExactReference() bool {
    return ir.RepoDigest != "" || ir.ID != ""  // Soit on a un digest, soit un ID local
}