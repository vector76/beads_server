package model

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Bead represents an issue/task in the tracker.
type Bead struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Type        BeadType  `json:"type"`
	Tags        []string  `json:"tags"`
	BlockedBy   []string  `json:"blocked_by"`
	Assignee    string    `json:"assignee"`
	ParentID    string    `json:"parent_id"`
	Comments    []Comment `json:"comments"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const IDPrefix = "bd-"
const IDMinLen = 4
const IDMaxLen = 8
const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateID creates a new bead ID in the format bd-XXXX
// where X is a random lowercase alphanumeric character.
// The random portion length defaults to IDMinLen (4).
// For collision-aware generation, use store.GenerateUniqueID instead.
func GenerateID() string {
	return GenerateIDN(IDMinLen)
}

// GenerateIDN creates a bead ID with n random characters.
func GenerateIDN(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random ID: %v", err))
	}
	for i := range b {
		b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
	}
	return IDPrefix + string(b)
}

// NewBead creates a new Bead with defaults but no ID.
// The store layer assigns a collision-free ID on Create.
func NewBead(title string) Bead {
	now := time.Now().UTC()
	return Bead{
		Title:     title,
		Status:    StatusOpen,
		Priority:  PriorityMedium,
		Type:      TypeTask,
		Tags:      []string{},
		BlockedBy: []string{},
		Comments:  []Comment{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
