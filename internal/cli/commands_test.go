package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/server"
	"github.com/yourorg/beads_server/internal/store"
)

const testToken = "test-secret"

// startTestServer creates a test HTTP server backed by a real store.
func startTestServer(t *testing.T) *httptest.Server {
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

// setClientEnv sets BS_URL and BS_TOKEN env vars for a test, restoring them after.
func setClientEnv(t *testing.T, url string) {
	t.Helper()
	os.Setenv("BS_URL", url)
	os.Setenv("BS_TOKEN", testToken)
	t.Cleanup(func() {
		os.Unsetenv("BS_URL")
		os.Unsetenv("BS_TOKEN")
	})
}

// runCmd executes a CLI command and returns stdout output.
func runCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
	return buf.String()
}

// runCmdErr executes a CLI command and expects an error.
func runCmdErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	return cmd.Execute()
}

// parseBeadFromOutput parses a bead from pretty-printed JSON output.
func parseBeadFromOutput(t *testing.T, output string) model.Bead {
	t.Helper()
	var b model.Bead
	if err := json.Unmarshal([]byte(output), &b); err != nil {
		t.Fatalf("failed to parse bead from output: %v\noutput: %s", err, output)
	}
	return b
}

func TestCRUDCycle(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// 1. Add a bead
	out := runCmd(t, "add", "My test bead", "--type", "bug", "--priority", "high", "--description", "A test bug", "--tags", "cli,test")
	created := parseBeadFromOutput(t, out)

	if created.Title != "My test bead" {
		t.Errorf("title = %q, want %q", created.Title, "My test bead")
	}
	if created.Type != model.TypeBug {
		t.Errorf("type = %q, want %q", created.Type, model.TypeBug)
	}
	if created.Priority != model.PriorityHigh {
		t.Errorf("priority = %q, want %q", created.Priority, model.PriorityHigh)
	}
	if created.Description != "A test bug" {
		t.Errorf("description = %q, want %q", created.Description, "A test bug")
	}
	if len(created.Tags) != 2 || created.Tags[0] != "cli" || created.Tags[1] != "test" {
		t.Errorf("tags = %v, want [cli test]", created.Tags)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// 2. Show the bead
	out = runCmd(t, "show", created.ID)
	shown := parseBeadFromOutput(t, out)
	if shown.ID != created.ID {
		t.Errorf("show ID = %q, want %q", shown.ID, created.ID)
	}
	if shown.Title != "My test bead" {
		t.Errorf("show title = %q, want %q", shown.Title, "My test bead")
	}

	// 3. Edit the bead
	out = runCmd(t, "edit", created.ID, "--title", "Updated title", "--priority", "low", "--add-tag", "urgent")
	edited := parseBeadFromOutput(t, out)
	if edited.Title != "Updated title" {
		t.Errorf("edited title = %q, want %q", edited.Title, "Updated title")
	}
	if edited.Priority != model.PriorityLow {
		t.Errorf("edited priority = %q, want %q", edited.Priority, model.PriorityLow)
	}
	if len(edited.Tags) != 3 {
		t.Errorf("edited tags = %v, want 3 tags", edited.Tags)
	}

	// 4. Close the bead
	out = runCmd(t, "close", created.ID)
	closed := parseBeadFromOutput(t, out)
	if closed.Status != model.StatusClosed {
		t.Errorf("closed status = %q, want %q", closed.Status, model.StatusClosed)
	}

	// 5. Reopen the bead
	out = runCmd(t, "reopen", created.ID)
	reopened := parseBeadFromOutput(t, out)
	if reopened.Status != model.StatusOpen {
		t.Errorf("reopened status = %q, want %q", reopened.Status, model.StatusOpen)
	}

	// 6. Delete the bead
	out = runCmd(t, "delete", created.ID)
	deleted := parseBeadFromOutput(t, out)
	if deleted.ID != created.ID {
		t.Errorf("deleted ID = %q, want %q", deleted.ID, created.ID)
	}

	// 7. Show the deleted bead — should return with status "deleted" (soft delete)
	out = runCmd(t, "show", created.ID)
	afterDelete := parseBeadFromOutput(t, out)
	if afterDelete.Status != model.StatusDeleted {
		t.Errorf("after delete status = %q, want %q", afterDelete.Status, model.StatusDeleted)
	}
}

func TestAdd_MissingToken(t *testing.T) {
	os.Unsetenv("BS_TOKEN")
	os.Unsetenv("BS_URL")

	err := runCmdErr(t, "add", "some title")
	if err == nil {
		t.Fatal("expected error when BS_TOKEN is missing")
	}
}

func TestShow_PrefixFails(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a bead
	out := runCmd(t, "add", "Prefix test bead")
	created := parseBeadFromOutput(t, out)

	// Prefix should NOT work — exact ID required
	prefix := created.ID[:6] // "bd-" + 3 chars
	err := runCmdErr(t, "show", prefix)
	if err == nil {
		t.Errorf("expected error for prefix match, ID=%q prefix=%q", created.ID, prefix)
	}
}

func TestEdit_NoFlags(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Edit no-flags test")
	created := parseBeadFromOutput(t, out)

	err := runCmdErr(t, "edit", created.ID)
	if err == nil {
		t.Error("expected error when no edit flags provided")
	}
}

func TestClean_RemovesClosedBeads(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a closed bead and an open bead
	runCmd(t, "add", "Closed bead")
	out := runCmd(t, "add", "Open bead")
	openBead := parseBeadFromOutput(t, out)

	// Close the first bead via list + close
	out = runCmd(t, "list")
	var result store.ListResult
	json.Unmarshal([]byte(out), &result)
	for _, b := range result.Beads {
		if b.Title == "Closed bead" {
			runCmd(t, "close", b.ID)
			break
		}
	}

	// Clean with days=0 to remove all closed beads
	out = runCmd(t, "clean", "--days", "0")
	var resp struct {
		Removed int `json:"removed"`
	}
	json.Unmarshal([]byte(out), &resp)

	if resp.Removed != 1 {
		t.Fatalf("expected 1 removed, got %d", resp.Removed)
	}

	// Open bead should still exist
	out = runCmd(t, "show", openBead.ID)
	shown := parseBeadFromOutput(t, out)
	if shown.ID != openBead.ID {
		t.Errorf("expected open bead to remain, got %q", shown.ID)
	}
}

func TestClean_FractionalDays(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Closed bead")
	created := parseBeadFromOutput(t, out)
	runCmd(t, "close", created.ID)

	// 0.5 days = 12 hours; just-closed bead should NOT be removed
	out = runCmd(t, "clean", "--days", "0.5")
	var resp struct {
		Removed int `json:"removed"`
	}
	json.Unmarshal([]byte(out), &resp)

	if resp.Removed != 0 {
		t.Fatalf("expected 0 removed (bead is recent), got %d", resp.Removed)
	}
}

func TestClean_Hours(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Closed bead")
	created := parseBeadFromOutput(t, out)
	runCmd(t, "close", created.ID)

	// --hours 0 should remove all closed beads
	out = runCmd(t, "clean", "--hours", "0")
	var resp struct {
		Removed int `json:"removed"`
	}
	json.Unmarshal([]byte(out), &resp)

	if resp.Removed != 1 {
		t.Fatalf("expected 1 removed, got %d", resp.Removed)
	}
}

func TestClean_DaysAndHoursMutuallyExclusive(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	err := runCmdErr(t, "clean", "--days", "1", "--hours", "12")
	if err == nil {
		t.Fatal("expected error when both --days and --hours specified")
	}
	if !strings.Contains(err.Error(), "cannot specify both --days and --hours") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateAlias(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// "create" should behave identically to "add"
	out := runCmd(t, "create", "Alias test bead", "--type", "feature", "--priority", "low")
	created := parseBeadFromOutput(t, out)

	if created.Title != "Alias test bead" {
		t.Errorf("title = %q, want %q", created.Title, "Alias test bead")
	}
	if created.Type != model.TypeFeature {
		t.Errorf("type = %q, want %q", created.Type, model.TypeFeature)
	}
	if created.Priority != model.PriorityLow {
		t.Errorf("priority = %q, want %q", created.Priority, model.PriorityLow)
	}
}

func TestResolveAlias(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a bead, then close it with "resolve"
	out := runCmd(t, "add", "Resolve test bead")
	created := parseBeadFromOutput(t, out)

	out = runCmd(t, "resolve", created.ID)
	resolved := parseBeadFromOutput(t, out)

	if resolved.Status != model.StatusClosed {
		t.Errorf("resolved status = %q, want %q", resolved.Status, model.StatusClosed)
	}
}

func TestAliasesHiddenFromHelp(t *testing.T) {
	// The aliases must not appear as commands in help output.
	// Help lists commands with leading whitespace: "  create ..."
	// We check for that pattern to avoid false matches against
	// descriptions that may contain the word (e.g. "Create a new bead").
	out := runCmd(t, "--help")

	if strings.Contains(out, "\n  create") {
		t.Error("'create' alias should not appear as a command in help output")
	}
	if strings.Contains(out, "\n  resolve") {
		t.Error("'resolve' alias should not appear as a command in help output")
	}
}

func TestAdd_WithParent(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a parent bead
	out := runCmd(t, "add", "Parent bead")
	parent := parseBeadFromOutput(t, out)

	// Create a child with --parent
	out = runCmd(t, "add", "Child bead", "--parent", parent.ID)
	child := parseBeadFromOutput(t, out)

	if child.ParentID != parent.ID {
		t.Errorf("parent_id = %q, want %q", child.ParentID, parent.ID)
	}
}

func TestMove_IntoAndOut(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create epic and bead
	out := runCmd(t, "add", "Epic")
	epic := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Standalone")
	bead := parseBeadFromOutput(t, out)

	// Move into
	out = runCmd(t, "move", bead.ID, "--into", epic.ID)
	moved := parseBeadFromOutput(t, out)
	if moved.ParentID != epic.ID {
		t.Errorf("after move --into: parent_id = %q, want %q", moved.ParentID, epic.ID)
	}

	// Move out
	out = runCmd(t, "move", bead.ID, "--out")
	movedOut := parseBeadFromOutput(t, out)
	if movedOut.ParentID != "" {
		t.Errorf("after move --out: parent_id = %q, want empty", movedOut.ParentID)
	}
}

func TestMove_RequiresFlag(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Bead")
	bead := parseBeadFromOutput(t, out)

	err := runCmdErr(t, "move", bead.ID)
	if err == nil {
		t.Fatal("expected error when neither --into nor --out specified")
	}
}

func TestMove_BothFlagsRejected(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Epic")
	epic := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Bead")
	bead := parseBeadFromOutput(t, out)

	err := runCmdErr(t, "move", bead.ID, "--into", epic.ID, "--out")
	if err == nil {
		t.Fatal("expected error when both --into and --out specified")
	}
}

func TestDelete_NotFound(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	err := runCmdErr(t, "delete", "bd-nonexistent")
	if err == nil {
		t.Error("expected error deleting non-existent bead")
	}
}
