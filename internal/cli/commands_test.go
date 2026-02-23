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

// startTestServerWithVersion creates a test HTTP server reporting the given version string.
func startTestServerWithVersion(t *testing.T, v string) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Load(filepath.Join(dir, "beads.json"))
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	p := server.NewSingleStoreProvider(testToken, s)
	srv, err := server.New(server.Config{Port: 0, LogOutput: io.Discard, Version: v}, p)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
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

func TestAdd_TitleFlag(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "--title", "Flag title")
	created := parseBeadFromOutput(t, out)
	if created.Title != "Flag title" {
		t.Errorf("title = %q, want %q", created.Title, "Flag title")
	}
}

func TestAdd_TitleBothFails(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	err := runCmdErr(t, "add", "Positional title", "--title", "Flag title")
	if err == nil {
		t.Fatal("expected error when both positional arg and --title are provided")
	}
}

func TestAdd_TitleNeitherFails(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	err := runCmdErr(t, "add")
	if err == nil {
		t.Fatal("expected error when neither positional arg nor --title is provided")
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

func TestShow_EpicDetail(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a parent and child
	out := runCmd(t, "add", "My Epic")
	epic := parseBeadFromOutput(t, out)

	runCmd(t, "add", "Child one", "--parent", epic.ID)
	runCmd(t, "add", "Child two", "--parent", epic.ID)

	// Show the epic — should have is_epic, progress, children
	out = runCmd(t, "show", epic.ID)
	var detail map[string]any
	json.Unmarshal([]byte(out), &detail)

	if detail["is_epic"] != true {
		t.Errorf("expected is_epic=true, got %v", detail["is_epic"])
	}

	progress, ok := detail["progress"].(map[string]any)
	if !ok {
		t.Fatal("expected progress object")
	}
	if progress["total"] != float64(2) {
		t.Errorf("expected progress.total=2, got %v", progress["total"])
	}

	children, ok := detail["children"].([]any)
	if !ok {
		t.Fatal("expected children array")
	}
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestShow_ChildDetail(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Parent Epic")
	epic := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Child task", "--parent", epic.ID)
	child := parseBeadFromOutput(t, out)

	// Show the child — should have parent_id and parent_title
	out = runCmd(t, "show", child.ID)
	var detail map[string]any
	json.Unmarshal([]byte(out), &detail)

	if detail["parent_id"] != epic.ID {
		t.Errorf("expected parent_id %s, got %v", epic.ID, detail["parent_id"])
	}
	if detail["parent_title"] != "Parent Epic" {
		t.Errorf("expected parent_title 'Parent Epic', got %v", detail["parent_title"])
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

// runCmdErrWithStderr executes a CLI command, expects an error, and returns stderr output.
func runCmdErrWithStderr(t *testing.T, args ...string) (error, string) {
	t.Helper()
	cmd := NewRootCmd()
	errBuf := new(bytes.Buffer)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(errBuf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return err, errBuf.String()
}

func TestRedirectCmd_Depend(t *testing.T) {
	// no args
	err, stderr := runCmdErrWithStderr(t, "depend")
	if err == nil {
		t.Fatal("expected error from depend (no args), got nil")
	}
	if !strings.Contains(stderr, `unknown command "depend" for "bs"`) {
		t.Errorf("stderr should contain redirect message, got: %s", stderr)
	}
	if !strings.Contains(stderr, "bs link") {
		t.Errorf("stderr should mention 'bs link', got: %s", stderr)
	}
	if !strings.Contains(stderr, "--blocked-by") {
		t.Errorf("stderr should include link help with --blocked-by, got: %s", stderr)
	}

	// with multiple args — args are accepted and ignored
	err, _ = runCmdErrWithStderr(t, "depend", "some-id", "other-id")
	if err == nil {
		t.Fatal("expected error from depend (with args), got nil")
	}
}

func TestRedirectCmd_Block(t *testing.T) {
	// no args
	err, stderr := runCmdErrWithStderr(t, "block")
	if err == nil {
		t.Fatal("expected error from block (no args), got nil")
	}
	if !strings.Contains(stderr, `unknown command "block" for "bs"`) {
		t.Errorf("stderr should contain redirect message, got: %s", stderr)
	}
	if !strings.Contains(stderr, "bs link") {
		t.Errorf("stderr should mention 'bs link', got: %s", stderr)
	}
	if !strings.Contains(stderr, "--blocked-by") {
		t.Errorf("stderr should include link help with --blocked-by, got: %s", stderr)
	}

	// with an arg — accepted and ignored
	err, _ = runCmdErrWithStderr(t, "block", "some-id")
	if err == nil {
		t.Fatal("expected error from block (with args), got nil")
	}
}

func TestRedirectCmd_DependHelp(t *testing.T) {
	// --help must exit 0 and show the short description referencing bs link
	out := runCmd(t, "depend", "--help")
	if !strings.Contains(out, "bs link") {
		t.Errorf("depend --help stdout should reference 'bs link', got: %s", out)
	}
}

func TestRedirectCmd_BlockHelp(t *testing.T) {
	// --help must exit 0 and show the short description referencing bs link
	out := runCmd(t, "block", "--help")
	if !strings.Contains(out, "bs link") {
		t.Errorf("block --help stdout should reference 'bs link', got: %s", out)
	}
}

func TestRedirectCmdsHiddenFromHelp(t *testing.T) {
	out := runCmd(t, "--help")
	if strings.Contains(out, "\n  depend") {
		t.Error("'depend' should not appear as a command in help output")
	}
	if strings.Contains(out, "\n  block") {
		t.Error("'block' should not appear as a command in help output")
	}
}
