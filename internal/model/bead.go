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
	Comments    []Comment `json:"comments"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const idPrefix = "bd-"
const idRandomLen = 8
const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateID creates a new bead ID in the format bd-XXXXXXXX
// where X is a random lowercase alphanumeric character.
func GenerateID() string {
	b := make([]byte, idRandomLen)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random ID: %v", err))
	}
	for i := range b {
		b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
	}
	return idPrefix + string(b)
}

// NewBead creates a new Bead with defaults and a generated ID.
func NewBead(title string) Bead {
	now := time.Now().UTC()
	return Bead{
		ID:        GenerateID(),
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
