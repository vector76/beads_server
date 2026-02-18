package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"
)

// --- ID generation tests ---

func TestGenerateIDFormat(t *testing.T) {
	id := GenerateID()
	matched, _ := regexp.MatchString(`^bd-[a-z0-9]{4}$`, id)
	if !matched {
		t.Errorf("ID %q does not match expected format bd-[a-z0-9]{4}", id)
	}
}

func TestGenerateIDNFormat(t *testing.T) {
	for _, n := range []int{4, 5, 6, 7, 8} {
		id := GenerateIDN(n)
		pattern := fmt.Sprintf(`^bd-[a-z0-9]{%d}$`, n)
		matched, _ := regexp.MatchString(pattern, id)
		if !matched {
			t.Errorf("GenerateIDN(%d) = %q, does not match %s", n, id, pattern)
		}
	}
}

// --- NewBead defaults tests ---

func TestNewBeadDefaults(t *testing.T) {
	b := NewBead("Test issue")

	if b.Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got %q", b.Title)
	}
	if b.Status != StatusOpen {
		t.Errorf("expected status 'open', got %q", b.Status)
	}
	if b.Priority != PriorityMedium {
		t.Errorf("expected priority 'medium', got %q", b.Priority)
	}
	if b.Type != TypeTask {
		t.Errorf("expected type 'task', got %q", b.Type)
	}
	if b.Description != "" {
		t.Errorf("expected empty description, got %q", b.Description)
	}
	if b.Assignee != "" {
		t.Errorf("expected empty assignee, got %q", b.Assignee)
	}
	if b.Tags == nil || len(b.Tags) != 0 {
		t.Errorf("expected empty tags slice, got %v", b.Tags)
	}
	if b.BlockedBy == nil || len(b.BlockedBy) != 0 {
		t.Errorf("expected empty blocked_by slice, got %v", b.BlockedBy)
	}
	if b.Comments == nil || len(b.Comments) != 0 {
		t.Errorf("expected empty comments slice, got %v", b.Comments)
	}
	if b.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if b.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}

	if b.ID != "" {
		t.Errorf("expected empty ID (assigned by store), got %q", b.ID)
	}
}

// --- JSON round-trip tests ---

func TestBeadJSONRoundTrip(t *testing.T) {
	original := Bead{
		ID:          "bd-a1b2c3d4",
		Title:       "Fix login bug",
		Description: "Users can't log in with special characters",
		Status:      StatusInProgress,
		Priority:    PriorityHigh,
		Type:        TypeBug,
		Tags:        []string{"auth", "urgent"},
		BlockedBy:   []string{"bd-x1y2z3w4"},
		Assignee:    "agent-1",
		Comments: []Comment{
			{Author: "agent-1", Text: "Working on it", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal bead: %v", err)
	}

	var decoded Bead
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal bead: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, original.Title)
	}
	if decoded.Description != original.Description {
		t.Errorf("Description: got %q, want %q", decoded.Description, original.Description)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.Priority != original.Priority {
		t.Errorf("Priority: got %q, want %q", decoded.Priority, original.Priority)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Assignee != original.Assignee {
		t.Errorf("Assignee: got %q, want %q", decoded.Assignee, original.Assignee)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(decoded.Tags), len(original.Tags))
	}
	if len(decoded.BlockedBy) != len(original.BlockedBy) {
		t.Errorf("BlockedBy length: got %d, want %d", len(decoded.BlockedBy), len(original.BlockedBy))
	}
	if len(decoded.Comments) != len(original.Comments) {
		t.Errorf("Comments length: got %d, want %d", len(decoded.Comments), len(original.Comments))
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}
	if !decoded.UpdatedAt.Equal(original.UpdatedAt) {
		t.Errorf("UpdatedAt: got %v, want %v", decoded.UpdatedAt, original.UpdatedAt)
	}
}

func TestCommentJSONRoundTrip(t *testing.T) {
	original := Comment{
		Author:    "agent-1",
		Text:      "This is a comment",
		CreatedAt: time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal comment: %v", err)
	}

	var decoded Comment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal comment: %v", err)
	}

	if decoded.Author != original.Author {
		t.Errorf("Author: got %q, want %q", decoded.Author, original.Author)
	}
	if decoded.Text != original.Text {
		t.Errorf("Text: got %q, want %q", decoded.Text, original.Text)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}
}

// --- Enum validation tests ---

func TestStatusValid(t *testing.T) {
	valid := []Status{StatusOpen, StatusInProgress, StatusClosed, StatusDeleted}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("expected status %q to be valid", s)
		}
	}

	if Status("bogus").Valid() {
		t.Error("expected 'bogus' status to be invalid")
	}
	if Status("").Valid() {
		t.Error("expected empty status to be invalid")
	}
	if Status("resolved").Valid() {
		t.Error("expected 'resolved' status to be invalid")
	}
	if Status("wontfix").Valid() {
		t.Error("expected 'wontfix' status to be invalid")
	}
}

func TestPriorityValid(t *testing.T) {
	valid := []Priority{PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow, PriorityNone}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("expected priority %q to be valid", p)
		}
	}

	if Priority("bogus").Valid() {
		t.Error("expected 'bogus' priority to be invalid")
	}
}

func TestBeadTypeValid(t *testing.T) {
	valid := []BeadType{TypeBug, TypeFeature, TypeTask, TypeChore}
	for _, bt := range valid {
		if !bt.Valid() {
			t.Errorf("expected type %q to be valid", bt)
		}
	}

	if BeadType("bogus").Valid() {
		t.Error("expected 'bogus' type to be invalid")
	}
	if BeadType("epic").Valid() {
		t.Error("expected 'epic' type to be invalid (removed)")
	}
}

// --- Enum JSON unmarshal validation tests ---

func TestStatusUnmarshalInvalid(t *testing.T) {
	var s Status
	err := json.Unmarshal([]byte(`"bogus"`), &s)
	if err == nil {
		t.Error("expected error unmarshaling invalid status")
	}
}

func TestStatusUnmarshalValid(t *testing.T) {
	var s Status
	if err := json.Unmarshal([]byte(`"open"`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != StatusOpen {
		t.Errorf("expected 'open', got %q", s)
	}
}

func TestPriorityUnmarshalInvalid(t *testing.T) {
	var p Priority
	err := json.Unmarshal([]byte(`"bogus"`), &p)
	if err == nil {
		t.Error("expected error unmarshaling invalid priority")
	}
}

func TestPriorityUnmarshalValid(t *testing.T) {
	var p Priority
	if err := json.Unmarshal([]byte(`"high"`), &p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != PriorityHigh {
		t.Errorf("expected 'high', got %q", p)
	}
}

func TestBeadTypeUnmarshalInvalid(t *testing.T) {
	var bt BeadType
	err := json.Unmarshal([]byte(`"bogus"`), &bt)
	if err == nil {
		t.Error("expected error unmarshaling invalid bead type")
	}
}

func TestBeadTypeUnmarshalValid(t *testing.T) {
	var bt BeadType
	if err := json.Unmarshal([]byte(`"bug"`), &bt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bt != TypeBug {
		t.Errorf("expected 'bug', got %q", bt)
	}
}

// --- Bead JSON with invalid enum rejects ---

func TestBeadUnmarshalInvalidStatus(t *testing.T) {
	raw := `{"id":"bd-a1b2c3d4","title":"test","status":"bogus","priority":"medium","type":"task"}`
	var b Bead
	err := json.Unmarshal([]byte(raw), &b)
	if err == nil {
		t.Error("expected error unmarshaling bead with invalid status")
	}
}

func TestBeadUnmarshalRejectsResolved(t *testing.T) {
	raw := `{"id":"bd-a1b2c3d4","title":"test","status":"resolved","priority":"medium","type":"task"}`
	var b Bead
	err := json.Unmarshal([]byte(raw), &b)
	if err == nil {
		t.Error("expected error unmarshaling bead with status 'resolved'")
	}
}

func TestBeadUnmarshalRejectsWontfix(t *testing.T) {
	raw := `{"id":"bd-a1b2c3d4","title":"test","status":"wontfix","priority":"medium","type":"task"}`
	var b Bead
	err := json.Unmarshal([]byte(raw), &b)
	if err == nil {
		t.Error("expected error unmarshaling bead with status 'wontfix'")
	}
}

func TestBeadUnmarshalInvalidPriority(t *testing.T) {
	raw := `{"id":"bd-a1b2c3d4","title":"test","status":"open","priority":"bogus","type":"task"}`
	var b Bead
	err := json.Unmarshal([]byte(raw), &b)
	if err == nil {
		t.Error("expected error unmarshaling bead with invalid priority")
	}
}

func TestBeadUnmarshalInvalidType(t *testing.T) {
	raw := `{"id":"bd-a1b2c3d4","title":"test","status":"open","priority":"medium","type":"bogus"}`
	var b Bead
	err := json.Unmarshal([]byte(raw), &b)
	if err == nil {
		t.Error("expected error unmarshaling bead with invalid type")
	}
}
