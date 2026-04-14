package store_test

import (
	"testing"

	"github.com/narrowcastdev/cronguard/internal/store"
)

func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndGetCheck(t *testing.T) {
	db := newTestDB(t)
	check, err := db.CreateCheck("nightly backup", "0 3 * * *", "5m")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if check.Name != "nightly backup" {
		t.Errorf("name = %q, want %q", check.Name, "nightly backup")
	}
	if check.Slug != "nightly-backup" {
		t.Errorf("slug = %q, want %q", check.Slug, "nightly-backup")
	}
	if check.Schedule != "0 3 * * *" {
		t.Errorf("schedule = %q, want %q", check.Schedule, "0 3 * * *")
	}
	if check.Grace != "5m" {
		t.Errorf("grace = %q, want %q", check.Grace, "5m")
	}

	got, err := db.GetCheck(check.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Slug != "nightly-backup" {
		t.Errorf("slug = %q, want %q", got.Slug, "nightly-backup")
	}
}

func TestListChecks(t *testing.T) {
	db := newTestDB(t)
	db.CreateCheck("backup", "0 3 * * *", "5m")
	db.CreateCheck("certbot", "0 0 1 * *", "10m")

	checks, err := db.ListChecks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(checks) != 2 {
		t.Errorf("got %d checks, want 2", len(checks))
	}
}

func TestUpdateCheck(t *testing.T) {
	db := newTestDB(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")

	err := db.UpdateCheck(check.ID, "daily backup", "0 4 * * *", "10m")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := db.GetCheck(check.ID)
	if got.Name != "daily backup" {
		t.Errorf("name = %q, want %q", got.Name, "daily backup")
	}
	if got.Slug != "daily-backup" {
		t.Errorf("slug = %q, want %q", got.Slug, "daily-backup")
	}
}

func TestDeleteCheck(t *testing.T) {
	db := newTestDB(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")

	err := db.DeleteCheck(check.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = db.GetCheck(check.ID)
	if err == nil {
		t.Error("expected error getting deleted check")
	}
}

func TestDuplicateSlug(t *testing.T) {
	db := newTestDB(t)
	_, err := db.CreateCheck("backup", "0 3 * * *", "5m")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = db.CreateCheck("backup", "0 4 * * *", "5m")
	if err == nil {
		t.Error("expected error for duplicate slug")
	}
}

func TestRecordPing(t *testing.T) {
	db := newTestDB(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")

	_, err := db.RecordPing(check.Slug, "output data")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}

	got, _ := db.GetCheckBySlug(check.Slug)
	if got.LastPing == nil {
		t.Error("last_ping should not be nil after ping")
	}
	if got.LastOutput != "output data" {
		t.Errorf("last_output = %q, want %q", got.LastOutput, "output data")
	}
}

func TestAlertDestinations(t *testing.T) {
	db := newTestDB(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")

	dest, err := db.CreateAlertDest(check.ID, "webhook", "https://hooks.slack.com/xxx")
	if err != nil {
		t.Fatalf("create alert: %v", err)
	}
	if dest.Type != "webhook" {
		t.Errorf("type = %q, want %q", dest.Type, "webhook")
	}

	dests, err := db.ListAlertDests(check.ID)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(dests) != 1 {
		t.Errorf("got %d dests, want 1", len(dests))
	}

	err = db.DeleteAlertDest(dest.ID)
	if err != nil {
		t.Fatalf("delete alert: %v", err)
	}

	dests, _ = db.ListAlertDests(check.ID)
	if len(dests) != 0 {
		t.Errorf("got %d dests after delete, want 0", len(dests))
	}
}

func TestDeleteCheckCascadesAlerts(t *testing.T) {
	db := newTestDB(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")
	db.CreateAlertDest(check.ID, "webhook", "https://example.com")

	db.DeleteCheck(check.ID)

	dests, _ := db.ListAlertDests(check.ID)
	if len(dests) != 0 {
		t.Errorf("alert destinations should be deleted with check")
	}
}

func TestSettings(t *testing.T) {
	db := newTestDB(t)

	s, err := db.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if s.SMTPPort != 587 {
		t.Errorf("default smtp_port = %d, want 587", s.SMTPPort)
	}

	err = db.UpdateSettings(store.Settings{
		SMTPHost:     "smtp.example.com",
		SMTPPort:     465,
		SMTPUser:     "user",
		SMTPPassword: "pass",
		SMTPFrom:     "cronguard@example.com",
	})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	s, _ = db.GetSettings()
	if s.SMTPHost != "smtp.example.com" {
		t.Errorf("smtp_host = %q, want %q", s.SMTPHost, "smtp.example.com")
	}
}
