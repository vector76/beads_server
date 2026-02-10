package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

// --- List tests ---

func TestListBeads_Default(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Open bead"})
	createViaAPI(t, srv, map[string]any{"title": "In progress", "status": "in_progress"})
	createViaAPI(t, srv, map[string]any{"title": "Closed bead", "status": "closed"})

	req := authReq(http.MethodGet, "/api/v1/beads", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	// Default filter: open + in_progress only
	if result.Total != 2 {
		t.Fatalf("expected 2 beads (open + in_progress), got %d", result.Total)
	}
}

func TestListBeads_StatusFilter(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Open"})
	createViaAPI(t, srv, map[string]any{"title": "Closed", "status": "closed"})

	req := authReq(http.MethodGet, "/api/v1/beads?status=closed", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 1 {
		t.Fatalf("expected 1 closed bead, got %d", result.Total)
	}
	if result.Beads[0].Status != model.StatusClosed {
		t.Fatalf("expected closed status, got %s", result.Beads[0].Status)
	}
}

func TestListBeads_PriorityFilter(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "High", "priority": "high"})
	createViaAPI(t, srv, map[string]any{"title": "Low", "priority": "low"})

	req := authReq(http.MethodGet, "/api/v1/beads?priority=high", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 1 {
		t.Fatalf("expected 1 high-priority bead, got %d", result.Total)
	}
}

func TestListBeads_AllFlag(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Open"})
	createViaAPI(t, srv, map[string]any{"title": "Closed", "status": "closed"})
	createViaAPI(t, srv, map[string]any{"title": "Also closed", "status": "closed"})

	req := authReq(http.MethodGet, "/api/v1/beads?all=true", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 3 {
		t.Fatalf("expected 3 beads with all=true, got %d", result.Total)
	}
}

func TestListBeads_Pagination(t *testing.T) {
	srv := crudServer(t)
	for i := 0; i < 5; i++ {
		createViaAPI(t, srv, map[string]any{"title": fmt.Sprintf("Bead %d", i)})
	}

	req := authReq(http.MethodGet, "/api/v1/beads?per_page=2&page=1", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 5 {
		t.Fatalf("expected total=5, got %d", result.Total)
	}
	if len(result.Beads) != 2 {
		t.Fatalf("expected 2 beads on page, got %d", len(result.Beads))
	}
	if result.TotalPages != 3 {
		t.Fatalf("expected 3 total pages, got %d", result.TotalPages)
	}
	if result.Page != 1 {
		t.Fatalf("expected page=1, got %d", result.Page)
	}

	// Page 2
	req = authReq(http.MethodGet, "/api/v1/beads?per_page=2&page=2", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&result)
	if len(result.Beads) != 2 {
		t.Fatalf("expected 2 beads on page 2, got %d", len(result.Beads))
	}

	// Page 3 (last page)
	req = authReq(http.MethodGet, "/api/v1/beads?per_page=2&page=3", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&result)
	if len(result.Beads) != 1 {
		t.Fatalf("expected 1 bead on last page, got %d", len(result.Beads))
	}
}

func TestListBeads_TagFilter(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Tagged", "tags": []string{"backend"}})
	createViaAPI(t, srv, map[string]any{"title": "Untagged"})

	req := authReq(http.MethodGet, "/api/v1/beads?tag=backend", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 1 {
		t.Fatalf("expected 1 tagged bead, got %d", result.Total)
	}
}

// --- Search tests ---

func TestSearch_Success(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Fix login bug"})
	createViaAPI(t, srv, map[string]any{"title": "Add feature"})

	req := authReq(http.MethodGet, "/api/v1/search?q=login", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 1 {
		t.Fatalf("expected 1 result, got %d", result.Total)
	}
	if result.Beads[0].Title != "Fix login bug" {
		t.Fatalf("expected 'Fix login bug', got %q", result.Beads[0].Title)
	}
}

func TestSearch_MissingQuery(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodGet, "/api/v1/search", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearch_Pagination(t *testing.T) {
	srv := crudServer(t)
	for i := 0; i < 5; i++ {
		createViaAPI(t, srv, map[string]any{"title": fmt.Sprintf("Item %d", i)})
	}

	req := authReq(http.MethodGet, "/api/v1/search?q=Item&per_page=2&page=1", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result store.ListResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Total != 5 {
		t.Fatalf("expected total=5, got %d", result.Total)
	}
	if len(result.Beads) != 2 {
		t.Fatalf("expected 2 results on page, got %d", len(result.Beads))
	}
}

// --- Claim tests ---

func TestClaimBead_Success(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Claim me"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/claim", map[string]any{
		"user": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)

	if b.Status != model.StatusInProgress {
		t.Fatalf("expected status in_progress, got %q", b.Status)
	}
	if b.Assignee != "alice" {
		t.Fatalf("expected assignee 'alice', got %q", b.Assignee)
	}
}

func TestClaimBead_Conflict(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Contested"})

	// First claim succeeds
	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/claim", map[string]any{
		"user": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first claim: expected 200, got %d", w.Code)
	}

	// Second claim by different user returns 409
	req = authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/claim", map[string]any{
		"user": "bob",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClaimBead_Idempotent(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Idempotent"})

	// Claim twice with same user
	for i := 0; i < 2; i++ {
		req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/claim", map[string]any{
			"user": "alice",
		})
		w := httptest.NewRecorder()
		srv.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("claim %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestClaimBead_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads/bd-nonexistent/claim", map[string]any{
		"user": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestClaimBead_MissingUser(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "No user"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/claim", map[string]any{})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClaimBead_TerminalState(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Closed", "status": "closed"})

	req := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/claim", map[string]any{
		"user": "alice",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for terminal state, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Clean tests ---

func TestClean_DaysZeroRemovesAll(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Closed 1", "status": "closed"})
	createViaAPI(t, srv, map[string]any{"title": "Closed 2", "status": "closed"})
	createViaAPI(t, srv, map[string]any{"title": "Open stays"})

	req := authReq(http.MethodPost, "/api/v1/clean", map[string]any{"days": 0})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp cleanResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Removed != 2 {
		t.Fatalf("expected 2 removed, got %d", resp.Removed)
	}

	// Open bead should still exist
	all := srv.Store.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 remaining bead, got %d", len(all))
	}
}

func TestClean_NegativeDaysRejected(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/clean", map[string]any{"days": -1})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClean_KeepsRecentClosedBeads(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Just closed", "status": "closed"})

	req := authReq(http.MethodPost, "/api/v1/clean", map[string]any{"days": 5})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var resp cleanResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Removed != 0 {
		t.Fatalf("expected 0 removed, got %d", resp.Removed)
	}
}

func TestClean_RequiresAuth(t *testing.T) {
	srv := crudServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clean", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
