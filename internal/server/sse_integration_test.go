package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSSEIntegration_RouteNoAuth verifies that GET /events is accessible without
// an Authorization header and returns the correct SSE headers.
func TestSSEIntegration_RouteNoAuth(t *testing.T) {
	srv := crudServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so the handler exits immediately after writing headers

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
}

// TestSSEIntegration_MutationTriggersBroadcast verifies the full path:
// API mutation → broadcaster → SSE client receives "data: update\n\n".
func TestSSEIntegration_MutationTriggersBroadcast(t *testing.T) {
	srv := crudServer(t)

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	// Connect SSE client in a goroutine and collect received lines.
	received := make(chan string, 16)
	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()

	go func() {
		req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, ts.URL+"/events", nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			received <- line
		}
	}()

	// Give the SSE client time to connect and subscribe.
	time.Sleep(50 * time.Millisecond)

	// Perform a write mutation via the API.
	body, _ := json.Marshal(map[string]any{"title": "integration test bead"})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/beads", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/beads: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Wait for the debounced SSE event (debounce is 200ms; allow 1s total).
	deadline := time.After(1 * time.Second)
	for {
		select {
		case line := <-received:
			if strings.Contains(line, "data: update") {
				return // success
			}
		case <-deadline:
			t.Fatal("timed out waiting for SSE update event after mutation")
		}
	}
}
