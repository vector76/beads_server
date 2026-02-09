package store

import (
	"fmt"

	"github.com/yourorg/beads_server/internal/model"
)

// DepsResult holds the dependency information for a bead.
type DepsResult struct {
	ActiveBlockers   []model.Bead `json:"active_blockers"`
	ResolvedBlockers []model.Bead `json:"resolved_blockers"`
	Blocks           []model.Bead `json:"blocks"`
}

// Link adds blockedByID to beadID's blocked_by list.
// Rejects self-links, non-existent/deleted targets, duplicates, and circular dependencies.
func (s *Store) Link(beadID, blockedByID string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Self-link check
	if beadID == blockedByID {
		return model.Bead{}, fmt.Errorf("cannot link bead to itself")
	}

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	target, ok := s.beads[blockedByID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", blockedByID)}
	}

	// Reject deleted targets
	if target.Status == model.StatusDeleted {
		return model.Bead{}, fmt.Errorf("cannot link to deleted bead %s", blockedByID)
	}

	// Duplicate check
	for _, id := range b.BlockedBy {
		if id == blockedByID {
			return model.Bead{}, fmt.Errorf("bead %s already blocked by %s", beadID, blockedByID)
		}
	}

	// Circular dependency check: would blockedByID be transitively blocked by beadID?
	if s.wouldCreateCycle(beadID, blockedByID) {
		return model.Bead{}, fmt.Errorf("circular dependency: %s is already blocked by %s (directly or transitively)", blockedByID, beadID)
	}

	old := s.beads[beadID]

	// Build a new slice to avoid mutating the shared backing array.
	newBlocked := make([]string, len(b.BlockedBy)+1)
	copy(newBlocked, b.BlockedBy)
	newBlocked[len(b.BlockedBy)] = blockedByID
	b.BlockedBy = newBlocked

	s.beads[beadID] = b

	if err := s.save(); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	return b, nil
}

// wouldCreateCycle checks whether adding beadID->blockedByID would create a cycle.
// It walks the blocked_by chain starting from blockedByID to see if beadID is reachable.
// Caller must hold s.mu.
func (s *Store) wouldCreateCycle(beadID, blockedByID string) bool {
	visited := make(map[string]bool)
	queue := []string{blockedByID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == beadID {
			return true
		}
		if visited[current] {
			continue
		}
		visited[current] = true

		if b, ok := s.beads[current]; ok {
			queue = append(queue, b.BlockedBy...)
		}
	}

	return false
}

// Unlink removes blockedByID from beadID's blocked_by list.
func (s *Store) Unlink(beadID, blockedByID string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	idx := -1
	for i, id := range b.BlockedBy {
		if id == blockedByID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return model.Bead{}, fmt.Errorf("bead %s is not blocked by %s", beadID, blockedByID)
	}

	old := s.beads[beadID]

	// Build a new slice to avoid mutating the shared backing array.
	newBlocked := make([]string, 0, len(b.BlockedBy)-1)
	newBlocked = append(newBlocked, b.BlockedBy[:idx]...)
	newBlocked = append(newBlocked, b.BlockedBy[idx+1:]...)
	b.BlockedBy = newBlocked

	s.beads[beadID] = b

	if err := s.save(); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	return b, nil
}

// Deps returns the dependency information for a bead:
// active blockers, resolved blockers, and beads this one blocks (inverse lookup).
func (s *Store) Deps(beadID string) (DepsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.beads[beadID]
	if !ok {
		return DepsResult{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	var active, resolved []model.Bead
	for _, blockerID := range b.BlockedBy {
		blocker, ok := s.beads[blockerID]
		if !ok {
			continue
		}
		if isActiveBlocker(blocker.Status) {
			active = append(active, blocker)
		} else {
			resolved = append(resolved, blocker)
		}
	}

	// Inverse lookup: find beads that have beadID in their blocked_by list.
	// Only include active (non-deleted) beads.
	var blocks []model.Bead
	for _, other := range s.beads {
		if other.Status == model.StatusDeleted {
			continue
		}
		for _, depID := range other.BlockedBy {
			if depID == beadID {
				blocks = append(blocks, other)
				break
			}
		}
	}

	if active == nil {
		active = []model.Bead{}
	}
	if resolved == nil {
		resolved = []model.Bead{}
	}
	if blocks == nil {
		blocks = []model.Bead{}
	}

	return DepsResult{
		ActiveBlockers:   active,
		ResolvedBlockers: resolved,
		Blocks:           blocks,
	}, nil
}

// GetUnblocked is a public wrapper around ComputeUnblocked that acquires the read lock.
func (s *Store) GetUnblocked(beadID string) []model.Bead {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ComputeUnblocked(beadID)
}

// ComputeUnblocked finds beads that were blocked only by the given bead and are now
// unblocked because that bead reached a terminal state (closed/resolved/wontfix/deleted).
// Caller must hold s.mu (at least RLock).
func (s *Store) ComputeUnblocked(beadID string) []model.Bead {
	var unblocked []model.Bead

	for _, b := range s.beads {
		if b.Status == model.StatusDeleted {
			continue
		}

		// Only consider beads that reference the changed bead
		referencesChanged := false
		for _, depID := range b.BlockedBy {
			if depID == beadID {
				referencesChanged = true
				break
			}
		}
		if !referencesChanged {
			continue
		}

		// Check if all remaining blockers are now inactive
		hasActiveBlocker := false
		for _, depID := range b.BlockedBy {
			if dep, ok := s.beads[depID]; ok {
				if isActiveBlocker(dep.Status) {
					hasActiveBlocker = true
					break
				}
			}
		}

		if !hasActiveBlocker {
			unblocked = append(unblocked, b)
		}
	}

	if unblocked == nil {
		unblocked = []model.Bead{}
	}

	return unblocked
}
