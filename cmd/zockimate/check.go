package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types/options"
)

func newCheckCmd(cfg *config.Config) *cobra.Command {
	var opts = options.NewCheckOptions()

	cmd := &cobra.Command{
		Use:   "check [container...]",
		Short: "Check for available updates",
		Long: `Check one or more containers for available updates.
If no containers are specified, checks all enabled containers.

Examples:
  # Check all running containers
  zockimate check

  # Check specific containers
  zockimate check wireguard plex

  # Check all containers including stopped ones
  zockimate -A check

  # Force check even with local image
  zockimate check -f wireguard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				logrus.Debugf("Failed to create container manager: %v", err)
				return err
			}
			defer m.Close()

			ctx := context.Background()
			containers := args

			if len(containers) == 0 {
				containers, err = m.GetContainers(ctx)
				if err != nil {
					return err
				}
				if len(containers) == 0 {
					cfg.Logger.Info("No containers found to check")
					return nil
				}
			}

			var needsUpdate, upToDate, failed int
			var updateDetails []string
			var updates []string

			for _, name := range containers {
				result, err := m.CheckContainer(ctx, name, opts)
				if err != nil {
					failed++
					cfg.Logger.Errorf("✗ %s: %v", name, err)
					continue
				}

				if result.NeedsUpdate {
					needsUpdate++
					updateMsg := fmt.Sprintf("%s: %s → %s",
						name, result.CurrentImage.String(), result.UpdateImage.String())
					cfg.Logger.Infof("✓ %s", updateMsg)
					updates = append(updates, name)
					updateDetails = append(updateDetails, updateMsg)
				} else {
					upToDate++
					cfg.Logger.Debugf("- %s: up to date", name)
				}
			}

			summaryMsg := fmt.Sprintf("Summary: %d need update, %d up to date, %d failed",
				needsUpdate, upToDate, failed)
			cfg.Logger.Info(summaryMsg)

			// Envoyer une notification unique si des mises à jour sont disponibles
			if opts.Notify && needsUpdate > 0 && cfg.AppriseURL != "" {
				notifTitle := fmt.Sprintf("Updates Available (%d/%d)", needsUpdate, len(containers))

				notifMsg := fmt.Sprintf("%d need update, %d up to date, %d failed.",
					needsUpdate, upToDate, failed) + "\nUpdate needed: " + strings.Join(updates, "\n")

				if failed > 0 {
					var failedContainers []string
					for _, name := range containers {
						if _, err := m.CheckContainer(ctx, name, opts); err != nil {
							failedContainers = append(failedContainers, name)
						}
					}
					notifMsg += fmt.Sprintf("\nFailed: %s", strings.Join(failedContainers, ", "))
				}

				if err := m.SendNotification(notifTitle, notifMsg); err != nil {
					cfg.Logger.Warnf("Failed to send notification: %v", err)
				}
			}

			if len(updates) > 0 {
				cfg.Logger.Infof("Updates available for: %s", strings.Join(updates, ", "))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.Notify, "notify", false,
		"Send a summary notification through Apprise when updates are found")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
		"Force check even with local image")
	cmd.Flags().BoolVarP(&opts.Cleanup, "cleanup", "c", true,
		"Cleanup pulled images after check")

	return cmd
}
