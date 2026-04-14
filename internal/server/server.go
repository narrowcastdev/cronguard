package server

import (
	"crypto/subtle"
	"io/fs"
	"net/http"

	"github.com/narrowcastdev/cronguard/internal/checker"
	"github.com/narrowcastdev/cronguard/internal/store"
)

// Server is the HTTP server for cronguard.
type Server struct {
	db       *store.DB
	checker  *checker.Checker
	password string
	uiFS     fs.FS
	mux      *http.ServeMux
}

// New creates a new Server. If uiFS is nil, the UI will not be served.
func New(db *store.DB, chk *checker.Checker, password string, uiFS ...fs.FS) *Server {
	s := &Server{
		db:       db,
		checker:  chk,
		password: password,
		mux:      http.NewServeMux(),
	}

	if len(uiFS) > 0 && uiFS[0] != nil {
		s.uiFS = uiFS[0]
	}

	// Ping endpoints — no auth required.
	s.mux.HandleFunc("GET /ping/{slug}", s.handlePing)
	s.mux.HandleFunc("POST /ping/{slug}", s.handlePing)
	s.mux.HandleFunc("GET /ping/{slug}/fail", s.handlePingFail)
	s.mux.HandleFunc("POST /ping/{slug}/fail", s.handlePingFail)

	// API endpoints — auth required.
	s.mux.HandleFunc("GET /api/checks", s.requireAuth(s.handleListChecks))
	s.mux.HandleFunc("POST /api/checks", s.requireAuth(s.handleCreateCheck))
	s.mux.HandleFunc("PUT /api/checks/{id}", s.requireAuth(s.handleUpdateCheck))
	s.mux.HandleFunc("DELETE /api/checks/{id}", s.requireAuth(s.handleDeleteCheck))
	s.mux.HandleFunc("GET /api/checks/{id}/alerts", s.requireAuth(s.handleListAlertDests))
	s.mux.HandleFunc("POST /api/checks/{id}/alerts", s.requireAuth(s.handleCreateAlertDest))
	s.mux.HandleFunc("DELETE /api/alerts/{id}", s.requireAuth(s.handleDeleteAlertDest))
	s.mux.HandleFunc("POST /api/checks/{checkId}/alerts/{alertId}/test", s.requireAuth(s.handleTestAlert))
	s.mux.HandleFunc("GET /api/settings", s.requireAuth(s.handleGetSettings))
	s.mux.HandleFunc("PUT /api/settings", s.requireAuth(s.handleUpdateSettings))

	// UI — auth required.
	if s.uiFS != nil {
		fileServer := http.FileServer(http.FS(s.uiFS))
		s.mux.HandleFunc("/", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			fileServer.ServeHTTP(w, r)
		}))
	}

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.password == "" {
			next(w, r)
			return
		}

		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="cronguard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
