// internal/notify/apprise.go
package notify

import (
    "fmt"
    "time"
    "net/http"
    "encoding/json"
    "github.com/sirupsen/logrus"
    "zockimate/internal/types"
    "net/url"
    "strconv"
    "io"
    "strings"
    "bytes"

)

// Types de notification
const (
    // Types de notification
    NotificationInfo     = "info"
    NotificationSuccess  = "success"
    NotificationWarning = "warning"
    NotificationError   = "error"

    // Formats
    FormatText     = "text"
    FormatMarkdown = "markdown"
    FormatHTML     = "html"

    // Overflow
    OverflowTruncate = "truncate"
    OverflowSplit    = "split"
)

type Notification struct {
    Title    string   `json:"title"`
    Body     string   `json:"body"`
    Type     string   `json:"type"`
    Tags     []string `json:"tags,omitempty"`
}

type AppriseClient struct {
    url             string
    tags            []string
    format          string     // text/markdown/html
    type_           string     // info/success/warning/error
    title           string     // Titre par défaut
    body_format     string     // notification body format template
    overflow        string     // truncate/split
    maxLength       int        // max length for messages
    interpret_emoji bool       // interpréter les émojis
    httpClient      *http.Client
    logger          *logrus.Logger
}

type AppriseOptions struct {
    Format         string   // Format du message (text/markdown/html)
    Type           string   // Type de notification (info/success/warning/error)
    Title          string   // Titre par défaut
    BodyFormat     string   // Template pour le corps du message
    Overflow       string   // Gestion du dépassement (truncate/split)
    MaxLength      int      // Longueur max des messages
    InterpretEmoji bool     // Interpréter les émojis
}

func NewAppriseClient(appriseURL string, logger *logrus.Logger, opts AppriseOptions) (*AppriseClient, error) {
    if logger == nil {
        logger = logrus.New()
    }

    u, err := url.Parse(appriseURL)
    if err != nil {
        return nil, fmt.Errorf("invalid URL: %w", err)
    }

    // Valider les valeurs acceptables
    if opts.Format != "" && opts.Format != FormatText && opts.Format != FormatMarkdown && opts.Format != FormatHTML {
        return nil, fmt.Errorf("invalid format: %s", opts.Format)
    }

    if opts.Type != "" && opts.Type != NotificationInfo && opts.Type != NotificationSuccess && 
    opts.Type != NotificationWarning && opts.Type != NotificationError {
        return nil, fmt.Errorf("invalid notification type: %s", opts.Type)
    }

    if opts.Overflow != "" && opts.Overflow != OverflowTruncate && opts.Overflow != OverflowSplit {
        return nil, fmt.Errorf("invalid overflow value: %s", opts.Overflow)
    }

    if opts.MaxLength < 0 {
        return nil, fmt.Errorf("maxLength cannot be negative")
    }

    // Extraire les paramètres de l'URL
    query := u.Query()
    tags := []string{}
    if tagParam := query.Get("tags"); tagParam != "" {
        tags = strings.Split(tagParam, ",")
        for i := range tags {
            tags[i] = strings.TrimSpace(tags[i])
        }
        query.Del("tags")
    }

    // Type par défaut si non spécifié
    type_ := opts.Type
    if type_ == "" {
        type_ = NotificationInfo
    }

    // Format par défaut
    format := opts.Format
    if format == "" {
        format = "text"
    }

    u.RawQuery = query.Encode()

    return &AppriseClient{
        url:             u.String(),
        tags:            tags,
        format:          format,
        type_:          type_,
        title:          opts.Title,
        body_format:     opts.BodyFormat,
        overflow:       opts.Overflow,
        maxLength:      opts.MaxLength,
        interpret_emoji: opts.InterpretEmoji,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
        logger: logger,
    }, nil
}

// Ajouter la méthode SendNotification et helper
func (a *AppriseClient) sendNotification(notification Notification) error {
    a.logger.Debugf("Sending notification: %s", notification.Title)

    // Fusionner les tags par défaut avec ceux de la notification
    if len(a.tags) > 0 {
        notification.Tags = mergeTags(a.tags, notification.Tags)
    }

    // Appliquer le type par défaut si non spécifié
    if notification.Type == "" {
        notification.Type = a.type_
    }

    jsonData, err := json.Marshal(notification)
    if err != nil {
        return fmt.Errorf("failed to marshal notification: %w", err)
    }

    // Appliquer les options configurées
    query := make(url.Values)
    if a.format != "" {
        query.Set("format", a.format)
    }
    if a.overflow != "" {
        query.Set("overflow", a.overflow)
    }
    if a.maxLength > 0 {
        query.Set("maxLength", strconv.Itoa(a.maxLength))
    }
    if a.interpret_emoji {
        query.Set("interpret_emoji", "true")
    }

    // Construire l'URL finale avec les paramètres
    finalURL := a.url
    if len(query) > 0 {
        if strings.Contains(finalURL, "?") {
            finalURL += "&" + query.Encode()
        } else {
            finalURL += "?" + query.Encode()
        }
    }

    a.logger.Debugf("POST %s with data: %s", finalURL, string(jsonData))

    // Utiliser finalURL au lieu de a.url pour la requête
    req, err := http.NewRequest("POST", finalURL, bytes.NewBuffer(jsonData))

    req.Header.Set("Content-Type", "application/json")

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("failed to read response body: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        a.logger.Debugf("Notification failed with status %d: %s", resp.StatusCode, string(body))
        return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
    }

    a.logger.Debugf("Notification sent successfully")
    return nil
}

func (a *AppriseClient) SendNotification(title, message string, tags []string) error {
    return a.sendNotification(Notification{
        Title: title,
        Body:  message,
        Type:  NotificationInfo,
        Tags:  tags,
    })
}

// Mettre à jour les autres méthodes pour utiliser sendNotification
func (a *AppriseClient) NotifyUpdateAvailable(container string, currentImg, newImg *types.ImageReference) error {
    return a.sendNotification(Notification{
        Title: "Container Update Available",
        Body:  fmt.Sprintf("Update available for %s:\nCurrent: %s\nNew: %s",
            container, currentImg.String(), newImg.String()),
        Type:  NotificationInfo,
        Tags:  []string{"info", "update-available"},
    })
}

func (a *AppriseClient) NotifyUpdateSuccess(container string, oldImg, newImg *types.ImageReference) error {
    return a.sendNotification(Notification{
        Title: "Container Updated",
        Body:  fmt.Sprintf("Successfully updated %s:\nFrom: %s\nTo: %s",
            container, oldImg.String(), newImg.String()),
        Type:  NotificationSuccess,
        Tags:  []string{"success", "update"},
    })
}

func (a *AppriseClient) NotifyUpdateError(container string, err error) error {
    return a.sendNotification(Notification{
        Title: "Container Update Failed",
        Body:  fmt.Sprintf("Failed to update container %s:\n%v", container, err),
        Type:  NotificationError,
        Tags:  []string{"error", "update"},
    })
}

func (a *AppriseClient) Close() error {
    // Pas besoin de close pour le client HTTP
    return nil
}

func mergeTags(baseTags []string, additionalTags []string) []string {
    if len(baseTags) == 0 {
        return additionalTags
    }
    if len(additionalTags) == 0 {
        return baseTags
    }

    // Utiliser une map pour dédédupliquer
    allTags := make(map[string]struct{})
    for _, tag := range baseTags {
        allTags[tag] = struct{}{}
    }
    for _, tag := range additionalTags {
        allTags[tag] = struct{}{}
    }
    
    // Convertir la map en slice
    uniqueTags := make([]string, 0, len(allTags))
    for tag := range allTags {
        uniqueTags = append(uniqueTags, tag)
    }

    return uniqueTags
}