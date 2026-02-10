package store

import (
	"sort"
	"time"

	"github.com/yourorg/beads_server/internal/model"
)

// BeadSummary contains the key fields returned by list and search.
type BeadSummary struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Status      model.Status   `json:"status"`
	Priority    model.Priority `json:"priority"`
	Type        model.BeadType `json:"type"`
	Assignee    string         `json:"assignee"`
	UpdatedAt   time.Time      `json:"updated_at"`
	IsEpic      bool           `json:"is_epic,omitempty"`
	Children    []BeadSummary  `json:"children,omitempty"`
	ParentID    string         `json:"parent_id,omitempty"`
	ParentTitle string         `json:"parent_title,omitempty"`
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
// For ready or assignee-filtered listings, returns a flat list of leaf beads.
// Otherwise, returns a hierarchical view with children nested under epics.
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

	// Ready and assignee-filtered (mine) modes produce flat leaf-bead views.
	isFlatMode := filters.Ready || filters.Assignee != nil

	if isFlatMode {
		return s.listFlat(filters, statusSet)
	}
	return s.listHierarchical(filters, statusSet)
}

// listFlat returns a flat list of leaf beads (no epics), with parent context.
// Used for --ready and --mine modes.
func (s *Store) listFlat(filters ListFilters, statusSet map[model.Status]bool) ListResult {
	var matched []model.Bead
	for _, b := range s.beads {
		// Skip epics in flat mode — they are containers, not claimable.
		if s.hasChildren(b.ID) {
			continue
		}
		if !s.matchesFilters(b, statusSet, filters) {
			continue
		}
		matched = append(matched, b)
	}

	sortBeads(matched)

	total := len(matched)
	page := paginate(matched, filters.Page, filters.PerPage)
	summaries := make([]BeadSummary, len(page))
	for i, b := range page {
		sum := summaryFromBead(b)
		if b.ParentID != "" {
			sum.ParentID = b.ParentID
			if parent, ok := s.beads[b.ParentID]; ok {
				sum.ParentTitle = parent.Title
			}
		}
		summaries[i] = sum
	}

	totalPages := (total + filters.PerPage - 1) / filters.PerPage
	if totalPages < 1 {
		totalPages = 1
	}

	return ListResult{
		Beads:      summaries,
		Page:       filters.Page,
		PerPage:    filters.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

// listHierarchical returns epics with nested children and standalone beads.
// Children appear only under their parent, not as top-level items.
func (s *Store) listHierarchical(filters ListFilters, statusSet map[model.Status]bool) ListResult {
	// Collect top-level items: standalone beads and epics.
	// Children are excluded from top-level and nested under their parent.
	var topLevel []model.Bead
	for _, b := range s.beads {
		// Skip children — they will be nested under their parent.
		if b.ParentID != "" {
			continue
		}
		if !s.matchesFilters(b, statusSet, filters) {
			continue
		}
		topLevel = append(topLevel, b)
	}

	sortBeads(topLevel)

	total := len(topLevel)
	page := paginate(topLevel, filters.Page, filters.PerPage)
	summaries := make([]BeadSummary, len(page))
	for i, b := range page {
		sum := summaryFromBead(b)
		children := s.childrenOf(b.ID)
		if len(children) > 0 {
			sum.IsEpic = true
			// Exclude deleted children from list output.
			var childSummaries []BeadSummary
			sortBeads(children)
			for _, c := range children {
				if c.Status == model.StatusDeleted {
					continue
				}
				childSummaries = append(childSummaries, summaryFromBead(c))
			}
			if childSummaries == nil {
				childSummaries = []BeadSummary{}
			}
			sum.Children = childSummaries
		}
		summaries[i] = sum
	}

	totalPages := (total + filters.PerPage - 1) / filters.PerPage
	if totalPages < 1 {
		totalPages = 1
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

	// Ready filter: no active blockers (own or inherited from parent epic)
	if filters.Ready {
		if s.hasActiveBlocker(b) {
			return false
		}
	}

	return true
}

// hasActiveBlocker returns true if the bead has any active blocker,
// either directly or inherited from its parent epic.
// Caller must hold s.mu (at least RLock).
func (s *Store) hasActiveBlocker(b model.Bead) bool {
	// Check own blockers
	for _, blockerID := range b.BlockedBy {
		if blocker, ok := s.beads[blockerID]; ok {
			if isActiveBlocker(blocker.Status) {
				return true
			}
		}
	}
	// Check parent epic's blockers
	if b.ParentID != "" {
		if parent, ok := s.beads[b.ParentID]; ok {
			for _, blockerID := range parent.BlockedBy {
				if blocker, ok := s.beads[blockerID]; ok {
					if isActiveBlocker(blocker.Status) {
						return true
					}
				}
			}
		}
	}
	return false
}

// sortBeads sorts beads by priority (critical first), then created_at (newest first).
func sortBeads(beads []model.Bead) {
	sort.Slice(beads, func(i, j int) bool {
		ri := beads[i].Priority.Rank()
		rj := beads[j].Priority.Rank()
		if ri != rj {
			return ri < rj
		}
		return beads[j].CreatedAt.Before(beads[i].CreatedAt)
	})
}

// paginate returns the slice of beads for the given page.
func paginate(beads []model.Bead, page, perPage int) []model.Bead {
	total := len(beads)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return beads[start:end]
}
