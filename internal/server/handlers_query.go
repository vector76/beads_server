package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vector76/beads_server/internal/model"
	"github.com/vector76/beads_server/internal/store"
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

	result := s.storeFor(r).List(filters)
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

	result := s.storeFor(r).Search(query, page, perPage)
	jsonOK(w, result)
}

// claimRequest is the JSON body for claiming a bead.
type claimRequest struct {
	User string `json:"user"`
}

// handleClaimBead handles POST /api/v1/beads/:id/claim.
func (s *Server) handleClaimBead(w http.ResponseWriter, r *http.Request) {
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

	// Reject claim on epics.
	if err := st.ValidateClaimOnEpic(existing.ID); err != nil {
		var conflictErr *store.ConflictError
		if errors.As(err, &conflictErr) {
			jsonError(w, conflictErr.Message, http.StatusConflict)
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

	claimed, err := st.Claim(existing.ID, req.User)
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

// cleanRequest is the JSON body for the clean operation.
type cleanRequest struct {
	Days *float64 `json:"days"`
}

// cleanResponse is the JSON response for the clean operation.
type cleanResponse struct {
	Removed int `json:"removed"`
}

// handleClean handles POST /api/v1/clean.
func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	var req cleanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	days := 5.0
	if req.Days != nil {
		if *req.Days < 0 {
			jsonError(w, "days must be non-negative", http.StatusBadRequest)
			return
		}
		days = *req.Days
	}

	cutoff := time.Now().UTC().Add(-time.Duration(days * 24 * float64(time.Hour)))

	removed, err := s.storeFor(r).Clean(cutoff)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, cleanResponse{Removed: removed})
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
