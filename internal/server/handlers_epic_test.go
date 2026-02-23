package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vector76/beads_server/internal/model"
)

// --- Create with parent_id ---

func TestCreateBead_WithParentID(t *testing.T) {
	srv := crudServer(t)
	parent := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child task",
		"parent_id": parent.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	if child.ParentID != parent.ID {
		t.Errorf("expected parent_id %s, got %s", parent.ID, child.ParentID)
	}
}

func TestCreateBead_WithParentID_NotFound(t *testing.T) {
	srv := crudServer(t)

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Orphan",
		"parent_id": "bd-nonexistent",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateBead_WithParentID_TargetIsChild(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Create a child
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create child: expected 201, got %d", w.Code)
	}
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Try to create under the child (nesting)
	req = authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Grandchild",
		"parent_id": child.ID,
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GET bead detail with epic info ---

func TestGetBead_EpicDetail(t *testing.T) {
	srv := crudServer(t)
	parent := createViaAPI(t, srv, map[string]any{"title": "Auth rewrite", "type": "feature"})

	// Create children
	for _, title := range []string{"Design schema", "Write middleware", "Test endpoints"} {
		req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
			"title":     title,
			"parent_id": parent.ID,
		})
		w := httptest.NewRecorder()
		srv.Router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create child %q: expected 201, got %d", title, w.Code)
		}
	}

	// GET the epic
	req := authReq(http.MethodGet, "/api/v1/beads/"+parent.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["is_epic"] != true {
		t.Errorf("expected is_epic=true, got %v", resp["is_epic"])
	}

	progress, ok := resp["progress"].(map[string]any)
	if !ok {
		t.Fatal("expected progress object")
	}
	if progress["total"] != float64(3) {
		t.Errorf("expected progress.total=3, got %v", progress["total"])
	}
	if progress["open"] != float64(3) {
		t.Errorf("expected progress.open=3, got %v", progress["open"])
	}

	children, ok := resp["children"].([]any)
	if !ok {
		t.Fatal("expected children array")
	}
	if len(children) != 3 {
		t.Errorf("expected 3 children, got %d", len(children))
	}
}

func TestGetBead_ChildDetail(t *testing.T) {
	srv := crudServer(t)
	parent := createViaAPI(t, srv, map[string]any{"title": "My Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child task",
		"parent_id": parent.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// GET the child
	req = authReq(http.MethodGet, "/api/v1/beads/"+child.ID, nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["parent_id"] != parent.ID {
		t.Errorf("expected parent_id %s, got %v", parent.ID, resp["parent_id"])
	}
	if resp["parent_title"] != "My Epic" {
		t.Errorf("expected parent_title 'My Epic', got %v", resp["parent_title"])
	}
}

func TestGetBead_StandaloneNoEpicFields(t *testing.T) {
	srv := crudServer(t)
	bead := createViaAPI(t, srv, map[string]any{"title": "Standalone"})

	req := authReq(http.MethodGet, "/api/v1/beads/"+bead.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if _, ok := resp["is_epic"]; ok {
		t.Error("standalone bead should not have is_epic field")
	}
	if _, ok := resp["progress"]; ok {
		t.Error("standalone bead should not have progress field")
	}
	if _, ok := resp["children"]; ok {
		t.Error("standalone bead should not have children field")
	}
}

// --- Status change on epic rejected ---

func TestUpdateBead_StatusChangeOnEpicRejected(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Create a child to make it an epic
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Try to close the epic
	req = authReq(http.MethodPatch, "/api/v1/beads/"+epic.ID, map[string]any{
		"status": "closed",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateBead_NonStatusFieldsAllowedOnEpic(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Editing title should work
	req = authReq(http.MethodPatch, "/api/v1/beads/"+epic.ID, map[string]any{
		"title": "Updated Epic",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var b model.Bead
	json.NewDecoder(w.Body).Decode(&b)
	if b.Title != "Updated Epic" {
		t.Errorf("expected title 'Updated Epic', got %q", b.Title)
	}
}

// --- Claim on epic rejected ---

func TestClaimBead_EpicRejected(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Try to claim the epic
	req = authReq(http.MethodPost, "/api/v1/beads/"+epic.ID+"/claim", map[string]any{
		"user": "alice",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Delete on epic ---

func TestDeleteBead_EpicWithOpenChildrenRejected(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Open child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Try to delete the epic
	req = authReq(http.MethodDelete, "/api/v1/beads/"+epic.ID, nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteBead_EpicWithTerminalChildrenAllowed(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Close the child
	req = authReq(http.MethodPatch, "/api/v1/beads/"+child.ID, map[string]any{
		"status": "closed",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Delete the epic should succeed
	req = authReq(http.MethodDelete, "/api/v1/beads/"+epic.ID, nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Delete child recomputes parent ---

func TestDeleteBead_ChildRecomputesParent(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Only child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Delete the child (soft-delete)
	req = authReq(http.MethodDelete, "/api/v1/beads/"+child.ID, nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Epic should now be closed (only child is deleted = terminal)
	got, _ := srv.Store.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Errorf("expected epic status closed, got %s", got.Status)
	}
}

// --- Move via PATCH parent_id ---

func TestUpdateBead_MoveInto(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})
	bead := createViaAPI(t, srv, map[string]any{"title": "Standalone"})

	req := authReq(http.MethodPatch, "/api/v1/beads/"+bead.ID, map[string]any{
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.Bead
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, updated.ParentID)
	}
}

func TestUpdateBead_MoveOut(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Move out
	req = authReq(http.MethodPatch, "/api/v1/beads/"+child.ID, map[string]any{
		"parent_id": "",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.Bead
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.ParentID != "" {
		t.Errorf("expected empty parent_id, got %s", updated.ParentID)
	}
}

func TestUpdateBead_MoveIntoNestingRejected(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Give it a child
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Try to move the epic into another bead
	other := createViaAPI(t, srv, map[string]any{"title": "Other"})
	req = authReq(http.MethodPatch, "/api/v1/beads/"+epic.ID, map[string]any{
		"parent_id": other.ID,
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Link parent-child rejected ---

func TestLinkBead_ParentChildRejected(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Try to link child blocked by parent
	req = authReq(http.MethodPost, "/api/v1/beads/"+child.ID+"/link", map[string]any{
		"blocked_by": epic.ID,
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for parent-child link, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLinkBead_ParentBlockedByChildRejected(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Try to link parent blocked by child
	req = authReq(http.MethodPost, "/api/v1/beads/"+epic.ID+"/link", map[string]any{
		"blocked_by": child.ID,
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- List with epics ---

func TestListBeads_HierarchicalView(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Create two children
	for _, title := range []string{"C1", "C2"} {
		req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
			"title":     title,
			"parent_id": epic.ID,
		})
		w := httptest.NewRecorder()
		srv.Router.ServeHTTP(w, req)
	}
	standalone := createViaAPI(t, srv, map[string]any{"title": "Standalone"})
	_ = standalone

	req := authReq(http.MethodGet, "/api/v1/beads", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)

	// total should be 2 (epic + standalone, not 4)
	total := int(result["total"].(float64))
	if total != 2 {
		t.Fatalf("expected total=2, got %d", total)
	}

	beads := result["beads"].([]any)
	if len(beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(beads))
	}

	// Find the epic and check children
	for _, raw := range beads {
		b := raw.(map[string]any)
		if b["id"] == epic.ID {
			if b["is_epic"] != true {
				t.Error("expected is_epic=true")
			}
			children, ok := b["children"].([]any)
			if !ok {
				t.Fatal("expected children array")
			}
			if len(children) != 2 {
				t.Errorf("expected 2 children, got %d", len(children))
			}
			return
		}
	}
	t.Error("epic not found in results")
}

// --- List --ready via API ---

func TestListBeads_ReadyExcludesEpicsShowsChildContext(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Create a child
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Ready child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create child: expected 201, got %d", w.Code)
	}

	// List --ready
	req = authReq(http.MethodGet, "/api/v1/beads?ready=true", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)

	total := int(result["total"].(float64))
	if total != 1 {
		t.Fatalf("expected total=1 (child only), got %d", total)
	}

	beads := result["beads"].([]any)
	child := beads[0].(map[string]any)
	if child["parent_id"] != epic.ID {
		t.Errorf("expected parent_id %s, got %v", epic.ID, child["parent_id"])
	}
	if child["parent_title"] != "Epic" {
		t.Errorf("expected parent_title 'Epic', got %v", child["parent_title"])
	}
}

func TestListBeads_FilterByTypeTopLevelOnly(t *testing.T) {
	srv := crudServer(t)
	// Create a feature epic
	epic := createViaAPI(t, srv, map[string]any{"title": "Feature Epic", "type": "feature"})

	// Create a bug child
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Bug child",
		"type":      "bug",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Create standalone bug
	createViaAPI(t, srv, map[string]any{"title": "Standalone bug", "type": "bug"})

	// List type=bug should only show standalone bug, not the feature epic
	req = authReq(http.MethodGet, "/api/v1/beads?type=bug", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)

	total := int(result["total"].(float64))
	if total != 1 {
		t.Fatalf("expected total=1 (standalone bug only), got %d", total)
	}

	beads := result["beads"].([]any)
	b := beads[0].(map[string]any)
	if b["title"] != "Standalone bug" {
		t.Errorf("expected 'Standalone bug', got %v", b["title"])
	}
}

// --- Search via API with epic context ---

func TestSearch_ReturnsParentContext(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Auth Epic"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Fix login bug",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Search for "login"
	req = authReq(http.MethodGet, "/api/v1/search?q=login", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)

	total := int(result["total"].(float64))
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}

	beads := result["beads"].([]any)
	b := beads[0].(map[string]any)
	if b["parent_id"] != epic.ID {
		t.Errorf("expected parent_id %s, got %v", epic.ID, b["parent_id"])
	}
	if b["parent_title"] != "Auth Epic" {
		t.Errorf("expected parent_title 'Auth Epic', got %v", b["parent_title"])
	}
}

func TestSearch_EpicShowsIsEpicFlag(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Auth Rewrite"})

	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Subtask",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Search for "Auth" should find the epic
	req = authReq(http.MethodGet, "/api/v1/search?q=Auth", nil)
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)

	beads := result["beads"].([]any)
	if len(beads) != 1 {
		t.Fatalf("expected 1 result, got %d", len(beads))
	}

	b := beads[0].(map[string]any)
	if b["is_epic"] != true {
		t.Errorf("expected is_epic=true in search result, got %v", b["is_epic"])
	}
}

// --- Derived status through API ---

func TestDerivedStatus_ClosingAllChildren(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Create child
	req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
		"title":     "Only child",
		"parent_id": epic.ID,
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	var child model.Bead
	json.NewDecoder(w.Body).Decode(&child)

	// Close child
	req = authReq(http.MethodPatch, "/api/v1/beads/"+child.ID, map[string]any{
		"status": "closed",
	})
	w = httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Epic should be closed
	got, _ := srv.Store.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Errorf("expected epic status closed, got %s", got.Status)
	}
}

func TestDerivedStatus_MixedChildren(t *testing.T) {
	srv := crudServer(t)
	epic := createViaAPI(t, srv, map[string]any{"title": "Epic"})

	// Create two children
	var child1ID string
	for i, title := range []string{"C1", "C2"} {
		req := authReq(http.MethodPost, "/api/v1/beads", map[string]any{
			"title":     title,
			"parent_id": epic.ID,
		})
		w := httptest.NewRecorder()
		srv.Router.ServeHTTP(w, req)
		if i == 0 {
			var c model.Bead
			json.NewDecoder(w.Body).Decode(&c)
			child1ID = c.ID
		}
	}

	// Close first child only
	req := authReq(http.MethodPatch, "/api/v1/beads/"+child1ID, map[string]any{
		"status": "closed",
	})
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Epic should be open (one closed + one open child = open under derived-status rules)
	got, _ := srv.Store.Get(epic.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected epic status open, got %s", got.Status)
	}
}
