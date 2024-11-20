// internal/notify/apprise.go
package notify

import (
    "fmt"
    "time"
    "net/http"
    "encoding/json"
    "github.com/sirupsen/logrus"
    "zockimate/internal/types"

    "io"
    "strings"
    "bytes"

)

type AppriseClient struct {
    url        string
    httpClient *http.Client
    logger     *logrus.Logger
}

// Types de notification
const (
    NotificationInfo    = "info"
    NotificationSuccess = "success"
    NotificationError   = "error"
)

type Notification struct {
    Title    string   `json:"title"`
    Body     string   `json:"body"`
    Type     string   `json:"type"`
    Tags     []string `json:"tags,omitempty"`
}

func NewAppriseClient(appriseURL string, logger *logrus.Logger) (*AppriseClient, error) {
    if logger == nil {
        logger = logrus.New()
    }

    // Convertir apprise:// en http:// si nécessaire
    url := appriseURL
    if strings.HasPrefix(url, "apprise://") {
        url = "http://" + strings.TrimPrefix(url, "apprise://")
        logger.Debugf("Converted Apprise URL from %s to %s", appriseURL, url)
    }

    return &AppriseClient{
        url: url,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
        logger: logger,
    }, nil
}

func (a *AppriseClient) SendNotification(title, message string, tags []string) error {
    notification := Notification{
        Title: title,
        Body:  message,
        Type:  NotificationInfo,
        Tags:  tags,
    }

    jsonData, err := json.Marshal(notification)
    if err != nil {
        return fmt.Errorf("failed to marshal notification: %w", err)
    }

    resp, err := a.httpClient.Post(a.url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
    }

    return nil
}

func (a *AppriseClient) NotifyUpdateAvailable(container string, currentImg, newImg *types.ImageReference) error {
    msg := fmt.Sprintf("Update available for %s:\nCurrent: %s\nNew: %s",
        container, currentImg.String(), newImg.String())
    
    notification := Notification{
        Title: "Container Update Available",
        Body:  msg,
        Type:  NotificationInfo,
        Tags:  []string{"info", "update-available"},
    }

    jsonData, err := json.Marshal(notification)
    if err != nil {
        return fmt.Errorf("failed to marshal notification: %w", err)
    }

    resp, err := a.httpClient.Post(a.url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
    }

    return nil
}

func (a *AppriseClient) NotifyUpdateSuccess(container string, oldImg, newImg *types.ImageReference) error {
    msg := fmt.Sprintf("Successfully updated %s:\nFrom: %s\nTo: %s",
        container, oldImg.String(), newImg.String())
    
    notification := Notification{
        Title: "Container Updated",
        Body:  msg,
        Type:  NotificationSuccess,
        Tags:  []string{"success", "update"},
    }

    jsonData, err := json.Marshal(notification)
    if err != nil {
        return fmt.Errorf("failed to marshal notification: %w", err)
    }

    resp, err := a.httpClient.Post(a.url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
    }

    return nil
}

func (a *AppriseClient) NotifyUpdateError(container string, err error) error {
    msg := fmt.Sprintf("Failed to update container %s:\n%v", container, err)
    
    notification := Notification{
        Title: "Container Update Failed",
        Body:  msg,
        Type:  NotificationError,
        Tags:  []string{"error", "update"},
    }

    jsonData, err := json.Marshal(notification)
    if err != nil {
        return fmt.Errorf("failed to marshal notification: %w", err)
    }

    resp, err := a.httpClient.Post(a.url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
    }

    return nil
}

func (a *AppriseClient) Close() error {
    // Pas besoin de close pour le client HTTP
    return nil
}