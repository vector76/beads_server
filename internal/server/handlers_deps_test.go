package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

// --- Comment tests ---

func TestAddComment_Success(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Commentable"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/comments", map[string]any{
		"author": "alice",
		"text":   "This looks good!",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if len(b.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(b.Comments))
	}
	if b.Comments[0].Author != "alice" {
		t.Fatalf("author mismatch: %q", b.Comments[0].Author)
	}
	if b.Comments[0].Text != "This looks good!" {
		t.Fatalf("text mismatch: %q", b.Comments[0].Text)
	}
	if b.Comments[0].CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
}

func TestAddComment_MissingAuthor(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Test"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/comments", map[string]any{
		"text": "No author",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddComment_MissingText(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Test"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/comments", map[string]any{
		"author": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddComment_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads/bd-nonexistent/comments", map[string]any{
		"author": "alice",
		"text":   "Ghost bead",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Link tests ---

func TestLinkBead_Success(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+a.ID+"/link", map[string]any{
		"blocked_by": b.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.Bead
	json.NewDecoder(w.Body).Decode(&updated)

	if len(updated.BlockedBy) != 1 || updated.BlockedBy[0] != b.ID {
		t.Fatalf("expected blocked_by [%s], got %v", b.ID, updated.BlockedBy)
	}
}

func TestLinkBead_CircularRejected(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})

	// A blocked by B
	req := authReq(http.MethodPost, "/api/v1/beads/"+a.ID+"/link", map[string]any{
		"blocked_by": b.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first link: expected 200, got %d", w.Code)
	}

	// B blocked by A — circular
	req = authReq(http.MethodPost, "/api/v1/beads/"+b.ID+"/link", map[string]any{
		"blocked_by": a.ID,
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for circular, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLinkBead_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads/bd-nonexistent/link", map[string]any{
		"blocked_by": "bd-other",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestLinkBead_MissingBlockedBy(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+a.ID+"/link", map[string]any{})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Unlink tests ---

func TestUnlinkBead_Success(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})

	// Link first
	srv.Store.Link(a.ID, b.ID)

	req := authReq(http.MethodDelete, "/api/v1/beads/"+a.ID+"/link/"+b.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.Bead
	json.NewDecoder(w.Body).Decode(&updated)

	if len(updated.BlockedBy) != 0 {
		t.Fatalf("expected empty blocked_by, got %v", updated.BlockedBy)
	}
}

func TestUnlinkBead_NotLinked(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})

	req := authReq(http.MethodDelete, "/api/v1/beads/"+a.ID+"/link/"+b.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Deps tests ---

func TestGetDeps_Success(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})
	c := createViaAPI(t, srv, map[string]any{"title": "C"})

	// A blocked by B (active) and C (will close)
	srv.Store.Link(a.ID, b.ID)
	srv.Store.Link(a.ID, c.ID)

	// Close C
	closed := model.StatusClosed
	srv.Store.Update(c.ID, store.UpdateFields{Status: &closed})

	req := authReq(http.MethodGet, "/api/v1/beads/"+a.ID+"/deps", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var deps store.DepsResult
	json.NewDecoder(w.Body).Decode(&deps)

	if len(deps.ActiveBlockers) != 1 {
		t.Fatalf("expected 1 active blocker, got %d", len(deps.ActiveBlockers))
	}
	if deps.ActiveBlockers[0].ID != b.ID {
		t.Fatalf("expected active blocker %s, got %s", b.ID, deps.ActiveBlockers[0].ID)
	}

	if len(deps.ResolvedBlockers) != 1 {
		t.Fatalf("expected 1 resolved blocker, got %d", len(deps.ResolvedBlockers))
	}
	if deps.ResolvedBlockers[0].ID != c.ID {
		t.Fatalf("expected resolved blocker %s, got %s", c.ID, deps.ResolvedBlockers[0].ID)
	}
}

func TestGetDeps_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodGet, "/api/v1/beads/bd-nonexistent/deps", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetDeps_Empty(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "Standalone"})

	req := authReq(http.MethodGet, "/api/v1/beads/"+a.ID+"/deps", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var deps store.DepsResult
	json.NewDecoder(w.Body).Decode(&deps)

	if len(deps.ActiveBlockers) != 0 {
		t.Fatalf("expected empty active blockers")
	}
	if len(deps.ResolvedBlockers) != 0 {
		t.Fatalf("expected empty resolved blockers")
	}
	if len(deps.Blocks) != 0 {
		t.Fatalf("expected empty blocks")
	}
}
