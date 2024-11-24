package options

import (
    "time"
    "fmt"
)

type UpdateOptions struct {
    Force     bool
    DryRun    bool
    Timeout   time.Duration
    ContainerReadyTimeout   time.Duration
    Notify   bool
}

// Pour UpdateOptions
func NewUpdateOptions(opts ...UpdateOption) UpdateOptions {
    options := UpdateOptions{
        Force:     false,
        DryRun:    false,
        Timeout:   DefaultUpdateTimeout,
        ContainerReadyTimeout: DefaultContainerReadyTimeout,
        Notify: false,
    }
    for _, opt := range opts {
        opt(&options)
    }
    return options
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

// Optionnellement, on peut aussi ajouter des fonctions Option comme pour CheckOptions
type UpdateOption func(*UpdateOptions)

func WithUpdateForce(force bool) UpdateOption {
    return func(o *UpdateOptions) {
        o.Force = force
    }
}

func WithUpdateDryRun(dryRun bool) UpdateOption {
    return func(o *UpdateOptions) {
        o.DryRun = dryRun
    }
}

func WithUpdateTimeout(timeout time.Duration) UpdateOption {
    return func(o *UpdateOptions) {
        o.Timeout = timeout
    }
}

func WithUpdateContainerReadyTimeout(timeout time.Duration) UpdateOption {
    return func(o *UpdateOptions) {
        o.ContainerReadyTimeout = timeout
    }
}

func WithUpdateNotify(notify bool) UpdateOption {
    return func(o *UpdateOptions) {
        o.Notify = notify
    }
}