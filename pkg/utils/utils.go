// pkg/utils/utils.go
package utils

import (
    "fmt"
    "strings"
    "time"

    "github.com/sirupsen/logrus"
)

// Docker ID helpers
// ----------------

// ShortenID raccourcit un ID Docker à sa forme courte (12 caractères)
func ShortenID(id string) string {
    if len(id) > 12 {
        return id[:12]
    }
    return id
}

// Time helpers
// -----------

// GetTimeout récupère un timeout depuis un label avec valeur par défaut
func GetTimeout(labels map[string]string, defaultTimeout time.Duration, logger *logrus.Logger) time.Duration {
    if timeout, ok := labels["zockimate.timeout"]; ok {
        // Parser le timeout depuis le label
        if d, err := time.ParseDuration(timeout); err == nil {
            // Vérifier que le timeout est positif et raisonnable (max 24h)
            if d > 0 && d <= 24*time.Hour {
                return d
            }
            logger.Warnf("Invalid timeout value in label: %s, using default", timeout)
        } else {
            logger.Warnf("Failed to parse timeout from label: %s, using default", timeout)
        }
    }
    return defaultTimeout
}

// Docker Label helpers
// ------------------

// IsContainerEnabled vérifie si un conteneur est activé pour zockimate
func IsContainerEnabled(labels map[string]string) bool {
    enabled, ok := labels["zockimate.enable"]
    return ok && enabled == "true"
}

// GetZFSDataset récupère le dataset ZFS configuré pour un conteneur
func GetZFSDataset(labels map[string]string) string {
    return labels["zockimate.zfs_dataset"]
}

// ParseTime essaie de parser une chaîne de date avec différents formats
func ParseTime(timeStr string) (time.Time, error) {
    for _, layout := range []string{
        "2006-01-02 15:04:05",
        time.RFC3339,      // Format avec T et Z
        "2006-01-02T15:04:05Z07:00",
        time.RFC3339Nano,
    } {
        if t, err := time.Parse(layout, timeStr); err == nil {
            return t, nil
        }
    }
    return time.Time{}, fmt.Errorf("failed to parse time: %q", timeStr)
}

// Log helpers
// ----------

// CleanContainerName retire le "/" initial du nom d'un conteneur
func CleanContainerName(name string) string {
    return strings.TrimPrefix(name, "/")
}