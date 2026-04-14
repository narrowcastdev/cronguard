package server

import (
	"io"
	"net/http"
)

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var output string
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 10240))
		if err == nil {
			output = string(body)
		}
	}

	wasAlerted, err := s.db.RecordPing(slug, output)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if wasAlerted && s.checker != nil {
		check, err := s.db.GetCheckBySlug(slug)
		if err == nil {
			go s.checker.SendRecoveryAlerts(check)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handlePingFail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var output string
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 10240))
		if err == nil {
			output = string(body)
		}
	}

	err := s.db.RecordFail(slug, output)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if s.checker != nil {
		check, err := s.db.GetCheckBySlug(slug)
		if err == nil {
			go s.checker.SendFailAlerts(check, output)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
