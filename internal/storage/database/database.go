// internal/storage/database/database.go
package database

import (
    "database/sql"
    "fmt"
    "path/filepath"
    "os"
    "strings"
    "time"
    _ "github.com/mattn/go-sqlite3"
    "github.com/sirupsen/logrus"

    "zockimate/internal/storage/zfs"
    "zockimate/internal/types"
    "zockimate/internal/types/options"
    "zockimate/pkg/utils"
)

// Database gère les opérations de base de données
type Database struct {
    db      *sql.DB
    zfs     *zfs.ZFSManager  // Ajouter ce champ
    logger  *logrus.Logger
}

// NewDatabase initialise une nouvelle instance de base de données
func NewDatabase(dbPath string, logger *logrus.Logger) (*Database, error) {
    // Créer le répertoire si nécessaire
    if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
        return nil, fmt.Errorf("failed to create database directory: %w", err)
    }

    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Créer le schéma
    if err := initSchema(db); err != nil {
        db.Close()
        return nil, err
    }

    return &Database{
        db:     db,
        zfs:    zfs.NewZFSManager(logger),  // Initialiser le ZFS manager
        logger: logger,
    }, nil
}

// Close ferme la connexion à la base de données
func (d *Database) Close() error {
    return d.db.Close()
}

// initSchema initialise le schéma de la base de données
func initSchema(db *sql.DB) error {
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS container_snapshots (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            container_name TEXT NOT NULL,
            image_id TEXT NOT NULL,
            image_digest TEXT,
            image_tag TEXT,
            original_image TEXT NOT NULL,
            config BLOB,
            host_config BLOB,
            network_config BLOB,
            zfs_snapshot TEXT,
            status TEXT,
            message TEXT,
            created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),  -- Format ISO en UTC
            UNIQUE(container_name, created_at)
        );
        CREATE INDEX IF NOT EXISTS idx_container_name ON container_snapshots(container_name);
        CREATE INDEX IF NOT EXISTS idx_created_at ON container_snapshots(created_at);
		CREATE INDEX IF NOT EXISTS idx_container_status ON container_snapshots(status);
		CREATE INDEX IF NOT EXISTS idx_container_message ON container_snapshots(message);
    `)
    if err != nil {
        return fmt.Errorf("failed to create schema: %w", err)
    }
    return nil
}

// SaveSnapshot sauvegarde un snapshot dans la base de données
func (d *Database) SaveSnapshot(snapshot *types.ContainerSnapshot) error {
    result, err := d.db.Exec(`
        INSERT INTO container_snapshots (
            container_name, image_id, image_digest, image_tag, original_image,
            config, host_config, network_config, zfs_snapshot, status, message, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        snapshot.ContainerName,
        snapshot.ImageRef.ID,
        snapshot.ImageRef.RepoDigest,
        snapshot.ImageRef.Tag,
        snapshot.ImageRef.Original,
        snapshot.Config,
        snapshot.HostConfig,
        snapshot.NetworkConfig,
        snapshot.ZFSSnapshot,
        snapshot.Status,
        snapshot.Message,
        time.Now().UTC().Format(time.RFC3339),
    )
    if err != nil {
        return fmt.Errorf("failed to save snapshot: %w", err)
    }

    id, err := result.LastInsertId()
    if err != nil {
        return fmt.Errorf("failed to get last insert ID: %w", err)
    }
    snapshot.ID = id

    d.logger.Debugf("Saved snapshot %d for container %s", id, snapshot.ContainerName)
    return nil
}

// GetSnapshot récupère un snapshot spécifique
func (d *Database) GetSnapshot(containerName string, id int64) (*types.ContainerSnapshot, error) {
    var query string
    var args []interface{}

    if id > 0 {
        query = `SELECT * FROM container_snapshots WHERE container_name = ? AND id = ?`
        args = []interface{}{containerName, id}
    } else {
        query = `SELECT * FROM container_snapshots WHERE container_name = ? 
                 ORDER BY created_at DESC LIMIT 1`
        args = []interface{}{containerName}
    }

    var snapshot types.ContainerSnapshot
    var imageRef types.ImageReference
    var createdAt string

    err := d.db.QueryRow(query, args...).Scan(
        &snapshot.ID,
        &snapshot.ContainerName,
        &imageRef.ID,
        &imageRef.RepoDigest,
        &imageRef.Tag,
        &imageRef.Original,
        &snapshot.Config,
        &snapshot.HostConfig,
        &snapshot.NetworkConfig,
        &snapshot.ZFSSnapshot,
        &snapshot.Status,
        &snapshot.Message,
        &createdAt,
    )

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("no snapshot found")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to query snapshot: %w", err)
    }

    snapshot.ImageRef = imageRef

    // Ajouter après le Scan :
    snapshot.CreatedAt, err = utils.ParseTime(createdAt)
    if err != nil {
        return nil, fmt.Errorf("failed to parse created_at: %w", err)
    }

    return &snapshot, nil
}

// GetHistory récupère l'historique des snapshots
func (d *Database) GetHistory(opts options.HistoryOptions) ([]types.SnapshotMetadata, error) {
    var conditions []string
    var args []interface{}
    
    query := `SELECT id, container_name, image_tag, image_id, 
              image_digest, status, message, created_at 
              FROM container_snapshots`

    // Appliquer les filtres
    if len(opts.Container) > 0 {
        placeholders := make([]string, len(opts.Container))
        for i, name := range opts.Container {
            placeholders[i] = "?"
            args = append(args, name)
        }
        conditions = append(conditions, 
            fmt.Sprintf("container_name IN (%s)", strings.Join(placeholders, ",")))
    }

    if !opts.Since.IsZero() {
        conditions = append(conditions, "created_at >= ?")
        args = append(args, opts.Since.Format("2006-01-02 15:04:05"))
    }

    if !opts.Before.IsZero() {
        conditions = append(conditions, "created_at <= ?")
        args = append(args, opts.Before.Format("2006-01-02 15:04:05"))
    }

    if opts.Search != "" {
        conditions = append(conditions, "(message LIKE ? OR status LIKE ?)")
        searchTerm := "%" + opts.Search + "%"
        args = append(args, searchTerm, searchTerm)
    }

    if len(conditions) > 0 {
        query += " WHERE " + strings.Join(conditions, " AND ")
    }

    // Tri
    query += " ORDER BY " + func() string {
        if opts.SortBy == "container" {
            return "container_name, created_at DESC"
        }
        return "created_at DESC"
    }()

    rows, err := d.db.Query(query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to query history: %w", err)
    }
    defer rows.Close()

    var entries []types.SnapshotMetadata
    for rows.Next() {
        var entry types.SnapshotMetadata
        var createdAt string
        
        err := rows.Scan(
            &entry.ID,
            &entry.ContainerName,
            &entry.ImageTag,
            &entry.ImageID,
            &entry.RepoDigest,
            &entry.Status,
            &entry.Message,
            &createdAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan history entry: %w", err)
        }

        t, err := utils.ParseTime(createdAt)
        if err != nil {
            return nil, fmt.Errorf("failed to parse time createdAt: %w", err)
        }
        entry.CreatedAt = t

        entries = append(entries, entry)

    }

    // Post-processing
    if opts.Last {
        seen := make(map[string]bool)
        var filtered []types.SnapshotMetadata
        for _, entry := range entries {
            if !seen[entry.ContainerName] {
                filtered = append(filtered, entry)
                seen[entry.ContainerName] = true
            }
        }
        entries = filtered
    }

    if opts.Limit > 0 && len(entries) > opts.Limit {
        entries = entries[:opts.Limit]
    }

    return entries, nil
}

// CleanupSnapshots nettoie les anciens snapshots
func (d *Database) CleanupSnapshots(containerName string, retain int) error {
    rows, err := d.db.Query(`
        SELECT id, zfs_snapshot 
        FROM container_snapshots 
        WHERE container_name = ? 
        ORDER BY created_at DESC 
        LIMIT -1 OFFSET ?`,
        containerName, retain,
    )
    if err != nil {
        return fmt.Errorf("failed to query old snapshots: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var id int64
        var zfsSnapshot sql.NullString
        if err := rows.Scan(&id, &zfsSnapshot); err != nil {
            return fmt.Errorf("failed to scan snapshot row: %w", err)
        }

        // Supprimer le snapshot ZFS si présent
        if zfsSnapshot.Valid && zfsSnapshot.String != "" {
            if err := d.zfs.DeleteSnapshot(zfsSnapshot.String); err != nil {
                d.logger.Warnf("Failed to delete ZFS snapshot %s: %v", 
                    zfsSnapshot.String, err)
            }
        }

        // Supprimer l'entrée de la base
        if _, err := d.db.Exec(
            "DELETE FROM container_snapshots WHERE id = ?", 
            id,
        ); err != nil {
            return fmt.Errorf("failed to delete snapshot %d: %w", id, err)
        }
    }

    return nil
}

func (d *Database) RenameContainer(oldName, newName string) (int64, error) {
    // Check if new name exists
    var count int
    err := d.db.QueryRow("SELECT COUNT(*) FROM container_snapshots WHERE container_name = ?", newName).Scan(&count)
    if err != nil {
        return 0, fmt.Errorf("failed to check for existing name: %w", err)
    }
    if count > 0 {
        return 0, fmt.Errorf("container with name %s already exists in database", newName)
    }

    // Perform rename
    result, err := d.db.Exec("UPDATE container_snapshots SET container_name = ? WHERE container_name = ?", 
        newName, oldName)
    if err != nil {
        return 0, fmt.Errorf("failed to update container name: %w", err)
    }

    return result.RowsAffected()
}
func (d *Database) RemoveEntries(containerName string, opts options.RemoveOptions) (int64, error) {
    var conditions []string
    var args []interface{}

    query := "DELETE FROM container_snapshots WHERE container_name = ?"
    args = append(args, containerName)

    if !opts.All {
        if !opts.Before.IsZero() {
            conditions = append(conditions, "created_at < ?")
            args = append(args, opts.Before.Format("2006-01-02"))
        }
        if opts.OlderThan > 0 {
            conditions = append(conditions, "created_at < ?")
            args = append(args, time.Now().Add(-opts.OlderThan).Format("2006-01-02"))
        }
    }

    if len(conditions) > 0 {
        query += " AND " + strings.Join(conditions, " AND ")
    }

    // Si ZFS activé, d'abord récupérer les snapshots à supprimer
    if opts.Zfs {
        var snapshots []string
        rows, err := d.db.Query("SELECT zfs_snapshot FROM container_snapshots WHERE "+query, args...)
        if err != nil {
            return 0, fmt.Errorf("failed to query ZFS snapshots: %w", err)
        }
        defer rows.Close()

        for rows.Next() {
            var snapshot string
            if err := rows.Scan(&snapshot); err != nil {
                return 0, fmt.Errorf("failed to scan snapshot: %w", err)
            }
            if snapshot != "" {
                snapshots = append(snapshots, snapshot)
            }
        }

        // Supprimer les snapshots ZFS
        for _, snapshot := range snapshots {
            if err := d.zfs.DeleteSnapshot(snapshot); err != nil {
                d.logger.Warnf("Failed to delete ZFS snapshot %s: %v", snapshot, err)
            }
        }
    }

    // Supprimer les entrées
    result, err := d.db.Exec(query, args...)
    if err != nil {
        return 0, fmt.Errorf("failed to delete entries: %w", err)
    }

    return result.RowsAffected()
}