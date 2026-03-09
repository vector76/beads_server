package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE_StatusAndContentType(t *testing.T) {
	srv := crudServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Router.ServeHTTP(w, req)
		close(done)
	}()

	// Give the handler time to write headers.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
}

func TestSSE_ReceivesUpdateAfterPublish(t *testing.T) {
	srv := crudServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Router.ServeHTTP(w, req)
		close(done)
	}()

	// Wait for handler to subscribe, then publish.
	time.Sleep(50 * time.Millisecond)
	srv.broadcaster.publish()

	// Wait for debounce (200ms) plus a little margin.
	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "data: update\n\n") {
		t.Errorf("expected body to contain 'data: update\\n\\n', got %q", body)
	}
}

func TestSSE_ExitsOnContextCancel(t *testing.T) {
	srv := crudServer(t)

	ctx, cancel := context.WithCancel(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Handler exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after context cancellation")
	}
}

func TestSSE_NoAuthRequired(t *testing.T) {
	srv := crudServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	// No Authorization header — must still succeed.
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Router.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", w.Code)
	}
}
