package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

func TestSortByUpdatedDesc(t *testing.T) {
	now := time.Now().UTC()
	beads := []store.BeadSummary{
		{ID: "bd-old", UpdatedAt: now.Add(-3 * time.Hour)},
		{ID: "bd-new", UpdatedAt: now},
		{ID: "bd-mid", UpdatedAt: now.Add(-1 * time.Hour)},
	}

	sortByUpdatedDesc(beads)

	expected := []string{"bd-new", "bd-mid", "bd-old"}
	for i, want := range expected {
		if beads[i].ID != want {
			t.Errorf("position %d: got %s, want %s", i, beads[i].ID, want)
		}
	}
}

func TestSortByUpdatedDesc_Empty(t *testing.T) {
	// Should not panic on empty or single-element slices.
	sortByUpdatedDesc(nil)
	sortByUpdatedDesc([]store.BeadSummary{})
	sortByUpdatedDesc([]store.BeadSummary{{ID: "bd-only"}})
}

func TestDashboardSortOrder(t *testing.T) {
	srv := crudServer(t)

	// Create beads with different statuses.
	// Create them in order so UpdatedAt differs.
	b1 := createViaAPI(t, srv, map[string]any{"title": "Open old", "status": "open"})
	time.Sleep(10 * time.Millisecond)
	b2 := createViaAPI(t, srv, map[string]any{"title": "Open new", "status": "open"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// "Open new" should appear before "Open old" in the HTML
	posNew := strings.Index(body, b2.Title)
	posOld := strings.Index(body, b1.Title)

	if posNew == -1 || posOld == -1 {
		t.Fatalf("expected both beads in HTML; got body:\n%s", body)
	}
	if posNew >= posOld {
		t.Errorf("expected %q (newer) before %q (older) in dashboard", b2.Title, b1.Title)
	}
}

func TestDashboardShowsUpdatedColumn(t *testing.T) {
	srv := crudServer(t)

	createViaAPI(t, srv, map[string]any{"title": "Check updated col", "status": "open"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<th>Updated</th>") {
		t.Error("expected 'Updated' column header in dashboard HTML")
	}
}

func TestDashboardRendersUpdateTime(t *testing.T) {
	srv := crudServer(t)

	created := createViaAPI(t, srv, map[string]any{"title": "Time display", "status": "open"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()

	// The formatted time should appear in YYYY-MM-DD HH:MM format
	expectedTime := created.UpdatedAt.Format("2006-01-02 15:04")
	if !strings.Contains(body, expectedTime) {
		t.Errorf("expected formatted time %q in dashboard HTML, got:\n%s", expectedTime, body)
	}
}

func TestDashboardSortOrderInProgress(t *testing.T) {
	srv := crudServer(t)

	b1 := createViaAPI(t, srv, map[string]any{"title": "IP old", "status": "in_progress", "assignee": "a"})
	time.Sleep(10 * time.Millisecond)
	b2 := createViaAPI(t, srv, map[string]any{"title": "IP new", "status": "in_progress", "assignee": "b"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	posNew := strings.Index(body, b2.Title)
	posOld := strings.Index(body, b1.Title)

	if posNew == -1 || posOld == -1 {
		t.Fatalf("expected both beads in HTML")
	}
	if posNew >= posOld {
		t.Errorf("expected %q (newer) before %q (older) in In Progress section", b2.Title, b1.Title)
	}
}

func TestListReturnsSummaryUpdatedAt(t *testing.T) {
	srv := crudServer(t)

	created := createViaAPI(t, srv, map[string]any{"title": "UpdatedAt check"})

	result := srv.Store.List(store.ListFilters{})
	if len(result.Beads) == 0 {
		t.Fatal("expected at least one bead")
	}

	var found *store.BeadSummary
	for i := range result.Beads {
		if result.Beads[i].ID == created.ID {
			found = &result.Beads[i]
			break
		}
	}
	if found == nil {
		t.Fatal("created bead not found in list")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be populated in BeadSummary")
	}

	// UpdatedAt should match what was returned on creation
	if !found.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("UpdatedAt mismatch: list=%v, created=%v", found.UpdatedAt, created.UpdatedAt)
	}
}

func TestDashboardSortOrderClosed(t *testing.T) {
	srv := crudServer(t)

	b1 := createViaAPI(t, srv, map[string]any{"title": "Closed old", "status": string(model.StatusClosed)})
	time.Sleep(10 * time.Millisecond)
	b2 := createViaAPI(t, srv, map[string]any{"title": "Closed new", "status": string(model.StatusClosed)})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	posNew := strings.Index(body, b2.Title)
	posOld := strings.Index(body, b1.Title)

	if posNew == -1 || posOld == -1 {
		t.Fatalf("expected both closed beads in HTML")
	}
	if posNew >= posOld {
		t.Errorf("expected %q (newer) before %q (older) in Closed section", b2.Title, b1.Title)
	}
}
