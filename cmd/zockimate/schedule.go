package main

import (
	"github.com/spf13/cobra"

	"zockimate/internal/config"
	"zockimate/internal/manager"
	"zockimate/internal/scheduler"
	"zockimate/internal/types/options"
)

func newScheduleCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule operations",
		Long: `Schedule automatic container operations.

Cron Expression Format:
  ┌───────────── minute (0 - 59)
  │ ┌───────────── hour (0 - 23)
  │ │ ┌───────────── day of month (1 - 31)
  │ │ │ ┌───────────── month (1 - 12)
  │ │ │ │ ┌───────────── day of week (0 - 6)
  │ │ │ │ │
  * * * * *`,
	}

	cmd.AddCommand(newScheduleUpdateCmd(cfg), newScheduleCheckCmd(cfg))
	return cmd
}

func newScheduleUpdateCmd(cfg *config.Config) *cobra.Command {
	var opts = options.NewUpdateOptions()

	cmd := &cobra.Command{
		Use:   `update "cron-expression" [container...]`,
		Short: "Schedule container updates",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			cronExpr := args[0]
			containers := args[1:]

			s := scheduler.NewScheduler(m, scheduler.Options{
				Containers: containers,
				CheckOnly:  false,
				UpdateOpts: opts,
				Logger:     cfg.Logger,
			})

			if err := s.Start(cronExpr); err != nil {
				return err
			}

			next := s.NextRun()
			if next != nil {
				cfg.Logger.Infof("First update scheduled at: %s",
					next.Format("2006-01-02 15:04:05"))
			}

			// Attendre indéfiniment ou jusqu'à Ctrl+C
			select {}
		},
	}

	cmd.Flags().BoolVar(&opts.Notify, "notify", true,
		"Send notifications through Apprise")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
		"Force update even if no new image available")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false,
		"Show what would be updated without making changes")

	return cmd
}

func newScheduleCheckCmd(cfg *config.Config) *cobra.Command {
	var opts = options.NewCheckOptions()

	cmd := &cobra.Command{
		Use:   `check "cron-expression" [container...]`,
		Short: "Schedule update checks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manager.NewContainerManager(cfg)
			if err != nil {
				return err
			}
			defer m.Close()

			cronExpr := args[0]
			containers := args[1:]

			s := scheduler.NewScheduler(m, scheduler.Options{
				Containers: containers,
				CheckOnly:  true,
				CheckOpts:  opts,
				Logger:     cfg.Logger,
			})

			if err := s.Start(cronExpr); err != nil {
				return err
			}

			next := s.NextRun()
			if next != nil {
				cfg.Logger.Infof("First check scheduled at: %s",
					next.Format("2006-01-02 15:04:05"))
			}

			// Attendre indéfiniment ou jusqu'à Ctrl+C
			select {}
		},
	}

	cmd.Flags().BoolVar(&opts.Notify, "notify", true,
		"Send notifications through Apprise")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
		"Force check even with local image")
	cmd.Flags().BoolVarP(&opts.Cleanup, "cleanup", "c", true,
		"Cleanup pulled images after check")

	return cmd
}
