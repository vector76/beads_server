package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/beads_server/internal/store"
)

// Config holds the server configuration.
type Config struct {
	Port     int
	Token    string
	DataFile string
}

// Server is the HTTP server for the beads API.
type Server struct {
	Router *chi.Mux
	Store  *store.Store
	config Config
}

// New creates a new Server with the given config and store.
// Returns an error if the token is not configured.
func New(cfg Config, s *store.Store) (*Server, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("auth token must be configured")
	}

	srv := &Server{
		Router: chi.NewRouter(),
		Store:  s,
		config: cfg,
	}

	srv.Router.Use(middleware.Recoverer)

	// Health endpoint — no auth required
	srv.Router.Get("/api/v1/health", srv.handleHealth)

	// All other API routes require auth
	srv.Router.Group(func(r chi.Router) {
		r.Use(srv.authMiddleware)
		r.Post("/api/v1/beads", srv.handleCreateBead)
		r.Get("/api/v1/beads/{id}", srv.handleGetBead)
		r.Patch("/api/v1/beads/{id}", srv.handleUpdateBead)
		r.Delete("/api/v1/beads/{id}", srv.handleDeleteBead)
	})

	return srv, nil
}

// ListenAddr returns the address the server should listen on.
func (s *Server) ListenAddr() string {
	return fmt.Sprintf(":%d", s.config.Port)
}

// authMiddleware checks for a valid Bearer token.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token != s.config.Token {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
