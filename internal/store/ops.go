package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourorg/beads_server/internal/model"
)

// NotFoundError represents a 404 Not Found error for bead lookups.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

// ConflictError represents a 409 Conflict error for claim operations.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}

// Search performs a case-insensitive substring search across title and description.
// Deleted beads are excluded. Results use the same pagination and summary fields as List.
func (s *Store) Search(query string, page, perPage int) ListResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 100
	}

	queryLower := strings.ToLower(query)

	var matched []model.Bead
	for _, b := range s.beads {
		if b.Status == model.StatusDeleted {
			continue
		}
		titleMatch := strings.Contains(strings.ToLower(b.Title), queryLower)
		descMatch := strings.Contains(strings.ToLower(b.Description), queryLower)
		if titleMatch || descMatch {
			matched = append(matched, b)
		}
	}

	// Sort same as list: priority then created_at descending
	sortBeads(matched)

	// Pagination
	total := len(matched)
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	pageSlice := matched[start:end]
	summaries := make([]BeadSummary, len(pageSlice))
	for i, b := range pageSlice {
		sum := summaryFromBead(b)
		if b.ParentID != "" {
			sum.ParentID = b.ParentID
			if parent, ok := s.beads[b.ParentID]; ok {
				sum.ParentTitle = parent.Title
			}
		}
		if s.hasChildren(b.ID) {
			sum.IsEpic = true
		}
		summaries[i] = sum
	}

	return ListResult{
		Beads:      summaries,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

// AddComment appends a comment to a bead and persists.
func (s *Store) AddComment(beadID string, comment model.Comment) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	comment.CreatedAt = time.Now().UTC()
	b.Comments = append(b.Comments, comment)
	b.UpdatedAt = time.Now().UTC()

	old := s.beads[beadID]
	s.beads[beadID] = b

	if err := s.save(); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	return b, nil
}

// Clean permanently removes beads with status closed or deleted whose updated_at
// is older than the given cutoff time. Epics are treated as units: a fully closed
// epic is cleaned with all its children when the most recent updated_at across
// the unit is before cutoff. In-progress epics retain all children. Standalone
// closed beads (no parent, not an epic) are cleaned individually. Children of
// in-progress epics are never cleaned individually.
func (s *Store) Clean(cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removeSet := make(map[string]bool)

	for id, b := range s.beads {
		// Skip beads that aren't in a terminal state.
		if b.Status != model.StatusClosed && b.Status != model.StatusDeleted {
			continue
		}
		// Skip children — they are handled as part of their parent epic unit.
		// Exception: orphaned children (parent no longer exists) are cleaned individually.
		if b.ParentID != "" {
			if _, parentExists := s.beads[b.ParentID]; parentExists {
				continue
			}
			// Orphaned child: parent was hard-deleted. Clean individually.
			if b.UpdatedAt.Before(cutoff) {
				removeSet[id] = true
			}
			continue
		}

		children := s.childrenOf(id)
		if len(children) == 0 {
			// Standalone bead: clean if old enough.
			if b.UpdatedAt.Before(cutoff) {
				removeSet[id] = true
			}
			continue
		}

		// Epic: only clean if fully terminal (all children closed/deleted).
		allTerminal := true
		for _, c := range children {
			if c.Status != model.StatusClosed && c.Status != model.StatusDeleted {
				allTerminal = false
				break
			}
		}
		if !allTerminal {
			continue
		}

		// Check the most recent updated_at across the unit.
		latest := b.UpdatedAt
		for _, c := range children {
			if c.UpdatedAt.After(latest) {
				latest = c.UpdatedAt
			}
		}
		if latest.Before(cutoff) {
			removeSet[id] = true
			for _, c := range children {
				removeSet[c.ID] = true
			}
		}
	}

	if len(removeSet) == 0 {
		return 0, nil
	}

	// Backup for rollback
	removed := make(map[string]model.Bead, len(removeSet))
	for id := range removeSet {
		removed[id] = s.beads[id]
		delete(s.beads, id)
	}

	if err := s.save(); err != nil {
		// Rollback
		for id, b := range removed {
			s.beads[id] = b
		}
		return 0, err
	}

	return len(removeSet), nil
}

// Claim atomically sets a bead's status to in_progress and assignee to the given user.
// Returns ConflictError if the bead is already claimed by a different user, in a terminal state, or not_ready.
// Idempotent: claiming a bead already claimed by the same user succeeds.
func (s *Store) Claim(beadID, user string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	// Check terminal states and not_ready
	switch b.Status {
	case model.StatusClosed, model.StatusDeleted, model.StatusNotReady:
		return model.Bead{}, &ConflictError{
			Message: fmt.Sprintf("bead %s has status %s and cannot be claimed", beadID, b.Status),
		}
	}

	// Check if already in_progress and assigned to different user
	if b.Status == model.StatusInProgress && b.Assignee != "" && b.Assignee != user {
		return model.Bead{}, &ConflictError{
			Message: fmt.Sprintf("bead %s is already claimed by %s", beadID, b.Assignee),
		}
	}

	// Idempotent: already claimed by same user
	if b.Status == model.StatusInProgress && b.Assignee == user {
		return b, nil
	}

	// Perform the claim
	b.Status = model.StatusInProgress
	b.Assignee = user
	b.UpdatedAt = time.Now().UTC()

	old := s.beads[beadID]
	s.beads[beadID] = b

	if err := s.save(); err != nil {
		s.beads[beadID] = old
		return model.Bead{}, err
	}

	return b, nil
}
