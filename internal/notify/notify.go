// Package notify sends OS desktop notifications and, optionally, webhook
// notifications. Desktop notifications go through a command runner, and
// webhooks through an injectable *http.Client, so both are testable
// without touching the real OS or network.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"

	"github.com/richardcase/jumux/internal/run"
)

// Send delivers a desktop notification with title and message via the
// platform's notifier: osascript on macOS, notify-send on Linux. On any
// other platform Send is a silent no-op.
func Send(runner run.Runner, title, message string) error {
	return send(runner, runtime.GOOS, title, message)
}

func send(runner run.Runner, goos, title, message string) error {
	switch goos {
	case "darwin":
		script := fmt.Sprintf("display notification %q with title %q", message, title)
		_, err := runner.Run("", "osascript", "-e", script)
		return err
	case "linux":
		_, err := runner.Run("", "notify-send", title, message)
		return err
	default:
		return nil
	}
}

// WebhookPayload is the JSON body POSTed to a configured notify_webhook
// URL.
type WebhookPayload struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// SendWebhook POSTs a JSON notification payload to url using client (a nil
// client uses http.DefaultClient). It errors if the request fails or the
// server responds with a non-2xx/3xx status.
func SendWebhook(client *http.Client, url, title, message string) error {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(WebhookPayload{Title: title, Message: message})
	if err != nil {
		return err
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook %s: status %s", url, resp.Status)
	}
	return nil
}
