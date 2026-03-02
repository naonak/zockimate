package main

import (
	"os"

	"github.com/spf13/cobra"

	"zockimate/internal/config"
)

func main() {
	cfg := config.NewConfig()

	rootCmd := &cobra.Command{
		Use:   "zockimate",
		Short: "Docker container update manager",
		Long: `A robust Docker container update manager with versioning capabilities.

Enables automatic updates, rollbacks, and monitoring of your Docker containers.

Environment variables:
  ZOCKIMATE_LOG_LEVEL   : Logging level (debug, info, warn, error)
  ZOCKIMATE_DB         : Database path
  ZOCKIMATE_APPRISE_URL: Apprise URL for notifications
  ZOCKIMATE_RETENTION  : Number of snapshots to retain
  ZOCKIMATE_TIMEOUT    : Default operation timeout in seconds`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.LoadFromEnv(); err != nil {
				return err
			}
			return cfg.Validate()
		},
	}

	// Flags globaux
	rootCmd.PersistentFlags().StringVarP(&cfg.LogLevel, "log-level", "l",
		config.DefaultLogLevel, "Log level")
	rootCmd.PersistentFlags().StringVarP(&cfg.DbPath, "db", "D",
		config.DefaultDbPath, "Database path")
	rootCmd.PersistentFlags().StringVarP(&cfg.AppriseURL, "apprise-url", "a",
		"", "Apprise URL for notifications")
	rootCmd.PersistentFlags().BoolVarP(&cfg.All, "all", "A",
		false, "Include stopped containers")
	rootCmd.PersistentFlags().BoolVarP(&cfg.NoFilter, "no-filter", "N",
		false, "Don't filter on zockimate.enable label")
	rootCmd.PersistentFlags().IntVar(&cfg.Retention, "retention",
		config.DefaultRetention, "Number of snapshots to retain")
	rootCmd.PersistentFlags().IntVar(&cfg.Timeout, "timeout",
		config.DefaultTimeout, "Operation timeout in seconds")

	// Sous-commandes
	rootCmd.AddCommand(
		newUpdateCmd(cfg),
		newCheckCmd(cfg),
		newRollbackCmd(cfg),
		newHistoryCmd(cfg),
		newScheduleCmd(cfg),
		newSaveCmd(cfg),
		newRenameCmd(cfg),
		newRemoveCmd(cfg),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
