package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types"
	"zockimate/internal/types/options"
)

func newRemoveCmd(cfg *config.Config) *cobra.Command {
	var opts options.RemoveOptions
	var olderThan string
	var before string

	cmd := &cobra.Command{
		Use:   "remove [flags] container [container...]",
		Short: "Remove container entries from database",
		Long: `Remove container entries from the database and optionally remove the Docker container.

By default, refuses to remove entries if container still exists in Docker.
Use --force to remove entries anyway or --with-container to also remove the Docker container.

Can also clean up old entries based on age or date.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			var results []*types.RemoveResult

			ctx := context.Background()

			for _, name := range args {
				result, err := m.RemoveContainer(ctx, name, opts)
				if err != nil {
					cfg.Logger.Errorf("Fatal error removing %s: %v", name, err)
					continue
				}
				results = append(results, result)
			}

			var removed, failed int
			for _, r := range results {
				if r.Success {
					removed++
					status := "removed from database"
					if r.ContainerRemoved {
						status = "removed from Docker and database"
					}
					cfg.Logger.Infof("✓ %s: %s (%d entries)", r.ContainerName, status, r.EntriesDeleted)
				} else {
					failed++
					cfg.Logger.Errorf("✗ %s: %v", r.ContainerName, r.Error)
				}
			}

			if removed > 0 || failed > 0 {
				cfg.Logger.Infof("Summary: %d removed, %d failed", removed, failed)
			}

			if failed > 0 {
				return fmt.Errorf("some containers failed to be removed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
		"Force removal even if container exists")
	cmd.Flags().BoolVarP(&opts.WithContainer, "with-container", "c", false,
		"Stop and remove Docker container")
	cmd.Flags().StringVar(&olderThan, "older-than", "",
		"Remove entries older than duration (e.g., 30d, 6m, 1y)")
	cmd.Flags().StringVar(&before, "before", "",
		"Remove entries before date (YYYY-MM-DD)")
	cmd.Flags().BoolVarP(&opts.All, "all", "a", false,
		"Remove all entries for containers")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false,
		"Show what would be removed without taking action")
	cmd.Flags().BoolVar(&opts.Zfs, "zfs", false,
		"Also remove associated ZFS snapshots")

	return cmd
}
