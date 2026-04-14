package checker_test

import (
	"testing"
	"time"

	"github.com/narrowcastdev/cronguard/internal/alert"
	"github.com/narrowcastdev/cronguard/internal/checker"
	"github.com/narrowcastdev/cronguard/internal/store"
)

type mockSender struct {
	sent []alert.Payload
}

func (m *mockSender) Send(target string, p alert.Payload) error {
	m.sent = append(m.sent, p)
	return nil
}

func setupCheckerTest(t *testing.T) (*store.DB, *mockSender, *checker.Checker) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	sender := &mockSender{}
	c := checker.New(db, sender)
	return db, sender, c
}

func TestEvaluateNoChecks(t *testing.T) {
	_, _, c := setupCheckerTest(t)
	err := c.Evaluate()
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
}

func TestEvaluateCheckIsUp(t *testing.T) {
	db, sender, c := setupCheckerTest(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")
	db.CreateAlertDest(check.ID, "webhook", "http://example.com")

	// Simulate a recent ping
	db.RecordPing(check.Slug, "")

	c.Evaluate()

	if len(sender.sent) != 0 {
		t.Errorf("should not alert for up check, got %d alerts", len(sender.sent))
	}
}

func TestEvaluateCheckIsDown(t *testing.T) {
	db, sender, c := setupCheckerTest(t)
	check, _ := db.CreateCheck("backup", "*/1 * * * *", "1m")
	db.CreateAlertDest(check.ID, "webhook", "http://example.com")

	// Simulate a ping that's old enough to be past next_expected + grace
	db.RecordPingAt(check.Slug, "", time.Now().Add(-5*time.Minute))

	c.Evaluate()

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(sender.sent))
	}
	if sender.sent[0].Status != "down" {
		t.Errorf("status = %q, want %q", sender.sent[0].Status, "down")
	}
}

func TestEvaluateDoesNotDuplicateAlerts(t *testing.T) {
	db, sender, c := setupCheckerTest(t)
	check, _ := db.CreateCheck("backup", "*/1 * * * *", "1m")
	db.CreateAlertDest(check.ID, "webhook", "http://example.com")

	db.RecordPingAt(check.Slug, "", time.Now().Add(-5*time.Minute))

	c.Evaluate()
	c.Evaluate()

	if len(sender.sent) != 1 {
		t.Errorf("expected 1 alert (no duplicates), got %d", len(sender.sent))
	}
}

func TestEvaluateNewCheckNoAlert(t *testing.T) {
	db, sender, c := setupCheckerTest(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")
	db.CreateAlertDest(check.ID, "webhook", "http://example.com")

	// No pings yet — status is "new", should not alert
	_ = check
	c.Evaluate()

	if len(sender.sent) != 0 {
		t.Errorf("should not alert for new check, got %d alerts", len(sender.sent))
	}
}
