package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types"
	"zockimate/internal/types/options"
)

func newUpdateCmd(cfg *config.Config) *cobra.Command {
	var opts = options.NewUpdateOptions()

	cmd := &cobra.Command{
		Use:   "update [container...]",
		Short: "Update containers",
		Long: `Update one or more containers to their latest image versions.
If no containers are specified, updates all enabled containers.

Examples:
  # Update all running containers
  zockimate update

  # Update specific containers
  zockimate update wireguard plex

  # Update all containers including stopped ones
  zockimate -A update

  # Force update even if no new image
  zockimate update -f wireguard

  # Dry run to see what would be updated
  zockimate update -n`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			ctx := context.Background()
			containers := args

			// Si aucun conteneur spécifié, obtenir tous les conteneurs gérés
			if len(containers) == 0 {
				containers, err = m.GetContainers(ctx)
				if err != nil {
					return err
				}
				if len(containers) == 0 {
					cfg.Logger.Info("No containers found to update")
					return nil
				}
			}

			var results []*types.UpdateResult
			for _, name := range containers {
				result, err := m.UpdateContainer(ctx, name, opts)
				if err != nil {
					cfg.Logger.Errorf("Fatal error updating %s: %v", name, err)
					continue
				}
				results = append(results, result)
			}

			var updated, skipped, failed int
			var errors []string
			var updateDetails []string
			var failureDetails []string

			for _, r := range results {
				if r.Success {
					updated++
					updateMsg := fmt.Sprintf("%s: %s → %s",
						r.ContainerName, r.OldImage.String(), r.NewImage.String())
					cfg.Logger.Infof("✓ %s", updateMsg)
					updateDetails = append(updateDetails, updateMsg)
				} else if r.Error != nil {
					failed++
					errMsg := fmt.Sprintf("%s: %v", r.ContainerName, r.Error)
					cfg.Logger.Errorf("✗ %s", errMsg)
					errors = append(errors, errMsg)
					failureDetails = append(failureDetails, errMsg)
				} else if !r.NeedsUpdate {
					skipped++
					cfg.Logger.Infof("- %s: no update needed", r.ContainerName)
				}
			}

			summaryMsg := fmt.Sprintf("Summary: %d updated, %d skipped, %d failed",
				updated, skipped, failed)
			cfg.Logger.Info(summaryMsg)

			// Pour la commande update
			if opts.Notify && !opts.DryRun && cfg.AppriseURL != "" {
				var updatedContainers []string
				var failedContainers []string

				for _, r := range results {
					if r.Success {
						updatedContainers = append(updatedContainers, r.ContainerName)
					} else if r.Error != nil {
						failedContainers = append(failedContainers, r.ContainerName)
					}
				}

				notifTitle := fmt.Sprintf("Updates Completed (%d/%d)", updated, len(containers))

				var parts []string
				if len(updatedContainers) > 0 {
					parts = append(parts, strings.Join(updatedContainers, ", "))
				}
				if len(failedContainers) > 0 {
					parts = append(parts, fmt.Sprintf("Failed: %s", strings.Join(failedContainers, ", ")))
				}
				notifMsg := fmt.Sprintf("%d updated, %d skipped, %d failed.",
					updated, skipped, failed) + "\nUpdated: " + strings.Join(parts, "\n")

				if err := m.SendNotification(notifTitle, notifMsg); err != nil {
					cfg.Logger.Warnf("Failed to send notification: %v", err)
				}
			}

			if failed > 0 {
				return fmt.Errorf("update errors:\n%s", strings.Join(errors, "\n"))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.Notify, "notify", false,
		"Send a summary notification through Apprise when updates complete")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
		"Force update even if no new image available")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false,
		"Show what would be updated without making changes")

	return cmd
}
