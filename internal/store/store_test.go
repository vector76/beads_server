package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
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

	// Write a valid data file
	b := model.NewBead("Existing bead")
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

	got, err := s.Get(b.ID)
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
	if created.ID != b.ID {
		t.Errorf("expected ID %q, got %q", b.ID, created.ID)
	}

	got, err := s.Get(b.ID)
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
	if _, err := s.Create(b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = s.Create(b)
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
	if _, err := s.Create(b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newTitle := "Updated title"
	newPriority := model.PriorityHigh
	updated, err := s.Update(b.ID, UpdateFields{
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
	if !updated.UpdatedAt.After(b.CreatedAt) || updated.UpdatedAt.Equal(b.CreatedAt) {
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

	b := model.NewBead("To delete")
	if _, err := s.Create(b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleted, err := s.Delete(b.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted.Status != model.StatusDeleted {
		t.Errorf("expected status 'deleted', got %q", deleted.Status)
	}

	// Should still be retrievable
	got, err := s.Get(b.ID)
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
	b := model.NewBead("CRUD test")
	created, err := s.Create(b)
	if err != nil {
		t.Fatalf("create failed: %v", err)
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

func TestAtomicWriteFileExists(t *testing.T) {
	path := tempPath(t)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b := model.NewBead("Persist test")
	if _, err := s.Create(b); err != nil {
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
