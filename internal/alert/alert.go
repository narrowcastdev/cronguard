package alert

import "time"

// Payload is the JSON body sent to webhook destinations and used to format emails.
type Payload struct {
	Check      string     `json:"check"`
	Status     string     `json:"status"`
	LastPing   *time.Time `json:"last_ping,omitempty"`
	Expected   *time.Time `json:"expected,omitempty"`
	LastOutput string     `json:"last_output,omitempty"`
	Message    string     `json:"message"`
}

// Sender sends an alert payload to a target (URL or email address).
type Sender interface {
	Send(target string, payload Payload) error
}
