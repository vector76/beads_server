package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestShow_WithPrefix(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a bead
	out := runCmd(t, "add", "Prefix test bead")
	created := parseBeadFromOutput(t, out)

	// Show using a prefix of the ID (at least 3 chars from the random part)
	prefix := created.ID[:6] // "bd-" + 3 chars
	out = runCmd(t, "show", prefix)
	shown := parseBeadFromOutput(t, out)
	if shown.ID != created.ID {
		t.Errorf("prefix show ID = %q, want %q", shown.ID, created.ID)
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

func TestDelete_NotFound(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	err := runCmdErr(t, "delete", "bd-nonexistent")
	if err == nil {
		t.Error("expected error deleting non-existent bead")
	}
}
