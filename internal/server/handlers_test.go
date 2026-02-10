package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

func authReq(method, url string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func crudServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Load(filepath.Join(dir, "beads.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := NewSingleStoreProvider(testToken, s)
	srv, err := New(Config{Port: 0, DataFile: filepath.Join(dir, "beads.json"), LogOutput: io.Discard}, p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Store = s
	return srv
}

func createViaAPI(t *testing.T, srv *Server, body map[string]any) model.Bead {
	t.Helper()
	req := authReq(http.MethodPost, "/api/v1/beads", body)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)
	return b
}

func TestCreateBead_Minimal(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title": "Test bead",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.Title != "Test bead" {
		t.Fatalf("expected title 'Test bead', got %q", b.Title)
	}
	if b.Status != model.StatusOpen {
		t.Fatalf("expected default status 'open', got %q", b.Status)
	}
	if b.Priority != model.PriorityMedium {
		t.Fatalf("expected default priority 'medium', got %q", b.Priority)
	}
	if b.ID == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestCreateBead_AllFields(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":       "Full bead",
		"description": "A detailed description",
		"status":      "in_progress",
		"priority":    "high",
		"type":        "bug",
		"tags":        []string{"backend", "urgent"},
		"assignee":    "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.Description != "A detailed description" {
		t.Fatalf("description mismatch: %q", b.Description)
	}
	if b.Status != model.StatusInProgress {
		t.Fatalf("status mismatch: %q", b.Status)
	}
	if b.Priority != model.PriorityHigh {
		t.Fatalf("priority mismatch: %q", b.Priority)
	}
	if b.Type != model.TypeBug {
		t.Fatalf("type mismatch: %q", b.Type)
	}
	if len(b.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", b.Tags)
	}
	if b.Assignee != "alice" {
		t.Fatalf("assignee mismatch: %q", b.Assignee)
	}
}

func TestCreateBead_MissingTitle(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"description": "No title here",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetBead_Existing(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Get me"})

	req := authReq(http.MethodGet, "/api/v1/beads/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.ID != created.ID {
		t.Fatalf("ID mismatch: got %s, want %s", b.ID, created.ID)
	}
	if b.Title != "Get me" {
		t.Fatalf("title mismatch: %q", b.Title)
	}
}

func TestGetBead_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodGet, "/api/v1/beads/bd-nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetBead_ByPrefix(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Prefix test"})

	// Use first 6 chars of ID (after "bd-") as prefix
	prefix := created.ID[:6] // "bd-" + 3 chars
	req := authReq(http.MethodGet, "/api/v1/beads/"+prefix, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for prefix match, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.ID != created.ID {
		t.Fatalf("prefix resolve mismatch: got %s, want %s", b.ID, created.ID)
	}
}

func TestUpdateBead_Fields(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Original"})

	req := authReq(http.MethodPatch, "/api/v1/beads/"+created.ID, map[string]any{
		"title":    "Updated",
		"priority": "high",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.Title != "Updated" {
		t.Fatalf("title not updated: %q", b.Title)
	}
	if b.Priority != model.PriorityHigh {
		t.Fatalf("priority not updated: %q", b.Priority)
	}
}

func TestUpdateBead_Tags(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{
		"title": "Tag test",
		"tags":  []string{"alpha", "beta"},
	})

	// Add and remove tags
	req := authReq(http.MethodPatch, "/api/v1/beads/"+created.ID, map[string]any{
		"add_tags":    []string{"gamma"},
		"remove_tags": []string{"alpha"},
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	// Should have beta and gamma, but not alpha
	tagSet := make(map[string]bool)
	for _, tag := range b.Tags {
		tagSet[tag] = true
	}

	if tagSet["alpha"] {
		t.Fatal("alpha should have been removed")
	}
	if !tagSet["beta"] {
		t.Fatal("beta should still be present")
	}
	if !tagSet["gamma"] {
		t.Fatal("gamma should have been added")
	}
}

func TestUpdateBead_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPatch, "/api/v1/beads/bd-nonexistent", map[string]any{
		"title": "Ghost",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteBead(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Delete me"})

	req := authReq(http.MethodDelete, "/api/v1/beads/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.Status != model.StatusDeleted {
		t.Fatalf("expected status 'deleted', got %q", b.Status)
	}

	// Verify it's actually deleted in the store
	got, _ := srv.Store.Get(created.ID)
	if got.Status != model.StatusDeleted {
		t.Fatal("bead not actually soft-deleted in store")
	}
}

func TestDeleteBead_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodDelete, "/api/v1/beads/bd-nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateBead_UnblockedField(t *testing.T) {
	srv := crudServer(t)

	// Create blocker and blocked beads
	blocker := createViaAPI(t, srv, map[string]any{"title": "Blocker"})
	blocked := createViaAPI(t, srv, map[string]any{"title": "Blocked"})

	// Link them
	srv.Store.Link(blocked.ID, blocker.ID)

	// Close the blocker
	req := authReq(http.MethodPatch, "/api/v1/beads/"+blocker.ID, map[string]any{
		"status": "closed",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Decode the response - should have unblocked field
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	unblocked, ok := resp["unblocked"]
	if !ok {
		t.Fatal("expected 'unblocked' field in response")
	}

	unblockedList, ok := unblocked.([]any)
	if !ok {
		t.Fatalf("expected unblocked to be array, got %T", unblocked)
	}

	if len(unblockedList) != 1 {
		t.Fatalf("expected 1 unblocked bead, got %d", len(unblockedList))
	}
}

func TestDeleteBead_UnblockedField(t *testing.T) {
	srv := crudServer(t)

	// Create blocker and blocked beads
	blocker := createViaAPI(t, srv, map[string]any{"title": "Blocker"})
	blocked := createViaAPI(t, srv, map[string]any{"title": "Blocked"})

	// Link them
	srv.Store.Link(blocked.ID, blocker.ID)

	// Delete the blocker
	req := authReq(http.MethodDelete, "/api/v1/beads/"+blocker.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Decode the response - should have unblocked field
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	unblocked, ok := resp["unblocked"]
	if !ok {
		t.Fatal("expected 'unblocked' field in response")
	}

	unblockedList, ok := unblocked.([]any)
	if !ok {
		t.Fatalf("expected unblocked to be array, got %T", unblocked)
	}

	if len(unblockedList) != 1 {
		t.Fatalf("expected 1 unblocked bead, got %d", len(unblockedList))
	}
}
