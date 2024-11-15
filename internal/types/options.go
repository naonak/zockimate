// internal/types/options.go

package types

import (
    "fmt"
    "time"
)

const (
    DefaultContainerReadyTimeout = 30 * time.Minute
    DefaultOperationTimeout = 10 * time.Minute
    DefaultPullTimeout      = 30 * time.Minute   
    DefaultCheckTimeout     = 30 * time.Minute   
    DefaultRollbackTimeout  = 30 * time.Minute   
    DefaultUpdateTimeout    = 30 * time.Minute   
    DefaultStopTimeout      = 30 * time.Second
    DefaultStartTimeout     = 30 * time.Second
    DefaultHealthTimeout    = 5 * time.Minute
    MinContainerTimeout    = 30 * time.Second
    MaxContainerTimeout    = 24 * time.Hour
)

type UpdateOptions struct {
    Force     bool
    DryRun    bool
    Timeout   time.Duration
    ContainerReadyTimeout   time.Duration
}

func NewUpdateOptions() UpdateOptions {
    return UpdateOptions{
        Force:   false,
        DryRun:  false,
        Timeout: DefaultUpdateTimeout,
        ContainerReadyTimeout: DefaultContainerReadyTimeout,
    }
}

// Validate vérifie que les options sont valides
func (o *UpdateOptions) Validate() error {
    if o.Timeout < MinContainerTimeout {
        return fmt.Errorf("timeout too short (minimum %s)", MinContainerTimeout)
    }
    if o.Timeout > MaxContainerTimeout {
        return fmt.Errorf("timeout too long (maximum %s)", MaxContainerTimeout)
    }
    return nil
}


type RollbackOptions struct {
    SnapshotID  int64
    Image       bool
    Data        bool
    Config      bool
    Force       bool
    Timeout     time.Duration
}

// CheckOptions définit les options pour la vérification des mises à jour
type CheckOptions struct {
    Force    bool      // Forcer la vérification même avec image locale
    Cleanup  bool      // Nettoyer les images téléchargées après vérification
    Timeout   time.Duration
}

// HistoryOptions définit les options pour la consultation de l'historique
type HistoryOptions struct {
    Limit     int           // Limite du nombre d'entrées
    Last      bool          // Seulement la dernière entrée par conteneur
    SortBy    string        // Tri (date|container)
    JSON      bool          // Sortie au format JSON
    Search    string        // Recherche dans les messages et statuts
    Since     time.Time     // Depuis date
    Before    time.Time     // Avant date
    Container []string      // Filtrer par conteneurs
}

// Error types personnalisés
type ErrorType int

const (
    ErrTypeGeneric ErrorType = iota
    ErrTypeDocker
    ErrTypeDatabase
    ErrTypeZFS
    ErrTypeValidation
)

// CustomError représente une erreur avec contexte
type CustomError struct {
    Type    ErrorType
    Message string
    Cause   error
}

func (e *CustomError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Cause)
    }
    return e.Message
}

// NewError crée une nouvelle erreur personnalisée
func NewError(errType ErrorType, message string, cause error) error {
    return &CustomError{
        Type:    errType,
        Message: message,
        Cause:   cause,
    }
}

type SnapshotOptions struct {
    Message    string
    DryRun     bool
    Force      bool
    NoCleanup  bool
}

func NewSnapshotOptions(opts ...func(*SnapshotOptions)) SnapshotOptions {
    options := SnapshotOptions{}
    for _, opt := range opts {
        opt(&options)
    }
    return options
}

func WithMessage(message string) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.Message = message
    }
}

func WithDryRun(dryRun bool) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.DryRun = dryRun
    }
}

func WithForce(force bool) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.Force = force
    }
}

func WithNoCleanup(noCleanup bool) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.NoCleanup = noCleanup
    }
}