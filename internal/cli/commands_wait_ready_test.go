package cli

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"time"
)

// runWaitReadyCmd executes the wait-ready command and returns (stdout, stderr, error).
func runWaitReadyCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := NewRootCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs(append([]string{"wait-ready"}, args...))
	err := cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// TestWaitReady_ImmediatelyReady verifies that the command exits 0 when a ready bead
// already exists before the SSE connection is established.
func TestWaitReady_ImmediatelyReady(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a bead (no dependencies → immediately ready)
	runCmd(t, "add", "Ready bead")

	_, _, err := runWaitReadyCmd(t, "--timeout", "5")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// TestWaitReady_Timeout verifies that the command returns errTimeout when no ready bead
// appears within the timeout window.
func TestWaitReady_Timeout(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// No beads → nothing is ready.
	_, _, err := runWaitReadyCmd(t, "--timeout", "1")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, errTimeout) {
		t.Fatalf("expected errTimeout, got %v", err)
	}
}

// TestWaitReady_BecomesReadyViaSSE verifies that the command exits 0 once a bead becomes
// ready after the initial check (simulated via a second goroutine adding a bead).
func TestWaitReady_BecomesReadyViaSSE(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Add a bead with a blocking dependency so it's not initially ready.
	out := runCmd(t, "add", "Blocker bead")
	blocker := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Blocked bead")
	blocked := parseBeadFromOutput(t, out)
	runCmd(t, "link", blocked.ID, "--blocked-by", blocker.ID)

	// Unblock the bead after a short delay in a goroutine.
	// Use a direct HTTP call instead of runCmd so t.Fatalf is not called from a goroutine
	// after the test may have completed.
	serverURL := ts.URL
	go func() {
		time.Sleep(200 * time.Millisecond)
		req, _ := http.NewRequest(http.MethodPost, serverURL+"/api/v1/beads/"+blocker.ID+"/status", bytes.NewBufferString(`{"status":"closed"}`))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(req) //nolint:errcheck
	}()

	_, _, err := runWaitReadyCmd(t, "--timeout", "5")
	if err != nil {
		t.Fatalf("expected nil error after bead became ready, got %v", err)
	}
}

// TestWaitReady_MissingTimeout verifies that --timeout is required.
func TestWaitReady_MissingTimeout(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	_, _, err := runWaitReadyCmd(t)
	if err == nil {
		t.Fatal("expected error when --timeout is missing")
	}
}

// TestWaitReady_TagFilter verifies that --tag filters beads by tag.
func TestWaitReady_TagFilter(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a bead without the required tag — should NOT match.
	runCmd(t, "add", "Untagged bead")

	// With a tag filter for a tag no bead has, it should time out.
	_, _, err := runWaitReadyCmd(t, "--timeout", "1", "--tag", "mytag")
	if err == nil {
		t.Fatal("expected timeout when no bead matches tag filter")
	}
	if !errors.Is(err, errTimeout) {
		t.Fatalf("expected errTimeout, got %v", err)
	}

	// Now create a bead with the matching tag.
	runCmd(t, "add", "Tagged bead", "--tags", "mytag")

	_, _, err = runWaitReadyCmd(t, "--timeout", "5", "--tag", "mytag")
	if err != nil {
		t.Fatalf("expected nil error with matching tagged bead, got %v", err)
	}
}
