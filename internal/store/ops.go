package store

import (
	"fmt"
	"sort"
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
	sort.Slice(matched, func(i, j int) bool {
		ri := matched[i].Priority.Rank()
		rj := matched[j].Priority.Rank()
		if ri != rj {
			return ri < rj
		}
		return matched[j].CreatedAt.Before(matched[i].CreatedAt)
	})

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
		summaries[i] = summaryFromBead(b)
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

// Claim atomically sets a bead's status to in_progress and assignee to the given user.
// Returns ConflictError if the bead is already claimed by a different user or in a terminal state.
// Idempotent: claiming a bead already claimed by the same user succeeds.
func (s *Store) Claim(beadID, user string) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[beadID]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", beadID)}
	}

	// Check terminal states
	switch b.Status {
	case model.StatusClosed, model.StatusResolved, model.StatusWontfix, model.StatusDeleted:
		return model.Bead{}, &ConflictError{
			Message: fmt.Sprintf("bead %s is %s and cannot be claimed", beadID, b.Status),
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
