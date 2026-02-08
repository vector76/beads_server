package store

import (
	"path/filepath"
	"testing"

	"github.com/yourorg/beads_server/internal/model"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Load(filepath.Join(dir, "beads.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func createBead(t *testing.T, s *Store, title string) model.Bead {
	t.Helper()
	b := model.NewBead(title)
	b, err := s.Create(b)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return b
}

// --- Link tests ---

func TestLink_Success(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	updated, err := s.Link(a.ID, b.ID)
	if err != nil {
		t.Fatalf("Link: %v", err)
	}

	if len(updated.BlockedBy) != 1 || updated.BlockedBy[0] != b.ID {
		t.Fatalf("expected blocked_by = [%s], got %v", b.ID, updated.BlockedBy)
	}

	// Verify persisted
	got, _ := s.Get(a.ID)
	if len(got.BlockedBy) != 1 {
		t.Fatal("link not persisted")
	}
}

func TestLink_SelfLink(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")

	_, err := s.Link(a.ID, a.ID)
	if err == nil {
		t.Fatal("expected error for self-link")
	}
}

func TestLink_NonExistentTarget(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")

	_, err := s.Link(a.ID, "bd-nonexist")
	if err == nil {
		t.Fatal("expected error for non-existent target")
	}
}

func TestLink_DeletedTarget(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	s.Delete(b.ID)

	_, err := s.Link(a.ID, b.ID)
	if err == nil {
		t.Fatal("expected error for deleted target")
	}
}

func TestLink_Duplicate(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	_, err := s.Link(a.ID, b.ID)
	if err != nil {
		t.Fatalf("first Link: %v", err)
	}

	_, err = s.Link(a.ID, b.ID)
	if err == nil {
		t.Fatal("expected error for duplicate link")
	}
}

func TestLink_CircularDirect(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	// A blocked by B
	_, err := s.Link(a.ID, b.ID)
	if err != nil {
		t.Fatalf("Link A->B: %v", err)
	}

	// B blocked by A would create a cycle
	_, err = s.Link(b.ID, a.ID)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
}

func TestLink_CircularTransitive(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")
	c := createBead(t, s, "C")

	// A blocked by B, B blocked by C
	if _, err := s.Link(a.ID, b.ID); err != nil {
		t.Fatalf("Link A->B: %v", err)
	}
	if _, err := s.Link(b.ID, c.ID); err != nil {
		t.Fatalf("Link B->C: %v", err)
	}

	// C blocked by A would create a cycle: A->B->C->A
	_, err := s.Link(c.ID, a.ID)
	if err == nil {
		t.Fatal("expected error for transitive circular dependency")
	}
}

func TestLink_NonExistentSource(t *testing.T) {
	s := tempStore(t)
	b := createBead(t, s, "B")

	_, err := s.Link("bd-nonexist", b.ID)
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
}

// --- Unlink tests ---

func TestUnlink_Success(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	s.Link(a.ID, b.ID)

	updated, err := s.Unlink(a.ID, b.ID)
	if err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	if len(updated.BlockedBy) != 0 {
		t.Fatalf("expected empty blocked_by, got %v", updated.BlockedBy)
	}
}

func TestUnlink_NotLinked(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	_, err := s.Unlink(a.ID, b.ID)
	if err == nil {
		t.Fatal("expected error for unlinking non-linked beads")
	}
}

func TestUnlink_NonExistentSource(t *testing.T) {
	s := tempStore(t)

	_, err := s.Unlink("bd-nonexist", "bd-other")
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
}

// --- Deps tests ---

func TestDeps_Structure(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")
	c := createBead(t, s, "C")
	d := createBead(t, s, "D")

	// A is blocked by B (active) and C (will resolve)
	s.Link(a.ID, b.ID)
	s.Link(a.ID, c.ID)

	// D is blocked by A (so A "blocks" D)
	s.Link(d.ID, a.ID)

	// Resolve C
	resolved := model.StatusResolved
	s.Update(c.ID, UpdateFields{Status: &resolved})

	deps, err := s.Deps(a.ID)
	if err != nil {
		t.Fatalf("Deps: %v", err)
	}

	if len(deps.ActiveBlockers) != 1 || deps.ActiveBlockers[0].ID != b.ID {
		t.Fatalf("expected 1 active blocker (B), got %v", deps.ActiveBlockers)
	}

	if len(deps.ResolvedBlockers) != 1 || deps.ResolvedBlockers[0].ID != c.ID {
		t.Fatalf("expected 1 resolved blocker (C), got %v", deps.ResolvedBlockers)
	}

	if len(deps.Blocks) != 1 || deps.Blocks[0].ID != d.ID {
		t.Fatalf("expected 1 blocked bead (D), got %v", deps.Blocks)
	}
}

func TestDeps_ExcludesDeletedFromBlocks(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	// B is blocked by A
	s.Link(b.ID, a.ID)

	// Delete B
	s.Delete(b.ID)

	deps, err := s.Deps(a.ID)
	if err != nil {
		t.Fatalf("Deps: %v", err)
	}

	// Deleted bead should not appear in blocks
	if len(deps.Blocks) != 0 {
		t.Fatalf("expected empty blocks (deleted excluded), got %v", deps.Blocks)
	}
}

func TestDeps_EmptyResult(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")

	deps, err := s.Deps(a.ID)
	if err != nil {
		t.Fatalf("Deps: %v", err)
	}

	if len(deps.ActiveBlockers) != 0 {
		t.Fatalf("expected empty active blockers")
	}
	if len(deps.ResolvedBlockers) != 0 {
		t.Fatalf("expected empty resolved blockers")
	}
	if len(deps.Blocks) != 0 {
		t.Fatalf("expected empty blocks")
	}
}

func TestDeps_NonExistent(t *testing.T) {
	s := tempStore(t)

	_, err := s.Deps("bd-nonexist")
	if err == nil {
		t.Fatal("expected error for non-existent bead")
	}
}

// --- ComputeUnblocked tests ---

func TestComputeUnblocked_OnResolve(t *testing.T) {
	s := tempStore(t)
	blocker := createBead(t, s, "Blocker")
	blocked := createBead(t, s, "Blocked")

	// blocked is blocked by blocker
	s.Link(blocked.ID, blocker.ID)

	// Resolve the blocker
	resolved := model.StatusResolved
	s.Update(blocker.ID, UpdateFields{Status: &resolved})

	// Compute unblocked - need read lock since ComputeUnblocked expects caller to hold lock
	s.mu.RLock()
	result := s.ComputeUnblocked(blocker.ID)
	s.mu.RUnlock()

	if len(result) != 1 || result[0].ID != blocked.ID {
		t.Fatalf("expected [%s] unblocked, got %v", blocked.ID, result)
	}
}

func TestComputeUnblocked_StillBlocked(t *testing.T) {
	s := tempStore(t)
	blocker1 := createBead(t, s, "Blocker1")
	blocker2 := createBead(t, s, "Blocker2")
	blocked := createBead(t, s, "Blocked")

	// blocked is blocked by both blockers
	s.Link(blocked.ID, blocker1.ID)
	s.Link(blocked.ID, blocker2.ID)

	// Resolve only blocker1
	resolved := model.StatusResolved
	s.Update(blocker1.ID, UpdateFields{Status: &resolved})

	s.mu.RLock()
	result := s.ComputeUnblocked(blocker1.ID)
	s.mu.RUnlock()

	// Still blocked by blocker2, so should not be unblocked
	if len(result) != 0 {
		t.Fatalf("expected no unblocked beads, got %v", result)
	}
}

func TestComputeUnblocked_OnClose(t *testing.T) {
	s := tempStore(t)
	blocker := createBead(t, s, "Blocker")
	blocked := createBead(t, s, "Blocked")

	s.Link(blocked.ID, blocker.ID)

	closed := model.StatusClosed
	s.Update(blocker.ID, UpdateFields{Status: &closed})

	s.mu.RLock()
	result := s.ComputeUnblocked(blocker.ID)
	s.mu.RUnlock()

	if len(result) != 1 || result[0].ID != blocked.ID {
		t.Fatalf("expected [%s] unblocked, got %v", blocked.ID, result)
	}
}

func TestComputeUnblocked_ExcludesDeleted(t *testing.T) {
	s := tempStore(t)
	blocker := createBead(t, s, "Blocker")
	blocked := createBead(t, s, "Blocked")

	s.Link(blocked.ID, blocker.ID)

	// Delete the blocked bead first
	s.Delete(blocked.ID)

	// Now resolve the blocker
	resolved := model.StatusResolved
	s.Update(blocker.ID, UpdateFields{Status: &resolved})

	s.mu.RLock()
	result := s.ComputeUnblocked(blocker.ID)
	s.mu.RUnlock()

	// Deleted bead should not appear as unblocked
	if len(result) != 0 {
		t.Fatalf("expected no unblocked beads (deleted excluded), got %v", result)
	}
}

func TestComputeUnblocked_MultipleUnblocked(t *testing.T) {
	s := tempStore(t)
	blocker := createBead(t, s, "Blocker")
	b1 := createBead(t, s, "Blocked1")
	b2 := createBead(t, s, "Blocked2")

	s.Link(b1.ID, blocker.ID)
	s.Link(b2.ID, blocker.ID)

	resolved := model.StatusResolved
	s.Update(blocker.ID, UpdateFields{Status: &resolved})

	s.mu.RLock()
	result := s.ComputeUnblocked(blocker.ID)
	s.mu.RUnlock()

	if len(result) != 2 {
		t.Fatalf("expected 2 unblocked beads, got %d", len(result))
	}
}

// Verify persistence of Link operations survives store reload
func TestLink_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "beads.json")

	s, _ := Load(path)
	a := model.NewBead("A")
	b := model.NewBead("B")
	s.Create(a)
	s.Create(b)
	s.Link(a.ID, b.ID)

	// Reload store from disk
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, _ := s2.Get(a.ID)
	if len(got.BlockedBy) != 1 || got.BlockedBy[0] != b.ID {
		t.Fatalf("link not persisted across reload: blocked_by = %v", got.BlockedBy)
	}
}
