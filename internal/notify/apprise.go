// internal/notify/apprise.go
package notify

import (
    "fmt"
    "net/http"
    "net/url"
    "io"
    "strings"
    "time"
    "github.com/sirupsen/logrus"
    "zockimate/internal/types" 
)

// AppriseClient gère les notifications via Apprise
type AppriseClient struct {
    url        string
    httpClient *http.Client
    logger     *logrus.Logger
}

// NewAppriseClient crée un nouveau client Apprise
func NewAppriseClient(appriseURL string, logger *logrus.Logger) *AppriseClient {
    return &AppriseClient{
        url: appriseURL,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
        logger: logger,
    }
}

// SendNotification envoie une notification via Apprise
func (a *AppriseClient) SendNotification(title, message string, tags []string) error {
    if a.url == "" {
        return nil  // Pas d'URL = pas de notification
    }

    data := url.Values{}
    data.Set("title", title)
    data.Set("body", message)
    if len(tags) > 0 {
        data.Set("tags", strings.Join(tags, ","))
    }

    resp, err := a.httpClient.PostForm(a.url, data)
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("notification failed with status %d: %s", 
            resp.StatusCode, string(body))
    }

    a.logger.Debugf("Notification sent: %s", title)
    return nil
}

// Helpers pour les messages communs
func (a *AppriseClient) NotifyUpdateAvailable(container string, currentImg, newImg *types.ImageReference) {
    msg := fmt.Sprintf("Update available for %s:\nCurrent: %s\nNew: %s",
        container, currentImg.String(), newImg.String())
    
    a.SendNotification(
        "Container Update Available",
        msg,
        []string{"info", "update-available"},
    )
}

func (a *AppriseClient) NotifyUpdateSuccess(container string, oldImg, newImg *types.ImageReference) {
    msg := fmt.Sprintf("Successfully updated %s:\nFrom: %s\nTo: %s",
        container, oldImg.String(), newImg.String())
    
    a.SendNotification(
        "Container Updated",
        msg,
        []string{"success", "update"},
    )
}

func (a *AppriseClient) NotifyUpdateError(container string, err error) {
    msg := fmt.Sprintf("Failed to update container %s:\n%v", container, err)
    
    a.SendNotification(
        "Container Update Failed",
        msg,
        []string{"error", "update"},
    )
}