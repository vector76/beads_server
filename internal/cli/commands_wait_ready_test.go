package cli

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

type waitReadyResult struct {
	err    error
	stderr string
}

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
// appears within the timeout window, and produces no stderr output (silent per spec).
func TestWaitReady_Timeout(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// No beads → nothing is ready.
	_, stderr, err := runWaitReadyCmd(t, "--timeout", "1")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, errTimeout) {
		t.Fatalf("expected errTimeout, got %v", err)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr on timeout, got: %q", stderr)
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

// TestWaitReady_WaitThenSuccess verifies that the command exits 0 when a blocked bead
// becomes ready after its blocker is closed, using the async pattern with bs close.
func TestWaitReady_WaitThenSuccess(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Blocker")
	blocker := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Blocked bead")
	blocked := parseBeadFromOutput(t, out)
	runCmd(t, "link", blocked.ID, "--blocked-by", blocker.ID)

	ch := make(chan waitReadyResult, 1)
	go func() {
		cmd := NewRootCmd()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{"wait-ready", "--timeout", "10"})
		err := cmd.Execute()
		ch <- waitReadyResult{err, errBuf.String()}
	}()

	time.Sleep(200 * time.Millisecond)
	runCmd(t, "close", blocker.ID)

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("expected nil error after blocker closed, got %v (stderr: %q)", r.err, r.stderr)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for wait-ready to complete")
	}
}

// TestWaitReady_FilterNoMatchThenSuccess verifies that wait-ready with a tag filter
// exits 0 when a bead with the matching tag is added after the command starts.
func TestWaitReady_FilterNoMatchThenSuccess(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	// Create a ready bead with the wrong tag — should not match.
	runCmd(t, "add", "Wrong bead", "--tags", "wrong")

	ch := make(chan waitReadyResult, 1)
	go func() {
		cmd := NewRootCmd()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{"wait-ready", "--timeout", "10", "--tag", "right"})
		err := cmd.Execute()
		ch <- waitReadyResult{err, errBuf.String()}
	}()

	time.Sleep(200 * time.Millisecond)
	// Adding a bead triggers SSE; wait-ready re-checks and finds the tagged bead.
	runCmd(t, "add", "Right bead", "--tags", "right")

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("expected nil error after right-tagged bead added, got %v (stderr: %q)", r.err, r.stderr)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for wait-ready to complete")
	}
}

// TestWaitReady_FilterNoMatchTimeout verifies that wait-ready times out silently when
// only beads with the wrong tag exist.
func TestWaitReady_FilterNoMatchTimeout(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	runCmd(t, "add", "Wrong bead", "--tags", "wrong")

	err, stderr := runCmdErrWithStderr(t, "wait-ready", "--timeout", "1", "--tag", "right")
	if err == nil {
		t.Fatal("expected non-zero exit when no bead matches tag filter")
	}
	if stderr != "" {
		t.Errorf("expected empty stderr on timeout, got: %q", stderr)
	}
}

// TestWaitReady_ZeroTimeoutSuccess verifies that --timeout 0 (indefinite) waits until
// a bead becomes ready.
func TestWaitReady_ZeroTimeoutSuccess(t *testing.T) {
	ts := startTestServer(t)
	setClientEnv(t, ts.URL)

	out := runCmd(t, "add", "Blocker for zero-timeout test")
	blocker := parseBeadFromOutput(t, out)

	out = runCmd(t, "add", "Blocked bead")
	blocked := parseBeadFromOutput(t, out)
	runCmd(t, "link", blocked.ID, "--blocked-by", blocker.ID)

	ch := make(chan waitReadyResult, 1)
	go func() {
		cmd := NewRootCmd()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{"wait-ready", "--timeout", "0"})
		err := cmd.Execute()
		ch <- waitReadyResult{err, errBuf.String()}
	}()

	time.Sleep(200 * time.Millisecond)
	runCmd(t, "close", blocker.ID)

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("expected nil error with zero timeout, got %v (stderr: %q)", r.err, r.stderr)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for wait-ready (zero timeout) to complete")
	}
}

// TestWaitReady_ConnectivityError verifies that the command exits non-zero with a
// non-empty stderr when the server is unreachable.
func TestWaitReady_ConnectivityError(t *testing.T) {
	os.Setenv("BS_URL", "http://localhost:1")
	os.Setenv("BS_TOKEN", testToken)
	t.Cleanup(func() {
		os.Unsetenv("BS_URL")
		os.Unsetenv("BS_TOKEN")
	})

	err, stderr := runCmdErrWithStderr(t, "wait-ready", "--timeout", "5")
	if err == nil {
		t.Fatal("expected non-zero exit when server is unreachable")
	}
	if stderr == "" {
		t.Error("expected non-empty stderr for connectivity error")
	}
}

// TestWaitReady_SSEDropMidWait verifies that the command exits non-zero with non-empty
// stderr when the SSE connection is dropped while the command is waiting.
func TestWaitReady_SSEDropMidWait(t *testing.T) {
	ts := startTestServer(t)
	// startTestServer registers t.Cleanup(ts.Close); closing early is safe — idempotent.
	setClientEnv(t, ts.URL)

	ch := make(chan waitReadyResult, 1)
	go func() {
		cmd := NewRootCmd()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{"wait-ready", "--timeout", "10"})
		err := cmd.Execute()
		ch <- waitReadyResult{err, errBuf.String()}
	}()

	// Wait for the SSE connection to be established, then immediately drop all
	// active connections. CloseClientConnections does not block like ts.Close().
	time.Sleep(100 * time.Millisecond)
	ts.CloseClientConnections()

	select {
	case r := <-ch:
		if r.err == nil {
			t.Fatal("expected non-zero exit after server closed mid-wait")
		}
		if r.stderr == "" {
			t.Error("expected non-empty stderr after SSE drop, got empty")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for wait-ready to detect server close")
	}
}
