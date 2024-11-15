// pkg/utils/utils.go
package utils

import (
    "context"
    "encoding/json"
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
func GetTimeout(labels map[string]string, defaultTimeout time.Duration) time.Duration {
    if timeout, ok := labels["zockimate.timeout"]; ok {
        // Parser le timeout depuis le label
        if d, err := time.ParseDuration(timeout); err == nil {
            // Vérifier que le timeout est positif et raisonnable (max 24h)
            if d > 0 && d <= 24*time.Hour {
                return d
            }
            logrus.Warnf("Invalid timeout value in label: %s, using default", timeout)
        } else {
            logrus.Warnf("Failed to parse timeout from label: %s, using default", timeout)
        }
    }
    return defaultTimeout
}

// JSON helpers
// -----------

// PrettyJSON retourne une représentation JSON indentée
func PrettyJSON(v interface{}) string {
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return fmt.Sprintf("error marshaling JSON: %v", err)
    }
    return string(b)
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

// Retry helpers
// ------------

// RetryOptions définit les options pour la fonction Retry
type RetryOptions struct {
    MaxAttempts int
    Delay       time.Duration
    OnRetry     func(attempt int, err error)
}

// Retry exécute une fonction avec retry
func Retry(ctx context.Context, fn func() error, opts RetryOptions) error {
    if opts.MaxAttempts == 0 {
        opts.MaxAttempts = 3
    }
    if opts.Delay == 0 {
        opts.Delay = time.Second * 5
    }

    var lastErr error
    for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := fn(); err == nil {
                return nil
            } else {
                lastErr = err
                if opts.OnRetry != nil {
                    opts.OnRetry(attempt, err)
                }
                if attempt < opts.MaxAttempts {
                    time.Sleep(opts.Delay)
                }
            }
        }
    }
    return fmt.Errorf("failed after %d attempts: %w", opts.MaxAttempts, lastErr)
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