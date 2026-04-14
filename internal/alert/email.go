package alert

import (
	"fmt"
	"net/smtp"
	"strings"
)

// EmailSender sends alert payloads via SMTP email.
type EmailSender struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

// NewEmailSender creates an EmailSender with the given SMTP configuration.
func NewEmailSender(host string, port int, user, password, from string) *EmailSender {
	return &EmailSender{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
	}
}

// Format returns the subject and body for an alert email.
func (e *EmailSender) Format(payload Payload) (subject, body string) {
	subject = fmt.Sprintf("[cronguard] %s is %s", payload.Check, payload.Status)
	body = payload.Message
	return subject, body
}

// Send sends the alert payload as an email to the given address.
func (e *EmailSender) Send(target string, payload Payload) error {
	subject, body := e.Format(payload)

	msg := strings.Join([]string{
		"From: " + e.from,
		"To: " + target,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	var auth smtp.Auth
	if e.user != "" {
		auth = smtp.PlainAuth("", e.user, e.password, e.host)
	}

	return smtp.SendMail(addr, auth, e.from, []string{target}, []byte(msg))
}
