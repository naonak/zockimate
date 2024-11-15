// internal/storage/zfs/zfs.go
package zfs

import (
    "context"
    "fmt"
    "os/exec"
    "time"
    "github.com/sirupsen/logrus"
)

// ZFSManager gère les opérations ZFS
type ZFSManager struct {
    logger *logrus.Logger
}

// NewZFSManager crée une nouvelle instance du gestionnaire ZFS
func NewZFSManager(logger *logrus.Logger) *ZFSManager {
    return &ZFSManager{
        logger: logger,
    }
}

// CreateSnapshot crée un nouveau snapshot ZFS
func (z *ZFSManager) CreateSnapshot(dataset string) (string, error) {
    snapshotName := fmt.Sprintf("%s@snapshot_%s", 
        dataset, 
        time.Now().Format("20060102_150405"),
    )
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "zfs", "snapshot", snapshotName)
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("failed to create ZFS snapshot: %w", err)
    }
    
    z.logger.Debugf("Created ZFS snapshot: %s", snapshotName)
    return snapshotName, nil
}

// RollbackSnapshot effectue un rollback vers un snapshot ZFS
func (z *ZFSManager) RollbackSnapshot(snapshot string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "zfs", "rollback", "-r", snapshot)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to rollback ZFS snapshot: %w", err)
    }

    z.logger.Debugf("Rolled back to ZFS snapshot: %s", snapshot)
    return nil
}

// DeleteSnapshot supprime un snapshot ZFS
func (z *ZFSManager) DeleteSnapshot(snapshot string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "zfs", "destroy", snapshot)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to delete ZFS snapshot: %w", err)
    }

    z.logger.Debugf("Deleted ZFS snapshot: %s", snapshot)
    return nil
}