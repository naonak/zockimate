// internal/config/config.go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"

    "github.com/sirupsen/logrus"
)

const (
    // Defaults
    DefaultLogLevel    = "info"
    DefaultDbPath     = "zockimate.db"
    DefaultTimeout    = 180
    DefaultRetention  = 10
    DefaultSortBy     = "date"

    // Environment variables
    EnvPrefix         = "ZOCKIMATE_"
    EnvLogLevel       = EnvPrefix + "LOG_LEVEL"
    EnvDbPath         = EnvPrefix + "DB"
    EnvAppriseURL     = EnvPrefix + "APPRISE_URL"
    EnvRetention      = EnvPrefix + "RETENTION"
    EnvTimeout        = EnvPrefix + "TIMEOUT"
)

// Config représente la configuration globale de l'application
type Config struct {
    // Paramètres généraux
    LogLevel    string
    DbPath      string
    AppriseURL  string
    
    // Filtres et comportement
    All         bool    // Inclure les conteneurs arrêtés
    NoFilter    bool    // Ne pas filtrer sur zockimate.enable
    Force       bool    // Forcer les opérations
    DryRun      bool    // Mode simulation
    
    // Paramètres historique
    Limit       int     // Limite d'entrées
    Last        bool    // Dernière entrée seulement
    SortBy      string  // Critère de tri
    JSON        bool    // Format JSON
    Search      string  // Terme de recherche
    Since       string  // Date début
    Before      string  // Date fin
    
    // Paramètres système
    Retention   int     // Nombre de snapshots à conserver
    Timeout     int     // Timeout global en secondes

    // Logger configuré
    Logger     *logrus.Logger
}

// NewConfig crée une nouvelle configuration avec les valeurs par défaut
func NewConfig() *Config {
    return &Config{
        LogLevel:   DefaultLogLevel,
        DbPath:     DefaultDbPath,
        Retention:  DefaultRetention,
        Timeout:    DefaultTimeout,
        SortBy:     DefaultSortBy,
        Logger:     newLogger(DefaultLogLevel),
    }
}

// LoadFromEnv charge la configuration depuis les variables d'environnement
func (c *Config) LoadFromEnv() error {
    // Log level
    if level := os.Getenv(EnvLogLevel); level != "" {
        if err := c.SetLogLevel(level); err != nil {
            return fmt.Errorf("invalid log level: %w", err)
        }
    }

    // Database path
    if path := os.Getenv(EnvDbPath); path != "" {
        c.DbPath = path
    }

    // Apprise URL
    if url := os.Getenv(EnvAppriseURL); url != "" {
        c.AppriseURL = url
    }

    // Retention
    if ret := os.Getenv(EnvRetention); ret != "" {
        retention, err := strconv.Atoi(ret)
        if err != nil {
            return fmt.Errorf("invalid retention value: %w", err)
        }
        c.Retention = retention
    }

    // Timeout
    if timeout := os.Getenv(EnvTimeout); timeout != "" {
        t, err := strconv.Atoi(timeout)
        if err != nil {
            return fmt.Errorf("invalid timeout value: %w", err)
        }
        c.Timeout = t
    }

    return nil
}

// Validate vérifie la validité de la configuration
func (c *Config) Validate() error {
    // Vérifier le log level
    if _, err := logrus.ParseLevel(c.LogLevel); err != nil {
        return fmt.Errorf("invalid log level '%s': %w", c.LogLevel, err)
    }

    // Vérifier le chemin de la base de données
    if c.DbPath == "" {
        return fmt.Errorf("database path cannot be empty")
    }

    // Vérifier la rétention
    if c.Retention < 1 {
        return fmt.Errorf("retention must be at least 1")
    }

    // Vérifier le timeout
    if c.Timeout < 1 {
        return fmt.Errorf("timeout must be at least 1 second")
    }

    // Vérifier les dates si spécifiées
    if c.Since != "" {
        if _, err := time.Parse("2006-01-02", c.Since); err != nil {
            return fmt.Errorf("invalid since date format (use YYYY-MM-DD): %w", err)
        }
    }
    if c.Before != "" {
        if _, err := time.Parse("2006-01-02", c.Before); err != nil {
            return fmt.Errorf("invalid before date format (use YYYY-MM-DD): %w", err)
        }
    }

    // Vérifier le critère de tri
    if c.SortBy != "date" && c.SortBy != "container" {
        return fmt.Errorf("invalid sort criteria: must be 'date' or 'container'")
    }

    return nil
}

// SetLogLevel configure le niveau de log
func (c *Config) SetLogLevel(level string) error {
    lvl, err := logrus.ParseLevel(level)
    if err != nil {
        return err
    }
    c.LogLevel = level
    c.Logger.SetLevel(lvl)
    return nil
}

// newLogger crée un nouveau logger configuré
func newLogger(level string) *logrus.Logger {
    logger := logrus.New()
    
    // Configuration par défaut du logger
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp:   true,
        TimestampFormat: "2006-01-02 15:04:05",
    })
    
    // Niveau de log
    if lvl, err := logrus.ParseLevel(level); err == nil {
        logger.SetLevel(lvl)
    } else {
        logger.SetLevel(logrus.InfoLevel)
    }

    return logger
}

// Clone crée une copie de la configuration
func (c *Config) Clone() *Config {
    return &Config{
        LogLevel:   c.LogLevel,
        DbPath:     c.DbPath,
        AppriseURL: c.AppriseURL,
        All:        c.All,
        NoFilter:   c.NoFilter,
        Force:      c.Force,
        DryRun:     c.DryRun,
        Limit:      c.Limit,
        Last:       c.Last,
        SortBy:     c.SortBy,
        JSON:       c.JSON,
        Search:     c.Search,
        Since:      c.Since,
        Before:     c.Before,
        Retention:  c.Retention,
        Timeout:    c.Timeout,
        Logger:     c.Logger, // Partagé intentionnellement
    }
}