package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/beads_server/internal/store"
)

const testToken = "test-secret-token"

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Load(filepath.Join(dir, "beads.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p := NewSingleStoreProvider(testToken, s)
	srv, err := New(Config{Port: 0, DataFile: filepath.Join(dir, "beads.json")}, p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Store = s

	// Add a test route behind the auth middleware to verify auth works
	srv.Router.Group(func(r chi.Router) {
		r.Use(srv.authMiddleware)
		r.Get("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"test":"ok"}`))
		})
	})

	return srv
}

func TestHealthEndpoint(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body)
	}
}

func TestHealthNoAuthRequired(t *testing.T) {
	srv := testServer(t)

	// No Authorization header — should still work for health
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", w.Code)
	}
}

func TestAuthMissingToken(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthWrongToken(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthValidToken(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthInvalidFormat(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestNewServerRequiresProvider(t *testing.T) {
	_, err := New(Config{Port: 8080, DataFile: "beads.json"}, nil)
	if err == nil {
		t.Fatal("expected error when provider is nil")
	}
}

func TestMultiProjectAuth(t *testing.T) {
	dir := t.TempDir()

	// Create two separate stores
	s1, err := store.Load(filepath.Join(dir, "project1.json"))
	if err != nil {
		t.Fatalf("Load s1: %v", err)
	}
	s2, err := store.Load(filepath.Join(dir, "project2.json"))
	if err != nil {
		t.Fatalf("Load s2: %v", err)
	}

	token1 := "tok-project1"
	token2 := "tok-project2"

	p := NewMultiStoreProvider(map[string]*store.Store{
		token1: s1,
		token2: s2,
	})

	srv, err := New(Config{Port: 0}, p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Helper to create an authenticated request with a specific token
	reqWith := func(method, url, token string, body any) *http.Request {
		var buf bytes.Buffer
		if body != nil {
			json.NewEncoder(&buf).Encode(body)
		}
		r := httptest.NewRequest(method, url, &buf)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		return r
	}

	// Create a bead in project 1
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, reqWith(http.MethodPost, "/api/v1/beads", token1, map[string]any{
		"title": "Project 1 bead",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create p1: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Create a bead in project 2
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, reqWith(http.MethodPost, "/api/v1/beads", token2, map[string]any{
		"title": "Project 2 bead",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create p2: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List beads with token1 — should see only project 1's bead
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, reqWith(http.MethodGet, "/api/v1/beads", token1, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list p1: expected 200, got %d", w.Code)
	}
	var result1 store.ListResult
	json.NewDecoder(w.Body).Decode(&result1)
	if result1.Total != 1 {
		t.Fatalf("project 1: expected 1 bead, got %d", result1.Total)
	}
	if result1.Beads[0].Title != "Project 1 bead" {
		t.Fatalf("project 1: expected title 'Project 1 bead', got %q", result1.Beads[0].Title)
	}

	// List beads with token2 — should see only project 2's bead
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, reqWith(http.MethodGet, "/api/v1/beads", token2, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list p2: expected 200, got %d", w.Code)
	}
	var result2 store.ListResult
	json.NewDecoder(w.Body).Decode(&result2)
	if result2.Total != 1 {
		t.Fatalf("project 2: expected 1 bead, got %d", result2.Total)
	}
	if result2.Beads[0].Title != "Project 2 bead" {
		t.Fatalf("project 2: expected title 'Project 2 bead', got %q", result2.Beads[0].Title)
	}

	// Unknown token should get 401
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, reqWith(http.MethodGet, "/api/v1/beads", "tok-unknown", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown token: expected 401, got %d", w.Code)
	}
}
