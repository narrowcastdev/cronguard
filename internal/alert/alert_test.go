package alert_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/narrowcastdev/cronguard/internal/alert"
)

func TestWebhookSendSuccess(t *testing.T) {
	var received atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
		var payload alert.Payload
		json.NewDecoder(r.Body).Decode(&payload)
		if payload.Check != "backup" {
			t.Errorf("check = %q, want %q", payload.Check, "backup")
		}
		if payload.Status != "down" {
			t.Errorf("status = %q, want %q", payload.Status, "down")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	sender := alert.NewWebhookSenderWithDelays([]time.Duration{0, 0, 0})
	payload := alert.Payload{
		Check:   "backup",
		Status:  "down",
		Message: "Check \"backup\" is down.",
	}
	err := sender.Send(srv.URL, payload)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !received.Load() {
		t.Error("webhook was not received")
	}
}

func TestWebhookRetries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	sender := alert.NewWebhookSenderWithDelays([]time.Duration{0, 0, 0})
	payload := alert.Payload{Check: "backup", Status: "down", Message: "down"}
	err := sender.Send(srv.URL, payload)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestWebhookAllRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	sender := alert.NewWebhookSenderWithDelays([]time.Duration{0, 0, 0})
	payload := alert.Payload{Check: "backup", Status: "down", Message: "down"}
	err := sender.Send(srv.URL, payload)
	if err == nil {
		t.Error("expected error after all retries fail")
	}
}

func TestEmailFormat(t *testing.T) {
	sender := alert.NewEmailSender("", 0, "", "", "cronguard@example.com")
	subject, body := sender.Format(alert.Payload{
		Check:   "backup",
		Status:  "down",
		Message: "Check \"backup\" is down. Last ping was 26 hours ago.",
	})
	if subject != "[cronguard] backup is down" {
		t.Errorf("subject = %q", subject)
	}
	if body != "Check \"backup\" is down. Last ping was 26 hours ago." {
		t.Errorf("body = %q", body)
	}
}
