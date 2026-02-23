package store

import (
	"errors"
	"testing"
	"time"

	"github.com/vector76/beads_server/internal/model"
)

// --- Search tests ---

func TestSearchMatchesTitle(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-srch0001", "Fix login bug", model.StatusOpen, model.PriorityMedium, model.TypeBug, "", nil, nil, now)
	s.Create(b)

	result := s.Search("login", 1, 100)
	if result.Total != 1 {
		t.Fatalf("expected 1 result, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-srch0001" {
		t.Errorf("expected bd-srch0001, got %s", result.Beads[0].ID)
	}
}

func TestSearchMatchesDescription(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-srch0001", "Some task", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	b.Description = "Users cannot authenticate properly"
	s.Create(b)

	result := s.Search("authenticate", 1, 100)
	if result.Total != 1 {
		t.Fatalf("expected 1 result, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-srch0001" {
		t.Errorf("expected bd-srch0001, got %s", result.Beads[0].ID)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-srch0001", "Fix LOGIN Bug", model.StatusOpen, model.PriorityMedium, model.TypeBug, "", nil, nil, now)
	s.Create(b)

	result := s.Search("login", 1, 100)
	if result.Total != 1 {
		t.Errorf("expected 1 result for case-insensitive search, got %d", result.Total)
	}

	result = s.Search("LOGIN", 1, 100)
	if result.Total != 1 {
		t.Errorf("expected 1 result for uppercase search, got %d", result.Total)
	}
}

func TestSearchExcludesDeleted(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-srch0001", "Deleted task", model.StatusDeleted, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	result := s.Search("Deleted", 1, 100)
	if result.Total != 0 {
		t.Errorf("expected 0 results (deleted excluded), got %d", result.Total)
	}
}

func TestSearchNoResults(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-srch0001", "Some task", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	result := s.Search("nonexistent", 1, 100)
	if result.Total != 0 {
		t.Errorf("expected 0 results, got %d", result.Total)
	}
}

func TestSearchPagination(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		b := newBeadWithFields("bd-srch000"+string(rune('a'+i)), "Searchable item", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now.Add(time.Duration(-i)*time.Minute))
		s.Create(b)
	}

	result := s.Search("Searchable", 1, 2)
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if result.TotalPages != 3 {
		t.Errorf("expected 3 pages, got %d", result.TotalPages)
	}
	if len(result.Beads) != 2 {
		t.Errorf("expected 2 beads on page 1, got %d", len(result.Beads))
	}
}

// --- AddComment tests ---

func TestAddComment(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-cmnt0001", "Commentable", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	comment := model.Comment{Author: "agent-1", Text: "Working on this"}
	updated, err := s.AddComment("bd-cmnt0001", comment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updated.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(updated.Comments))
	}
	if updated.Comments[0].Author != "agent-1" {
		t.Errorf("expected author 'agent-1', got %q", updated.Comments[0].Author)
	}
	if updated.Comments[0].Text != "Working on this" {
		t.Errorf("expected text 'Working on this', got %q", updated.Comments[0].Text)
	}
	if updated.Comments[0].CreatedAt.IsZero() {
		t.Error("expected created_at to be set on comment")
	}
	if !updated.UpdatedAt.After(b.CreatedAt) {
		t.Error("expected bead updated_at to be updated")
	}
}

func TestAddCommentToNonexistent(t *testing.T) {
	s, _ := Load(tempPath(t))

	comment := model.Comment{Author: "agent-1", Text: "test"}
	_, err := s.AddComment("bd-nonexist", comment)
	if err == nil {
		t.Error("expected error adding comment to non-existent bead")
	}
}

func TestAddMultipleComments(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-cmnt0001", "Commentable", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	s.AddComment("bd-cmnt0001", model.Comment{Author: "agent-1", Text: "First"})
	updated, _ := s.AddComment("bd-cmnt0001", model.Comment{Author: "agent-2", Text: "Second"})

	if len(updated.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(updated.Comments))
	}
	if updated.Comments[0].Text != "First" {
		t.Errorf("expected first comment 'First', got %q", updated.Comments[0].Text)
	}
	if updated.Comments[1].Text != "Second" {
		t.Errorf("expected second comment 'Second', got %q", updated.Comments[1].Text)
	}
}

// --- Claim tests ---

func TestClaimSuccess(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-clm00001", "Claimable", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	claimed, err := s.Claim("bd-clm00001", "agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimed.Status != model.StatusInProgress {
		t.Errorf("expected status in_progress, got %q", claimed.Status)
	}
	if claimed.Assignee != "agent-1" {
		t.Errorf("expected assignee 'agent-1', got %q", claimed.Assignee)
	}
}

func TestClaimIdempotent(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-clm00001", "Claimable", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	s.Claim("bd-clm00001", "agent-1")
	claimed, err := s.Claim("bd-clm00001", "agent-1")
	if err != nil {
		t.Fatalf("expected idempotent claim to succeed, got error: %v", err)
	}
	if claimed.Status != model.StatusInProgress {
		t.Errorf("expected status in_progress, got %q", claimed.Status)
	}
	if claimed.Assignee != "agent-1" {
		t.Errorf("expected assignee 'agent-1', got %q", claimed.Assignee)
	}
}

func TestClaimConflictDifferentUser(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-clm00001", "Claimable", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	s.Claim("bd-clm00001", "agent-1")
	_, err := s.Claim("bd-clm00001", "agent-2")
	if err == nil {
		t.Fatal("expected conflict error when claiming bead owned by different user")
	}

	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestClaimConflictTerminalClosed(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-clm00001", "Closed", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	_, err := s.Claim("bd-clm00001", "agent-1")
	if err == nil {
		t.Fatal("expected conflict error when claiming closed bead")
	}

	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestClaimConflictTerminalDeleted(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-clm00001", "Deleted", model.StatusDeleted, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	_, err := s.Claim("bd-clm00001", "agent-1")
	if err == nil {
		t.Fatal("expected conflict error when claiming deleted bead")
	}

	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestClaimNotFound(t *testing.T) {
	s, _ := Load(tempPath(t))

	_, err := s.Claim("bd-nonexist", "agent-1")
	if err == nil {
		t.Fatal("expected error claiming non-existent bead")
	}

	// Should NOT be a ConflictError
	var conflictErr *ConflictError
	if errors.As(err, &conflictErr) {
		t.Error("expected non-ConflictError for not found, got ConflictError")
	}
}

// --- Clean tests ---

func TestCleanRemovesOldClosedBeads(t *testing.T) {
	s, _ := Load(tempPath(t))

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	b := newBeadWithFields("bd-cln00001", "Old closed", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(b)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	_, err = s.Get("bd-cln00001")
	if err == nil {
		t.Error("expected bead to be removed")
	}
}

func TestCleanRemovesOldDeletedBeads(t *testing.T) {
	s, _ := Load(tempPath(t))

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	b := newBeadWithFields("bd-cln00001", "Old deleted", model.StatusDeleted, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(b)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
}

func TestCleanKeepsRecentClosedBeads(t *testing.T) {
	s, _ := Load(tempPath(t))

	recent := time.Now().UTC().Add(-1 * 24 * time.Hour)
	b := newBeadWithFields("bd-cln00001", "Recent closed", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, recent)
	s.Create(b)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}

	_, err = s.Get("bd-cln00001")
	if err != nil {
		t.Error("expected bead to still exist")
	}
}

func TestCleanKeepsOpenBeads(t *testing.T) {
	s, _ := Load(tempPath(t))

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	b := newBeadWithFields("bd-cln00001", "Old open", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(b)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestCleanKeepsInProgressBeads(t *testing.T) {
	s, _ := Load(tempPath(t))

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	b := newBeadWithFields("bd-cln00001", "Old in progress", model.StatusInProgress, model.PriorityMedium, model.TypeTask, "", nil, nil, old)
	s.Create(b)

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestCleanMixedBeads(t *testing.T) {
	s, _ := Load(tempPath(t))

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	recent := time.Now().UTC().Add(-1 * 24 * time.Hour)

	// Old closed - should be removed
	s.Create(newBeadWithFields("bd-cln00001", "Old closed", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old))
	// Old deleted - should be removed
	s.Create(newBeadWithFields("bd-cln00002", "Old deleted", model.StatusDeleted, model.PriorityMedium, model.TypeTask, "", nil, nil, old))
	// Old open - should stay
	s.Create(newBeadWithFields("bd-cln00003", "Old open", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, old))
	// Recent closed - should stay
	s.Create(newBeadWithFields("bd-cln00004", "Recent closed", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, recent))

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 remaining beads, got %d", len(all))
	}
}

func TestCleanPersists(t *testing.T) {
	path := tempPath(t)
	s, _ := Load(path)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	s.Create(newBeadWithFields("bd-cln00001", "Old closed", model.StatusClosed, model.PriorityMedium, model.TypeTask, "", nil, nil, old))

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	s.Clean(cutoff)

	// Reload and verify
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(s2.All()) != 0 {
		t.Fatalf("expected 0 beads after reload, got %d", len(s2.All()))
	}
}

func TestCleanNothingToRemove(t *testing.T) {
	s, _ := Load(tempPath(t))

	recent := time.Now().UTC()
	s.Create(newBeadWithFields("bd-cln00001", "Recent open", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, recent))

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	removed, err := s.Clean(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestClaim_NotReadyRejected(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-clm00001", "Not ready", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	_, err := s.Claim("bd-clm00001", "agent-1")
	if err == nil {
		t.Fatal("expected error when claiming not_ready bead")
	}

	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestDelete_NotReadySucceeds(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	b := newBeadWithFields("bd-del00001", "Not ready", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b)

	_, err := s.Delete("bd-del00001")
	if err != nil {
		t.Errorf("expected no error deleting not_ready bead, got: %v", err)
	}
}
