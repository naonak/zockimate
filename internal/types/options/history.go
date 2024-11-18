package options

import "time"

// HistoryOptions définit les options pour la consultation de l'historique
type HistoryOptions struct {
    Limit     int           // Limite du nombre d'entrées
    Last      bool          // Seulement la dernière entrée par conteneur
    SortBy    string        // Tri (date|container)
    JSON      bool          // Sortie au format JSON
    Search    string        // Recherche dans les messages et statuts
    Since     time.Time     // Depuis date
    Before    time.Time     // Avant date
    Container []string      // Filtrer par conteneurs
}