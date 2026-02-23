package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vector76/beads_server/internal/model"
	"github.com/vector76/beads_server/internal/store"
)

// jsonError writes a JSON error response with the given status code.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// jsonOK writes a JSON response with status 200.
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// jsonCreated writes a JSON response with status 201.
func jsonCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v)
}

// createRequest is the JSON body for creating a bead.
type createRequest struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      model.Status   `json:"status"`
	Priority    model.Priority `json:"priority"`
	Type        model.BeadType `json:"type"`
	Tags        []string       `json:"tags"`
	BlockedBy   []string       `json:"blocked_by"`
	Assignee    string         `json:"assignee"`
	ParentID    string         `json:"parent_id"`
}

// updateRequest is the JSON body for updating a bead.
type updateRequest struct {
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	Status      *model.Status   `json:"status"`
	Priority    *model.Priority `json:"priority"`
	Type        *model.BeadType `json:"type"`
	Tags        *[]string       `json:"tags"`
	AddTags     []string        `json:"add_tags"`
	RemoveTags  []string        `json:"remove_tags"`
	BlockedBy   *[]string       `json:"blocked_by"`
	Assignee    *string         `json:"assignee"`
	ParentID    *string         `json:"parent_id"`
}

// unblockedResponse wraps a bead with an optional unblocked field.
type unblockedResponse struct {
	model.Bead
	Unblocked []model.Bead `json:"unblocked,omitempty"`
}

// progressInfo contains the progress summary of an epic.
type progressInfo struct {
	Total      int `json:"total"`
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Closed     int `json:"closed"`
	Deleted    int `json:"deleted"`
	NotReady   int `json:"not_ready"`
}

// beadDetailResponse is the enriched response for GET /beads/:id.
type beadDetailResponse struct {
	model.Bead
	IsEpic      bool             `json:"is_epic,omitempty"`
	Progress    *progressInfo    `json:"progress,omitempty"`
	Children    []childSummary   `json:"children,omitempty"`
	ParentTitle string           `json:"parent_title,omitempty"`
}

type childSummary struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Status   model.Status   `json:"status"`
	Priority model.Priority `json:"priority"`
	Type     model.BeadType `json:"type"`
	Assignee string         `json:"assignee"`
}

// handleCreateBead handles POST /api/v1/beads.
func (s *Server) handleCreateBead(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	b := model.NewBead(req.Title)

	if req.Description != "" {
		b.Description = req.Description
	}
	if req.Status != "" {
		if req.Status != model.StatusOpen && req.Status != model.StatusNotReady {
			jsonError(w, "status at creation must be 'open' or 'not_ready'", http.StatusBadRequest)
			return
		}
		b.Status = req.Status
	}
	if req.Priority != "" {
		b.Priority = req.Priority
	}
	if req.Type != "" {
		b.Type = req.Type
	}
	if req.Tags != nil {
		b.Tags = req.Tags
	}
	if req.BlockedBy != nil {
		b.BlockedBy = req.BlockedBy
	}
	if req.Assignee != "" {
		b.Assignee = req.Assignee
	}

	st := s.storeFor(r)

	if req.ParentID != "" {
		created, err := st.CreateWithParent(b, req.ParentID)
		if err != nil {
			code := errorCode(err)
			jsonError(w, err.Error(), code)
			return
		}
		jsonCreated(w, created)
		return
	}

	created, err := st.Create(b)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonCreated(w, created)
}

// handleGetBead handles GET /api/v1/beads/:id.
func (s *Server) handleGetBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	st := s.storeFor(r)
	b, err := st.Resolve(id)
	if err != nil {
		var notFoundErr *store.NotFoundError
		if errors.As(err, &notFoundErr) {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := beadDetailResponse{Bead: b}

	// Epic: add progress + children (including deleted for show)
	children := st.ChildrenOf(b.ID)
	if len(children) > 0 {
		resp.IsEpic = true
		progress := progressInfo{Total: len(children)}
		var childList []childSummary
		for _, c := range children {
			switch c.Status {
			case model.StatusOpen:
				progress.Open++
			case model.StatusInProgress:
				progress.InProgress++
			case model.StatusClosed:
				progress.Closed++
			case model.StatusDeleted:
				progress.Deleted++
			case model.StatusNotReady:
				progress.NotReady++
			}
			childList = append(childList, childSummary{
				ID:       c.ID,
				Title:    c.Title,
				Status:   c.Status,
				Priority: c.Priority,
				Type:     c.Type,
				Assignee: c.Assignee,
			})
		}
		resp.Progress = &progress
		resp.Children = childList
	}

	// Child: add parent_title
	if b.ParentID != "" {
		parent, err := st.Resolve(b.ParentID)
		if err == nil {
			resp.ParentTitle = parent.Title
		}
	}

	jsonOK(w, resp)
}

// handleUpdateBead handles PATCH /api/v1/beads/:id.
func (s *Server) handleUpdateBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	st := s.storeFor(r)

	// Resolve the ID first
	existing, err := st.Resolve(id)
	if err != nil {
		var notFoundErr *store.NotFoundError
		if errors.As(err, &notFoundErr) {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Handle parent_id changes (move operations) via dedicated store methods.
	if req.ParentID != nil {
		newParent := *req.ParentID
		if newParent == "" {
			// Move out
			updated, err := st.MoveOut(existing.ID)
			if err != nil {
				code := errorCode(err)
				jsonError(w, err.Error(), code)
				return
			}
			jsonOK(w, updated)
			return
		}
		// Move into
		updated, err := st.MoveInto(existing.ID, newParent)
		if err != nil {
			code := errorCode(err)
			jsonError(w, err.Error(), code)
			return
		}
		jsonOK(w, updated)
		return
	}

	// Reject status changes on epics.
	if req.Status != nil {
		if err := st.ValidateStatusChangeOnEpic(existing.ID); err != nil {
			var conflictErr *store.ConflictError
			if errors.As(err, &conflictErr) {
				jsonError(w, conflictErr.Message, http.StatusConflict)
				return
			}
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	fields := store.UpdateFields{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		Type:        req.Type,
		Tags:        req.Tags,
		BlockedBy:   req.BlockedBy,
		Assignee:    req.Assignee,
	}

	// Handle add_tags / remove_tags
	if len(req.AddTags) > 0 || len(req.RemoveTags) > 0 {
		tags := existing.Tags

		// Add tags (avoid duplicates)
		for _, t := range req.AddTags {
			found := false
			for _, et := range tags {
				if et == t {
					found = true
					break
				}
			}
			if !found {
				tags = append(tags, t)
			}
		}

		// Remove tags
		if len(req.RemoveTags) > 0 {
			removeSet := make(map[string]bool, len(req.RemoveTags))
			for _, t := range req.RemoveTags {
				removeSet[t] = true
			}
			filtered := make([]string, 0, len(tags))
			for _, t := range tags {
				if !removeSet[t] {
					filtered = append(filtered, t)
				}
			}
			tags = filtered
		}

		fields.Tags = &tags
	}

	updated, err := st.Update(existing.ID, fields)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Recompute parent epic status if this is a child and status changed.
	if req.Status != nil && existing.ParentID != "" {
		st.RecomputeParentStatus(existing.ID)
	}

	// Check if status changed to a terminal state and compute unblocked
	if req.Status != nil && isTerminalStatus(*req.Status) {
		unblocked := st.GetUnblocked(existing.ID)
		if len(unblocked) > 0 {
			jsonOK(w, unblockedResponse{Bead: updated, Unblocked: unblocked})
			return
		}
	}

	jsonOK(w, updated)
}

// handleDeleteBead handles DELETE /api/v1/beads/:id.
func (s *Server) handleDeleteBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	st := s.storeFor(r)

	// Resolve the ID first
	existing, err := st.Resolve(id)
	if err != nil {
		var notFoundErr *store.NotFoundError
		if errors.As(err, &notFoundErr) {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Reject delete on epics with open children.
	if err := st.ValidateDeleteOnEpic(existing.ID); err != nil {
		var conflictErr *store.ConflictError
		if errors.As(err, &conflictErr) {
			jsonError(w, conflictErr.Message, http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	deleted, err := st.Delete(existing.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Recompute parent epic status if this is a child.
	if existing.ParentID != "" {
		st.RecomputeParentStatus(existing.ID)
	}

	// Compute unblocked beads
	unblocked := st.GetUnblocked(existing.ID)
	if len(unblocked) > 0 {
		jsonOK(w, unblockedResponse{Bead: deleted, Unblocked: unblocked})
		return
	}

	jsonOK(w, deleted)
}

// isTerminalStatus returns true for statuses that could unblock other beads.
func isTerminalStatus(s model.Status) bool {
	switch s {
	case model.StatusClosed, model.StatusDeleted:
		return true
	}
	return false
}

// errorCode returns the appropriate HTTP status code for a store error.
func errorCode(err error) int {
	var notFoundErr *store.NotFoundError
	if errors.As(err, &notFoundErr) {
		return http.StatusNotFound
	}
	var conflictErr *store.ConflictError
	if errors.As(err, &conflictErr) {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}
