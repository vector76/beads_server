package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/beads_server/internal/model"
)

// commentRequest is the JSON body for adding a comment.
type commentRequest struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

// linkRequest is the JSON body for adding a dependency.
type linkRequest struct {
	BlockedBy string `json:"blocked_by"`
}

// handleAddComment handles POST /api/v1/beads/:id/comments.
func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := s.storeFor(r).Resolve(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req commentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Author == "" {
		jsonError(w, "author is required", http.StatusBadRequest)
		return
	}
	if req.Text == "" {
		jsonError(w, "text is required", http.StatusBadRequest)
		return
	}

	comment := model.Comment{
		Author: req.Author,
		Text:   req.Text,
	}

	updated, err := s.storeFor(r).AddComment(existing.ID, comment)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonCreated(w, updated)
}

// handleLinkBead handles POST /api/v1/beads/:id/link.
func (s *Server) handleLinkBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := s.storeFor(r).Resolve(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req linkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.BlockedBy == "" {
		jsonError(w, "blocked_by is required", http.StatusBadRequest)
		return
	}

	// Resolve the target ID as well
	target, err := s.storeFor(r).Resolve(req.BlockedBy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := s.storeFor(r).Link(existing.ID, target.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonOK(w, updated)
}

// handleUnlinkBead handles DELETE /api/v1/beads/:id/link/:other_id.
func (s *Server) handleUnlinkBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	otherID := chi.URLParam(r, "other_id")

	existing, err := s.storeFor(r).Resolve(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Resolve the other ID as well
	other, err := s.storeFor(r).Resolve(otherID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := s.storeFor(r).Unlink(existing.ID, other.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonOK(w, updated)
}

// handleGetDeps handles GET /api/v1/beads/:id/deps.
func (s *Server) handleGetDeps(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := s.storeFor(r).Resolve(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	deps, err := s.storeFor(r).Deps(existing.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonOK(w, deps)
}
