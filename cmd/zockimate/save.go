package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types/options"
)

func newSaveCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save [flags] container [container...]",
		Short: "Create a snapshot of containers",
		Long: `Create a snapshot of the specified containers.
Each snapshot includes:
- Container configuration
- ZFS snapshot if configured
- Custom message for identification`,
		Example: `  # Save single container
  zockimate save nginx -m "Pre-update backup"

  # Save multiple containers
  zockimate save nginx mysql redis -m "Daily backup"

  # Show what would be saved without taking action
  zockimate save --dry-run nginx

  # Force snapshot of stopped container
  zockimate save --force nginx`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			// Récupérer toutes les options
			message, _ := cmd.Flags().GetString("message")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			force, _ := cmd.Flags().GetBool("force")
			noCleanup, _ := cmd.Flags().GetBool("no-cleanup")

			ctx := context.Background()
			var failed bool

			for _, name := range args {
				opts := options.NewSnapshotOptions(
					options.WithSnapshotMessage(message),
					options.WithSnapshotDryRun(dryRun),
					options.WithSnapshotForce(force),
					options.WithSnapshotNoCleanup(noCleanup),
				)

				if dryRun {
					cfg.Logger.Infof("Would create snapshot for container %s", name)
					continue
				}

				snapshot, err := m.CreateSnapshot(ctx, name, opts)
				if err != nil {
					cfg.Logger.Errorf("Failed to save container %s: %v", name, err)
					failed = true
					continue
				}

				cfg.Logger.Infof("Created snapshot %d for container %s", snapshot.ID, name)
			}

			if failed {
				return fmt.Errorf("failed to save one or more containers")
			}
			return nil
		},
	}

	cmd.Flags().StringP("message", "m", "", "Message to attach to the snapshot")
	cmd.Flags().BoolP("dry-run", "n", false, "Show what would be saved without taking action")
	cmd.Flags().BoolP("force", "f", false, "Force snapshot even if container is stopped")
	cmd.Flags().Bool("no-cleanup", false, "Skip cleanup of old snapshots")

	return cmd
}
