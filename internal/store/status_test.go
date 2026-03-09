package store

import (
	"testing"

	"github.com/vector76/beads_server/internal/model"
)

func TestStatusMapAllExist(t *testing.T) {
	s, _ := Load(tempPath(t))

	b1 := model.NewBead("Open bead")
	b1.ID = "bd-sm0001"
	if _, err := s.Create(b1); err != nil {
		t.Fatalf("create b1: %v", err)
	}

	b2 := model.NewBead("In progress bead")
	b2.ID = "bd-sm0002"
	b2.Status = model.StatusInProgress
	if _, err := s.Create(b2); err != nil {
		t.Fatalf("create b2: %v", err)
	}

	b3 := model.NewBead("Deleted bead")
	b3.ID = "bd-sm0003"
	b3.Status = model.StatusDeleted
	if _, err := s.Create(b3); err != nil {
		t.Fatalf("create b3: %v", err)
	}

	result := s.StatusMap([]string{"bd-sm0001", "bd-sm0002", "bd-sm0003"})

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if result["bd-sm0001"] != "open" {
		t.Errorf("bd-sm0001: expected 'open', got %q", result["bd-sm0001"])
	}
	if result["bd-sm0002"] != "in_progress" {
		t.Errorf("bd-sm0002: expected 'in_progress', got %q", result["bd-sm0002"])
	}
	if result["bd-sm0003"] != "deleted" {
		t.Errorf("bd-sm0003: expected 'deleted', got %q", result["bd-sm0003"])
	}
}

func TestStatusMapMixedExistingAndMissing(t *testing.T) {
	s, _ := Load(tempPath(t))

	b := model.NewBead("Exists")
	b.ID = "bd-sm0010"
	if _, err := s.Create(b); err != nil {
		t.Fatalf("create b: %v", err)
	}

	result := s.StatusMap([]string{"bd-sm0010", "bd-missing1", "bd-missing2"})

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result["bd-sm0010"] != "open" {
		t.Errorf("bd-sm0010: expected 'open', got %q", result["bd-sm0010"])
	}
	if _, found := result["bd-missing1"]; found {
		t.Error("bd-missing1 should be absent from result")
	}
	if _, found := result["bd-missing2"]; found {
		t.Error("bd-missing2 should be absent from result")
	}
}

func TestStatusMapEmptyInput(t *testing.T) {
	s, _ := Load(tempPath(t))

	result := s.StatusMap([]string{})

	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestStatusMapDuplicateIDs(t *testing.T) {
	s, _ := Load(tempPath(t))

	b := model.NewBead("Dup bead")
	b.ID = "bd-sm0020"
	if _, err := s.Create(b); err != nil {
		t.Fatalf("create b: %v", err)
	}

	result := s.StatusMap([]string{"bd-sm0020", "bd-sm0020", "bd-sm0020"})

	if len(result) != 1 {
		t.Fatalf("expected 1 entry (map deduplicates), got %d", len(result))
	}
	if result["bd-sm0020"] != "open" {
		t.Errorf("bd-sm0020: expected 'open', got %q", result["bd-sm0020"])
	}
}
