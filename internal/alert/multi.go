package alert

import (
	"fmt"
	"strings"
)

// SettingsProvider retrieves SMTP configuration on demand.
type SettingsProvider interface {
	SMTPConfig() (host string, port int, user, password, from string, err error)
}

// MultiSender routes alerts to the appropriate sender based on destination type.
type MultiSender struct {
	webhook  *WebhookSender
	settings SettingsProvider
}

// NewMultiSender creates a sender that dispatches to webhook or email based on target.
func NewMultiSender(webhook *WebhookSender, settings SettingsProvider) *MultiSender {
	return &MultiSender{webhook: webhook, settings: settings}
}

// Send routes the alert to the correct sender. Targets containing "@" are treated
// as email; everything else is treated as a webhook URL.
func (m *MultiSender) Send(target string, payload Payload) error {
	if strings.Contains(target, "@") {
		return m.sendEmail(target, payload)
	}
	return m.webhook.Send(target, payload)
}

func (m *MultiSender) sendEmail(target string, payload Payload) error {
	host, port, user, password, from, err := m.settings.SMTPConfig()
	if err != nil {
		return fmt.Errorf("loading SMTP settings: %w", err)
	}
	if host == "" {
		return fmt.Errorf("SMTP not configured — set SMTP settings in the dashboard")
	}
	sender := NewEmailSender(host, port, user, password, from)
	return sender.Send(target, payload)
}
