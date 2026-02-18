package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/beads_server/internal/cli"
	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/server"
	"github.com/yourorg/beads_server/internal/store"
)

const testToken = "e2e-test-secret"

func startServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Load(filepath.Join(dir, "beads.json"))
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	p := server.NewSingleStoreProvider(testToken, s)
	srv, err := server.New(server.Config{Port: 0, DataFile: filepath.Join(dir, "beads.json"), LogOutput: io.Discard}, p)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	srv.Store = s
	ts := httptest.NewServer(srv.Router)
	t.Cleanup(ts.Close)
	return ts
}

func setEnv(t *testing.T, serverURL, user string) {
	t.Helper()
	os.Setenv("BS_URL", serverURL)
	os.Setenv("BS_TOKEN", testToken)
	os.Setenv("BS_USER", user)
	t.Cleanup(func() {
		os.Unsetenv("BS_URL")
		os.Unsetenv("BS_TOKEN")
		os.Unsetenv("BS_USER")
	})
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	cmd := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
	return buf.String()
}

func runExpectErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := cli.NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	return cmd.Execute()
}

func parseBead(t *testing.T, output string) model.Bead {
	t.Helper()
	var b model.Bead
	if err := json.Unmarshal([]byte(output), &b); err != nil {
		t.Fatalf("parse bead: %v\noutput: %s", err, output)
	}
	return b
}

func parseListResult(t *testing.T, output string) store.ListResult {
	t.Helper()
	var r store.ListResult
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		t.Fatalf("parse list result: %v\noutput: %s", err, output)
	}
	return r
}

func parseDeps(t *testing.T, output string) store.DepsResult {
	t.Helper()
	var d store.DepsResult
	if err := json.Unmarshal([]byte(output), &d); err != nil {
		t.Fatalf("parse deps: %v\noutput: %s", err, output)
	}
	return d
}

// TestFullLifecycle exercises the complete bead lifecycle through the CLI
// against a real server: create, list, show, edit, comment, claim, search,
// close, reopen, delete.
func TestFullLifecycle(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "e2e-agent")

	// 1. Create a bead
	out := run(t, "add", "E2E lifecycle bead", "--type", "bug", "--priority", "high",
		"--description", "End-to-end test bead", "--tags", "e2e,test")
	bead := parseBead(t, out)

	if bead.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if bead.Title != "E2E lifecycle bead" {
		t.Errorf("title = %q, want %q", bead.Title, "E2E lifecycle bead")
	}
	if bead.Type != model.TypeBug {
		t.Errorf("type = %q, want %q", bead.Type, model.TypeBug)
	}
	if bead.Priority != model.PriorityHigh {
		t.Errorf("priority = %q, want %q", bead.Priority, model.PriorityHigh)
	}
	if bead.Status != model.StatusOpen {
		t.Errorf("status = %q, want %q", bead.Status, model.StatusOpen)
	}

	// 2. List — should contain our bead
	out = run(t, "list")
	result := parseListResult(t, out)
	if result.Total != 1 {
		t.Errorf("list total = %d, want 1", result.Total)
	}

	// 3. Show by ID
	out = run(t, "show", bead.ID)
	shown := parseBead(t, out)
	if shown.ID != bead.ID {
		t.Errorf("show ID = %q, want %q", shown.ID, bead.ID)
	}

	// 4. Edit — change title and add tag
	out = run(t, "edit", bead.ID, "--title", "Updated E2E bead", "--add-tag", "updated")
	edited := parseBead(t, out)
	if edited.Title != "Updated E2E bead" {
		t.Errorf("edited title = %q, want %q", edited.Title, "Updated E2E bead")
	}

	// 5. Add a comment
	out = run(t, "comment", bead.ID, "E2E test comment")
	commented := parseBead(t, out)
	if len(commented.Comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(commented.Comments))
	}
	if commented.Comments[0].Author != "e2e-agent" {
		t.Errorf("comment author = %q, want %q", commented.Comments[0].Author, "e2e-agent")
	}

	// 6. Search
	out = run(t, "search", "Updated E2E")
	searchResult := parseListResult(t, out)
	if searchResult.Total != 1 {
		t.Errorf("search total = %d, want 1", searchResult.Total)
	}

	// 7. Claim
	out = run(t, "claim", bead.ID)
	claimed := parseBead(t, out)
	if claimed.Status != model.StatusInProgress {
		t.Errorf("claimed status = %q, want %q", claimed.Status, model.StatusInProgress)
	}
	if claimed.Assignee != "e2e-agent" {
		t.Errorf("claimed assignee = %q, want %q", claimed.Assignee, "e2e-agent")
	}

	// 8. Mine — should show our claimed bead
	out = run(t, "mine")
	mineResult := parseListResult(t, out)
	if mineResult.Total != 1 {
		t.Errorf("mine total = %d, want 1", mineResult.Total)
	}

	// 9. Close
	out = run(t, "close", bead.ID)
	closed := parseBead(t, out)
	if closed.Status != model.StatusClosed {
		t.Errorf("closed status = %q, want %q", closed.Status, model.StatusClosed)
	}

	// 10. Reopen
	out = run(t, "reopen", bead.ID)
	reopened := parseBead(t, out)
	if reopened.Status != model.StatusOpen {
		t.Errorf("reopened status = %q, want %q", reopened.Status, model.StatusOpen)
	}

	// 11. Delete (soft delete)
	out = run(t, "delete", bead.ID)
	deleted := parseBead(t, out)
	if deleted.Status != model.StatusDeleted {
		t.Errorf("deleted status = %q, want %q", deleted.Status, model.StatusDeleted)
	}

	// 12. List with --all should still include the deleted bead
	out = run(t, "list", "--all")
	allResult := parseListResult(t, out)
	if allResult.Total != 1 {
		t.Errorf("list --all total = %d, want 1", allResult.Total)
	}

	// 14. Default list should exclude deleted bead
	out = run(t, "list")
	defaultResult := parseListResult(t, out)
	if defaultResult.Total != 0 {
		t.Errorf("list (default) total = %d, want 0", defaultResult.Total)
	}
}

// TestCleanPurgesClosedBeads tests that clean removes closed/deleted beads
// and leaves open ones untouched.
func TestCleanPurgesClosedBeads(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "clean-agent")

	// Create beads in various states
	out := run(t, "add", "Open bead")
	openBead := parseBead(t, out)

	out = run(t, "add", "To close")
	toClose := parseBead(t, out)
	run(t, "close", toClose.ID)

	out = run(t, "add", "To delete")
	toDelete := parseBead(t, out)
	run(t, "delete", toDelete.ID)

	// Verify we have 3 beads total
	out = run(t, "list", "--all")
	allResult := parseListResult(t, out)
	if allResult.Total != 3 {
		t.Fatalf("expected 3 beads before clean, got %d", allResult.Total)
	}

	// Clean with --days 0 to remove all closed/deleted
	out = run(t, "clean", "--days", "0")
	var resp struct {
		Removed int `json:"removed"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse clean response: %v\noutput: %s", err, out)
	}
	if resp.Removed != 2 {
		t.Fatalf("expected 2 removed, got %d", resp.Removed)
	}

	// Only the open bead should remain
	out = run(t, "list", "--all")
	allResult = parseListResult(t, out)
	if allResult.Total != 1 {
		t.Fatalf("expected 1 bead after clean, got %d", allResult.Total)
	}
	if allResult.Beads[0].ID != openBead.ID {
		t.Errorf("remaining bead = %q, want %q", allResult.Beads[0].ID, openBead.ID)
	}

	// Cleaned beads should be gone (hard deleted)
	err := runExpectErr(t, "show", toClose.ID)
	if err == nil {
		t.Error("expected error showing cleaned bead, got nil")
	}
}

// TestMultiAgentConflict tests the scenario where two agents try to claim
// the same bead, verifying conflict handling.
func TestMultiAgentConflict(t *testing.T) {
	ts := startServer(t)

	// Agent 1 creates and claims a bead
	setEnv(t, ts.URL, "agent-alpha")
	out := run(t, "add", "Contested bead")
	bead := parseBead(t, out)

	out = run(t, "claim", bead.ID)
	claimed := parseBead(t, out)
	if claimed.Assignee != "agent-alpha" {
		t.Errorf("expected assignee agent-alpha, got %q", claimed.Assignee)
	}

	// Agent 1 claims again (idempotent) — should succeed
	out = run(t, "claim", bead.ID)
	reClaimed := parseBead(t, out)
	if reClaimed.Assignee != "agent-alpha" {
		t.Errorf("idempotent claim: expected agent-alpha, got %q", reClaimed.Assignee)
	}

	// Agent 2 tries to claim the same bead — should fail
	os.Setenv("BS_USER", "agent-beta")
	err := runExpectErr(t, "claim", bead.ID)
	if err == nil {
		t.Fatal("expected claim conflict error for agent-beta, got nil")
	}

	// Agent 1 closes the bead
	os.Setenv("BS_USER", "agent-alpha")
	run(t, "close", bead.ID)

	// Agent 2 tries to claim the closed bead — should fail (terminal state)
	os.Setenv("BS_USER", "agent-beta")
	err = runExpectErr(t, "claim", bead.ID)
	if err == nil {
		t.Fatal("expected error claiming closed bead, got nil")
	}

	// Create a second bead for agent-beta to claim successfully
	os.Setenv("BS_USER", "agent-alpha")
	out = run(t, "add", "Second bead")
	bead2 := parseBead(t, out)

	os.Setenv("BS_USER", "agent-beta")
	out = run(t, "claim", bead2.ID)
	claimed2 := parseBead(t, out)
	if claimed2.Assignee != "agent-beta" {
		t.Errorf("expected assignee agent-beta, got %q", claimed2.Assignee)
	}

	// Each agent's "mine" should show only their bead
	os.Setenv("BS_USER", "agent-beta")
	out = run(t, "mine")
	betaMine := parseListResult(t, out)
	if betaMine.Total != 1 {
		t.Errorf("agent-beta mine total = %d, want 1", betaMine.Total)
	}
	if betaMine.Beads[0].Title != "Second bead" {
		t.Errorf("agent-beta mine title = %q, want %q", betaMine.Beads[0].Title, "Second bead")
	}
}

// TestMultiProjectIsolation tests that two projects with different tokens
// have completely isolated data — beads created in one project are not
// visible in the other.
func TestMultiProjectIsolation(t *testing.T) {
	dir := t.TempDir()

	token1 := "tok-webapp"
	token2 := "tok-backend"

	s1, err := store.Load(filepath.Join(dir, "webapp.json"))
	if err != nil {
		t.Fatalf("store.Load s1: %v", err)
	}
	s2, err := store.Load(filepath.Join(dir, "backend.json"))
	if err != nil {
		t.Fatalf("store.Load s2: %v", err)
	}

	p := server.NewMultiStoreProvider([]server.ProviderEntry{
		{Name: "webapp", Token: token1, Store: s1},
		{Name: "backend", Token: token2, Store: s2},
	})
	srv, err := server.New(server.Config{Port: 0, LogOutput: io.Discard}, p)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(srv.Router)
	t.Cleanup(ts.Close)

	// Project 1: create a bead using token1
	os.Setenv("BS_URL", ts.URL)
	os.Setenv("BS_TOKEN", token1)
	os.Setenv("BS_USER", "webapp-dev")
	t.Cleanup(func() {
		os.Unsetenv("BS_URL")
		os.Unsetenv("BS_TOKEN")
		os.Unsetenv("BS_USER")
	})

	out := run(t, "add", "Webapp bug")
	webappBead := parseBead(t, out)
	if webappBead.Title != "Webapp bug" {
		t.Errorf("webapp bead title = %q, want %q", webappBead.Title, "Webapp bug")
	}

	// List with token1 — should see 1 bead
	out = run(t, "list")
	result := parseListResult(t, out)
	if result.Total != 1 {
		t.Fatalf("webapp list total = %d, want 1", result.Total)
	}

	// Switch to project 2
	os.Setenv("BS_TOKEN", token2)
	os.Setenv("BS_USER", "backend-dev")

	// List with token2 — should see 0 beads (isolated)
	out = run(t, "list")
	result = parseListResult(t, out)
	if result.Total != 0 {
		t.Fatalf("backend list total = %d, want 0 (should be isolated)", result.Total)
	}

	// Create a bead in project 2
	out = run(t, "add", "Backend task")
	backendBead := parseBead(t, out)
	if backendBead.Title != "Backend task" {
		t.Errorf("backend bead title = %q, want %q", backendBead.Title, "Backend task")
	}

	// List with token2 — should see 1 bead
	out = run(t, "list")
	result = parseListResult(t, out)
	if result.Total != 1 {
		t.Fatalf("backend list total = %d, want 1", result.Total)
	}

	// Switch back to project 1 — should still see only 1 bead
	os.Setenv("BS_TOKEN", token1)
	os.Setenv("BS_USER", "webapp-dev")

	out = run(t, "list")
	result = parseListResult(t, out)
	if result.Total != 1 {
		t.Fatalf("webapp list total after backend add = %d, want 1", result.Total)
	}
	if result.Beads[0].Title != "Webapp bug" {
		t.Errorf("webapp bead title = %q, want %q", result.Beads[0].Title, "Webapp bug")
	}
}

// TestEpicLifecycle exercises the full epic lifecycle: create parent, add children,
// close children, verify derived status, move in/out, and delete.
func TestEpicLifecycle(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "epic-agent")

	// 1. Create parent bead (will become an epic)
	out := run(t, "add", "Auth rewrite", "--type", "feature", "--priority", "high")
	epic := parseBead(t, out)

	// 2. Create children using --parent
	out = run(t, "add", "Design token schema", "--parent", epic.ID)
	child1 := parseBead(t, out)
	if child1.ParentID != epic.ID {
		t.Fatalf("child1 parent_id = %q, want %q", child1.ParentID, epic.ID)
	}

	out = run(t, "add", "Write middleware", "--parent", epic.ID)
	child2 := parseBead(t, out)

	// 3. Show epic — should have progress and children
	out = run(t, "show", epic.ID)
	var epicDetail map[string]any
	json.Unmarshal([]byte(out), &epicDetail)
	if epicDetail["is_epic"] != true {
		t.Error("expected is_epic=true")
	}
	progress := epicDetail["progress"].(map[string]any)
	if progress["total"] != float64(2) {
		t.Errorf("expected progress.total=2, got %v", progress["total"])
	}

	// 4. Show child — should have parent context
	out = run(t, "show", child1.ID)
	var childDetail map[string]any
	json.Unmarshal([]byte(out), &childDetail)
	if childDetail["parent_title"] != "Auth rewrite" {
		t.Errorf("expected parent_title 'Auth rewrite', got %v", childDetail["parent_title"])
	}

	// 5. List — children should be nested under epic
	out = run(t, "list")
	var listResult map[string]any
	json.Unmarshal([]byte(out), &listResult)
	total := int(listResult["total"].(float64))
	if total != 1 {
		t.Fatalf("expected 1 top-level (epic only), got %d", total)
	}

	// 6. List --ready — should show children (not epic)
	out = run(t, "list", "--ready")
	readyResult := parseListResult(t, out)
	if readyResult.Total != 2 {
		t.Fatalf("expected 2 ready children, got %d", readyResult.Total)
	}

	// 7. Claim on epic should fail
	err := runExpectErr(t, "claim", epic.ID)
	if err == nil {
		t.Fatal("expected error claiming epic")
	}

	// 8. Close on epic should fail
	err = runExpectErr(t, "close", epic.ID)
	if err == nil {
		t.Fatal("expected error closing epic")
	}

	// 9. Close child1 — epic should become open (mixed: closed + open = open)
	run(t, "close", child1.ID)
	out = run(t, "show", epic.ID)
	json.Unmarshal([]byte(out), &epicDetail)
	if epicDetail["status"] != "open" {
		t.Errorf("expected epic status open after closing one child, got %v", epicDetail["status"])
	}

	// 10. Close child2 — epic should become closed
	run(t, "close", child2.ID)
	out = run(t, "show", epic.ID)
	json.Unmarshal([]byte(out), &epicDetail)
	if epicDetail["status"] != "closed" {
		t.Errorf("expected epic status closed after closing all children, got %v", epicDetail["status"])
	}

	// 11. Move a standalone bead into the epic
	out = run(t, "add", "Late addition")
	lateBead := parseBead(t, out)
	out = run(t, "move", lateBead.ID, "--into", epic.ID)
	moved := parseBead(t, out)
	if moved.ParentID != epic.ID {
		t.Errorf("expected parent_id %s after move, got %s", epic.ID, moved.ParentID)
	}

	// Epic should reopen (new open child among closed children = open)
	out = run(t, "show", epic.ID)
	json.Unmarshal([]byte(out), &epicDetail)
	if epicDetail["status"] != "open" {
		t.Errorf("expected epic status open after adding child, got %v", epicDetail["status"])
	}

	// 12. Move out
	out = run(t, "move", lateBead.ID, "--out")
	movedOut := parseBead(t, out)
	if movedOut.ParentID != "" {
		t.Errorf("expected empty parent_id after move out, got %s", movedOut.ParentID)
	}

	// Epic should be closed again (only terminal children remain)
	out = run(t, "show", epic.ID)
	json.Unmarshal([]byte(out), &epicDetail)
	if epicDetail["status"] != "closed" {
		t.Errorf("expected epic status closed after removing open child, got %v", epicDetail["status"])
	}

	// 13. Delete epic (all children are terminal)
	out = run(t, "delete", epic.ID)
	deletedEpic := parseBead(t, out)
	if deletedEpic.Status != "deleted" {
		t.Errorf("expected deleted status, got %s", deletedEpic.Status)
	}
}

// TestEpicQueryPaths exercises the epic-aware read paths: search with parent context,
// list --ready with inherited blockers, and mine with parent context.
func TestEpicQueryPaths(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "query-agent")

	// Create an epic with children
	out := run(t, "add", "Auth Rewrite Epic", "--type", "feature")
	epic := parseBead(t, out)

	out = run(t, "add", "Fix login bug", "--parent", epic.ID)
	child1 := parseBead(t, out)

	out = run(t, "add", "Add token refresh", "--parent", epic.ID)
	child2 := parseBead(t, out)

	// 1. Search for child — should include parent context
	out = run(t, "search", "login")
	searchResult := parseListResult(t, out)
	if searchResult.Total != 1 {
		t.Fatalf("search: expected 1 result, got %d", searchResult.Total)
	}
	if searchResult.Beads[0].ParentID != epic.ID {
		t.Errorf("search: expected parent_id %s, got %s", epic.ID, searchResult.Beads[0].ParentID)
	}
	if searchResult.Beads[0].ParentTitle != "Auth Rewrite Epic" {
		t.Errorf("search: expected parent_title 'Auth Rewrite Epic', got %q", searchResult.Beads[0].ParentTitle)
	}

	// 2. Search for epic — should show is_epic flag
	out = run(t, "search", "Auth Rewrite")
	searchResult = parseListResult(t, out)
	if searchResult.Total != 1 {
		t.Fatalf("search epic: expected 1 result, got %d", searchResult.Total)
	}
	if !searchResult.Beads[0].IsEpic {
		t.Error("search epic: expected is_epic=true")
	}

	// 3. List --ready with inherited blockers
	// Create a blocker and block the epic
	out = run(t, "add", "External blocker")
	blocker := parseBead(t, out)
	run(t, "link", epic.ID, "--blocked-by", blocker.ID)

	// Children should not be ready since parent epic is blocked
	out = run(t, "list", "--ready")
	readyResult := parseListResult(t, out)
	for _, b := range readyResult.Beads {
		if b.ID == child1.ID || b.ID == child2.ID {
			t.Errorf("child %s should not be ready while parent epic is blocked", b.ID)
		}
	}

	// Close the blocker — children should become ready
	run(t, "close", blocker.ID)

	out = run(t, "list", "--ready")
	readyResult = parseListResult(t, out)
	if readyResult.Total != 2 {
		t.Fatalf("after unblocking: expected 2 ready children, got %d", readyResult.Total)
	}

	// 4. Mine with parent context
	run(t, "claim", child1.ID)
	out = run(t, "mine")
	mineResult := parseListResult(t, out)
	if mineResult.Total != 1 {
		t.Fatalf("mine: expected 1, got %d", mineResult.Total)
	}
	if mineResult.Beads[0].ID != child1.ID {
		t.Errorf("mine: expected child %s, got %s", child1.ID, mineResult.Beads[0].ID)
	}
	if mineResult.Beads[0].ParentTitle != "Auth Rewrite Epic" {
		t.Errorf("mine: expected parent_title 'Auth Rewrite Epic', got %q", mineResult.Beads[0].ParentTitle)
	}

	_ = child2
}

// TestEpicClean exercises epic-aware cleaning through the CLI: unit-based
// cleaning removes a closed epic with all its children as a unit, partial
// epics are never cleaned, and the age threshold uses max updated_at across
// the unit.
func TestEpicClean(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "clean-agent")

	// --- Setup: create two epics ---

	// Epic A: fully closed (both children closed)
	out := run(t, "add", "Completed Epic")
	epicA := parseBead(t, out)
	out = run(t, "add", "Done task 1", "--parent", epicA.ID)
	childA1 := parseBead(t, out)
	out = run(t, "add", "Done task 2", "--parent", epicA.ID)
	childA2 := parseBead(t, out)
	run(t, "close", childA1.ID)
	run(t, "close", childA2.ID)

	// Verify epic A is closed (derived)
	out = run(t, "show", epicA.ID)
	var detail map[string]any
	json.Unmarshal([]byte(out), &detail)
	if detail["status"] != "closed" {
		t.Fatalf("epicA: expected status closed, got %v", detail["status"])
	}

	// Epic B: partially closed (one child open, one closed)
	out = run(t, "add", "In Progress Epic")
	epicB := parseBead(t, out)
	out = run(t, "add", "Open task", "--parent", epicB.ID)
	_ = parseBead(t, out) // childB1 - stays open
	out = run(t, "add", "Closed task", "--parent", epicB.ID)
	childB2 := parseBead(t, out)
	run(t, "close", childB2.ID)

	// Verify epic B is open (derived: mixed children — closed + open = open)
	out = run(t, "show", epicB.ID)
	json.Unmarshal([]byte(out), &detail)
	if detail["status"] != "open" {
		t.Fatalf("epicB: expected status open, got %v", detail["status"])
	}

	// Also create a standalone closed bead
	out = run(t, "add", "Standalone closed")
	standalone := parseBead(t, out)
	run(t, "close", standalone.ID)

	// --- Verify totals before clean ---
	out = run(t, "list", "--all")
	allResult := parseListResult(t, out)
	// Top-level: epicA + epicB + standalone = 3
	if allResult.Total != 3 {
		t.Fatalf("before clean: expected 3 top-level, got %d", allResult.Total)
	}

	// --- Clean with --days 0 ---
	out = run(t, "clean", "--days", "0")
	var resp struct {
		Removed int `json:"removed"`
	}
	json.Unmarshal([]byte(out), &resp)

	// Expected: epicA (1) + its 2 children (2) + standalone (1) = 4
	// Epic B and its children should NOT be cleaned (partial, open — has open child)
	if resp.Removed != 4 {
		t.Fatalf("expected 4 removed (epicA unit + standalone), got %d", resp.Removed)
	}

	// --- Verify what remains ---
	out = run(t, "list", "--all")
	allResult = parseListResult(t, out)
	// Only epicB should remain
	if allResult.Total != 1 {
		t.Fatalf("after clean: expected 1 top-level (epicB), got %d", allResult.Total)
	}
	if allResult.Beads[0].ID != epicB.ID {
		t.Errorf("remaining bead should be epicB %s, got %s", epicB.ID, allResult.Beads[0].ID)
	}

	// Verify epicA is gone (hard deleted)
	err := runExpectErr(t, "show", epicA.ID)
	if err == nil {
		t.Error("expected error showing cleaned epicA")
	}

	// Verify epicB's children are intact
	out = run(t, "show", epicB.ID)
	json.Unmarshal([]byte(out), &detail)
	children := detail["children"].([]any)
	// 1 child shown (deleted excluded from show children, but the closed child
	// is still there since it wasn't cleaned — it's part of an in-progress epic)
	if len(children) != 2 {
		t.Errorf("epicB should still have 2 children, got %d", len(children))
	}
}

// TestEpicFullLifecycleWithClean exercises a comprehensive epic lifecycle:
// create, add children, work through states querying at each stage, and clean.
func TestEpicFullLifecycleWithClean(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "lifecycle-agent")

	// === Phase 1: Creation ===
	out := run(t, "add", "API Redesign", "--type", "feature", "--priority", "high",
		"--description", "Redesign the REST API with versioning")
	epic := parseBead(t, out)

	out = run(t, "add", "Define new endpoints", "--parent", epic.ID)
	child1 := parseBead(t, out)
	out = run(t, "add", "Update client SDK", "--parent", epic.ID)
	child2 := parseBead(t, out)
	out = run(t, "add", "Write migration guide", "--parent", epic.ID)
	child3 := parseBead(t, out)

	// Query: show epic has 3 children, all open
	out = run(t, "show", epic.ID)
	var detail map[string]any
	json.Unmarshal([]byte(out), &detail)
	if detail["status"] != "open" {
		t.Errorf("phase 1: expected epic status open, got %v", detail["status"])
	}
	progress := detail["progress"].(map[string]any)
	if progress["total"] != float64(3) {
		t.Errorf("phase 1: expected 3 children, got %v", progress["total"])
	}
	if progress["open"] != float64(3) {
		t.Errorf("phase 1: expected 3 open, got %v", progress["open"])
	}

	// Query: list shows 1 top-level item (the epic)
	out = run(t, "list")
	var listMap map[string]any
	json.Unmarshal([]byte(out), &listMap)
	if int(listMap["total"].(float64)) != 1 {
		t.Errorf("phase 1: expected 1 top-level, got %v", listMap["total"])
	}

	// Query: list --ready shows 3 children
	out = run(t, "list", "--ready")
	readyResult := parseListResult(t, out)
	if readyResult.Total != 3 {
		t.Errorf("phase 1: expected 3 ready children, got %d", readyResult.Total)
	}

	// === Phase 2: Work in progress ===
	run(t, "claim", child1.ID)

	// Query: mine shows 1 item with parent context
	out = run(t, "mine")
	mineResult := parseListResult(t, out)
	if mineResult.Total != 1 {
		t.Fatalf("phase 2: expected 1 mine result, got %d", mineResult.Total)
	}
	if mineResult.Beads[0].ParentTitle != "API Redesign" {
		t.Errorf("phase 2: expected parent_title 'API Redesign', got %q", mineResult.Beads[0].ParentTitle)
	}

	// Close child1
	run(t, "close", child1.ID)

	// Query: epic should be open (mixed: 1 closed, 2 open — closed + open = open)
	out = run(t, "show", epic.ID)
	json.Unmarshal([]byte(out), &detail)
	if detail["status"] != "open" {
		t.Errorf("phase 2: expected epic status open, got %v", detail["status"])
	}
	progress = detail["progress"].(map[string]any)
	if progress["closed"] != float64(1) {
		t.Errorf("phase 2: expected 1 closed, got %v", progress["closed"])
	}

	// Query: list --ready shows 2 remaining open children
	out = run(t, "list", "--ready")
	readyResult = parseListResult(t, out)
	if readyResult.Total != 2 {
		t.Errorf("phase 2: expected 2 ready children, got %d", readyResult.Total)
	}

	// === Phase 3: Close remaining children ===
	run(t, "claim", child2.ID)
	run(t, "close", child2.ID)
	run(t, "claim", child3.ID)
	run(t, "close", child3.ID)

	// Query: epic should be closed (all children terminal)
	out = run(t, "show", epic.ID)
	json.Unmarshal([]byte(out), &detail)
	if detail["status"] != "closed" {
		t.Errorf("phase 3: expected epic status closed, got %v", detail["status"])
	}
	progress = detail["progress"].(map[string]any)
	if progress["closed"] != float64(3) {
		t.Errorf("phase 3: expected 3 closed, got %v", progress["closed"])
	}

	// Query: list --ready shows 0 (all done)
	out = run(t, "list", "--ready")
	readyResult = parseListResult(t, out)
	if readyResult.Total != 0 {
		t.Errorf("phase 3: expected 0 ready, got %d", readyResult.Total)
	}

	// Query: default list shows 0 (epic is closed, default excludes closed)
	out = run(t, "list")
	json.Unmarshal([]byte(out), &listMap)
	if int(listMap["total"].(float64)) != 0 {
		t.Errorf("phase 3: expected 0 in default list (closed excluded), got %v", listMap["total"])
	}

	// Query: list --all shows 1 (the closed epic, children nested)
	out = run(t, "list", "--all")
	allResult := parseListResult(t, out)
	if allResult.Total != 1 {
		t.Errorf("phase 3: expected 1 in --all list, got %d", allResult.Total)
	}

	// Query: search still finds it
	out = run(t, "search", "API Redesign")
	searchResult := parseListResult(t, out)
	if searchResult.Total != 1 {
		t.Errorf("phase 3: search should find closed epic, got %d", searchResult.Total)
	}
	if !searchResult.Beads[0].IsEpic {
		t.Error("phase 3: search result should have is_epic=true")
	}

	// === Phase 4: Clean ===
	out = run(t, "clean", "--days", "0")
	var resp struct {
		Removed int `json:"removed"`
	}
	json.Unmarshal([]byte(out), &resp)

	// Epic + 3 children = 4
	if resp.Removed != 4 {
		t.Fatalf("phase 4: expected 4 removed (epic + 3 children), got %d", resp.Removed)
	}

	// Query: everything is gone
	out = run(t, "list", "--all")
	allResult = parseListResult(t, out)
	if allResult.Total != 0 {
		t.Errorf("phase 4: expected 0 after clean, got %d", allResult.Total)
	}

	// Verify hard deletion
	err := runExpectErr(t, "show", epic.ID)
	if err == nil {
		t.Error("phase 4: expected error showing cleaned epic")
	}
	err = runExpectErr(t, "show", child1.ID)
	if err == nil {
		t.Error("phase 4: expected error showing cleaned child")
	}
}

// TestDependencyChain tests creating a dependency chain, resolving blockers,
// and verifying the unblocked computation.
func TestDependencyChain(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "dep-agent")

	// Create a chain: C is blocked by B, B is blocked by A
	// So A must be closed first, then B, then C becomes unblocked.
	outA := run(t, "add", "Task A - foundation")
	beadA := parseBead(t, outA)

	outB := run(t, "add", "Task B - middle")
	beadB := parseBead(t, outB)

	outC := run(t, "add", "Task C - final")
	beadC := parseBead(t, outC)

	// Link: B blocked by A
	run(t, "link", beadB.ID, "--blocked-by", beadA.ID)

	// Link: C blocked by B
	run(t, "link", beadC.ID, "--blocked-by", beadB.ID)

	// Verify deps of B: A is active blocker, C is in "blocks"
	out := run(t, "deps", beadB.ID)
	depsB := parseDeps(t, out)
	if len(depsB.ActiveBlockers) != 1 {
		t.Fatalf("B active_blockers = %d, want 1", len(depsB.ActiveBlockers))
	}
	if depsB.ActiveBlockers[0].ID != beadA.ID {
		t.Errorf("B active_blockers[0].id = %q, want %q", depsB.ActiveBlockers[0].ID, beadA.ID)
	}
	if len(depsB.Blocks) != 1 {
		t.Fatalf("B blocks = %d, want 1", len(depsB.Blocks))
	}
	if depsB.Blocks[0].ID != beadC.ID {
		t.Errorf("B blocks[0].id = %q, want %q", depsB.Blocks[0].ID, beadC.ID)
	}

	// Verify deps of C: B is active blocker
	out = run(t, "deps", beadC.ID)
	depsC := parseDeps(t, out)
	if len(depsC.ActiveBlockers) != 1 {
		t.Fatalf("C active_blockers = %d, want 1", len(depsC.ActiveBlockers))
	}
	if depsC.ActiveBlockers[0].ID != beadB.ID {
		t.Errorf("C active_blockers[0].id = %q, want %q", depsC.ActiveBlockers[0].ID, beadB.ID)
	}

	// List --ready should show only A (no active blockers)
	out = run(t, "list", "--ready")
	readyResult := parseListResult(t, out)
	if readyResult.Total != 1 {
		t.Errorf("ready list total = %d, want 1", readyResult.Total)
	}
	if readyResult.Beads[0].ID != beadA.ID {
		t.Errorf("ready bead = %q, want %q", readyResult.Beads[0].ID, beadA.ID)
	}

	// Close A — this should unblock B
	out = run(t, "close", beadA.ID)
	// The response may include "unblocked" field; we just need to verify
	// B is now unblocked via deps check
	out = run(t, "deps", beadB.ID)
	depsB = parseDeps(t, out)
	if len(depsB.ActiveBlockers) != 0 {
		t.Errorf("after closing A, B active_blockers = %d, want 0", len(depsB.ActiveBlockers))
	}
	if len(depsB.ResolvedBlockers) != 1 {
		t.Fatalf("after closing A, B resolved_blockers = %d, want 1", len(depsB.ResolvedBlockers))
	}
	if depsB.ResolvedBlockers[0].ID != beadA.ID {
		t.Errorf("B resolved_blockers[0].id = %q, want %q", depsB.ResolvedBlockers[0].ID, beadA.ID)
	}

	// C should still be blocked by B
	out = run(t, "deps", beadC.ID)
	depsC = parseDeps(t, out)
	if len(depsC.ActiveBlockers) != 1 {
		t.Errorf("C still has active_blockers = %d, want 1", len(depsC.ActiveBlockers))
	}

	// Close B — this should unblock C
	run(t, "close", beadB.ID)

	out = run(t, "deps", beadC.ID)
	depsC = parseDeps(t, out)
	if len(depsC.ActiveBlockers) != 0 {
		t.Errorf("after closing B, C active_blockers = %d, want 0", len(depsC.ActiveBlockers))
	}
	if len(depsC.ResolvedBlockers) != 1 {
		t.Fatalf("after closing B, C resolved_blockers = %d, want 1", len(depsC.ResolvedBlockers))
	}

	// Now --ready should show only C (A and B are closed, not listed in default)
	out = run(t, "list", "--ready")
	readyResult = parseListResult(t, out)
	if readyResult.Total != 1 {
		t.Errorf("after closing A and B, ready list total = %d, want 1", readyResult.Total)
	}
	if readyResult.Beads[0].ID != beadC.ID {
		t.Errorf("ready bead = %q, want %q", readyResult.Beads[0].ID, beadC.ID)
	}

	// Unlink the closed dependency from C and verify
	run(t, "unlink", beadC.ID, "--blocked-by", beadB.ID)
	out = run(t, "deps", beadC.ID)
	depsC = parseDeps(t, out)
	if len(depsC.ActiveBlockers) != 0 {
		t.Errorf("after unlink, C active_blockers = %d, want 0", len(depsC.ActiveBlockers))
	}
	if len(depsC.ResolvedBlockers) != 0 {
		t.Errorf("after unlink, C resolved_blockers = %d, want 0", len(depsC.ResolvedBlockers))
	}
}
