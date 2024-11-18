package options

import "time"

// CheckOptions définit les options pour la vérification des mises à jour
type CheckOptions struct {
    Force    bool      // Forcer la vérification même avec image locale
    Cleanup  bool      // Nettoyer les images téléchargées après vérification
    Timeout   time.Duration
}

// Définir une fonction pour créer des CheckOptions avec des valeurs par défaut
func NewCheckOptions(opts ...CheckOption) CheckOptions {
    options := CheckOptions{
        Force:    false,
        Cleanup:  true,
        Timeout:  DefaultCheckTimeout,
    }
    for _, opt := range opts {
        opt(&options)
    }
    return options
}

// Optionnellement, on peut ajouter des fonctions de type Option pour une configuration flexible
type CheckOption func(*CheckOptions)

func WithCheckForce(force bool) CheckOption {
    return func(o *CheckOptions) {
        o.Force = force
    }
}

func WithCheckCleanup(cleanup bool) CheckOption {
    return func(o *CheckOptions) {
        o.Cleanup = cleanup
    }
}

func WithCheckTimeout(timeout time.Duration) CheckOption {
    return func(o *CheckOptions) {
        o.Timeout = timeout
    }
}
