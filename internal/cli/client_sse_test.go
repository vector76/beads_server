package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(serverURL string) *Client {
	return &Client{
		BaseURL:    serverURL,
		HTTPClient: http.DefaultClient,
	}
}

// TestStreamSSE_ServerSendsEvents verifies that 3 SSE events produce 3 signals,
// then the closed body causes a non-nil error.
func TestStreamSSE_ServerSendsEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not support Flush")
			return
		}
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: update\n\n")
			flusher.Flush()
		}
		// Close the connection by returning — body EOF triggers error path.
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx := context.Background()
	signals, errs := client.StreamSSE(ctx)

	count := 0
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case _, ok := <-signals:
			if !ok {
				break loop
			}
			count++
		case <-timeout:
			t.Fatal("timed out waiting for signals")
		}
	}

	if count != 3 {
		t.Fatalf("expected 3 signals, got %d", count)
	}

	err, ok := <-errs
	if !ok {
		t.Fatal("errs channel closed without a value")
	}
	if err == nil {
		t.Fatal("expected non-nil error for unexpected EOF, got nil")
	}
}

// TestStreamSSE_ContextCancel verifies that cancelling the context causes a nil error.
func TestStreamSSE_ContextCancel(t *testing.T) {
	// Server blocks indefinitely until the client disconnects.
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(ready)
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	signals, errs := client.StreamSSE(ctx)

	// Wait until the server has accepted the connection, then cancel.
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("server never became ready")
	}
	cancel()

	// Drain signals.
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-signals:
			if !ok {
				goto checkErr
			}
		case <-timeout:
			t.Fatal("timed out waiting for signal channel to close")
		}
	}
checkErr:
	select {
	case err, ok := <-errs:
		if ok && err != nil {
			t.Fatalf("expected nil error on context cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for error channel")
	}
}

// TestStreamSSE_NonTwoxxStatus verifies that a non-2xx response yields a non-nil error.
func TestStreamSSE_NonTwoxxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx := context.Background()
	signals, errs := client.StreamSSE(ctx)

	// Drain signal channel.
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-signals:
			if !ok {
				goto checkErr
			}
		case <-timeout:
			t.Fatal("timed out waiting for signal channel to close")
		}
	}
checkErr:
	select {
	case err, ok := <-errs:
		if !ok {
			t.Fatal("errs channel closed without a value")
		}
		if err == nil {
			t.Fatal("expected non-nil error for non-2xx status, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for error")
	}
}
