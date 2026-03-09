package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// assertBroadcast waits up to 500ms (longer than the ~200ms debounce) for a
// signal on ch, and fails the test if none arrives.
func assertBroadcast(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected broadcast signal, got none within 500ms")
	}
}

// assertNoBroadcast checks that no signal arrives within a short window.
func assertNoBroadcast(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("unexpected broadcast signal received")
	case <-time.After(50 * time.Millisecond):
	}
}

// drainBroadcast drains any pending signal from ch.
func drainBroadcast(ch chan struct{}) {
	select {
	case <-ch:
	default:
	}
}

// TestBroadcast_CreateBead verifies that a successful bead creation publishes
// a broadcast signal.
func TestBroadcast_CreateBead(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{"title": "hello"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_CreateBead_Error verifies that a failed create does not
// publish a broadcast.
func TestBroadcast_CreateBead_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	// Missing title — should return 400
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_UpdateBead_NormalPath verifies the normal (non-unblocked)
// success path publishes a broadcast.
func TestBroadcast_UpdateBead_NormalPath(t *testing.T) {
	srv := crudServer(t)
	bead := createViaAPI(t, srv, map[string]any{"title": "update-me"})

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPatch, "/api/v1/beads/"+bead.ID, map[string]any{
		"title": "updated-title",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_UpdateBead_UnblockedPath verifies the unblocked-response path
// publishes a broadcast.
func TestBroadcast_UpdateBead_UnblockedPath(t *testing.T) {
	srv := crudServer(t)

	// Create a blocker and a blocked bead.
	blocker := createViaAPI(t, srv, map[string]any{"title": "blocker"})
	blocked := createViaAPI(t, srv, map[string]any{
		"title":      "blocked",
		"blocked_by": []string{blocker.ID},
	})
	_ = blocked

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	// Close the blocker — this should produce an unblockedResponse.
	req := authReq(http.MethodPatch, "/api/v1/beads/"+blocker.ID, map[string]any{
		"status": "closed",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_UpdateBead_Error verifies that a failed update does not
// publish a broadcast.
func TestBroadcast_UpdateBead_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPatch, "/api/v1/beads/nonexistent-id", map[string]any{
		"title": "x",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_DeleteBead_NormalPath verifies the normal delete path
// publishes a broadcast.
func TestBroadcast_DeleteBead_NormalPath(t *testing.T) {
	srv := crudServer(t)
	bead := createViaAPI(t, srv, map[string]any{"title": "delete-me"})

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodDelete, "/api/v1/beads/"+bead.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_DeleteBead_UnblockedPath verifies that deleting a blocker
// (which produces an unblockedResponse) also publishes a broadcast.
func TestBroadcast_DeleteBead_UnblockedPath(t *testing.T) {
	srv := crudServer(t)

	blocker := createViaAPI(t, srv, map[string]any{"title": "blocker"})
	_ = createViaAPI(t, srv, map[string]any{
		"title":      "blocked",
		"blocked_by": []string{blocker.ID},
	})

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodDelete, "/api/v1/beads/"+blocker.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_DeleteBead_Error verifies a failed delete does not publish.
func TestBroadcast_DeleteBead_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodDelete, "/api/v1/beads/nonexistent-id", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_ClaimBead verifies a successful claim publishes a broadcast.
func TestBroadcast_ClaimBead(t *testing.T) {
	srv := crudServer(t)
	bead := createViaAPI(t, srv, map[string]any{"title": "claim-me"})

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/beads/"+bead.ID+"/claim", map[string]any{
		"user": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_ClaimBead_Error verifies a failed claim does not publish.
func TestBroadcast_ClaimBead_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/beads/nonexistent-id/claim", map[string]any{
		"user": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_Clean verifies that clean publishes a broadcast even when
// nothing is removed.
func TestBroadcast_Clean(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/clean", map[string]any{
		"days": 0.0,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_Clean_Error verifies that an invalid clean request does not
// publish a broadcast.
func TestBroadcast_Clean_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	// Negative days — should return 400
	req := authReq(http.MethodPost, "/api/v1/clean", map[string]any{
		"days": -1.0,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_AddComment verifies that adding a comment publishes a broadcast.
func TestBroadcast_AddComment(t *testing.T) {
	srv := crudServer(t)
	bead := createViaAPI(t, srv, map[string]any{"title": "comment-target"})

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/beads/"+bead.ID+"/comments", map[string]any{
		"author": "alice",
		"text":   "looks good",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_AddComment_Error verifies that a failed comment does not publish.
func TestBroadcast_AddComment_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/beads/nonexistent-id/comments", map[string]any{
		"author": "alice",
		"text":   "nope",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_LinkBead verifies that linking beads publishes a broadcast.
func TestBroadcast_LinkBead(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, fmt.Sprintf("/api/v1/beads/%s/link", a.ID), map[string]any{
		"blocked_by": b.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_LinkBead_Error verifies that a failed link does not publish.
func TestBroadcast_LinkBead_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodPost, "/api/v1/beads/nonexistent-id/link", map[string]any{
		"blocked_by": "also-nonexistent",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}

// TestBroadcast_UnlinkBead verifies that unlinking beads publishes a broadcast.
func TestBroadcast_UnlinkBead(t *testing.T) {
	srv := crudServer(t)
	a := createViaAPI(t, srv, map[string]any{"title": "A"})
	b := createViaAPI(t, srv, map[string]any{"title": "B"})

	// Link first
	linkReq := authReq(http.MethodPost, fmt.Sprintf("/api/v1/beads/%s/link", a.ID), map[string]any{
		"blocked_by": b.ID,
	})
	lw := httptest.NewRecorder()
	srv.Router.ServeHTTP(lw, linkReq)
	if lw.Code != http.StatusOK {
		t.Fatalf("link setup failed: %d %s", lw.Code, lw.Body.String())
	}

	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)
	drainBroadcast(ch) // drain any debounced signal from the link call above

	req := authReq(http.MethodDelete, fmt.Sprintf("/api/v1/beads/%s/link/%s", a.ID, b.ID), nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBroadcast(t, ch)
}

// TestBroadcast_UnlinkBead_Error verifies that a failed unlink does not publish.
func TestBroadcast_UnlinkBead_Error(t *testing.T) {
	srv := crudServer(t)
	ch := srv.broadcaster.subscribe()
	defer srv.broadcaster.unsubscribe(ch)

	req := authReq(http.MethodDelete, "/api/v1/beads/nonexistent-id/link/also-nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertNoBroadcast(t, ch)
}
