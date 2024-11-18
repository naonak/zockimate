// internal/types/options.go

package options

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
