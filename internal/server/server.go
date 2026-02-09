package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/beads_server/internal/store"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const storeContextKey contextKey = iota

// Config holds the server configuration.
type Config struct {
	Port     int
	DataFile string
}

// Server is the HTTP server for the beads API.
type Server struct {
	Router   *chi.Mux
	Store    *store.Store // retained for backward compatibility; handlers will migrate to storeFor(r)
	provider StoreProvider
	config   Config
}

// New creates a new Server with the given config and provider.
func New(cfg Config, p StoreProvider) (*Server, error) {
	if p == nil {
		return nil, fmt.Errorf("store provider must not be nil")
	}

	srv := &Server{
		Router:   chi.NewRouter(),
		provider: p,
		config:   cfg,
	}

	srv.Router.Use(middleware.Recoverer)

	// Unauthenticated endpoints
	srv.Router.Get("/", srv.handleDashboard)
	srv.Router.Get("/api/v1/health", srv.handleHealth)

	// All other API routes require auth
	srv.Router.Group(func(r chi.Router) {
		r.Use(srv.authMiddleware)
		r.Get("/api/v1/beads", srv.handleListBeads)
		r.Post("/api/v1/beads", srv.handleCreateBead)
		r.Get("/api/v1/beads/{id}", srv.handleGetBead)
		r.Patch("/api/v1/beads/{id}", srv.handleUpdateBead)
		r.Delete("/api/v1/beads/{id}", srv.handleDeleteBead)
		r.Post("/api/v1/beads/{id}/claim", srv.handleClaimBead)
		r.Post("/api/v1/beads/{id}/comments", srv.handleAddComment)
		r.Post("/api/v1/beads/{id}/link", srv.handleLinkBead)
		r.Delete("/api/v1/beads/{id}/link/{other_id}", srv.handleUnlinkBead)
		r.Get("/api/v1/beads/{id}/deps", srv.handleGetDeps)
		r.Get("/api/v1/search", srv.handleSearch)
	})

	return srv, nil
}

// ListenAddr returns the address the server should listen on.
func (s *Server) ListenAddr() string {
	return fmt.Sprintf(":%d", s.config.Port)
}

// storeFor retrieves the store from the request context (set by authMiddleware).
func (s *Server) storeFor(r *http.Request) *store.Store {
	return r.Context().Value(storeContextKey).(*store.Store)
}

// authMiddleware authenticates via the StoreProvider and stores the resolved
// store in the request context.
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
		st := s.provider.Resolve(token)
		if st == nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), storeContextKey, st)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
