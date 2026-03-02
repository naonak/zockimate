package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types/options"
)

func newRollbackCmd(cfg *config.Config) *cobra.Command {

	var opts = options.RollbackOptions{
		Image:   false,
		Data:    false,
		Config:  false,
		Force:   false,
		Timeout: options.DefaultRollbackTimeout,
	}

	cmd := &cobra.Command{
		Use:   "rollback container-name [snapshot-id]",
		Short: "Rollback container to a previous state",
		Long: `Rollback a container to a previous saved state.
If no snapshot ID is specified, uses the most recent snapshot.
At least one of --image, --data, or --config must be specified.

Examples:
  # Rollback everything to the last snapshot
  zockimate rollback wireguard -i -d -c

  # Rollback to a specific snapshot
  zockimate rollback wireguard 123 -i -d -c

  # Rollback only configuration
  zockimate rollback wireguard -c

  # Force rollback when exact image version cannot be guaranteed
  zockimate rollback wireguard -i -f`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.Image && !opts.Data && !opts.Config {
				return fmt.Errorf("at least one of --image, --data, or --config must be specified")
			}

			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			name := args[0]
			if len(args) > 1 {
				if id, err := strconv.ParseInt(args[1], 10, 64); err == nil {
					opts.SnapshotID = id
				} else {
					return fmt.Errorf("invalid snapshot ID: %v", err)
				}
			}

			result, err := m.RollbackContainer(context.Background(), name, opts)
			if err != nil {
				return err
			}
			if !result.Success {
				return fmt.Errorf("rollback failed: %v", result.Error)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.Image, "image", "i", false, "Rollback image")
	cmd.Flags().BoolVarP(&opts.Data, "data", "d", false, "Rollback data (ZFS snapshot)")
	cmd.Flags().BoolVarP(&opts.Config, "config", "c", false, "Rollback configuration")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
		"Force rollback even if exact image version cannot be guaranteed")

	return cmd
}
