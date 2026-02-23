package store

import (
	"fmt"
	"time"

	"github.com/vector76/beads_server/internal/model"
)

// childrenOf returns all beads whose ParentID equals the given id.
// Caller must hold s.mu (at least RLock).
func (s *Store) childrenOf(parentID string) []model.Bead {
	var children []model.Bead
	for _, b := range s.beads {
		if b.ParentID == parentID {
			children = append(children, b)
		}
	}
	return children
}

// hasChildren returns true if any bead has parentID == id.
// Caller must hold s.mu (at least RLock).
func (s *Store) hasChildren(id string) bool {
	for _, b := range s.beads {
		if b.ParentID == id {
			return true
		}
	}
	return false
}

// IsEpic returns true if the bead with the given ID has any children.
func (s *Store) IsEpic(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasChildren(id)
}

// ChildrenOf returns all children of the given bead.
func (s *Store) ChildrenOf(parentID string) []model.Bead {
	s.mu.RLock()
	defer s.mu.RUnlock()
	children := s.childrenOf(parentID)
	if children == nil {
		return []model.Bead{}
	}
	return children
}

// deriveEpicStatus computes what the epic's status should be based on its children.
// Caller must hold s.mu (at least RLock).
func (s *Store) deriveEpicStatus(epicID string) model.Status {
	children := s.childrenOf(epicID)
	if len(children) == 0 {
		// No children => not an epic; this shouldn't be called, but return open as default.
		return model.StatusOpen
	}

	allTerminal := true
	hasInProgress := false
	hasOpen := false
	hasNotReady := false
	for _, c := range children {
		if c.Status != model.StatusClosed && c.Status != model.StatusDeleted {
			allTerminal = false
		}
		switch c.Status {
		case model.StatusInProgress:
			hasInProgress = true
		case model.StatusOpen:
			hasOpen = true
		case model.StatusNotReady:
			hasNotReady = true
		}
	}

	if allTerminal {
		return model.StatusClosed
	}
	if hasInProgress {
		return model.StatusInProgress
	}
	if hasOpen {
		return model.StatusOpen
	}
	if hasNotReady {
		return model.StatusNotReady
	}
	// fallback: mixed terminal + not_ready
	return model.StatusInProgress
}

// recomputeEpicStatus recomputes and persists the derived status for the given epic.
// If the epic has no children left, it reverts to a regular bead with status "open".
// Caller must hold s.mu (write lock).
func (s *Store) recomputeEpicStatus(epicID string) error {
	epic, ok := s.beads[epicID]
	if !ok {
		return nil
	}

	children := s.childrenOf(epicID)
	if len(children) == 0 {
		// No children remain — revert to a regular bead with status open.
		epic.Status = model.StatusOpen
		epic.UpdatedAt = time.Now().UTC()
		s.beads[epicID] = epic
		return s.save()
	}

	newStatus := s.deriveEpicStatus(epicID)
	if epic.Status != newStatus {
		epic.Status = newStatus
		epic.UpdatedAt = time.Now().UTC()
		s.beads[epicID] = epic
		return s.save()
	}
	return nil
}

// ValidateCreateWithParent validates the parent_id when creating a child bead.
// Caller must hold s.mu (at least RLock).
func (s *Store) validateCreateWithParent(parentID string) error {
	parent, ok := s.beads[parentID]
	if !ok {
		return &NotFoundError{Message: fmt.Sprintf("parent bead %s not found", parentID)}
	}
	if parent.Status == model.StatusDeleted {
		return fmt.Errorf("cannot add child to deleted bead %s", parentID)
	}
	// Target must not itself be a child (single-level nesting).
	if parent.ParentID != "" {
		return &ConflictError{Message: "cannot nest epics; target is already a child of another bead"}
	}
	return nil
}

// CreateWithParent creates a bead with a parent_id, validating nesting constraints.
func (s *Store) CreateWithParent(b model.Bead, parentID string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateCreateWithParent(parentID); err != nil {
		return model.Bead{}, err
	}

	b.ParentID = parentID
	if b.ID == "" {
		b.ID = s.generateUniqueID()
	} else if _, exists := s.beads[b.ID]; exists {
		return model.Bead{}, fmt.Errorf("bead %s already exists", b.ID)
	}

	s.beads[b.ID] = b
	if err := s.save(); err != nil {
		delete(s.beads, b.ID)
		return model.Bead{}, err
	}

	// Recompute parent epic status (new open child may change it).
	if err := s.recomputeEpicStatus(parentID); err != nil {
		// Rollback the creation
		delete(s.beads, b.ID)
		return model.Bead{}, err
	}

	return b, nil
}

// MoveInto moves a bead into an epic (sets parent_id).
// Validates nesting constraints and parent-child blocking rules.
func (s *Store) MoveInto(beadID, targetID string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	target, ok := s.beads[targetID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", targetID)}
	}

	if target.Status == model.StatusDeleted {
		return model.Bead{}, fmt.Errorf("cannot move into deleted bead %s", targetID)
	}

	// Source must not have children (cannot nest an epic inside another epic).
	if s.hasChildren(beadID) {
		return model.Bead{}, &ConflictError{Message: "cannot nest epics; bead already has children"}
	}

	// Target must not itself be a child (single-level nesting).
	if target.ParentID != "" {
		return model.Bead{}, &ConflictError{Message: "cannot nest epics; target is already a child of another bead"}
	}

	// Already a child of this target?
	if b.ParentID == targetID {
		return model.Bead{}, &ConflictError{Message: "bead is already a child of this epic"}
	}

	// Check for blocking relationships between bead and target in either direction.
	for _, blockerID := range b.BlockedBy {
		if blockerID == targetID {
			return model.Bead{}, &ConflictError{
				Message: "cannot move into an epic that blocks or is blocked by this bead; this creates a deadlock",
			}
		}
	}
	for _, blockerID := range target.BlockedBy {
		if blockerID == beadID {
			return model.Bead{}, &ConflictError{
				Message: "cannot move into an epic that blocks or is blocked by this bead; this creates a deadlock",
			}
		}
	}

	oldParent := b.ParentID
	old := b

	b.ParentID = targetID
	b.UpdatedAt = time.Now().UTC()
	s.beads[beadID] = b

	if err := s.save(); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	// Recompute new parent's status.
	if err := s.recomputeEpicStatus(targetID); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	// Recompute old parent's status if applicable.
	if oldParent != "" && oldParent != targetID {
		if err := s.recomputeEpicStatus(oldParent); err != nil {
			// Best effort — the move itself succeeded.
			return b, nil
		}
	}

	return s.beads[beadID], nil
}

// MoveOut detaches a bead from its parent epic (clears parent_id).
func (s *Store) MoveOut(beadID string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	if b.ParentID == "" {
		return model.Bead{}, fmt.Errorf("bead %s has no parent", beadID)
	}

	oldParent := b.ParentID
	old := b

	b.ParentID = ""
	b.UpdatedAt = time.Now().UTC()
	s.beads[beadID] = b

	if err := s.save(); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	// Recompute old parent's status.
	if err := s.recomputeEpicStatus(oldParent); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	return s.beads[beadID], nil
}

// ValidateEpicConstraints checks whether a mutation is valid for epic-related constraints.
// Returns an error if:
// - Attempting to set status on an epic (status is derived)
// - Attempting to claim an epic
// - Attempting to delete an epic with open children
func (s *Store) ValidateStatusChangeOnEpic(beadID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.hasChildren(beadID) {
		return &ConflictError{Message: "cannot set status on an epic; status is derived from children"}
	}
	return nil
}

// ValidateClaimOnEpic returns an error if the bead is an epic.
func (s *Store) ValidateClaimOnEpic(beadID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.hasChildren(beadID) {
		return &ConflictError{Message: "cannot claim an epic; claim individual children"}
	}
	return nil
}

// ValidateDeleteOnEpic returns an error if the bead is an epic with open children.
func (s *Store) ValidateDeleteOnEpic(beadID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasChildren(beadID) {
		return nil
	}
	children := s.childrenOf(beadID)
	for _, c := range children {
		if c.Status == model.StatusOpen || c.Status == model.StatusInProgress || c.Status == model.StatusNotReady {
			return &ConflictError{Message: "cannot delete epic with open children; close or delete children first"}
		}
	}
	return nil
}

// ValidateLinkParentChild returns an error if one bead is the parent of the other.
func (s *Store) ValidateLinkParentChild(beadID, blockedByID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.beads[beadID]
	if !ok {
		return nil // Let normal validation handle not-found
	}
	b, ok := s.beads[blockedByID]
	if !ok {
		return nil
	}

	if a.ParentID == blockedByID || b.ParentID == beadID {
		return &ConflictError{
			Message: "cannot add dependency between an epic and its own children; this creates a deadlock",
		}
	}
	return nil
}

// RecomputeParentStatus recomputes the parent epic status after a child mutation.
// This is the public entry point called after Update/Delete operations on a child.
func (s *Store) RecomputeParentStatus(childID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	child, ok := s.beads[childID]
	if !ok {
		return nil
	}
	if child.ParentID == "" {
		return nil
	}
	return s.recomputeEpicStatus(child.ParentID)
}
