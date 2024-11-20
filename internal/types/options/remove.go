package options

import "time"

type RemoveOptions struct {
    Force         bool          // Force removal even if container exists
    WithContainer bool          // Stop and remove Docker container
    OlderThan     time.Duration // Remove entries older than duration
    Before        time.Time     // Remove entries before date
    All           bool          // Remove all entries
    DryRun        bool          // Show what would be removed
    Zfs           bool          // Also remove ZFS snapshots
}
