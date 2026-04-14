package checker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/narrowcastdev/cronguard/internal/alert"
	"github.com/narrowcastdev/cronguard/internal/cron"
	"github.com/narrowcastdev/cronguard/internal/store"
)

// Checker evaluates check schedules and dispatches alerts.
type Checker struct {
	db     *store.DB
	sender alert.Sender
}

// New creates a Checker with the given store and alert sender.
func New(db *store.DB, sender alert.Sender) *Checker {
	return &Checker{db: db, sender: sender}
}

// Evaluate checks all monitors and sends alerts for overdue checks.
func (c *Checker) Evaluate() error {
	checks, err := c.db.ListChecks()
	if err != nil {
		return fmt.Errorf("list checks: %w", err)
	}

	for _, check := range checks {
		if check.LastPing == nil {
			continue
		}

		sched, err := cron.Parse(check.Schedule)
		if err != nil {
			log.Printf("checker: invalid schedule for %q: %v", check.Name, err)
			continue
		}

		nextExpected := sched.Next(*check.LastPing)
		grace, err := time.ParseDuration(check.Grace)
		if err != nil {
			log.Printf("checker: invalid grace for %q: %v", check.Name, err)
			continue
		}

		deadline := nextExpected.Add(grace)
		if time.Now().After(deadline) && !check.Alerted {
			c.db.SetAlerted(check.ID, true)
			if err := c.sendDownAlerts(check, nextExpected); err != nil {
				log.Printf("checker: alert error for %q: %v", check.Name, err)
			}
		}
	}

	return nil
}

// SendRecoveryAlerts sends recovery alerts for a check that was down.
func (c *Checker) SendRecoveryAlerts(check store.Check) {
	if c.sender == nil {
		return
	}
	dests, err := c.db.ListAlertDests(check.ID)
	if err != nil {
		log.Printf("checker: list alert dests for recovery: %v", err)
		return
	}

	payload := alert.Payload{
		Check:   check.Slug,
		Status:  "up",
		Message: fmt.Sprintf("Check %q is back up.", check.Slug),
	}
	if check.LastPing != nil {
		payload.LastPing = check.LastPing
	}

	for _, dest := range dests {
		if err := c.sender.Send(dest.Target, payload); err != nil {
			log.Printf("checker: recovery alert error for %q -> %s: %v", check.Slug, dest.Target, err)
		}
	}
}

// SendFailAlerts sends immediate failure alerts for a check.
func (c *Checker) SendFailAlerts(check store.Check, output string) {
	if c.sender == nil {
		return
	}
	dests, err := c.db.ListAlertDests(check.ID)
	if err != nil {
		log.Printf("checker: list alert dests for fail: %v", err)
		return
	}

	payload := alert.Payload{
		Check:      check.Slug,
		Status:     "down",
		LastOutput: output,
		Message:    fmt.Sprintf("Check %q reported failure.", check.Slug),
	}

	for _, dest := range dests {
		if err := c.sender.Send(dest.Target, payload); err != nil {
			log.Printf("checker: fail alert error for %q -> %s: %v", check.Slug, dest.Target, err)
		}
	}
}

// SendTestAlert sends a test alert to a specific destination.
func (c *Checker) SendTestAlert(dest store.AlertDest) error {
	if c.sender == nil {
		return fmt.Errorf("no sender configured")
	}
	payload := alert.Payload{
		Check:   "test",
		Status:  "test",
		Message: "This is a test alert from cronguard.",
	}
	return c.sender.Send(dest.Target, payload)
}

func (c *Checker) sendDownAlerts(check store.Check, nextExpected time.Time) error {
	if c.sender == nil {
		return nil
	}

	dests, err := c.db.ListAlertDests(check.ID)
	if err != nil {
		return fmt.Errorf("list alert dests: %w", err)
	}

	payload := alert.Payload{
		Check:    check.Slug,
		Status:   "down",
		Expected: &nextExpected,
		Message:  fmt.Sprintf("Check %q is down. Last ping was %s.", check.Slug, formatAgo(*check.LastPing)),
	}
	if check.LastPing != nil {
		payload.LastPing = check.LastPing
	}
	if check.LastOutput != "" {
		payload.LastOutput = check.LastOutput
	}

	for _, dest := range dests {
		if err := c.sender.Send(dest.Target, payload); err != nil {
			log.Printf("checker: alert to %s failed: %v", dest.Target, err)
		}
	}

	return nil
}

func formatAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// Run starts the evaluation loop, ticking every 30 seconds until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	if err := c.Evaluate(); err != nil {
		log.Printf("checker: evaluate: %v", err)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Evaluate(); err != nil {
				log.Printf("checker: evaluate: %v", err)
			}
		}
	}
}
