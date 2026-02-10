package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yourorg/beads_server/internal/model"
)

// Store holds beads in memory and persists them to a JSON file.
type Store struct {
	mu       sync.RWMutex
	beads    map[string]model.Bead
	filePath string
}

// fileData is the on-disk JSON format.
type fileData struct {
	Beads []model.Bead `json:"beads"`
}

// rawBead mirrors model.Bead but uses a plain string for Status so that
// legacy values ("resolved", "wontfix") survive JSON unmarshaling and can
// be migrated to "closed" at load time.
type rawBead struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Priority    model.Priority  `json:"priority"`
	Type        model.BeadType  `json:"type"`
	Tags        []string        `json:"tags"`
	BlockedBy   []string        `json:"blocked_by"`
	Assignee    string          `json:"assignee"`
	Comments    []model.Comment `json:"comments"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Load reads beads from the given file path, or initializes an empty store
// if the file does not exist. Legacy statuses "resolved" and "wontfix" are
// silently migrated to "closed" at load time.
func Load(path string) (*Store, error) {
	s := &Store{
		beads:    make(map[string]model.Bead),
		filePath: path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading data file: %w", err)
	}

	var fd struct {
		Beads []rawBead `json:"beads"`
	}
	if err := json.Unmarshal(data, &fd); err != nil {
		return nil, fmt.Errorf("parsing data file: %w", err)
	}

	for _, rb := range fd.Beads {
		status := model.Status(rb.Status)
		// Migrate legacy statuses to closed.
		if status == "resolved" || status == "wontfix" {
			status = model.StatusClosed
		}
		s.beads[rb.ID] = model.Bead{
			ID:          rb.ID,
			Title:       rb.Title,
			Description: rb.Description,
			Status:      status,
			Priority:    rb.Priority,
			Type:        rb.Type,
			Tags:        rb.Tags,
			BlockedBy:   rb.BlockedBy,
			Assignee:    rb.Assignee,
			Comments:    rb.Comments,
			CreatedAt:   rb.CreatedAt,
			UpdatedAt:   rb.UpdatedAt,
		}
	}

	return s, nil
}

// save writes all beads to disk atomically (temp file + rename).
// Caller must hold s.mu.
func (s *Store) save() error {
	beads := make([]model.Bead, 0, len(s.beads))
	for _, b := range s.beads {
		beads = append(beads, b)
	}

	fd := fileData{Beads: beads}
	data, err := json.MarshalIndent(fd, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	tmp, err := os.CreateTemp(dir, "beads-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// Create adds a bead to the store and persists to disk.
func (s *Store) Create(b model.Bead) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.beads[b.ID]; exists {
		return model.Bead{}, fmt.Errorf("bead %s already exists", b.ID)
	}

	s.beads[b.ID] = b
	if err := s.save(); err != nil {
		delete(s.beads, b.ID)
		return model.Bead{}, err
	}

	return b, nil
}

// Get returns a bead by exact ID.
func (s *Store) Get(id string) (model.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.beads[id]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", id)}
	}
	return b, nil
}

// Resolve finds a bead by exact ID or unique prefix match.
// Accepts both "bd-xxx" and "xxx" forms (auto-prepends "bd-" if missing).
// Returns an error if the prefix is ambiguous (listing matching IDs) or not found.
func (s *Store) Resolve(prefix string) (model.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Auto-prepend bd- if missing
	if !strings.HasPrefix(prefix, "bd-") {
		prefix = "bd-" + prefix
	}

	// Try exact match first
	if b, ok := s.beads[prefix]; ok {
		return b, nil
	}

	// Try prefix match
	var matches []model.Bead
	for id, b := range s.beads {
		if strings.HasPrefix(id, prefix) {
			matches = append(matches, b)
		}
	}

	switch len(matches) {
	case 0:
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", prefix)}
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, b := range matches {
			ids[i] = b.ID
		}
		return model.Bead{}, fmt.Errorf("ambiguous prefix %s: matches %s", prefix, strings.Join(ids, ", "))
	}
}

// UpdateFields specifies which fields to update on a bead.
// Nil pointer fields are left unchanged.
type UpdateFields struct {
	Title       *string
	Description *string
	Status      *model.Status
	Priority    *model.Priority
	Type        *model.BeadType
	Tags        *[]string
	BlockedBy   *[]string
	Assignee    *string
}

// Update applies partial updates to a bead, sets updated_at, and persists.
func (s *Store) Update(id string, fields UpdateFields) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.beads[id]
	if !ok {
		return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", id)}
	}

	if fields.Title != nil {
		b.Title = *fields.Title
	}
	if fields.Description != nil {
		b.Description = *fields.Description
	}
	if fields.Status != nil {
		b.Status = *fields.Status
	}
	if fields.Priority != nil {
		b.Priority = *fields.Priority
	}
	if fields.Type != nil {
		b.Type = *fields.Type
	}
	if fields.Tags != nil {
		b.Tags = *fields.Tags
	}
	if fields.BlockedBy != nil {
		b.BlockedBy = *fields.BlockedBy
	}
	if fields.Assignee != nil {
		b.Assignee = *fields.Assignee
	}

	b.UpdatedAt = time.Now().UTC()
	old := s.beads[id]
	s.beads[id] = b

	if err := s.save(); err != nil {
		s.beads[id] = old
		return model.Bead{}, err
	}

	return b, nil
}

// Delete soft-deletes a bead by setting its status to deleted.
func (s *Store) Delete(id string) (model.Bead, error) {
	status := model.StatusDeleted
	return s.Update(id, UpdateFields{Status: &status})
}

// All returns all beads in the store.
func (s *Store) All() []model.Bead {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]model.Bead, 0, len(s.beads))
	for _, b := range s.beads {
		result = append(result, b)
	}
	return result
}
