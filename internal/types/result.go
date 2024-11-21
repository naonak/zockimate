
package types

type CheckResult struct {
    NeedsUpdate    bool              // Si une mise à jour est nécessaire
    CurrentImage   *ImageReference   // Référence de l'image actuelle
    UpdateImage    *ImageReference   // Référence de l'image à utiliser pour la mise à jour
    Error          error             // Erreur éventuelle
}

type UpdateResult struct {
    ContainerName   string
    Success        bool
    NeedsUpdate    bool
    RollbackNeeded bool
    SnapshotID     int64
    OldImage       *ImageReference
    NewImage       *ImageReference
    Error          error
}

type RollbackResult struct {
    ContainerName    string
    Success         bool
    SnapshotID      int64
    SafetySnapshot  int64
    ImageRollback   bool
    DataRollback    bool
    ConfigRollback  bool
    Error          error
}

type RenameResult struct {
    OldName         string
    NewName         string
    Success         bool
    DockerRenamed   bool
    EntriesRenamed  int64
    Error           error
}

type RemoveResult struct {
    ContainerName     string
    Success          bool
    ContainerRemoved bool
    EntriesDeleted   int64
    Error           error
}