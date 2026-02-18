package store

import (
	"testing"
	"time"

	"github.com/yourorg/beads_server/internal/model"
)

func newBeadWithFields(id, title string, status model.Status, priority model.Priority, beadType model.BeadType, assignee string, tags []string, blockedBy []string, createdAt time.Time) model.Bead {
	return model.Bead{
		ID:        id,
		Title:     title,
		Status:    status,
		Priority:  priority,
		Type:      beadType,
		Tags:      tags,
		BlockedBy: blockedBy,
		Assignee:  assignee,
		Comments:  []model.Comment{},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// setupListStore creates a store with varied beads for list testing.
func setupListStore(t *testing.T) *Store {
	t.Helper()
	s, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("failed to load store: %v", err)
	}

	now := time.Now().UTC()
	beads := []model.Bead{
		newBeadWithFields("bd-open0001", "Open high bug", model.StatusOpen, model.PriorityHigh, model.TypeBug, "agent-1", []string{"auth", "urgent"}, nil, now.Add(-5*time.Minute)),
		newBeadWithFields("bd-open0002", "Open medium task", model.StatusOpen, model.PriorityMedium, model.TypeTask, "agent-2", []string{"backend"}, nil, now.Add(-4*time.Minute)),
		newBeadWithFields("bd-open0003", "Open critical feature", model.StatusOpen, model.PriorityCritical, model.TypeFeature, "", nil, nil, now.Add(-3*time.Minute)),
		newBeadWithFields("bd-prog0001", "In progress low chore", model.StatusInProgress, model.PriorityLow, model.TypeChore, "agent-1", []string{"cleanup"}, nil, now.Add(-2*time.Minute)),
		newBeadWithFields("bd-clos0001", "Closed none task", model.StatusClosed, model.PriorityNone, model.TypeTask, "agent-3", nil, nil, now.Add(-1*time.Minute)),
		newBeadWithFields("bd-dele0001", "Deleted task", model.StatusDeleted, model.PriorityMedium, model.TypeTask, "", nil, nil, now),
	}

	for _, b := range beads {
		if _, err := s.Create(b); err != nil {
			t.Fatalf("failed to create bead %s: %v", b.ID, err)
		}
	}

	return s
}

func TestListDefaultFilters(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{})
	// Default: open + in_progress + not_ready (excludes closed, deleted)
	if result.Total != 4 {
		t.Errorf("expected 4 beads, got %d", result.Total)
	}
	for _, b := range result.Beads {
		if b.Status != model.StatusOpen && b.Status != model.StatusInProgress && b.Status != model.StatusNotReady {
			t.Errorf("unexpected status %q in default list", b.Status)
		}
	}
}

func TestListAllFlag(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{All: true})
	if result.Total != 6 {
		t.Errorf("expected 6 beads with --all, got %d", result.Total)
	}
}

func TestListFilterByStatus(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{Statuses: []model.Status{model.StatusClosed}})
	if result.Total != 1 {
		t.Fatalf("expected 1 closed bead, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-clos0001" {
		t.Errorf("expected bd-clos0001, got %s", result.Beads[0].ID)
	}
}

func TestListFilterByMultipleStatuses(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{Statuses: []model.Status{model.StatusClosed, model.StatusDeleted}})
	if result.Total != 2 {
		t.Errorf("expected 2 beads, got %d", result.Total)
	}
}

func TestListFilterByPriority(t *testing.T) {
	s := setupListStore(t)

	high := model.PriorityHigh
	result := s.List(ListFilters{Priority: &high})
	if result.Total != 1 {
		t.Fatalf("expected 1 high-priority bead, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-open0001" {
		t.Errorf("expected bd-open0001, got %s", result.Beads[0].ID)
	}
}

func TestListFilterByType(t *testing.T) {
	s := setupListStore(t)

	bug := model.TypeBug
	result := s.List(ListFilters{Type: &bug})
	if result.Total != 1 {
		t.Fatalf("expected 1 bug bead, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-open0001" {
		t.Errorf("expected bd-open0001, got %s", result.Beads[0].ID)
	}
}

func TestListFilterByAssignee(t *testing.T) {
	s := setupListStore(t)

	agent1 := "agent-1"
	result := s.List(ListFilters{Assignee: &agent1})
	// agent-1 has bd-open0001 (open) and bd-prog0001 (in_progress) in default filter
	if result.Total != 2 {
		t.Errorf("expected 2 beads for agent-1, got %d", result.Total)
	}
}

func TestListFilterByTag(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{Tags: []string{"auth"}})
	if result.Total != 1 {
		t.Fatalf("expected 1 bead with 'auth' tag, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-open0001" {
		t.Errorf("expected bd-open0001, got %s", result.Beads[0].ID)
	}
}

func TestListFilterByTagORSemantics(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{Tags: []string{"auth", "cleanup"}})
	// auth -> bd-open0001, cleanup -> bd-prog0001
	if result.Total != 2 {
		t.Errorf("expected 2 beads with 'auth' or 'cleanup' tags, got %d", result.Total)
	}
}

func TestListSortingOrder(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{})
	// Expected order: critical first, then high, then medium, then low
	// bd-open0003 (critical), bd-open0001 (high), bd-open0002 (medium), bd-prog0001 (low)
	if len(result.Beads) != 4 {
		t.Fatalf("expected 4 beads, got %d", len(result.Beads))
	}
	expectedOrder := []string{"bd-open0003", "bd-open0001", "bd-open0002", "bd-prog0001"}
	for i, expected := range expectedOrder {
		if result.Beads[i].ID != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, result.Beads[i].ID)
		}
	}
}

func TestListSortingSamePriorityByCreatedAtDesc(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	// Two beads with same priority, different creation times
	b1 := newBeadWithFields("bd-old00001", "Old", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now.Add(-10*time.Minute))
	b2 := newBeadWithFields("bd-new00001", "New", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(b1)
	s.Create(b2)

	result := s.List(ListFilters{})
	if len(result.Beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(result.Beads))
	}
	// Newest first
	if result.Beads[0].ID != "bd-new00001" {
		t.Errorf("expected newest first, got %s", result.Beads[0].ID)
	}
	if result.Beads[1].ID != "bd-old00001" {
		t.Errorf("expected oldest second, got %s", result.Beads[1].ID)
	}
}

func TestListPagination(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		b := newBeadWithFields("bd-page000"+string(rune('a'+i)), "Bead", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now.Add(time.Duration(-i)*time.Minute))
		s.Create(b)
	}

	// Page 1, 2 per page
	result := s.List(ListFilters{Page: 1, PerPage: 2})
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if result.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", result.TotalPages)
	}
	if result.Page != 1 {
		t.Errorf("expected page 1, got %d", result.Page)
	}
	if result.PerPage != 2 {
		t.Errorf("expected per_page 2, got %d", result.PerPage)
	}
	if len(result.Beads) != 2 {
		t.Errorf("expected 2 beads on page 1, got %d", len(result.Beads))
	}

	// Page 3 (last page, partial)
	result = s.List(ListFilters{Page: 3, PerPage: 2})
	if len(result.Beads) != 1 {
		t.Errorf("expected 1 bead on page 3, got %d", len(result.Beads))
	}

	// Page beyond range
	result = s.List(ListFilters{Page: 10, PerPage: 2})
	if len(result.Beads) != 0 {
		t.Errorf("expected 0 beads beyond last page, got %d", len(result.Beads))
	}
}

func TestListPaginationDefaults(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{})
	if result.Page != 1 {
		t.Errorf("expected default page 1, got %d", result.Page)
	}
	if result.PerPage != 100 {
		t.Errorf("expected default per_page 100, got %d", result.PerPage)
	}
}

func TestListReadyFilter(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	blocker := newBeadWithFields("bd-block001", "Blocker", model.StatusOpen, model.PriorityHigh, model.TypeTask, "", nil, nil, now)
	unblocked := newBeadWithFields("bd-unblk001", "Unblocked", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	blocked := newBeadWithFields("bd-blked001", "Blocked", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, []string{"bd-block001"}, now)
	s.Create(blocker)
	s.Create(unblocked)
	s.Create(blocked)

	result := s.List(ListFilters{Ready: true})
	// blocker and unblocked are ready; blocked is not
	if result.Total != 2 {
		t.Errorf("expected 2 ready beads, got %d", result.Total)
	}
	for _, b := range result.Beads {
		if b.ID == "bd-blked001" {
			t.Error("blocked bead should not appear in ready list")
		}
	}
}

func TestListReadyFilterWithClosedBlocker(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	closedBlocker := newBeadWithFields("bd-rblck001", "Closed blocker", model.StatusClosed, model.PriorityHigh, model.TypeTask, "", nil, nil, now)
	dependentBead := newBeadWithFields("bd-depnd001", "Depends on closed", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, []string{"bd-rblck001"}, now)
	s.Create(closedBlocker)
	s.Create(dependentBead)

	result := s.List(ListFilters{Ready: true})
	// dependentBead should be ready since its only blocker is closed
	if result.Total != 1 {
		t.Fatalf("expected 1 ready bead, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-depnd001" {
		t.Errorf("expected bd-depnd001, got %s", result.Beads[0].ID)
	}
}

func TestListReadyExcludesInProgress(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	inProgress := newBeadWithFields("bd-inpr0001", "In progress", model.StatusInProgress, model.PriorityMedium, model.TypeTask, "agent-1", nil, nil, now)
	open := newBeadWithFields("bd-open0001", "Open ready", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	s.Create(inProgress)
	s.Create(open)

	result := s.List(ListFilters{Ready: true})
	// Ready filter requires status=open
	if result.Total != 1 {
		t.Fatalf("expected 1 ready bead, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-open0001" {
		t.Errorf("expected bd-open0001, got %s", result.Beads[0].ID)
	}
}

func TestListDefault_IncludesNotReady(t *testing.T) {
	s, _ := Load(tempPath(t))

	b := newBeadWithFields("bd-nrdy0001", "Not ready bead", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, time.Now().UTC())
	s.Create(b)

	result := s.List(ListFilters{})
	if result.Total != 1 {
		t.Fatalf("expected 1 bead in default list, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-nrdy0001" {
		t.Errorf("expected bd-nrdy0001, got %s", result.Beads[0].ID)
	}
}

func TestListReady_ExcludesNotReady(t *testing.T) {
	s, _ := Load(tempPath(t))

	b := newBeadWithFields("bd-nrdy0002", "Not ready bead", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, time.Now().UTC())
	s.Create(b)

	result := s.List(ListFilters{Ready: true})
	if result.Total != 0 {
		t.Errorf("expected 0 beads in ready list, got %d", result.Total)
	}
}

func TestListAll_IncludesNotReady(t *testing.T) {
	s, _ := Load(tempPath(t))

	b := newBeadWithFields("bd-nrdy0003", "Not ready bead", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, time.Now().UTC())
	s.Create(b)

	result := s.List(ListFilters{All: true})
	if result.Total != 1 {
		t.Fatalf("expected 1 bead with --all, got %d", result.Total)
	}
	if result.Beads[0].ID != "bd-nrdy0003" {
		t.Errorf("expected bd-nrdy0003, got %s", result.Beads[0].ID)
	}
}

func TestActiveBlocker_NotReadyBlocks(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	blockerB := newBeadWithFields("bd-nrblk001", "Not-ready blocker", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	dependentA := newBeadWithFields("bd-nrdep001", "Dependent on not-ready", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, []string{"bd-nrblk001"}, now)
	s.Create(blockerB)
	s.Create(dependentA)

	result := s.List(ListFilters{Ready: true})
	for _, b := range result.Beads {
		if b.ID == "bd-nrdep001" {
			t.Error("dependent bead should not appear in ready list while blocker is not_ready")
		}
	}
}

func TestActiveBlocker_NotReadyThenClosed(t *testing.T) {
	s, _ := Load(tempPath(t))

	now := time.Now().UTC()
	blockerB := newBeadWithFields("bd-nrblk002", "Not-ready blocker", model.StatusNotReady, model.PriorityMedium, model.TypeTask, "", nil, nil, now)
	dependentA := newBeadWithFields("bd-nrdep002", "Dependent on not-ready", model.StatusOpen, model.PriorityMedium, model.TypeTask, "", nil, []string{"bd-nrblk002"}, now)
	s.Create(blockerB)
	s.Create(dependentA)

	closed := model.StatusClosed
	if _, err := s.Update("bd-nrblk002", UpdateFields{Status: &closed}); err != nil {
		t.Fatalf("failed to close blocker: %v", err)
	}

	result := s.List(ListFilters{Ready: true})
	found := false
	for _, b := range result.Beads {
		if b.ID == "bd-nrdep002" {
			found = true
		}
	}
	if !found {
		t.Error("dependent bead should appear in ready list after blocker is closed")
	}
}

func TestListReturnsSummaryFields(t *testing.T) {
	s := setupListStore(t)

	result := s.List(ListFilters{Statuses: []model.Status{model.StatusOpen}})
	if len(result.Beads) == 0 {
		t.Fatal("expected at least one bead")
	}
	b := result.Beads[0]
	// Verify summary fields are populated
	if b.ID == "" {
		t.Error("expected ID to be set")
	}
	if b.Title == "" {
		t.Error("expected Title to be set")
	}
	if b.Status == "" {
		t.Error("expected Status to be set")
	}
	if b.Priority == "" {
		t.Error("expected Priority to be set")
	}
	if b.Type == "" {
		t.Error("expected Type to be set")
	}
}
