package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vector76/beads_server/internal/model"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "beads.json")
}

func TestLoadMissingFileCreatesEmptyStore(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.All()) != 0 {
		t.Errorf("expected empty store, got %d beads", len(s.All()))
	}
}

func TestLoadExistingFile(t *testing.T) {
	path := tempPath(t)

	// Write a valid data file with an explicit ID
	b := model.NewBead("Existing bead")
	b.ID = "bd-exist01"
	fd := fileData{Beads: []model.Bead{b}}
	data, _ := json.MarshalIndent(fd, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.All()) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(s.All()))
	}

	got, err := s.Get("bd-exist01")
	if err != nil {
		t.Fatalf("unexpected error getting bead: %v", err)
	}
	if got.Title != "Existing bead" {
		t.Errorf("expected title 'Existing bead', got %q", got.Title)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	path := tempPath(t)
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error loading invalid JSON")
	}
}

func TestCreateAndGet(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b := model.NewBead("New bead")
	created, err := s.Create(b)
	if err != nil {
		t.Fatalf("unexpected error creating bead: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID after Create")
	}

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error getting bead: %v", err)
	}
	if got.Title != "New bead" {
		t.Errorf("expected title 'New bead', got %q", got.Title)
	}
}

func TestCreateDuplicateIDReturnsError(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b := model.NewBead("First")
	b.ID = "bd-dup00001"
	if _, err := s.Create(b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b2 := model.NewBead("Second")
	b2.ID = "bd-dup00001"
	_, err = s.Create(b2)
	if err == nil {
		t.Error("expected error creating duplicate bead")
	}
}

func TestGetNotFound(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = s.Get("bd-nonexist")
	if err == nil {
		t.Error("expected error getting non-existent bead")
	}
}

func TestUpdate(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b := model.NewBead("Original title")
	created, err := s.Create(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newTitle := "Updated title"
	newPriority := model.PriorityHigh
	updated, err := s.Update(created.ID, UpdateFields{
		Title:    &newTitle,
		Priority: &newPriority,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Title != "Updated title" {
		t.Errorf("expected title 'Updated title', got %q", updated.Title)
	}
	if updated.Priority != model.PriorityHigh {
		t.Errorf("expected priority 'high', got %q", updated.Priority)
	}
	// Status should remain unchanged
	if updated.Status != model.StatusOpen {
		t.Errorf("expected status 'open', got %q", updated.Status)
	}
	// UpdatedAt should be after CreatedAt
	if !updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt) {
		t.Error("expected updated_at to be after created_at")
	}
}

func TestUpdateNotFound(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	title := "nope"
	_, err = s.Update("bd-nonexist", UpdateFields{Title: &title})
	if err == nil {
		t.Error("expected error updating non-existent bead")
	}
}

func TestDeleteSoftDeletes(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	created, err := s.Create(model.NewBead("To delete"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleted, err := s.Delete(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted.Status != model.StatusDeleted {
		t.Errorf("expected status 'deleted', got %q", deleted.Status)
	}

	// Should still be retrievable
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error getting deleted bead: %v", err)
	}
	if got.Status != model.StatusDeleted {
		t.Errorf("expected status 'deleted', got %q", got.Status)
	}
}

func TestAllReturnsAllBeads(t *testing.T) {
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 3; i++ {
		b := model.NewBead("Bead")
		if _, err := s.Create(b); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	all := s.All()
	if len(all) != 3 {
		t.Errorf("expected 3 beads, got %d", len(all))
	}
}

func TestCRUDCycle(t *testing.T) {
	path := tempPath(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create
	created, err := s.Create(model.NewBead("CRUD test"))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// Read
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Title != "CRUD test" {
		t.Errorf("expected title 'CRUD test', got %q", got.Title)
	}

	// Update
	newTitle := "Updated CRUD test"
	updated, err := s.Update(created.ID, UpdateFields{Title: &newTitle})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Title != "Updated CRUD test" {
		t.Errorf("expected updated title, got %q", updated.Title)
	}

	// Delete
	deleted, err := s.Delete(created.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if deleted.Status != model.StatusDeleted {
		t.Errorf("expected deleted status, got %q", deleted.Status)
	}

	// Verify persisted - reload from file
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	got2, err := s2.Get(created.ID)
	if err != nil {
		t.Fatalf("get after reload failed: %v", err)
	}
	if got2.Status != model.StatusDeleted {
		t.Errorf("expected deleted status after reload, got %q", got2.Status)
	}
	if got2.Title != "Updated CRUD test" {
		t.Errorf("expected updated title after reload, got %q", got2.Title)
	}
}

// --- Resolve tests ---

func createBeadWithID(t *testing.T, s *Store, id, title string) model.Bead {
	t.Helper()
	b := model.NewBead(title)
	b.ID = id
	created, err := s.Create(b)
	if err != nil {
		t.Fatalf("failed to create bead %s: %v", id, err)
	}
	return created
}

func TestResolveExactMatch(t *testing.T) {
	s, _ := Load(tempPath(t))
	createBeadWithID(t, s, "bd-a1b2c3d4", "Exact match")

	got, err := s.Resolve("bd-a1b2c3d4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Exact match" {
		t.Errorf("expected title 'Exact match', got %q", got.Title)
	}
}

func TestResolveNotFound(t *testing.T) {
	s, _ := Load(tempPath(t))
	createBeadWithID(t, s, "bd-a1b2c3d4", "Exists")

	_, err := s.Resolve("bd-nonexist")
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}

func TestResolveRequiresExactID(t *testing.T) {
	s, _ := Load(tempPath(t))
	createBeadWithID(t, s, "bd-a1b2c3d4", "Full ID only")

	// Prefix should NOT match
	_, err := s.Resolve("bd-a1b2")
	if err == nil {
		t.Fatal("expected error: prefix matching should not work")
	}

	// Without bd- prefix should NOT match
	_, err = s.Resolve("a1b2c3d4")
	if err == nil {
		t.Fatal("expected error: missing bd- prefix should not auto-prepend")
	}
}

// --- ID generation tests ---

func TestCreateAssignsID(t *testing.T) {
	s, _ := Load(tempPath(t))
	b := model.NewBead("Auto ID")
	created, err := s.Create(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID after Create")
	}
	matched, _ := regexp.MatchString(`^bd-[a-z0-9]{4,8}$`, created.ID)
	if !matched {
		t.Errorf("ID %q does not match expected format bd-[a-z0-9]{4,8}", created.ID)
	}
}

func TestCreatePreservesExplicitID(t *testing.T) {
	s, _ := Load(tempPath(t))
	b := model.NewBead("Explicit ID")
	b.ID = "bd-myid1234"
	created, err := s.Create(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "bd-myid1234" {
		t.Errorf("expected ID 'bd-myid1234', got %q", created.ID)
	}
}

func TestCreateCollisionEscalatesLength(t *testing.T) {
	s, _ := Load(tempPath(t))

	// Fill the store with many 4-char IDs to increase collision probability.
	// We can't guarantee collisions with random generation, but we can verify
	// that generated IDs are always unique and within the valid length range.
	ids := make(map[string]bool)
	for i := 0; i < 50; i++ {
		b := model.NewBead("Bead")
		created, err := s.Create(b)
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if ids[created.ID] {
			t.Fatalf("duplicate ID generated: %s", created.ID)
		}
		ids[created.ID] = true

		matched, _ := regexp.MatchString(`^bd-[a-z0-9]{4,8}$`, created.ID)
		if !matched {
			t.Errorf("ID %q does not match expected format", created.ID)
		}
	}
}

// --- Migration tests ---

func TestLoadMigratesResolvedToClosed(t *testing.T) {
	path := tempPath(t)

	// Write a file with a "resolved" bead using raw JSON (bypassing validation)
	raw := `{"beads":[{"id":"bd-migr0001","title":"Resolved bead","status":"resolved","priority":"medium","type":"task","tags":[],"blocked_by":[],"comments":[],"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, err := s.Get("bd-migr0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != model.StatusClosed {
		t.Errorf("expected migrated status 'closed', got %q", got.Status)
	}
}

func TestLoadMigratesWontfixToClosed(t *testing.T) {
	path := tempPath(t)

	raw := `{"beads":[{"id":"bd-migr0002","title":"Wontfix bead","status":"wontfix","priority":"medium","type":"task","tags":[],"blocked_by":[],"comments":[],"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, err := s.Get("bd-migr0002")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != model.StatusClosed {
		t.Errorf("expected migrated status 'closed', got %q", got.Status)
	}
}

func TestLoadMigratesMixedStatuses(t *testing.T) {
	path := tempPath(t)

	raw := `{"beads":[
		{"id":"bd-migr0001","title":"Resolved","status":"resolved","priority":"medium","type":"task","tags":[],"blocked_by":[],"comments":[],"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"},
		{"id":"bd-migr0002","title":"Wontfix","status":"wontfix","priority":"medium","type":"task","tags":[],"blocked_by":[],"comments":[],"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"},
		{"id":"bd-migr0003","title":"Open","status":"open","priority":"medium","type":"task","tags":[],"blocked_by":[],"comments":[],"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
	]}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	resolved, _ := s.Get("bd-migr0001")
	if resolved.Status != model.StatusClosed {
		t.Errorf("resolved bead: expected 'closed', got %q", resolved.Status)
	}

	wontfix, _ := s.Get("bd-migr0002")
	if wontfix.Status != model.StatusClosed {
		t.Errorf("wontfix bead: expected 'closed', got %q", wontfix.Status)
	}

	open, _ := s.Get("bd-migr0003")
	if open.Status != model.StatusOpen {
		t.Errorf("open bead: expected 'open', got %q", open.Status)
	}
}

func TestAtomicWriteFileExists(t *testing.T) {
	path := tempPath(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := s.Create(model.NewBead("Persist test")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists on disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected data file to exist after create")
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		t.Fatalf("file contains invalid JSON: %v", err)
	}
	if len(fd.Beads) != 1 {
		t.Errorf("expected 1 bead in file, got %d", len(fd.Beads))
	}
}
