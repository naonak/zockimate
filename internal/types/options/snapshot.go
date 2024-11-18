package options

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

func WithSnapshotMessage(message string) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.Message = message
    }
}

func WithSnapshotDryRun(dryRun bool) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.DryRun = dryRun
    }
}

func WithSnapshotForce(force bool) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.Force = force
    }
}

func WithSnapshotNoCleanup(noCleanup bool) func(*SnapshotOptions) {
    return func(o *SnapshotOptions) {
        o.NoCleanup = noCleanup
    }
}