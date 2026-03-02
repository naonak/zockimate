package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types/options"
)

func newRenameCmd(cfg *config.Config) *cobra.Command {
	var opts options.RenameOptions

	cmd := &cobra.Command{
		Use:   "rename old-name new-name",
		Short: "Rename a container",
		Long: `Rename a container in both Docker (if it exists) and the database.
If the container exists in Docker, it will be renamed there and in the database.
If it only exists in the database, only the database entries will be updated.

Examples:
  # Rename a container everywhere
  zockimate rename old-container new-container

  # Rename only in database
  zockimate rename --db-only old-container new-container`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			ctx := context.Background()
			oldName, newName := args[0], args[1]

			result, err := m.RenameContainer(ctx, oldName, newName, opts)
			if err != nil {
				cfg.Logger.Errorf("Fatal error renaming %s: %v", oldName, err)
				return err
			}

			if result.Success {
				status := "renamed in database only"
				if result.DockerRenamed {
					status = "renamed in Docker and database"
				}
				cfg.Logger.Infof("✓ %s -> %s: %s (%d entries)",
					result.OldName, result.NewName, status, result.EntriesRenamed)
			} else {
				cfg.Logger.Errorf("✗ %s -> %s: %v",
					result.OldName, result.NewName, result.Error)
				return fmt.Errorf("rename failed")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.DbOnly, "db-only", false, "Only rename in database, ignore Docker")
	return cmd
}
