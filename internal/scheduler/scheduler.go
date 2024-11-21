// internal/scheduler/scheduler.go
package scheduler

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"
    "strings"

    "github.com/robfig/cron/v3"
    "github.com/sirupsen/logrus"

    "zockimate/internal/manager"
    "zockimate/internal/types/options"
    "zockimate/internal/types"
)

// Scheduler gère les opérations programmées sur les conteneurs
type Scheduler struct {
    manager    *manager.ContainerManager
    cron       *cron.Cron
    containers []string
    checkOpts  options.CheckOptions
    updateOpts  options.UpdateOptions
    checkOnly  bool              
    logger     *logrus.Logger
    stopChan   chan struct{}
    wg         sync.WaitGroup
}

// Options pour la configuration du scheduler
type Options struct {
    Containers []string
    CheckOpts  options.CheckOptions
    UpdateOpts options.UpdateOptions
    CheckOnly  bool 
    Logger     *logrus.Logger
}

// NewScheduler crée une nouvelle instance du scheduler
func NewScheduler(m *manager.ContainerManager, opts Options) *Scheduler {
    if opts.Logger == nil {
        opts.Logger = logrus.New()
        opts.Logger.SetLevel(logrus.InfoLevel)
    }

    return &Scheduler{
        manager:    m,
        cron:       cron.New(cron.WithParser(cron.NewParser(
            cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
        ))),
        containers: opts.Containers,
        checkOpts:  opts.CheckOpts,
        updateOpts: opts.UpdateOpts,
        checkOnly:  opts.CheckOnly,
        logger:     opts.Logger,
        stopChan:   make(chan struct{}),
    }
}

// Start démarre le scheduler avec l'expression cron donnée
func (s *Scheduler) Start(cronExpr string) error {
    // Valider l'expression cron
    if _, err := s.cron.AddFunc(cronExpr, s.runScheduledTask); err != nil {
        return fmt.Errorf("invalid cron expression: %w", err)
    }

    s.logger.Infof("Starting scheduler with cron expression: %s", cronExpr)
    if s.checkOnly {
        s.logger.Info("Running in check-only mode")
    }

    // Démarrer le cron
    s.cron.Start()

    // Gérer les signaux d'arrêt
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        select {
        case sig := <-sigChan:
            s.logger.Infof("Received signal %v, stopping scheduler...", sig)
            s.Stop()
        case <-s.stopChan:
            return
        }
    }()

    return nil
}

// runScheduledTask exécute la tâche programmée
func (s *Scheduler) runScheduledTask() {
    ctx := context.Background()

    // Si aucun conteneur n'est spécifié, obtenir tous les conteneurs gérés
    containers := s.containers
    if len(containers) == 0 {
        var err error
        containers, err = s.manager.GetContainers(ctx)
        if err != nil {
            s.logger.Errorf("Failed to list containers: %v", err)
            return
        }
    }

    if len(containers) == 0 {
        s.logger.Info("No containers to process")
        return
    }

    if s.checkOnly {
        s.performScheduledCheck(ctx, containers, s.checkOpts)
    } else {
        s.performScheduledUpdate(ctx, containers, s.updateOpts)
    }
}

// performScheduledCheck vérifie les mises à jour disponibles
func (s *Scheduler) performScheduledCheck(ctx context.Context, containers []string, opts options.CheckOptions) {
    var upToDate, needsUpdate, failed int
    
    for _, name := range containers {
        result, err := s.manager.CheckContainer(ctx, name, opts)
        if err != nil {
            failed++
            s.logger.Errorf("✗ %s: %v", name, err)
            continue
        }

        if result.NeedsUpdate {
            needsUpdate++
            s.logger.Infof("✓ %s: update available (%s -> %s)",
                name, result.CurrentImage.String(), result.UpdateImage.String())
        } else {
            upToDate++
            s.logger.Debugf("- %s: up to date", name)
        }
    }

    s.logger.Infof("Summary: %d need update, %d up to date, %d failed",
        needsUpdate, upToDate, failed)
}

// performScheduledUpdate met à jour les conteneurs
func (s *Scheduler) performScheduledUpdate(ctx context.Context, containers []string, opts options.UpdateOptions) {
    var results []*types.UpdateResult

    for _, name := range containers {
        result, err := s.manager.UpdateContainer(ctx, name, opts)
        if err != nil {
            s.logger.Errorf("Fatal error updating %s: %v", name, err)
            continue
        }
        results = append(results, result)
    }

    var updated, skipped, failed int
    var errors []string

    for _, r := range results {
        if r.Success {
            updated++
            s.logger.Infof("✓ %s: updated from %s to %s", 
                r.ContainerName, r.OldImage.String(), r.NewImage.String())
        } else if r.Error != nil {
            failed++
            s.logger.Errorf("✗ %s: %v", r.ContainerName, r.Error)
            errors = append(errors, fmt.Sprintf("%s: %v", r.ContainerName, r.Error))
        } else if !r.NeedsUpdate {
            skipped++
            s.logger.Infof("- %s: no update needed", r.ContainerName)
        }
    }

    // Log summary
    if updated > 0 || skipped > 0 {
        s.logger.Infof("Summary: %d updated, %d skipped, %d failed", 
            updated, skipped, failed)
    }
    
    if len(errors) > 0 {
        s.logger.Errorf("Failed updates:\n%s", strings.Join(errors, "\n"))
    }
}

// Stop arrête le scheduler
func (s *Scheduler) Stop() {
    s.logger.Info("Stopping scheduler...")
    
    // Arrêter le cron
    ctx := s.cron.Stop()
    
    // Signaler l'arrêt à la goroutine de gestion des signaux
    close(s.stopChan)
    
    // Attendre la fin de toutes les goroutines
    s.wg.Wait()
    
    // Attendre la fin des jobs en cours
    <-ctx.Done()
    
    s.logger.Info("Scheduler stopped")
}

// IsRunning indique si le scheduler est en cours d'exécution
func (s *Scheduler) IsRunning() bool {
    return s.cron.Entries() != nil
}

// NextRun retourne la prochaine exécution prévue
func (s *Scheduler) NextRun() *time.Time {
    entries := s.cron.Entries()
    if len(entries) == 0 {
        return nil
    }
    next := entries[0].Next
    return &next
}