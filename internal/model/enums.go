package model

import (
	"encoding/json"
	"fmt"
)

// Status represents the lifecycle state of a bead.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusResolved   Status = "resolved"
	StatusClosed     Status = "closed"
	StatusWontfix    Status = "wontfix"
	StatusDeleted    Status = "deleted"
)

var validStatuses = map[Status]bool{
	StatusOpen:       true,
	StatusInProgress: true,
	StatusResolved:   true,
	StatusClosed:     true,
	StatusWontfix:    true,
	StatusDeleted:    true,
}

func (s Status) Valid() bool {
	return validStatuses[s]
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	v := Status(str)
	if !v.Valid() {
		return fmt.Errorf("invalid status: %q", str)
	}
	*s = v
	return nil
}

// Priority represents the urgency of a bead.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
	PriorityNone     Priority = "none"
)

var validPriorities = map[Priority]bool{
	PriorityCritical: true,
	PriorityHigh:     true,
	PriorityMedium:   true,
	PriorityLow:      true,
	PriorityNone:     true,
}

func (p Priority) Valid() bool {
	return validPriorities[p]
}

func (p *Priority) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	v := Priority(str)
	if !v.Valid() {
		return fmt.Errorf("invalid priority: %q", str)
	}
	*p = v
	return nil
}

// BeadType represents the category of a bead.
type BeadType string

const (
	TypeBug     BeadType = "bug"
	TypeFeature BeadType = "feature"
	TypeTask    BeadType = "task"
	TypeEpic    BeadType = "epic"
	TypeChore   BeadType = "chore"
)

var validBeadTypes = map[BeadType]bool{
	TypeBug:     true,
	TypeFeature: true,
	TypeTask:    true,
	TypeEpic:    true,
	TypeChore:   true,
}

func (bt BeadType) Valid() bool {
	return validBeadTypes[bt]
}

func (bt *BeadType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	v := BeadType(str)
	if !v.Valid() {
		return fmt.Errorf("invalid bead type: %q", str)
	}
	*bt = v
	return nil
}
