package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/narrowcastdev/cronguard/internal/cron"
	"github.com/narrowcastdev/cronguard/internal/store"
)

type checkResponse struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Schedule     string     `json:"schedule"`
	Grace        string     `json:"grace"`
	Status       string     `json:"status"`
	Alerted      bool       `json:"alerted"`
	LastPing     *time.Time `json:"last_ping"`
	LastOutput   string     `json:"last_output,omitempty"`
	NextExpected *time.Time `json:"next_expected,omitempty"`
	PingURL      string     `json:"ping_url"`
	CreatedAt    time.Time  `json:"created_at"`
}

func toCheckResponse(c store.Check) checkResponse {
	resp := checkResponse{
		ID:        c.ID,
		Name:      c.Name,
		Slug:      c.Slug,
		Schedule:  c.Schedule,
		Grace:     c.Grace,
		Status:    computeStatus(c),
		Alerted:   c.Alerted,
		LastPing:  c.LastPing,
		PingURL:   "/ping/" + c.Slug,
		CreatedAt: c.CreatedAt,
	}

	if c.LastOutput != "" {
		resp.LastOutput = c.LastOutput
	}

	if c.LastPing != nil {
		sched, err := cron.Parse(c.Schedule)
		if err == nil {
			next := sched.Next(*c.LastPing)
			resp.NextExpected = &next
		}
	}

	return resp
}

func computeStatus(c store.Check) string {
	if c.LastPing == nil {
		return "new"
	}

	sched, err := cron.Parse(c.Schedule)
	if err != nil {
		return "new"
	}

	grace, err := time.ParseDuration(c.Grace)
	if err != nil {
		grace = 5 * time.Minute
	}

	nextExpected := sched.Next(*c.LastPing)
	now := time.Now()

	deadline := nextExpected.Add(grace)
	if now.After(deadline) {
		return "down"
	}
	if now.After(nextExpected) {
		return "late"
	}
	return "up"
}

func (s *Server) handleListChecks(w http.ResponseWriter, r *http.Request) {
	checks, err := s.db.ListChecks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]checkResponse, 0, len(checks))
	for _, c := range checks {
		resp = append(resp, toCheckResponse(c))
	}

	writeJSON(w, http.StatusOK, resp)
}

type createCheckRequest struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Grace    string `json:"grace"`
}

func (s *Server) handleCreateCheck(w http.ResponseWriter, r *http.Request) {
	var req createCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if _, err := cron.Parse(req.Schedule); err != nil {
		http.Error(w, fmt.Sprintf("invalid schedule: %v", err), http.StatusBadRequest)
		return
	}

	if req.Grace == "" {
		req.Grace = "5m"
	}
	if _, err := time.ParseDuration(req.Grace); err != nil {
		http.Error(w, fmt.Sprintf("invalid grace: %v", err), http.StatusBadRequest)
		return
	}

	check, err := s.db.CreateCheck(req.Name, req.Schedule, req.Grace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusCreated, toCheckResponse(check))
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req createCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Schedule != "" {
		if _, err := cron.Parse(req.Schedule); err != nil {
			http.Error(w, fmt.Sprintf("invalid schedule: %v", err), http.StatusBadRequest)
			return
		}
	}

	if err := s.db.UpdateCheck(id, req.Name, req.Schedule, req.Grace); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	check, err := s.db.GetCheck(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toCheckResponse(check))
}

func (s *Server) handleDeleteCheck(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteCheck(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAlertDests(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	dests, err := s.db.ListAlertDests(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if dests == nil {
		dests = []store.AlertDest{}
	}

	writeJSON(w, http.StatusOK, dests)
}

type createAlertDestRequest struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

func (s *Server) handleCreateAlertDest(w http.ResponseWriter, r *http.Request) {
	checkID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req createAlertDestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	dest, err := s.db.CreateAlertDest(checkID, req.Type, req.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, dest)
}

func (s *Server) handleDeleteAlertDest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteAlertDest(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestAlert(w http.ResponseWriter, r *http.Request) {
	alertID, err := strconv.ParseInt(r.PathValue("alertId"), 10, 64)
	if err != nil {
		http.Error(w, "invalid alert id", http.StatusBadRequest)
		return
	}

	// Retrieve the alert destination by listing all for the check and finding the right one.
	checkID, err := strconv.ParseInt(r.PathValue("checkId"), 10, 64)
	if err != nil {
		http.Error(w, "invalid check id", http.StatusBadRequest)
		return
	}

	dests, err := s.db.ListAlertDests(checkID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var dest *store.AlertDest
	for _, d := range dests {
		if d.ID == alertID {
			dest = &d
			break
		}
	}

	if dest == nil {
		http.NotFound(w, r)
		return
	}

	if s.checker == nil {
		http.Error(w, "checker not configured", http.StatusInternalServerError)
		return
	}

	if err := s.checker.SendTestAlert(*dest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type settingsResponse struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.GetSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := settingsResponse{
		SMTPHost:     settings.SMTPHost,
		SMTPPort:     settings.SMTPPort,
		SMTPUser:     settings.SMTPUser,
		SMTPPassword: "********",
		SMTPFrom:     settings.SMTPFrom,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	settings := store.Settings{
		SMTPHost:     req.SMTPHost,
		SMTPPort:     req.SMTPPort,
		SMTPUser:     req.SMTPUser,
		SMTPPassword: req.SMTPPassword,
		SMTPFrom:     req.SMTPFrom,
	}

	// If password is the redacted placeholder, keep the existing password.
	if req.SMTPPassword == "********" {
		existing, err := s.db.GetSettings()
		if err == nil {
			settings.SMTPPassword = existing.SMTPPassword
		}
	}

	if err := s.db.UpdateSettings(settings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
