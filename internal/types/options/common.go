// internal/types/options.go

package options

import "time"

const (
    DefaultContainerReadyTimeout = 30 * time.Minute
    DefaultOperationTimeout      = 10 * time.Minute
    DefaultPullTimeout           = 30 * time.Minute
    DefaultCheckTimeout          = 30 * time.Minute
    DefaultRollbackTimeout       = 30 * time.Minute
    DefaultUpdateTimeout         = 30 * time.Minute
    DefaultStopTimeout           = 30 * time.Second
    DefaultStartTimeout          = 30 * time.Second
    DefaultHealthTimeout         = 5 * time.Minute
    MinContainerTimeout          = 30 * time.Second
    MaxContainerTimeout          = 24 * time.Hour
)
