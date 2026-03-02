package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/types/options"
)

func newHistoryCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [container...]",
		Short: "Show container history",
		Long: `Show update history for containers.
If no containers are specified, shows history for all containers.

Examples:
  # Show history for all containers
  zockimate history

  # Show history for specific containers
  zockimate history wireguard plex

  # Show last 5 entries per container
  zockimate history -n 5

  # Show only last entry for each container
  zockimate history -L

  # Show as JSON
  zockimate history -j`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			opts := options.HistoryOptions{
				Container: args,
				Limit:     cfg.Limit,
				Last:      cfg.Last,
				SortBy:    cfg.SortBy,
				JSON:      cfg.JSON,
				Search:    cfg.Search,
			}

			if cfg.Since != "" {
				if t, err := time.Parse("2006-01-02", cfg.Since); err == nil {
					opts.Since = t
				} else {
					return fmt.Errorf("invalid --since date format (use YYYY-MM-DD)")
				}
			}

			if cfg.Before != "" {
				if t, err := time.Parse("2006-01-02", cfg.Before); err == nil {
					opts.Before = t
				} else {
					return fmt.Errorf("invalid --before date format (use YYYY-MM-DD)")
				}
			}

			history, err := m.GetHistory(opts)
			if err != nil {
				return err
			}

			if len(history) == 0 {
				cfg.Logger.Info("No history found")
				return nil
			}

			if cfg.JSON {
				if err := json.NewEncoder(os.Stdout).Encode(history); err != nil {
					return fmt.Errorf("failed to encode JSON: %v", err)
				}
				return nil
			}

			// Affichage formaté
			for _, entry := range history {
				cfg.Logger.Infof("[%s] %s (ID: %d)",
					entry.CreatedAt.Format("2006-01-02 15:04:05"),
					entry.ContainerName,
					entry.ID,
				)
				cfg.Logger.Infof("  Status: %s", entry.Status)
				if entry.RepoDigest != "" {
					cfg.Logger.Infof("  Image: %s", entry.RepoDigest)
				} else {
					cfg.Logger.Infof("  Image: %s (%s)",
						entry.ImageTag,
						entry.ImageID[:12],
					)
				}
				if entry.Message != "" {
					cfg.Logger.Infof("  Message: %s", entry.Message)
				}
				cfg.Logger.Info("")
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&cfg.Limit, "limit", "n", 0,
		"Limit number of entries per container")
	cmd.Flags().BoolVarP(&cfg.Last, "last", "L", false,
		"Show only last entry per container")
	cmd.Flags().StringVarP(&cfg.SortBy, "sort-by", "s", "date",
		"Sort by (date|container)")
	cmd.Flags().BoolVarP(&cfg.JSON, "json", "j", false,
		"Output in JSON format")
	cmd.Flags().StringVarP(&cfg.Search, "search", "q", "",
		"Search in messages and status")
	cmd.Flags().StringVarP(&cfg.Since, "since", "S", "",
		"Show entries since date (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&cfg.Before, "before", "b", "",
		"Show entries before date (YYYY-MM-DD)")

	return cmd
}
