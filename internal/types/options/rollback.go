package options

import "time"

type RollbackOptions struct {
    SnapshotID  int64
    Image       bool
    Data        bool
    Config      bool
    Force       bool
    Timeout     time.Duration
}