package e2e

import (
	"bytes"
	"encoding/json"
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
	srv, err := server.New(server.Config{Port: 0, Token: testToken, DataFile: filepath.Join(dir, "beads.json")}, s)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
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
// close, reopen, resolve, delete.
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

	// 11. Resolve
	out = run(t, "resolve", bead.ID)
	resolved := parseBead(t, out)
	if resolved.Status != model.StatusResolved {
		t.Errorf("resolved status = %q, want %q", resolved.Status, model.StatusResolved)
	}

	// 12. Delete (soft delete)
	out = run(t, "delete", bead.ID)
	deleted := parseBead(t, out)
	if deleted.Status != model.StatusDeleted {
		t.Errorf("deleted status = %q, want %q", deleted.Status, model.StatusDeleted)
	}

	// 13. List with --all should still include the deleted bead
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

	// Agent 1 resolves the bead
	os.Setenv("BS_USER", "agent-alpha")
	run(t, "resolve", bead.ID)

	// Agent 2 tries to claim the resolved bead — should fail (terminal state)
	os.Setenv("BS_USER", "agent-beta")
	err = runExpectErr(t, "claim", bead.ID)
	if err == nil {
		t.Fatal("expected error claiming resolved bead, got nil")
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

// TestDependencyChain tests creating a dependency chain, resolving blockers,
// and verifying the unblocked computation.
func TestDependencyChain(t *testing.T) {
	ts := startServer(t)
	setEnv(t, ts.URL, "dep-agent")

	// Create a chain: C is blocked by B, B is blocked by A
	// So A must be resolved first, then B, then C becomes unblocked.
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

	// Resolve A — this should unblock B
	out = run(t, "resolve", beadA.ID)
	// The response may include "unblocked" field; we just need to verify
	// B is now unblocked via deps check
	out = run(t, "deps", beadB.ID)
	depsB = parseDeps(t, out)
	if len(depsB.ActiveBlockers) != 0 {
		t.Errorf("after resolving A, B active_blockers = %d, want 0", len(depsB.ActiveBlockers))
	}
	if len(depsB.ResolvedBlockers) != 1 {
		t.Fatalf("after resolving A, B resolved_blockers = %d, want 1", len(depsB.ResolvedBlockers))
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

	// Resolve B — this should unblock C
	run(t, "resolve", beadB.ID)

	out = run(t, "deps", beadC.ID)
	depsC = parseDeps(t, out)
	if len(depsC.ActiveBlockers) != 0 {
		t.Errorf("after resolving B, C active_blockers = %d, want 0", len(depsC.ActiveBlockers))
	}
	if len(depsC.ResolvedBlockers) != 1 {
		t.Fatalf("after resolving B, C resolved_blockers = %d, want 1", len(depsC.ResolvedBlockers))
	}

	// Now --ready should show only C (A and B are resolved, not listed in default)
	out = run(t, "list", "--ready")
	readyResult = parseListResult(t, out)
	if readyResult.Total != 1 {
		t.Errorf("after resolving A and B, ready list total = %d, want 1", readyResult.Total)
	}
	if readyResult.Beads[0].ID != beadC.ID {
		t.Errorf("ready bead = %q, want %q", readyResult.Beads[0].ID, beadC.ID)
	}

	// Unlink the resolved dependency from C and verify
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
