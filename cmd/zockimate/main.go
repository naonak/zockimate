// cmd/zockimate/main.go
package main

// docker run --rm     --cap-drop=ALL     --cap-add=SYS_ADMIN     --cap-add=SYS_MODULE     --device=/dev/zfs     --security-opt=no-new-privileges       -v /mnt/ssd0/docker-apps/zockimate:/var/lib/zockimate:rw  -v /etc/localtime:/etc/localtime:ro   -v /proc:/proc     -v /var/run/docker.sock:/var/run/docker.sock:ro -e ZOCKIMATE_DB=/var/lib/zockimate/zockimate.db -e ZOCKIMATE_LOG_LEVEL=debug  --network=apprise-mailrise_net   naonak/zockimate-2:latest check dozzle

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"
    "strconv"

    "github.com/spf13/cobra"

    "zockimate/internal/config"
    "zockimate/internal/manager"
    "zockimate/internal/scheduler"
    "github.com/sirupsen/logrus"
    "zockimate/internal/types"
    "zockimate/internal/types/options"
)

func main() {
    cfg := config.NewConfig()

    // Commande racine
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
            // Charger la configuration depuis l'environnement
            if err := cfg.LoadFromEnv(); err != nil {
                return err
            }

            // Valider la configuration
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

    // Ajouter les sous-commandes
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

    // Exécuter la commande
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

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
            
            for _, r := range results {
                if r.Success {
                    updated++
                    cfg.Logger.Infof("✓ %s: updated from %s to %s", 
                        r.ContainerName, r.OldImage.String(), r.NewImage.String())
                } else if r.Error != nil {
                    failed++
                    cfg.Logger.Errorf("✗ %s: %v", r.ContainerName, r.Error)
                    errors = append(errors, fmt.Sprintf("%s: %v", r.ContainerName, r.Error))
                } else if !r.NeedsUpdate {
                    skipped++
                    cfg.Logger.Infof("- %s: no update needed", r.ContainerName)
                }
            }

            if updated > 0 || skipped > 0 {
                cfg.Logger.Infof("Summary: %d updated, %d skipped, %d failed", 
                    updated, skipped, failed)
            }

            if failed > 0 {
                return fmt.Errorf("update errors:\n%s", strings.Join(errors, "\n"))
            }
            
            return nil
        },
    }

    cmd.Flags().BoolVar(&opts.Notify, "notify", false,
        "Send notifications through Apprise")
    cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
        "Force update even if no new image available")
    cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false,
        "Show what would be updated without making changes")

    return cmd
}

// newCheckCmd crée la commande check
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
                    cfg.Logger.Infof("✓ %s: update available from %s to %s",
                        name, result.CurrentImage.String(), result.UpdateImage.String())
                    updates = append(updates, name)
                } else {
                    upToDate++
                    cfg.Logger.Debugf("- %s: up to date", name)
                }
            }
        
            if needsUpdate > 0 || upToDate > 0 {
                cfg.Logger.Infof("Summary: %d need update, %d up to date, %d failed",
                    needsUpdate, upToDate, failed)
            }
        
            // Exit code 1 si des mises à jour sont disponibles
            if len(updates) > 0 {
                return fmt.Errorf("updates available for: %s", strings.Join(updates, ", "))
            }
        
            return nil
        },
    }

    cmd.Flags().BoolVar(&opts.Notify, "notify", false,
        "Send notifications through Apprise")
    cmd.Flags().BoolVarP(&opts.Force, "force", "f", false,
        "Force check even with local image")
    cmd.Flags().BoolVarP(&opts.Cleanup, "cleanup", "c", true,
        "Cleanup pulled images after check")

    return cmd
}

// newRollbackCmd crée la commande rollback
func newRollbackCmd(cfg *config.Config) *cobra.Command {

    var opts = options.RollbackOptions{
        Image:     false,
        Data:      false,
        Config:    false,
        Force:     false,
        Timeout:   options.DefaultRollbackTimeout,
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

// newHistoryCmd crée la commande history
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
                Limit:    cfg.Limit,
                Last:     cfg.Last,
                SortBy:   cfg.SortBy,
                JSON:     cfg.JSON,
                Search:   cfg.Search,
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

// newScheduleCmd crée la commande schedule
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

// newScheduleUpdateCmd crée la sous-commande schedule update
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
                Logger:    cfg.Logger,
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

// newScheduleCheckCmd crée la sous-commande schedule check
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
                CheckOpts: opts,
                Logger:    cfg.Logger,
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

// Utilitaire pour formater l'aide des commandes
func formatExample(cmd *cobra.Command) string {
    if cmd.Example != "" {
        return fmt.Sprintf("\nExamples:\n%s", cmd.Example)
    }
    return ""
}

