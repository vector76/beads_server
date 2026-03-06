package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vector76/beads_server/internal/model"
	"github.com/vector76/beads_server/internal/store"
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

func TestSortByDepthThenOldest(t *testing.T) {
	now := time.Now().UTC()
	beads := []store.BeadSummary{
		{ID: "bd-d1-new", BlockDepth: 1, CreatedAt: now},
		{ID: "bd-d0-old", BlockDepth: 0, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: "bd-d0-new", BlockDepth: 0, CreatedAt: now},
		{ID: "bd-d2", BlockDepth: 2, CreatedAt: now},
		{ID: "bd-d1-old", BlockDepth: 1, CreatedAt: now.Add(-1 * time.Hour)},
	}

	sortByDepthThenOldest(beads)

	expected := []string{"bd-d0-old", "bd-d0-new", "bd-d1-old", "bd-d1-new", "bd-d2"}
	for i, want := range expected {
		if beads[i].ID != want {
			t.Errorf("position %d: got %s, want %s", i, beads[i].ID, want)
		}
	}
}

func TestSortByDepthThenOldest_Empty(t *testing.T) {
	// Should not panic on empty or single-element slices.
	sortByDepthThenOldest(nil)
	sortByDepthThenOldest([]store.BeadSummary{})
	sortByDepthThenOldest([]store.BeadSummary{{ID: "bd-only"}})
}

func TestDashboardSortOrder(t *testing.T) {
	srv := crudServer(t)

	// Create two open beads with different creation times.
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

	// Open section sorts by depth (asc) then oldest-first.
	// Both beads are unblocked (depth 0), so "Open old" should appear before "Open new".
	posNew := strings.Index(body, b2.Title)
	posOld := strings.Index(body, b1.Title)

	if posNew == -1 || posOld == -1 {
		t.Fatalf("expected both beads in HTML; got body:\n%s", body)
	}
	if posOld >= posNew {
		t.Errorf("expected %q (older) before %q (newer) in dashboard Open section", b1.Title, b2.Title)
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

func TestDashboardShowsNotReadySection(t *testing.T) {
	srv := crudServer(t)

	createViaAPI(t, srv, map[string]any{"title": "NR bead", "status": "not_ready"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Not Ready") {
		t.Error("expected 'Not Ready' section header in dashboard HTML")
	}
	if !strings.Contains(body, "NR bead") {
		t.Error("expected not_ready bead title in dashboard HTML")
	}
}

func TestDashboardNotReadyCountInCounts(t *testing.T) {
	srv := crudServer(t)

	createViaAPI(t, srv, map[string]any{"title": "NR1", "status": "not_ready"})
	createViaAPI(t, srv, map[string]any{"title": "NR2", "status": "not_ready"})
	createViaAPI(t, srv, map[string]any{"title": "Open one"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "<strong>Not Ready:</strong> 2") {
		t.Errorf("expected 'Not Ready: 2' in counts, got body:\n%s", body)
	}
}

func TestDashboardSortOrderNotReady(t *testing.T) {
	srv := crudServer(t)

	b1 := createViaAPI(t, srv, map[string]any{"title": "NR old", "status": "not_ready"})
	time.Sleep(10 * time.Millisecond)
	b2 := createViaAPI(t, srv, map[string]any{"title": "NR new", "status": "not_ready"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	posNew := strings.Index(body, b2.Title)
	posOld := strings.Index(body, b1.Title)

	if posNew == -1 || posOld == -1 {
		t.Fatalf("expected both not_ready beads in HTML")
	}
	// Not Ready section sorts by depth (asc) then oldest-first.
	if posOld >= posNew {
		t.Errorf("expected %q (older) before %q (newer) in Not Ready section", b1.Title, b2.Title)
	}
}

// linkBeads sets blocked as blocked-by blocker via the API.
func linkBeads(t *testing.T, srv *Server, blockedID, blockerID string) {
	t.Helper()
	req := authReq(http.MethodPost, "/api/v1/beads/"+blockedID+"/link",
		map[string]any{"blocked_by": blockerID})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("link: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func getDashboard(t *testing.T, srv *Server) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d", w.Code)
	}
	return w.Body.String()
}

func TestDashboardBlockedIndicator_OpenSectionBlocked(t *testing.T) {
	srv := crudServer(t)

	blocker := createViaAPI(t, srv, map[string]any{"title": "Blocker open", "status": "open"})
	blocked := createViaAPI(t, srv, map[string]any{"title": "Blocked open bead", "status": "open"})
	linkBeads(t, srv, blocked.ID, blocker.ID)

	body := getDashboard(t, srv)

	// The lock emoji with depth must appear in the Open section row for the blocked bead.
	blockedRow := `<td style="white-space:nowrap; padding:0.2em 0.3em">🔒1</td><td><a href="/bead/default/` + blocked.ID + `">`
	if !strings.Contains(body, blockedRow) {
		t.Errorf("expected lock emoji with depth in Open section row for blocked bead, body:\n%s", body)
	}
}

func TestDashboardBlockedIndicator_OpenSectionUnblocked(t *testing.T) {
	srv := crudServer(t)

	createViaAPI(t, srv, map[string]any{"title": "Unblocked open bead", "status": "open"})

	body := getDashboard(t, srv)

	if strings.Contains(body, "🔒") {
		t.Errorf("expected no lock emoji in Open section for unblocked bead, body:\n%s", body)
	}
}

func TestDashboardBlockedIndicator_NotReadySectionBlocked(t *testing.T) {
	srv := crudServer(t)

	blocker := createViaAPI(t, srv, map[string]any{"title": "Blocker NR", "status": "open"})
	blocked := createViaAPI(t, srv, map[string]any{"title": "Blocked NR bead", "status": "not_ready"})
	linkBeads(t, srv, blocked.ID, blocker.ID)

	body := getDashboard(t, srv)

	blockedRow := `<td style="white-space:nowrap; padding:0.2em 0.3em">🔒1</td><td><a href="/bead/default/` + blocked.ID + `">`
	if !strings.Contains(body, blockedRow) {
		t.Errorf("expected lock emoji with depth in Not Ready section row for blocked bead, body:\n%s", body)
	}
}

func TestDashboardBlockedIndicator_NotReadySectionUnblocked(t *testing.T) {
	srv := crudServer(t)

	createViaAPI(t, srv, map[string]any{"title": "Unblocked NR bead", "status": "not_ready"})

	body := getDashboard(t, srv)

	if strings.Contains(body, "🔒") {
		t.Errorf("expected no lock emoji in Not Ready section for unblocked bead, body:\n%s", body)
	}
}

func TestDashboardBlockedIndicator_InProgressNoLock(t *testing.T) {
	srv := crudServer(t)

	blocker := createViaAPI(t, srv, map[string]any{"title": "IP Blocker", "status": "open"})
	target := createViaAPI(t, srv, map[string]any{"title": "IP target", "status": "open"})
	linkBeads(t, srv, target.ID, blocker.ID)
	patchStatus(t, srv, target.ID, "in_progress")

	body := getDashboard(t, srv)

	// The In Progress section must not contain the lock emoji.
	// Bound the section to the next <h3> to avoid including the Open section.
	ipStart := strings.Index(body, "<h3>In Progress</h3>")
	if ipStart == -1 {
		t.Fatal("In Progress section not found in dashboard")
	}
	nextH3 := strings.Index(body[ipStart+1:], "<h3>")
	ipSection := body[ipStart:]
	if nextH3 != -1 {
		ipSection = body[ipStart : ipStart+1+nextH3]
	}
	if strings.Contains(ipSection, "🔒") {
		t.Errorf("expected no lock emoji in In Progress section, got:\n%s", ipSection)
	}
}

func TestDashboardBlockedIndicator_ClosedNoLock(t *testing.T) {
	srv := crudServer(t)

	blocker := createViaAPI(t, srv, map[string]any{"title": "Closed blocker", "status": "open"})
	target := createViaAPI(t, srv, map[string]any{"title": "Closed target", "status": "open"})
	linkBeads(t, srv, target.ID, blocker.ID)
	patchStatus(t, srv, target.ID, string(model.StatusClosed))

	body := getDashboard(t, srv)

	closedStart := strings.Index(body, "<h3>Closed")
	if closedStart == -1 {
		t.Fatal("Closed section not found in dashboard")
	}
	closedSection := body[closedStart:]
	if strings.Contains(closedSection, "🔒") {
		t.Errorf("expected no lock emoji in Closed section, got:\n%s", closedSection)
	}
}

func TestDashboardCountsRowOrder(t *testing.T) {
	srv := crudServer(t)

	// Create one bead of each status so all four counts are non-zero.
	createViaAPI(t, srv, map[string]any{"title": "NR bead", "status": "not_ready"})
	createViaAPI(t, srv, map[string]any{"title": "Open bead", "status": "open"})
	ip := createViaAPI(t, srv, map[string]any{"title": "IP bead", "status": "open"})
	patchStatus(t, srv, ip.ID, "in_progress")
	cl := createViaAPI(t, srv, map[string]any{"title": "Closed bead", "status": "open"})
	patchStatus(t, srv, cl.ID, string(model.StatusClosed))

	body := getDashboard(t, srv)

	// Extract the <div class="counts">...</div> substring.
	// The counts div contains nested divs, so we find its closing by locating
	// the </summary> tag that follows it and then finding the last </div> before that.
	countsStart := strings.Index(body, `<div class="counts">`)
	if countsStart == -1 {
		t.Fatal(`counts div not found in dashboard body`)
	}
	summaryEnd := strings.Index(body[countsStart:], "</summary>")
	if summaryEnd == -1 {
		t.Fatal(`</summary> after counts div not found`)
	}
	segment := body[countsStart : countsStart+summaryEnd]
	countsEnd := strings.LastIndex(segment, "</div>")
	if countsEnd == -1 {
		t.Fatal(`closing </div> for counts not found`)
	}
	counts := segment[:countsEnd+len("</div>")]

	posNotReady := strings.Index(counts, "Not Ready")
	posOpen := strings.Index(counts, "Open")
	posInProgress := strings.Index(counts, "In Progress")
	posClosed := strings.Index(counts, "Closed")

	labels := []struct {
		name string
		pos  int
	}{
		{"Not Ready", posNotReady},
		{"Open", posOpen},
		{"In Progress", posInProgress},
		{"Closed", posClosed},
	}
	for _, l := range labels {
		if l.pos == -1 {
			t.Errorf("label %q not found in counts row: %s", l.name, counts)
		}
	}
	if posNotReady > posOpen {
		t.Errorf("expected Not Ready before Open in counts row; positions: Not Ready=%d Open=%d", posNotReady, posOpen)
	}
	if posOpen > posInProgress {
		t.Errorf("expected Open before In Progress in counts row; positions: Open=%d In Progress=%d", posOpen, posInProgress)
	}
	if posInProgress > posClosed {
		t.Errorf("expected In Progress before Closed in counts row; positions: In Progress=%d Closed=%d", posInProgress, posClosed)
	}
}

func TestBeadDetailRendersMarkdown(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{
		"title":       "Markdown test",
		"description": "# My Heading\n- list item\n`code span`",
	})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<h1>") {
		t.Error("expected <h1> element from markdown heading")
	}
	if !strings.Contains(body, "<li>") {
		t.Error("expected <li> element from markdown list")
	}
	if !strings.Contains(body, "<code>") {
		t.Error("expected <code> element from markdown code span")
	}
}

func TestBeadDetailXSSEscaped(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{
		"title":       "XSS test",
		"description": "<script>alert(1)</script>",
	})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	body := w.Body.String()
	if strings.Contains(body, "<script>alert") {
		t.Error("XSS: raw <script> tag must not appear unescaped in detail page")
	}
}

func TestBeadDetailNoPreWrapOnDescription(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{
		"title":       "Pre-wrap test",
		"description": "some text",
	})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	body := w.Body.String()
	if strings.Contains(body, "white-space: pre-wrap; background") {
		t.Error("description container must not use white-space: pre-wrap")
	}
}

func TestBeadDetailCommentsPreserveWhitespace(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Comment whitespace test"})
	commentReq := authReq(http.MethodPost, "/api/v1/beads/"+created.ID+"/comments",
		map[string]any{"author": "bob", "text": "line1\nline2\n<b>not bold</b>"})
	cw := httptest.NewRecorder()
	srv.Router.ServeHTTP(cw, commentReq)
	if cw.Code != http.StatusCreated {
		t.Fatalf("add comment: expected 201, got %d", cw.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "white-space: pre-wrap") {
		t.Error("expected white-space: pre-wrap for .comment-text rule")
	}
	if !strings.Contains(body, "line1") || !strings.Contains(body, "line2") {
		t.Error("expected comment lines to appear in body")
	}
	if !strings.Contains(body, "&lt;b&gt;") {
		t.Error("expected HTML in comment to be escaped, not rendered")
	}
}

func TestBeadDetailDashboardUnaffected(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "# Not A Heading", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "# Not A Heading") {
		t.Error("expected markdown-syntax title to appear as literal text in dashboard")
	}
}

func TestBeadDetailPlainTextDescription(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{
		"title":       "Plain text test",
		"description": "just plain text here",
	})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "just plain text here") {
		t.Error("expected plain description text to appear in detail page")
	}
}

func TestDashboardSectionHasBorder(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Border test", "status": "open"})
	body := getDashboard(t, srv)
	// "border: 1px solid var(--color-border); border-radius" is unique to the .section rule;
	// the th,td rule also has "border: 1px solid var(--color-border)" but no border-radius.
	if !strings.Contains(body, "border: 1px solid var(--color-border); border-radius") {
		t.Error("expected .section CSS to include a border with border-radius")
	}
}

func TestDashboardSectionIsCollapsible(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Collapsible test", "status": "open"})
	body := getDashboard(t, srv)
	if !strings.Contains(body, "<details") {
		t.Error("expected dashboard sections to use <details> element for collapsibility")
	}
	if !strings.Contains(body, "<summary>") {
		t.Error("expected dashboard sections to use <summary> element")
	}
}

func TestDashboardSectionExpandedByDefault(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Open by default test", "status": "open"})
	body := getDashboard(t, srv)
	if !strings.Contains(body, `<details class="section" open>`) {
		t.Error("expected dashboard sections to be open (expanded) by default")
	}
}

func TestDashboardSectionCollapseStatePersistedInLocalStorage(t *testing.T) {
	srv := crudServer(t)
	createViaAPI(t, srv, map[string]any{"title": "Persist collapse test", "status": "open"})
	body := getDashboard(t, srv)
	if !strings.Contains(body, `localStorage.getItem(key)`) {
		t.Error("expected dashboard to restore section state from localStorage on load")
	}
	if !strings.Contains(body, `localStorage.setItem(key`) {
		t.Error("expected dashboard to save section state to localStorage on toggle")
	}
	if !strings.Contains(body, `"section-open:"`) {
		t.Error("expected dashboard to use 'section-open:' key prefix for localStorage")
	}
}

func TestDashboardThemeCookieDark(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `<html data-theme="dark">`) {
		t.Error(`expected <html data-theme="dark"> when theme=dark cookie is set`)
	}
}

func TestDashboardThemeCookieLight(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `<html data-theme="light">`) {
		t.Error(`expected <html data-theme="light"> when theme=light cookie is set`)
	}
}

func TestDashboardThemeNoCookie(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "<html data-theme=") {
		t.Error("expected no data-theme attribute on <html> when no cookie is set")
	}
}

func TestDashboardThemeInvalidCookie(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "invalid"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "<html data-theme=") {
		t.Error("expected no data-theme attribute on <html> when cookie value is unrecognized")
	}
}

func TestBeadDetailThemeCookieDark(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Theme detail test", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `<html data-theme="dark">`) {
		t.Error(`expected <html data-theme="dark"> in bead detail response when theme=dark cookie is set`)
	}
}

func TestBeadDetailThemeNoCookie(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Theme detail no-cookie test", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "<html data-theme=") {
		t.Error("expected no data-theme attribute on <html> in bead detail response when no cookie is set")
	}
}

func TestDashboardToggleButtonPresent(t *testing.T) {
	srv := crudServer(t)
	body := getDashboard(t, srv)
	if !strings.Contains(body, `aria-label="Toggle dark mode"`) {
		t.Error("expected theme toggle button on dashboard page")
	}
}

func TestBeadDetailToggleButtonPresent(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Toggle button test", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `aria-label="Toggle dark mode"`) {
		t.Error("expected theme toggle button on bead detail page")
	}
}

func TestDashboardDarkModeIntegration(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `<html data-theme="dark">`) {
		t.Error(`expected <html data-theme="dark"> when theme=dark cookie is set`)
	}
	if !strings.Contains(body, `aria-label="Toggle dark mode"`) {
		t.Error("expected toggle button present on dashboard with dark cookie")
	}
}

func TestDashboardNoCookieIntegration(t *testing.T) {
	srv := crudServer(t)
	body := getDashboard(t, srv)
	if strings.Contains(body, "<html data-theme=") {
		t.Error("expected no server-set data-theme on <html> when no cookie is present")
	}
	if !strings.Contains(body, `aria-label="Toggle dark mode"`) {
		t.Error("expected toggle button present on dashboard with no cookie")
	}
}

func TestBeadDetailDarkModeIntegration(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Dark mode integration", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `<html data-theme="dark">`) {
		t.Error(`expected <html data-theme="dark"> on bead detail page when theme=dark cookie is set`)
	}
	if !strings.Contains(body, `aria-label="Toggle dark mode"`) {
		t.Error("expected toggle button present on bead detail page with dark cookie")
	}
}

func TestBeadDetailNoCookieIntegration(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "No cookie integration", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "<html data-theme=") {
		t.Error("expected no server-set data-theme on <html> on bead detail page when no cookie is present")
	}
	if !strings.Contains(body, `aria-label="Toggle dark mode"`) {
		t.Error("expected toggle button present on bead detail page with no cookie")
	}
}

func TestDashboardDepthShownNextToLock(t *testing.T) {
	srv := crudServer(t)

	// Create a chain: C blocks B blocks A → A has depth 2, B has depth 1
	c := createViaAPI(t, srv, map[string]any{"title": "Chain C", "status": "open"})
	b := createViaAPI(t, srv, map[string]any{"title": "Chain B", "status": "open"})
	a := createViaAPI(t, srv, map[string]any{"title": "Chain A", "status": "open"})
	linkBeads(t, srv, b.ID, c.ID)
	linkBeads(t, srv, a.ID, b.ID)

	body := getDashboard(t, srv)

	depth1Row := `<td style="white-space:nowrap; padding:0.2em 0.3em">🔒1</td><td><a href="/bead/default/` + b.ID + `">`
	if !strings.Contains(body, depth1Row) {
		t.Errorf("expected 🔒1 for depth-1 bead %s", b.ID)
	}

	depth2Row := `<td style="white-space:nowrap; padding:0.2em 0.3em">🔒2</td><td><a href="/bead/default/` + a.ID + `">`
	if !strings.Contains(body, depth2Row) {
		t.Errorf("expected 🔒2 for depth-2 bead %s", a.ID)
	}
}

func TestDashboardSortByDepthThenOldest(t *testing.T) {
	srv := crudServer(t)

	// Create: blocker (unblocked, depth 0), blocked (depth 1), another unblocked (depth 0, newer)
	blocker := createViaAPI(t, srv, map[string]any{"title": "Depth0 oldest", "status": "open"})
	time.Sleep(10 * time.Millisecond)
	blocked := createViaAPI(t, srv, map[string]any{"title": "Depth1 item", "status": "open"})
	linkBeads(t, srv, blocked.ID, blocker.ID)
	time.Sleep(10 * time.Millisecond)
	unblocked2 := createViaAPI(t, srv, map[string]any{"title": "Depth0 newest", "status": "open"})

	body := getDashboard(t, srv)

	// Expected order in Open section: Depth0 oldest, Depth0 newest, Depth1 item
	pos0old := strings.Index(body, blocker.Title)
	pos0new := strings.Index(body, unblocked2.Title)
	pos1 := strings.Index(body, blocked.Title)

	if pos0old == -1 || pos0new == -1 || pos1 == -1 {
		t.Fatalf("expected all three beads in HTML")
	}
	if pos0old >= pos0new {
		t.Errorf("expected %q (depth 0, older) before %q (depth 0, newer)", blocker.Title, unblocked2.Title)
	}
	if pos0new >= pos1 {
		t.Errorf("expected %q (depth 0) before %q (depth 1)", unblocked2.Title, blocked.Title)
	}
}

func TestDashboardToggleButtonIconDark(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "☀️") {
		t.Error("expected sun icon (☀️) on toggle button when theme=dark cookie is set")
	}
}

func TestDashboardToggleButtonIconLight(t *testing.T) {
	srv := crudServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "🌙") {
		t.Error("expected moon icon (🌙) on toggle button when theme=light cookie is set")
	}
}

func TestBeadDetailToggleButtonIconDark(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Icon dark test", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "☀️") {
		t.Error("expected sun icon (☀️) on toggle button when theme=dark cookie is set")
	}
}

func TestBeadDetailToggleButtonIconLight(t *testing.T) {
	srv := crudServer(t)
	created := createViaAPI(t, srv, map[string]any{"title": "Icon light test", "status": "open"})
	req := httptest.NewRequest(http.MethodGet, "/bead/default/"+created.ID, nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "🌙") {
		t.Error("expected moon icon (🌙) on toggle button when theme=light cookie is set")
	}
}
