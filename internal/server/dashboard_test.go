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

	// Should render a <time> element with RFC3339 datetime attribute
	utcTime := created.UpdatedAt.UTC().Format(time.RFC3339)
	expectedAttr := `datetime="` + utcTime + `"`
	if !strings.Contains(body, expectedAttr) {
		t.Errorf("expected datetime attribute %q in dashboard HTML, got:\n%s", expectedAttr, body)
	}

	// The display text inside <time> should be YYYY-MM-DD HH:MM
	displayTime := created.UpdatedAt.UTC().Format("2006-01-02 15:04")
	expectedTag := `<time datetime="` + utcTime + `">` + displayTime + `</time>`
	if !strings.Contains(body, expectedTag) {
		t.Errorf("expected <time> element %q in dashboard HTML, got:\n%s", expectedTag, body)
	}
}

func TestDashboardContainsTimezoneScript(t *testing.T) {
	srv := crudServer(t)

	createViaAPI(t, srv, map[string]any{"title": "Script check", "status": "open"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `document.querySelectorAll("time[datetime]")`) {
		t.Error("expected inline timezone conversion script in dashboard HTML")
	}
}

func TestDashboardSortOrderInProgress(t *testing.T) {
	srv := crudServer(t)

	b1 := createViaAPI(t, srv, map[string]any{"title": "IP old", "assignee": "a"})
	patchStatus(t, srv, b1.ID, "in_progress")
	time.Sleep(10 * time.Millisecond)
	b2 := createViaAPI(t, srv, map[string]any{"title": "IP new", "assignee": "b"})
	patchStatus(t, srv, b2.ID, "in_progress")

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

	b1 := createViaAPI(t, srv, map[string]any{"title": "Closed old"})
	patchStatus(t, srv, b1.ID, string(model.StatusClosed))
	time.Sleep(10 * time.Millisecond)
	b2 := createViaAPI(t, srv, map[string]any{"title": "Closed new"})
	patchStatus(t, srv, b2.ID, string(model.StatusClosed))

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

func TestBeadDetailRendersFields(t *testing.T) {
	srv := crudServer(t)

	created := createViaAPI(t, srv, map[string]any{
		"title":       "Detail test bead",
		"description": "A detailed description\nwith multiple lines",
		"status":      "open",
		"priority":    "high",
		"type":        "bug",
		"assignee":    "alice",
		"tags":        []string{"urgent", "backend"},
	})

	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()

	checks := []string{
		created.ID,
		"Detail test bead",
		"A detailed description",
		"with multiple lines",
		"open",
		"high",
		"bug",
		"alice",
		"urgent",
		"backend",
		"Dashboard",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in detail page HTML", want)
		}
	}
}

func TestBeadDetailRendersComments(t *testing.T) {
	srv := crudServer(t)

	created := createViaAPI(t, srv, map[string]any{"title": "Comment test"})

	// Add a comment via API
	commentReq := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/comments",
		map[string]any{"author": "bob", "text": "This is a comment"})
	cw := httptest.NewRecorder()
	srv.Router.ServeHTTP(cw, commentReq)
	if cw.Code != http.StatusCreated {
		t.Fatalf("add comment: expected 201, got %d: %s", cw.Code, cw.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "bob") {
		t.Error("expected comment author 'bob' in detail page")
	}
	if !strings.Contains(body, "This is a comment") {
		t.Error("expected comment text in detail page")
	}
}

func TestBeadDetailRendersBlockers(t *testing.T) {
	srv := crudServer(t)

	blocker := createViaAPI(t, srv, map[string]any{"title": "Blocker bead", "status": "open"})
	blocked := createViaAPI(t, srv, map[string]any{"title": "Blocked bead", "status": "open"})

	// Link: blocked is blocked by blocker
	linkReq := authReq(http.MethodPost, "/api/v1/beads/"+blocked.ID+"/link",
		map[string]any{"blocked_by": blocker.ID})
	lw := httptest.NewRecorder()
	srv.Router.ServeHTTP(lw, linkReq)
	if lw.Code != http.StatusOK {
		t.Fatalf("link: expected 200, got %d: %s", lw.Code, lw.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+blocked.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Blocker bead") {
		t.Error("expected blocker title in detail page")
	}
	if !strings.Contains(body, blocker.ID) {
		t.Error("expected blocker ID in detail page")
	}
	if !strings.Contains(body, "Blocked By (Active)") {
		t.Error("expected 'Blocked By (Active)' section in detail page")
	}
}

func TestBeadDetailNotFoundBead(t *testing.T) {
	srv := crudServer(t)

	req := httptest.NewRequest(http.MethodGet, "/bead/default/bd-nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestBeadDetailNotFoundProject(t *testing.T) {
	srv := crudServer(t)

	req := httptest.NewRequest(http.MethodGet, "/bead/nonexistent/bd-1234", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDashboardBeadIDsAreLinks(t *testing.T) {
	srv := crudServer(t)

	created := createViaAPI(t, srv, map[string]any{"title": "Link test", "status": "open"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	expectedLink := `/bead/default/` + created.ID
	if !strings.Contains(body, expectedLink) {
		t.Errorf("expected link %q in dashboard HTML, got:\n%s", expectedLink, body)
	}
}

func TestBeadDetailUsesTimeElements(t *testing.T) {
	srv := crudServer(t)

	created := createViaAPI(t, srv, map[string]any{"title": "Time element test", "status": "open"})

	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()

	utcTime := created.UpdatedAt.UTC().Format(time.RFC3339)
	expectedAttr := `datetime="` + utcTime + `"`
	if !strings.Contains(body, expectedAttr) {
		t.Errorf("expected datetime attribute %q in detail page HTML", expectedAttr)
	}
}
