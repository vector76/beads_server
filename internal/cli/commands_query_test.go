package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

func TestVersionCmd(t *testing.T) {
	ts := startTestServerWithVersion(t, "9.8.7")
	setClientEnv(t, ts.URL)

	out := runCmd(t, "version")

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output: %v\noutput: %s", err, out)
	}
	if result["server"] != "9.8.7" {
		t.Errorf("server = %q, want 9.8.7", result["server"])
	}
	if result["client"] != version {
		t.Errorf("client = %q, want %q", result["client"], version)
	}
}

func TestList_Default(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create two beads
	runCmd(t, "add", "Bead one")
	runCmd(t, "add", "Bead two")

	out := runCmd(t, "list")

	var result store.ListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse list output: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if len(result.Beads) != 2 {
		t.Errorf("beads count = %d, want 2", len(result.Beads))
	}
}

func TestList_StatusFilter(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Open bead")
	created := parseBeadFromOutput(t, out)
	runCmd(t, "close", created.ID)

	runCmd(t, "add", "Another open bead")

	// List only closed beads
	out = runCmd(t, "list", "--status", "closed")
	var result store.ListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse list output: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

func TestList_AllFlag(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Open bead")
	created := parseBeadFromOutput(t, out)
	runCmd(t, "close", created.ID)

	runCmd(t, "add", "Another open bead")

	// List all beads (including closed)
	out = runCmd(t, "list", "--all")
	var result store.ListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse list output: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
}

func TestSearch(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	runCmd(t, "add", "Fix the login bug")
	runCmd(t, "add", "Add new feature")

	out := runCmd(t, "search", "login")

	var result store.ListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse search output: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if len(result.Beads) != 1 {
		t.Fatalf("beads count = %d, want 1", len(result.Beads))
	}
	if result.Beads[0].Title != "Fix the login bug" {
		t.Errorf("title = %q, want %q", result.Beads[0].Title, "Fix the login bug")
	}
}

func TestClaim(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)
	os.Setenv("BS_USER", "agent-42")
	t.Cleanup(func() { os.Unsetenv("BS_USER") })

	out := runCmd(t, "add", "Claimable bead")
	created := parseBeadFromOutput(t, out)

	out = runCmd(t, "claim", created.ID)
	claimed := parseBeadFromOutput(t, out)

	if claimed.Status != model.StatusInProgress {
		t.Errorf("claimed status = %q, want %q", claimed.Status, model.StatusInProgress)
	}
	if claimed.Assignee != "agent-42" {
		t.Errorf("claimed assignee = %q, want %q", claimed.Assignee, "agent-42")
	}
}

func TestMine(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)
	os.Setenv("BS_USER", "agent-42")
	t.Cleanup(func() { os.Unsetenv("BS_USER") })

	// Create and claim a bead
	out := runCmd(t, "add", "My claimed bead")
	created := parseBeadFromOutput(t, out)
	runCmd(t, "claim", created.ID)

	// Create another bead but don't claim it
	runCmd(t, "add", "Unclaimed bead")

	out = runCmd(t, "mine")

	var result store.ListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse mine output: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if len(result.Beads) != 1 {
		t.Fatalf("beads count = %d, want 1", len(result.Beads))
	}
	if result.Beads[0].Title != "My claimed bead" {
		t.Errorf("title = %q, want %q", result.Beads[0].Title, "My claimed bead")
	}
}

func TestList_HierarchicalWithEpic(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create epic with children
	out := runCmd(t, "add", "My Epic")
	epic := parseBeadFromOutput(t, out)
	runCmd(t, "add", "Child one", "--parent", epic.ID)
	runCmd(t, "add", "Child two", "--parent", epic.ID)

	// Create standalone bead
	runCmd(t, "add", "Standalone")

	out = runCmd(t, "list")
	var result map[string]any
	json.Unmarshal([]byte(out), &result)

	// Total should be 2 (epic + standalone), not 4
	total := int(result["total"].(float64))
	if total != 2 {
		t.Fatalf("expected 2 top-level, got %d", total)
	}

	// Find epic and verify children nested
	beads := result["beads"].([]any)
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

func TestList_ReadyWithEpicContext(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create epic with a child
	out := runCmd(t, "add", "Ready Epic")
	epic := parseBeadFromOutput(t, out)
	runCmd(t, "add", "Ready child", "--parent", epic.ID)

	out = runCmd(t, "list", "--ready")
	var result store.ListResult
	json.Unmarshal([]byte(out), &result)

	// Should show child only (not the epic)
	if result.Total != 1 {
		t.Fatalf("expected 1 ready child, got %d", result.Total)
	}
	if result.Beads[0].ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, result.Beads[0].ParentID)
	}
	if result.Beads[0].ParentTitle != "Ready Epic" {
		t.Errorf("expected parent_title 'Ready Epic', got %q", result.Beads[0].ParentTitle)
	}
}

func TestMine_ExcludesEpics(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)
	os.Setenv("BS_USER", "agent-42")
	t.Cleanup(func() { os.Unsetenv("BS_USER") })

	// Create epic with a child, claim the child
	out := runCmd(t, "add", "Epic for mine test")
	epic := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Child task", "--parent", epic.ID)
	child := parseBeadFromOutput(t, out)
	runCmd(t, "claim", child.ID)

	out = runCmd(t, "mine")
	var result store.ListResult
	json.Unmarshal([]byte(out), &result)

	if result.Total != 1 {
		t.Fatalf("expected 1 mine result, got %d", result.Total)
	}
	if result.Beads[0].ID != child.ID {
		t.Errorf("expected child %s, got %s", child.ID, result.Beads[0].ID)
	}
	// Verify parent context is included
	if result.Beads[0].ParentTitle != "Epic for mine test" {
		t.Errorf("expected parent_title 'Epic for mine test', got %q", result.Beads[0].ParentTitle)
	}
}

func TestSearch_WithEpicContext(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create epic and child
	out := runCmd(t, "add", "Auth Epic")
	epic := parseBeadFromOutput(t, out)

	runCmd(t, "add", "Fix login bug", "--parent", epic.ID)

	// Search for child
	out = runCmd(t, "search", "login")
	var result store.ListResult
	json.Unmarshal([]byte(out), &result)

	if result.Total != 1 {
		t.Fatalf("expected 1 search result, got %d", result.Total)
	}
	if result.Beads[0].ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, result.Beads[0].ParentID)
	}
	if result.Beads[0].ParentTitle != "Auth Epic" {
		t.Errorf("expected parent_title 'Auth Epic', got %q", result.Beads[0].ParentTitle)
	}

	// Search for epic — should show is_epic
	out = runCmd(t, "search", "Auth")
	json.Unmarshal([]byte(out), &result)

	if result.Total != 1 {
		t.Fatalf("expected 1 result, got %d", result.Total)
	}
	if !result.Beads[0].IsEpic {
		t.Error("expected is_epic=true in search result")
	}
}

func TestComment(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)
	os.Setenv("BS_USER", "commenter")
	t.Cleanup(func() { os.Unsetenv("BS_USER") })

	out := runCmd(t, "add", "Commentable bead")
	created := parseBeadFromOutput(t, out)

	out = runCmd(t, "comment", created.ID, "This is a test comment")
	commented := parseBeadFromOutput(t, out)

	if len(commented.Comments) != 1 {
		t.Fatalf("comments count = %d, want 1", len(commented.Comments))
	}
	if commented.Comments[0].Author != "commenter" {
		t.Errorf("comment author = %q, want %q", commented.Comments[0].Author, "commenter")
	}
	if commented.Comments[0].Text != "This is a test comment" {
		t.Errorf("comment text = %q, want %q", commented.Comments[0].Text, "This is a test comment")
	}
}

func TestLinkUnlinkDeps(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create two beads
	out := runCmd(t, "add", "Blocked bead")
	blocked := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Blocker bead")
	blocker := parseBeadFromOutput(t, out)

	// Link: blocked is blocked by blocker
	out = runCmd(t, "link", blocked.ID, "--blocked-by", blocker.ID)
	linked := parseBeadFromOutput(t, out)

	found := false
	for _, dep := range linked.BlockedBy {
		if dep == blocker.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("blocked_by = %v, expected to contain %s", linked.BlockedBy, blocker.ID)
	}

	// Deps: should show blocker as active blocker
	out = runCmd(t, "deps", blocked.ID)
	var deps store.DepsResult
	if err := json.Unmarshal([]byte(out), &deps); err != nil {
		t.Fatalf("failed to parse deps output: %v", err)
	}

	if len(deps.ActiveBlockers) != 1 {
		t.Fatalf("active_blockers count = %d, want 1", len(deps.ActiveBlockers))
	}
	if deps.ActiveBlockers[0].ID != blocker.ID {
		t.Errorf("active_blockers[0].id = %q, want %q", deps.ActiveBlockers[0].ID, blocker.ID)
	}

	// Unlink
	out = runCmd(t, "unlink", blocked.ID, "--blocked-by", blocker.ID)
	unlinked := parseBeadFromOutput(t, out)

	if len(unlinked.BlockedBy) != 0 {
		t.Errorf("after unlink blocked_by = %v, want empty", unlinked.BlockedBy)
	}
}

func TestLink_MissingBlockedBy(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Some bead")
	created := parseBeadFromOutput(t, out)

	err := runCmdErr(t, "link", created.ID)
	if err == nil {
		t.Error("expected error when --blocked-by is missing")
	}
}

func TestUnlink_MissingBlockedBy(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Some bead")
	created := parseBeadFromOutput(t, out)

	err := runCmdErr(t, "unlink", created.ID)
	if err == nil {
		t.Error("expected error when --blocked-by is missing")
	}
}
