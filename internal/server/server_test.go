package server_test

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/narrowcastdev/cronguard/internal/server"
	"github.com/narrowcastdev/cronguard/internal/store"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func newTestServer(t *testing.T) (*server.Server, *store.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	srv := server.New(db, nil, "")
	return srv, db
}

func TestPingSuccess(t *testing.T) {
	srv, db := newTestServer(t)
	db.CreateCheck("backup", "0 3 * * *", "5m")

	req := httptest.NewRequest("POST", "/ping/backup", strings.NewReader("all good"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	check, _ := db.GetCheckBySlug("backup")
	if check.LastPing == nil {
		t.Error("last_ping should be set")
	}
	if check.LastOutput != "all good" {
		t.Errorf("last_output = %q, want %q", check.LastOutput, "all good")
	}
}

func TestPingNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("POST", "/ping/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestPingGetMethod(t *testing.T) {
	srv, db := newTestServer(t)
	db.CreateCheck("backup", "0 3 * * *", "5m")

	req := httptest.NewRequest("GET", "/ping/backup", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestPingFailEndpoint(t *testing.T) {
	srv, db := newTestServer(t)
	db.CreateCheck("backup", "0 3 * * *", "5m")

	req := httptest.NewRequest("POST", "/ping/backup/fail", strings.NewReader("error: disk full"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	check, _ := db.GetCheckBySlug("backup")
	if check.LastFail == nil {
		t.Error("last_fail should be set")
	}
}

func TestPingNoAuth(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateCheck("backup", "0 3 * * *", "5m")

	// Server with password set — ping should still work without auth
	srv := server.New(db, nil, "secret123")

	req := httptest.NewRequest("POST", "/ping/backup", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("ping should not require auth, got status %d", w.Code)
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := server.New(db, nil, "secret")

	req := httptest.NewRequest("GET", "/api/checks", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAPIListChecks(t *testing.T) {
	srv, db := newTestServer(t)
	db.CreateCheck("backup", "0 3 * * *", "5m")

	req := httptest.NewRequest("GET", "/api/checks", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "backup") {
		t.Error("response should contain check name")
	}
}

func TestAPICreateCheck(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"name":"certbot","schedule":"0 0 1 * *","grace":"10m"}`
	req := httptest.NewRequest("POST", "/api/checks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("status = %d, want 201", w.Code)
	}
	if !strings.Contains(w.Body.String(), "certbot") {
		t.Error("response should contain slug")
	}
}

func TestAPIDeleteCheck(t *testing.T) {
	srv, db := newTestServer(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")

	req := httptest.NewRequest("DELETE", "/api/checks/"+itoa(check.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestAPICreateAlertDest(t *testing.T) {
	srv, db := newTestServer(t)
	check, _ := db.CreateCheck("backup", "0 3 * * *", "5m")

	body := `{"type":"webhook","target":"https://hooks.slack.com/xxx"}`
	req := httptest.NewRequest("POST", "/api/checks/"+itoa(check.ID)+"/alerts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("status = %d, want 201", w.Code)
	}
}
