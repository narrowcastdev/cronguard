package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookSender sends alert payloads to webhook URLs via HTTP POST.
type WebhookSender struct {
	client      *http.Client
	retryDelays []time.Duration
}

// NewWebhookSender creates a WebhookSender with default retry delays (0s, 30s, 5m).
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client:      &http.Client{Timeout: 10 * time.Second},
		retryDelays: []time.Duration{0, 2 * time.Second, 5 * time.Second},
	}
}

// NewWebhookSenderWithDelays creates a WebhookSender with custom retry delays (for testing).
func NewWebhookSenderWithDelays(delays []time.Duration) *WebhookSender {
	return &WebhookSender{
		client:      &http.Client{Timeout: 10 * time.Second},
		retryDelays: delays,
	}
}

// Send posts the payload as JSON to the given URL with retries.
func (w *WebhookSender) Send(target string, payload Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var lastErr error
	for i, delay := range w.retryDelays {
		if i > 0 {
			time.Sleep(delay)
		}

		req, err := http.NewRequest("POST", target, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := w.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("all %d webhook attempts failed: %w", len(w.retryDelays), lastErr)
}
