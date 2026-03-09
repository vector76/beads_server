package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vector76/beads_server/internal/model"
	"github.com/vector76/beads_server/internal/store"
)

func statusReq(query string) *http.Request {
	url := "/api/v1/beads/status"
	if query != "" {
		url += "?" + query
	}
	return httptest.NewRequest(http.MethodGet, url, nil)
}

func getStatus(t *testing.T, srv *Server, query string) (int, map[string]string) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, statusReq(query))
	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)
	return w.Code, result
}

func TestBeadsStatus_HappyPath(t *testing.T) {
	srv := crudServer(t)
	b1 := createViaAPI(t, srv, map[string]any{"title": "First"})
	b2 := createViaAPI(t, srv, map[string]any{"title": "Second"})
	patchStatus(t, srv, b2.ID, "closed")

	code, result := getStatus(t, srv, "ids="+b1.ID+","+b2.ID)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if result[b1.ID] != "open" {
		t.Errorf("expected open for %s, got %q", b1.ID, result[b1.ID])
	}
	if result[b2.ID] != "closed" {
		t.Errorf("expected closed for %s, got %q", b2.ID, result[b2.ID])
	}
}

func TestBeadsStatus_UnknownIDs(t *testing.T) {
	srv := crudServer(t)
	b := createViaAPI(t, srv, map[string]any{"title": "Known"})

	code, result := getStatus(t, srv, "ids="+b.ID+",bd-notexist")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if result[b.ID] != "open" {
		t.Errorf("expected open for %s, got %q", b.ID, result[b.ID])
	}
	if result["bd-notexist"] != statusUnknown {
		t.Errorf("expected unknown for bd-notexist, got %q", result["bd-notexist"])
	}
}

func TestBeadsStatus_MissingIDsParam(t *testing.T) {
	srv := crudServer(t)

	code, result := getStatus(t, srv, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestBeadsStatus_EmptyIDsParam(t *testing.T) {
	srv := crudServer(t)

	code, result := getStatus(t, srv, "ids=")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestBeadsStatus_DuplicateIDs(t *testing.T) {
	srv := crudServer(t)
	b := createViaAPI(t, srv, map[string]any{"title": "Dedup"})

	code, result := getStatus(t, srv, "ids="+b.ID+","+b.ID+","+b.ID)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d: %v", len(result), result)
	}
	if result[b.ID] != "open" {
		t.Errorf("expected open, got %q", result[b.ID])
	}
}

func TestBeadsStatus_DeletedStatus(t *testing.T) {
	srv := crudServer(t)
	b := createViaAPI(t, srv, map[string]any{"title": "To delete"})

	req := authReq(http.MethodDelete, "/api/v1/beads/"+b.ID, nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	code, result := getStatus(t, srv, "ids="+b.ID)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if result[b.ID] != "deleted" {
		t.Errorf("expected deleted, got %q", result[b.ID])
	}
}

func TestBeadsStatus_MultiProject(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	s1, err := store.Load(filepath.Join(dir1, "beads.json"))
	if err != nil {
		t.Fatalf("Load s1: %v", err)
	}
	s2, err := store.Load(filepath.Join(dir2, "beads.json"))
	if err != nil {
		t.Fatalf("Load s2: %v", err)
	}

	p := NewMultiStoreProvider([]ProviderEntry{
		{Name: "alpha", Token: "token-alpha", Store: s1},
		{Name: "beta", Token: "token-beta", Store: s2},
	})
	srv, err := New(Config{Port: 0, LogOutput: io.Discard}, p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	b1, err := s1.Create(model.NewBead("Alpha bead"))
	if err != nil {
		t.Fatalf("create b1: %v", err)
	}
	b2, err := s2.Create(model.NewBead("Beta bead"))
	if err != nil {
		t.Fatalf("create b2: %v", err)
	}

	code, result := getStatus(t, srv, "ids="+b1.ID+","+b2.ID)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if result[b1.ID] != "open" {
		t.Errorf("expected open for %s (alpha), got %q", b1.ID, result[b1.ID])
	}
	if result[b2.ID] != "open" {
		t.Errorf("expected open for %s (beta), got %q", b2.ID, result[b2.ID])
	}
}

func TestBeadsStatus_NoAuthRequired(t *testing.T) {
	srv := crudServer(t)

	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/beads/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", w.Code)
	}
}
