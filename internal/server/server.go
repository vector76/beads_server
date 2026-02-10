package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/beads_server/internal/store"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const storeContextKey contextKey = iota

// Config holds the server configuration.
type Config struct {
	Port      int
	DataFile  string
	LogOutput io.Writer // destination for request logs; nil defaults to os.Stdout
}

// Server is the HTTP server for the beads API.
type Server struct {
	Router   *chi.Mux
	Store    *store.Store // exported for direct store access in tests
	provider StoreProvider
	config   Config
	logger   *log.Logger
}

// New creates a new Server with the given config and provider.
func New(cfg Config, p StoreProvider) (*Server, error) {
	if p == nil {
		return nil, fmt.Errorf("store provider must not be nil")
	}

	logOut := cfg.LogOutput
	if logOut == nil {
		logOut = os.Stdout
	}

	srv := &Server{
		Router:   chi.NewRouter(),
		provider: p,
		config:   cfg,
		logger:   log.New(logOut, "", log.LstdFlags),
	}

	srv.Router.Use(middleware.Recoverer)
	srv.Router.Use(srv.requestLogger)

	// Unauthenticated endpoints
	srv.Router.Get("/", srv.handleDashboard)
	srv.Router.Get("/bead/{project}/{id}", srv.handleBeadDetail)
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
		r.Post("/api/v1/clean", srv.handleClean)
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

// requestLogger logs one line per request: method, path, status, and duration.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		s.logger.Printf("%s %s %d %s", r.Method, r.URL.Path, ww.status, time.Since(start).Round(time.Millisecond))
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	return rw.ResponseWriter.Write(b)
}
