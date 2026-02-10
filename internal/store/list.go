package store

import (
	"sort"
	"time"

	"github.com/yourorg/beads_server/internal/model"
)

// BeadSummary contains the key fields returned by list and search.
type BeadSummary struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Status    model.Status   `json:"status"`
	Priority  model.Priority `json:"priority"`
	Type      model.BeadType `json:"type"`
	Assignee  string         `json:"assignee"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ListFilters specifies filtering criteria for listing beads.
type ListFilters struct {
	Statuses []model.Status   // Filter by status (OR); empty = default [open, in_progress]
	Priority *model.Priority  // Filter by priority
	Type     *model.BeadType  // Filter by type
	Tags     []string         // Filter by tag (OR semantics)
	Assignee *string          // Filter by assignee
	All      bool             // If true, no status filter
	Ready    bool             // If true, status=open AND no active blockers
	Page     int              // 1-indexed page number (default: 1)
	PerPage  int              // Items per page (default: 100)
}

// ListResult contains the paginated list response.
type ListResult struct {
	Beads      []BeadSummary `json:"beads"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
}

func summaryFromBead(b model.Bead) BeadSummary {
	return BeadSummary{
		ID:        b.ID,
		Title:     b.Title,
		Status:    b.Status,
		Priority:  b.Priority,
		Type:      b.Type,
		Assignee:  b.Assignee,
		UpdatedAt: b.UpdatedAt,
	}
}

// isActiveBlocker returns true if the status counts as an active blocker.
func isActiveBlocker(s model.Status) bool {
	return s == model.StatusOpen || s == model.StatusInProgress
}

// List returns beads matching the given filters, sorted and paginated.
func (s *Store) List(filters ListFilters) ListResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Apply defaults
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PerPage < 1 {
		filters.PerPage = 100
	}

	// Default status filter
	statuses := filters.Statuses
	if filters.Ready {
		statuses = []model.Status{model.StatusOpen}
	} else if !filters.All && len(statuses) == 0 {
		statuses = []model.Status{model.StatusOpen, model.StatusInProgress}
	}

	statusSet := make(map[model.Status]bool, len(statuses))
	for _, st := range statuses {
		statusSet[st] = true
	}

	// Collect matching beads
	var matched []model.Bead
	for _, b := range s.beads {
		if !s.matchesFilters(b, statusSet, filters) {
			continue
		}
		matched = append(matched, b)
	}

	// Sort: priority (critical first), then created_at (newest first)
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
	totalPages := (total + filters.PerPage - 1) / filters.PerPage
	if totalPages < 1 {
		totalPages = 1
	}

	start := (filters.Page - 1) * filters.PerPage
	if start > total {
		start = total
	}
	end := start + filters.PerPage
	if end > total {
		end = total
	}

	page := matched[start:end]
	summaries := make([]BeadSummary, len(page))
	for i, b := range page {
		summaries[i] = summaryFromBead(b)
	}

	return ListResult{
		Beads:      summaries,
		Page:       filters.Page,
		PerPage:    filters.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

// matchesFilters checks if a bead passes all the active filters.
// Caller must hold s.mu (at least RLock).
func (s *Store) matchesFilters(b model.Bead, statusSet map[model.Status]bool, filters ListFilters) bool {
	// Status filter
	if len(statusSet) > 0 && !statusSet[b.Status] {
		return false
	}

	// Priority filter
	if filters.Priority != nil && b.Priority != *filters.Priority {
		return false
	}

	// Type filter
	if filters.Type != nil && b.Type != *filters.Type {
		return false
	}

	// Assignee filter
	if filters.Assignee != nil && b.Assignee != *filters.Assignee {
		return false
	}

	// Tag filter (OR semantics)
	if len(filters.Tags) > 0 {
		found := false
		for _, ft := range filters.Tags {
			for _, bt := range b.Tags {
				if ft == bt {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Ready filter: no active blockers
	if filters.Ready {
		for _, blockerID := range b.BlockedBy {
			if blocker, ok := s.beads[blockerID]; ok {
				if isActiveBlocker(blocker.Status) {
					return false
				}
			}
		}
	}

	return true
}
