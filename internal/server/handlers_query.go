package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

// handleListBeads handles GET /api/v1/beads.
func (s *Server) handleListBeads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := store.ListFilters{
		Page:    intParam(q.Get("page"), 1),
		PerPage: intParam(q.Get("per_page"), 100),
	}

	// Status filter: comma-separated list
	if statuses := q.Get("status"); statuses != "" {
		for _, s := range strings.Split(statuses, ",") {
			st := model.Status(strings.TrimSpace(s))
			if st.Valid() {
				filters.Statuses = append(filters.Statuses, st)
			}
		}
	}

	// Priority filter
	if p := q.Get("priority"); p != "" {
		pri := model.Priority(p)
		if pri.Valid() {
			filters.Priority = &pri
		}
	}

	// Type filter
	if t := q.Get("type"); t != "" {
		bt := model.BeadType(t)
		if bt.Valid() {
			filters.Type = &bt
		}
	}

	// Tag filter: comma-separated
	if tags := q.Get("tag"); tags != "" {
		for _, t := range strings.Split(tags, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				filters.Tags = append(filters.Tags, trimmed)
			}
		}
	}

	// Assignee filter
	if a := q.Get("assignee"); a != "" {
		filters.Assignee = &a
	}

	// All flag
	if q.Get("all") == "true" {
		filters.All = true
	}

	// Ready flag
	if q.Get("ready") == "true" {
		filters.Ready = true
	}

	result := s.Store.List(filters)
	jsonOK(w, result)
}

// handleSearch handles GET /api/v1/search.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	query := q.Get("q")
	if query == "" {
		jsonError(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	page := intParam(q.Get("page"), 1)
	perPage := intParam(q.Get("per_page"), 100)

	result := s.Store.Search(query, page, perPage)
	jsonOK(w, result)
}

// claimRequest is the JSON body for claiming a bead.
type claimRequest struct {
	User string `json:"user"`
}

// handleClaimBead handles POST /api/v1/beads/:id/claim.
func (s *Server) handleClaimBead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Resolve the ID first
	existing, err := s.Store.Resolve(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req claimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.User == "" {
		jsonError(w, "user is required", http.StatusBadRequest)
		return
	}

	claimed, err := s.Store.Claim(existing.ID, req.User)
	if err != nil {
		var conflictErr *store.ConflictError
		if errors.As(err, &conflictErr) {
			jsonError(w, conflictErr.Message, http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonOK(w, claimed)
}

// intParam parses an integer query parameter with a default value.
func intParam(val string, defaultVal int) int {
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
