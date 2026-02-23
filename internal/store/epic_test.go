package store

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/vector76/beads_server/internal/model"
)

// --- CreateWithParent tests ---

func TestCreateWithParent_Success(t *testing.T) {
	s := tempStore(t)
	parent := createBead(t, s, "Epic")
	child := model.NewBead("Child task")

	created, err := s.CreateWithParent(child, parent.ID)
	if err != nil {
		t.Fatalf("CreateWithParent: %v", err)
	}
	if created.ParentID != parent.ID {
		t.Errorf("expected parent_id %s, got %s", parent.ID, created.ParentID)
	}
	if !s.IsEpic(parent.ID) {
		t.Error("expected parent to be an epic")
	}
}

func TestCreateWithParent_ParentNotFound(t *testing.T) {
	s := tempStore(t)
	child := model.NewBead("Orphan")

	_, err := s.CreateWithParent(child, "bd-nonexist")
	if err == nil {
		t.Fatal("expected error for non-existent parent")
	}
	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestCreateWithParent_DeletedParent(t *testing.T) {
	s := tempStore(t)
	parent := createBead(t, s, "Deleted parent")
	s.Delete(parent.ID)

	child := model.NewBead("Child")
	_, err := s.CreateWithParent(child, parent.ID)
	if err == nil {
		t.Fatal("expected error for deleted parent")
	}
}

func TestCreateWithParent_ParentIsChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)

	// Try to create under the child (would be nesting)
	_, err := s.CreateWithParent(model.NewBead("Grandchild"), child.ID)
	if err == nil {
		t.Fatal("expected error for nesting (parent is a child)")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestCreateWithParent_ReopensClosedEpic(t *testing.T) {
	s := tempStore(t)
	parent := createBead(t, s, "Epic")
	child1, _ := s.CreateWithParent(model.NewBead("Child 1"), parent.ID)

	// Close the child -> epic closes
	closed := model.StatusClosed
	s.Update(child1.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(child1.ID)

	got, _ := s.Get(parent.ID)
	if got.Status != model.StatusClosed {
		t.Fatalf("expected epic to be closed, got %s", got.Status)
	}

	// Add new child -> epic reopens
	_, err := s.CreateWithParent(model.NewBead("Child 2"), parent.ID)
	if err != nil {
		t.Fatalf("CreateWithParent: %v", err)
	}

	got, _ = s.Get(parent.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected epic to be open after adding open child (one closed + one open), got %s", got.Status)
	}
}

// --- Derived status tests ---

func TestDeriveEpicStatus_AllOpen(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("C1"), epic.ID)
	s.CreateWithParent(model.NewBead("C2"), epic.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected open, got %s", got.Status)
	}
}

func TestDeriveEpicStatus_AllClosed(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.Update(c2.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}
}

func TestDeriveEpicStatus_Mixed(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	s.CreateWithParent(model.NewBead("C2"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c1.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected open (one closed + one open child), got %s", got.Status)
	}
}

func TestDeriveEpicStatus_DeletedTreatedAsTerminal(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.Delete(c2.ID)
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Errorf("expected closed (deleted + closed = all terminal), got %s", got.Status)
	}
}

// --- MoveInto tests ---

func TestMoveInto_Success(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	bead := createBead(t, s, "Standalone")

	moved, err := s.MoveInto(bead.ID, epic.ID)
	if err != nil {
		t.Fatalf("MoveInto: %v", err)
	}
	if moved.ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, moved.ParentID)
	}
	if !s.IsEpic(epic.ID) {
		t.Error("expected epic to be an epic after move")
	}
}

func TestMoveInto_TargetNotFound(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Bead")

	_, err := s.MoveInto(bead.ID, "bd-nonexist")
	if err == nil {
		t.Fatal("expected error")
	}
	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMoveInto_TargetDeleted(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.Delete(epic.ID)
	bead := createBead(t, s, "Bead")

	_, err := s.MoveInto(bead.ID, epic.ID)
	if err == nil {
		t.Fatal("expected error for deleted target")
	}
}

func TestMoveInto_SourceHasChildren(t *testing.T) {
	s := tempStore(t)
	epic1 := createBead(t, s, "Epic1")
	s.CreateWithParent(model.NewBead("Child"), epic1.ID)
	epic2 := createBead(t, s, "Epic2")

	_, err := s.MoveInto(epic1.ID, epic2.ID)
	if err == nil {
		t.Fatal("expected error: source has children")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestMoveInto_TargetIsChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)
	bead := createBead(t, s, "Standalone")

	_, err := s.MoveInto(bead.ID, child.ID)
	if err == nil {
		t.Fatal("expected error: target is a child")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestMoveInto_AlreadyChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)

	_, err := s.MoveInto(child.ID, epic.ID)
	if err == nil {
		t.Fatal("expected error: already a child")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestMoveInto_BlockingRelationship(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	bead := createBead(t, s, "Bead")

	// bead blocked by epic
	s.Link(bead.ID, epic.ID)

	_, err := s.MoveInto(bead.ID, epic.ID)
	if err == nil {
		t.Fatal("expected error: blocking relationship exists")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestMoveInto_ReverseBlockingRelationship(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	bead := createBead(t, s, "Bead")

	// epic blocked by bead
	s.Link(epic.ID, bead.ID)

	_, err := s.MoveInto(bead.ID, epic.ID)
	if err == nil {
		t.Fatal("expected error: reverse blocking relationship exists")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestMoveInto_Reparenting(t *testing.T) {
	s := tempStore(t)
	epic1 := createBead(t, s, "Epic1")
	epic2 := createBead(t, s, "Epic2")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic1.ID)

	// Move child from epic1 to epic2
	moved, err := s.MoveInto(child.ID, epic2.ID)
	if err != nil {
		t.Fatalf("MoveInto: %v", err)
	}
	if moved.ParentID != epic2.ID {
		t.Errorf("expected parent_id %s, got %s", epic2.ID, moved.ParentID)
	}

	// Old epic should revert to non-epic
	if s.IsEpic(epic1.ID) {
		t.Error("expected epic1 to no longer be an epic")
	}
	// Old epic should be open (reverted)
	got, _ := s.Get(epic1.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected epic1 status open after losing last child, got %s", got.Status)
	}
}

// --- MoveOut tests ---

func TestMoveOut_Success(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)

	moved, err := s.MoveOut(child.ID)
	if err != nil {
		t.Fatalf("MoveOut: %v", err)
	}
	if moved.ParentID != "" {
		t.Errorf("expected empty parent_id, got %s", moved.ParentID)
	}
}

func TestMoveOut_NoParent(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Standalone")

	_, err := s.MoveOut(bead.ID)
	if err == nil {
		t.Fatal("expected error: no parent")
	}
}

func TestMoveOut_LastChildRevertsEpic(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)

	s.MoveOut(child.ID)

	if s.IsEpic(epic.ID) {
		t.Error("expected former epic to no longer be an epic")
	}
	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected status open after losing last child, got %s", got.Status)
	}
}

func TestMoveOut_NotFound(t *testing.T) {
	s := tempStore(t)

	_, err := s.MoveOut("bd-nonexist")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Validation tests ---

func TestValidateStatusChangeOnEpic_Rejected(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("Child"), epic.ID)

	err := s.ValidateStatusChangeOnEpic(epic.ID)
	if err == nil {
		t.Fatal("expected error: cannot change status on epic")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestValidateStatusChangeOnEpic_AllowedOnLeaf(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Leaf")

	err := s.ValidateStatusChangeOnEpic(bead.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateClaimOnEpic_Rejected(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("Child"), epic.ID)

	err := s.ValidateClaimOnEpic(epic.ID)
	if err == nil {
		t.Fatal("expected error: cannot claim epic")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestValidateDeleteOnEpic_OpenChildren(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("Open child"), epic.ID)

	err := s.ValidateDeleteOnEpic(epic.ID)
	if err == nil {
		t.Fatal("expected error: epic has open children")
	}
}

func TestValidateDeleteOnEpic_AllTerminalChildren(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.Delete(c2.ID)

	err := s.ValidateDeleteOnEpic(epic.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDeleteOnEpic_NotAnEpic(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Leaf")

	err := s.ValidateDeleteOnEpic(bead.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeriveEpicStatus_AllNotReady(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	notReady := model.StatusNotReady
	s.Update(c1.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c1.ID)
	s.Update(c2.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusNotReady {
		t.Errorf("expected not_ready, got %s", got.Status)
	}
}

func TestDeriveEpicStatus_OpenAndNotReady(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	s.CreateWithParent(model.NewBead("C2"), epic.ID)

	notReady := model.StatusNotReady
	s.Update(c1.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c1.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected open, got %s", got.Status)
	}
}

func TestDeriveEpicStatus_InProgressAndNotReady(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	inProgress := model.StatusInProgress
	s.Update(c1.ID, UpdateFields{Status: &inProgress})
	s.RecomputeParentStatus(c1.ID)
	notReady := model.StatusNotReady
	s.Update(c2.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusInProgress {
		t.Errorf("expected in_progress, got %s", got.Status)
	}
}

func TestDeriveEpicStatus_SomeClosedSomeNotReady(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c1.ID)
	notReady := model.StatusNotReady
	s.Update(c2.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusNotReady {
		t.Errorf("expected not_ready (mixed terminal+not_ready), got %s", got.Status)
	}
}

func TestDeriveEpicStatus_AllClosed_WereNotReady(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	notReady := model.StatusNotReady
	s.Update(c1.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c1.ID)
	s.Update(c2.ID, UpdateFields{Status: &notReady})
	s.RecomputeParentStatus(c2.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c1.ID)
	s.Update(c2.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}
}

func TestValidateDeleteOnEpic_NotReadyChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)

	notReady := model.StatusNotReady
	s.Update(c1.ID, UpdateFields{Status: &notReady})

	err := s.ValidateDeleteOnEpic(epic.ID)
	if err == nil {
		t.Fatal("expected error: epic has not_ready child")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestValidateDeleteOnEpic_AllClosedAfterNotReady(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)

	notReady := model.StatusNotReady
	s.Update(c1.ID, UpdateFields{Status: &notReady})

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})

	err := s.ValidateDeleteOnEpic(epic.ID)
	if err != nil {
		t.Fatalf("unexpected error after closing child: %v", err)
	}
}

func TestValidateLinkParentChild_ChildBlockedByParent(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)

	err := s.ValidateLinkParentChild(child.ID, epic.ID)
	if err == nil {
		t.Fatal("expected error: parent-child blocking")
	}
}

func TestValidateLinkParentChild_ParentBlockedByChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	child, _ := s.CreateWithParent(model.NewBead("Child"), epic.ID)

	err := s.ValidateLinkParentChild(epic.ID, child.ID)
	if err == nil {
		t.Fatal("expected error: parent-child blocking")
	}
}

func TestValidateLinkParentChild_UnrelatedAllowed(t *testing.T) {
	s := tempStore(t)
	a := createBead(t, s, "A")
	b := createBead(t, s, "B")

	err := s.ValidateLinkParentChild(a.ID, b.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RecomputeParentStatus tests ---

func TestRecomputeParentStatus_ClosingLastChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c1.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}
}

func TestRecomputeParentStatus_ReopeningChild(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)

	closed := model.StatusClosed
	s.Update(c1.ID, UpdateFields{Status: &closed})
	s.Update(c2.ID, UpdateFields{Status: &closed})
	s.RecomputeParentStatus(c2.ID)

	got, _ := s.Get(epic.ID)
	if got.Status != model.StatusClosed {
		t.Fatalf("expected closed, got %s", got.Status)
	}

	// Reopen c1
	open := model.StatusOpen
	s.Update(c1.ID, UpdateFields{Status: &open})
	s.RecomputeParentStatus(c1.ID)

	got, _ = s.Get(epic.ID)
	if got.Status != model.StatusOpen {
		t.Errorf("expected open after reopening one child (one open + one closed), got %s", got.Status)
	}
}

func TestRecomputeParentStatus_NoParent(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Standalone")

	// Should be a no-op, no error
	err := s.RecomputeParentStatus(bead.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- IsEpic / ChildrenOf tests ---

func TestIsEpic_WithChildren(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("Child"), epic.ID)

	if !s.IsEpic(epic.ID) {
		t.Error("expected true")
	}
}

func TestIsEpic_WithoutChildren(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Leaf")

	if s.IsEpic(bead.ID) {
		t.Error("expected false")
	}
}

func TestChildrenOf_ReturnsAll(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("C1"), epic.ID)
	s.CreateWithParent(model.NewBead("C2"), epic.ID)
	s.CreateWithParent(model.NewBead("C3"), epic.ID)

	children := s.ChildrenOf(epic.ID)
	if len(children) != 3 {
		t.Errorf("expected 3 children, got %d", len(children))
	}
}

func TestChildrenOf_NoChildren(t *testing.T) {
	s := tempStore(t)
	bead := createBead(t, s, "Leaf")

	children := s.ChildrenOf(bead.ID)
	if len(children) != 0 {
		t.Errorf("expected 0 children, got %d", len(children))
	}
}

// --- List hierarchical tests ---

func TestListHierarchical_ChildrenNestedUnderEpic(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("C1"), epic.ID)
	s.CreateWithParent(model.NewBead("C2"), epic.ID)
	standalone := createBead(t, s, "Standalone")
	_ = standalone

	result := s.List(ListFilters{})
	// Top-level: epic + standalone = 2
	if result.Total != 2 {
		t.Fatalf("expected 2 top-level, got %d", result.Total)
	}

	// Find the epic in results
	var epicSummary *BeadSummary
	for i, b := range result.Beads {
		if b.ID == epic.ID {
			epicSummary = &result.Beads[i]
			break
		}
	}
	if epicSummary == nil {
		t.Fatal("epic not found in results")
	}
	if !epicSummary.IsEpic {
		t.Error("expected is_epic=true")
	}
	if len(epicSummary.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(epicSummary.Children))
	}
}

func TestListHierarchical_DeletedChildrenExcluded(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("C1"), epic.ID)
	c2, _ := s.CreateWithParent(model.NewBead("C2"), epic.ID)
	s.Delete(c2.ID)

	result := s.List(ListFilters{})
	var epicSummary *BeadSummary
	for i, b := range result.Beads {
		if b.ID == epic.ID {
			epicSummary = &result.Beads[i]
			break
		}
	}
	if epicSummary == nil {
		t.Fatal("epic not found")
	}
	// Deleted child excluded from list children
	if len(epicSummary.Children) != 1 {
		t.Errorf("expected 1 child (deleted excluded), got %d", len(epicSummary.Children))
	}
}

func TestListFlat_ReadyExcludesEpics(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	c1, _ := s.CreateWithParent(model.NewBead("C1"), epic.ID)
	_ = c1

	result := s.List(ListFilters{Ready: true})
	for _, b := range result.Beads {
		if b.ID == epic.ID {
			t.Error("epic should not appear in ready list")
		}
	}
	if result.Total != 1 {
		t.Errorf("expected 1 ready bead, got %d", result.Total)
	}
}

func TestListFlat_ReadyChildHasParentContext(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "My Epic")
	s.CreateWithParent(model.NewBead("Ready child"), epic.ID)

	result := s.List(ListFilters{Ready: true})
	if result.Total != 1 {
		t.Fatalf("expected 1, got %d", result.Total)
	}
	if result.Beads[0].ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, result.Beads[0].ParentID)
	}
	if result.Beads[0].ParentTitle != "My Epic" {
		t.Errorf("expected parent_title 'My Epic', got %q", result.Beads[0].ParentTitle)
	}
}

func TestListFlat_ReadyInheritsEpicBlockers(t *testing.T) {
	s := tempStore(t)
	blocker := createBead(t, s, "Blocker")
	epic := createBead(t, s, "Epic")
	s.Link(epic.ID, blocker.ID) // epic blocked by blocker
	s.CreateWithParent(model.NewBead("Child"), epic.ID)

	result := s.List(ListFilters{Ready: true})
	// Child should NOT be ready because parent epic is blocked
	for _, b := range result.Beads {
		if b.ParentID == epic.ID {
			t.Error("child of blocked epic should not be ready")
		}
	}
}

// --- Clean with epics tests ---

func TestClean_EpicAndChildrenCleanedTogether(t *testing.T) {
	s := tempStore(t)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	// Create all with old timestamps and terminal status, including parent_id
	epic := newBeadWithFields("bd-epic0001", "Old epic", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(epic)
	c1 := newBeadWithFields("bd-chld0001", "Old child 1", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	c1.ParentID = "bd-epic0001"
	s.Create(c1)
	c2 := newBeadWithFields("bd-chld0002", "Old child 2", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	c2.ParentID = "bd-epic0001"
	s.Create(c2)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	// Epic + 2 children = 3
	if removed != 3 {
		t.Errorf("expected 3 removed, got %d", removed)
	}
}

func TestClean_InProgressEpicRetainsChildren(t *testing.T) {
	s := tempStore(t)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	epic := newBeadWithFields("bd-epic0001", "In-progress epic", model.StatusInProgress, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(epic)
	c1 := newBeadWithFields("bd-chld0001", "Old closed child", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	c1.ParentID = "bd-epic0001"
	s.Create(c1)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, _ := s.Clean(cutoff)
	if removed != 0 {
		t.Errorf("expected 0 removed (epic is in-progress), got %d", removed)
	}
}

func TestClean_EpicRetainedIfChildRecent(t *testing.T) {
	s := tempStore(t)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	recent := time.Now().UTC().Add(-1 * 24 * time.Hour)

	epic := newBeadWithFields("bd-epic0001", "Epic", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(epic)
	c1 := newBeadWithFields("bd-chld0001", "Old child", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	c1.ParentID = "bd-epic0001"
	s.Create(c1)
	c2 := newBeadWithFields("bd-chld0002", "Recent child", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, recent)
	c2.ParentID = "bd-epic0001"
	s.Create(c2)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, _ := s.Clean(cutoff)
	if removed != 0 {
		t.Errorf("expected 0 removed (recent child), got %d", removed)
	}
}

func TestClean_ChildNotCleanedIndividually(t *testing.T) {
	s := tempStore(t)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	epic := newBeadWithFields("bd-epic0001", "Active epic", model.StatusInProgress, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(epic)
	c1 := newBeadWithFields("bd-chld0001", "Old closed child", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	c1.ParentID = "bd-epic0001"
	s.Create(c1)
	c2 := newBeadWithFields("bd-chld0002", "Open child", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	c2.ParentID = "bd-epic0001"
	s.Create(c2)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, _ := s.Clean(cutoff)
	// Even though c1 is old and closed, it shouldn't be cleaned individually
	if removed != 0 {
		t.Errorf("expected 0 removed (child of active epic), got %d", removed)
	}
}

func TestClean_OrphanedChildCleaned(t *testing.T) {
	s := tempStore(t)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	// Create a child whose parent doesn't exist (orphaned)
	orphan := newBeadWithFields("bd-orph0001", "Orphaned child", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	orphan.ParentID = "bd-gone0001" // parent doesn't exist
	s.Create(orphan)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed (orphaned child), got %d", removed)
	}
}

// --- Migration test ---

func TestLoadMigratesEpicTypeToTask(t *testing.T) {
	path := tempPath(t)

	raw := `{"beads":[{"id":"bd-migr0001","title":"Epic type bead","status":"open","priority":"medium","type":"epic","tags":[],"blocked_by":[],"comments":[],"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]}`
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
	if got.Type != model.TypeTask {
		t.Errorf("expected migrated type 'task', got %q", got.Type)
	}
}

// --- Persistence test ---

func TestEpicPersistence(t *testing.T) {
	path := tempPath(t)
	s, _ := Load(path)

	epic := model.NewBead("Epic")
	epic.ID = "bd-epic0001"
	s.Create(epic)

	child := model.NewBead("Child")
	s.CreateWithParent(child, "bd-epic0001")

	// Reload and verify
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !s2.IsEpic("bd-epic0001") {
		t.Error("expected epic after reload")
	}

	children := s2.ChildrenOf("bd-epic0001")
	if len(children) != 1 {
		t.Errorf("expected 1 child after reload, got %d", len(children))
	}
	if children[0].ParentID != "bd-epic0001" {
		t.Errorf("expected parent_id bd-epic0001, got %s", children[0].ParentID)
	}
}

// --- Filtering applies to top-level only ---

func TestListHierarchical_FilterAppliesToTopLevelOnly(t *testing.T) {
	s := tempStore(t)

	// Create a feature epic with a bug child
	featureType := model.TypeFeature
	epic := createBead(t, s, "Feature Epic")
	s.Update(epic.ID, UpdateFields{Type: &featureType})

	bugType := model.TypeBug
	child := model.NewBead("Bug child")
	child.Type = model.TypeBug
	s.CreateWithParent(child, epic.ID)

	// Create a standalone bug
	standaloneBug := createBead(t, s, "Standalone bug")
	s.Update(standaloneBug.ID, UpdateFields{Type: &bugType})

	// Filter by type=bug: should return only the standalone bug, not the feature epic
	result := s.List(ListFilters{Type: &bugType})
	if result.Total != 1 {
		t.Fatalf("expected 1 top-level bug, got %d", result.Total)
	}
	if result.Beads[0].ID != standaloneBug.ID {
		t.Errorf("expected standalone bug %s, got %s", standaloneBug.ID, result.Beads[0].ID)
	}
}

func TestListHierarchical_FilterByTagTopLevelOnly(t *testing.T) {
	s := tempStore(t)

	// Create epic with tag "backend"
	epic := createBead(t, s, "Backend Epic")
	tags := []string{"backend"}
	s.Update(epic.ID, UpdateFields{Tags: &tags})

	// Create child with tag "frontend"
	child := model.NewBead("Frontend child")
	child.Tags = []string{"frontend"}
	s.CreateWithParent(child, epic.ID)

	// Filter by tag=frontend: should return 0 (child has the tag, not a top-level item)
	result := s.List(ListFilters{Tags: []string{"frontend"}})
	if result.Total != 0 {
		t.Errorf("expected 0 top-level with 'frontend' tag, got %d", result.Total)
	}

	// Filter by tag=backend: should return the epic with its child nested
	result = s.List(ListFilters{Tags: []string{"backend"}})
	if result.Total != 1 {
		t.Fatalf("expected 1 top-level with 'backend' tag, got %d", result.Total)
	}
	if !result.Beads[0].IsEpic {
		t.Error("expected is_epic=true")
	}
	if len(result.Beads[0].Children) != 1 {
		t.Errorf("expected 1 child nested under epic, got %d", len(result.Beads[0].Children))
	}
}

func TestListHierarchical_FilterByAssigneeTopLevelOnly(t *testing.T) {
	s := tempStore(t)

	// Note: when assignee filter is set, it triggers flat mode.
	// This test verifies that flat mode with assignee correctly
	// excludes epics and shows only leaf beads.
	epic := createBead(t, s, "Epic")
	child := model.NewBead("Child task")
	created, _ := s.CreateWithParent(child, epic.ID)

	// Claim the child
	alice := "alice"
	inProg := model.StatusInProgress
	s.Update(created.ID, UpdateFields{Assignee: &alice, Status: &inProg})

	result := s.List(ListFilters{Assignee: &alice})
	if result.Total != 1 {
		t.Fatalf("expected 1 result for assignee filter, got %d", result.Total)
	}
	if result.Beads[0].ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, result.Beads[0].ParentID)
	}
}

// --- Mine excludes epics ---

func TestListFlat_MineExcludesEpics(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Epic")
	s.CreateWithParent(model.NewBead("Child"), epic.ID)

	// Even if an epic somehow had an assignee and in_progress status,
	// the flat list mode should exclude it because it has children.
	alice := "alice"
	inProg := model.StatusInProgress
	s.Update(epic.ID, UpdateFields{Assignee: &alice, Status: &inProg})

	result := s.List(ListFilters{Assignee: &alice})
	for _, b := range result.Beads {
		if b.ID == epic.ID {
			t.Error("epic should not appear in assignee-filtered (mine) results")
		}
	}
}

// --- Search with epics ---

func TestSearch_IncludesParentContext(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Auth Rewrite")
	child := model.NewBead("Fix login bug")
	child.Description = "Fix the login authentication issue"
	s.CreateWithParent(child, epic.ID)

	result := s.Search("login", 1, 100)
	if result.Total != 1 {
		t.Fatalf("expected 1 result, got %d", result.Total)
	}
	if result.Beads[0].ParentID != epic.ID {
		t.Errorf("expected parent_id %s, got %s", epic.ID, result.Beads[0].ParentID)
	}
	if result.Beads[0].ParentTitle != "Auth Rewrite" {
		t.Errorf("expected parent_title 'Auth Rewrite', got %q", result.Beads[0].ParentTitle)
	}
}

func TestSearch_EpicShowsIsEpic(t *testing.T) {
	s := tempStore(t)
	epic := createBead(t, s, "Auth Rewrite")
	s.CreateWithParent(model.NewBead("Subtask"), epic.ID)

	result := s.Search("Auth", 1, 100)
	if result.Total != 1 {
		t.Fatalf("expected 1 result, got %d", result.Total)
	}
	if !result.Beads[0].IsEpic {
		t.Error("expected is_epic=true in search result")
	}
}
