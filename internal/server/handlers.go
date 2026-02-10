package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
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
}

// unblockedResponse wraps a bead with an optional unblocked field.
type unblockedResponse struct {
	model.Bead
	Unblocked []model.Bead `json:"unblocked,omitempty"`
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

	created, err := s.storeFor(r).Create(b)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonCreated(w, created)
}

// handleGetBead handles GET /api/v1/beads/:id.
func (s *Server) handleGetBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	b, err := s.storeFor(r).Resolve(id)
	if err != nil {
		var notFoundErr *store.NotFoundError
		if errors.As(err, &notFoundErr) {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonOK(w, b)
}

// handleUpdateBead handles PATCH /api/v1/beads/:id.
func (s *Server) handleUpdateBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Resolve the ID first
	existing, err := s.storeFor(r).Resolve(id)
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

	updated, err := s.storeFor(r).Update(existing.ID, fields)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if status changed to a terminal state and compute unblocked
	if req.Status != nil && isTerminalStatus(*req.Status) {
		unblocked := s.storeFor(r).GetUnblocked(existing.ID)
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

	// Resolve the ID first
	existing, err := s.storeFor(r).Resolve(id)
	if err != nil {
		var notFoundErr *store.NotFoundError
		if errors.As(err, &notFoundErr) {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	deleted, err := s.storeFor(r).Delete(existing.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Compute unblocked beads
	unblocked := s.storeFor(r).GetUnblocked(existing.ID)
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
