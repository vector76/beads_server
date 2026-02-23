package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vector76/beads_server/internal/model"
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

// rawBead mirrors model.Bead but uses a plain string for Status and Type so
// that legacy values ("resolved", "wontfix", "epic") survive JSON unmarshaling
// and can be migrated at load time.
type rawBead struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Priority    model.Priority  `json:"priority"`
	Type        string          `json:"type"`
	Tags        []string        `json:"tags"`
	BlockedBy   []string        `json:"blocked_by"`
	Assignee    string          `json:"assignee"`
	ParentID    string          `json:"parent_id"`
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
		// Migrate legacy "epic" type to "task".
		beadType := model.BeadType(rb.Type)
		if beadType == "epic" {
			beadType = model.TypeTask
		}
		s.beads[rb.ID] = model.Bead{
			ID:          rb.ID,
			Title:       rb.Title,
			Description: rb.Description,
			Status:      status,
			Priority:    rb.Priority,
			Type:        beadType,
			Tags:        rb.Tags,
			BlockedBy:   rb.BlockedBy,
			Assignee:    rb.Assignee,
			ParentID:    rb.ParentID,
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

// generateUniqueID creates a collision-free bead ID.
// It starts at IDMinLen (4) random chars and retries up to 3 times per length.
// On exhaustion, it increases the length by 1 (up to IDMaxLen).
// At IDMaxLen, it retries indefinitely.
// Caller must hold s.mu.
func (s *Store) generateUniqueID() string {
	for n := model.IDMinLen; n <= model.IDMaxLen; n++ {
		retries := 3
		if n == model.IDMaxLen {
			retries = 100 // effectively unlimited at max length
		}
		for i := 0; i < retries; i++ {
			id := model.GenerateIDN(n)
			if _, exists := s.beads[id]; !exists {
				return id
			}
		}
	}
	// Should never reach here given 36^8 possible IDs.
	panic("failed to generate unique bead ID")
}

// Create adds a bead to the store and persists to disk.
// If b.ID is empty, a collision-free ID is generated automatically.
func (s *Store) Create(b model.Bead) (model.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b.ID == "" {
		b.ID = s.generateUniqueID()
	} else if _, exists := s.beads[b.ID]; exists {
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

// Resolve finds a bead by exact ID.
// The full ID including the "bd-" prefix is required.
func (s *Store) Resolve(id string) (model.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if b, ok := s.beads[id]; ok {
		return b, nil
	}

	return model.Bead{}, &NotFoundError{Message: fmt.Sprintf("bead %s not found", id)}
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
	ParentID    *string
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
	if fields.ParentID != nil {
		b.ParentID = *fields.ParentID
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
