package server

import (
	"path/filepath"
	"testing"

	"github.com/vector76/beads_server/internal/store"
)

func loadTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Load(filepath.Join(t.TempDir(), "beads.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

// --- singleStoreProvider tests ---

func TestSingleProvider_MatchingToken(t *testing.T) {
	s := loadTestStore(t)
	p := NewSingleStoreProvider("tok-abc", s)

	got := p.Resolve("tok-abc")
	if got != s {
		t.Fatal("expected the store for matching token, got nil")
	}
}

func TestSingleProvider_WrongToken(t *testing.T) {
	s := loadTestStore(t)
	p := NewSingleStoreProvider("tok-abc", s)

	got := p.Resolve("tok-wrong")
	if got != nil {
		t.Fatal("expected nil for non-matching token")
	}
}

func TestSingleProvider_EmptyToken(t *testing.T) {
	s := loadTestStore(t)
	p := NewSingleStoreProvider("tok-abc", s)

	got := p.Resolve("")
	if got != nil {
		t.Fatal("expected nil for empty token")
	}
}

// --- singleStoreProvider.Projects tests ---

func TestSingleProvider_Projects(t *testing.T) {
	s := loadTestStore(t)
	p := NewSingleStoreProvider("tok-abc", s)

	projects := p.Projects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "default" {
		t.Errorf("expected name %q, got %q", "default", projects[0].Name)
	}
	if projects[0].Store != s {
		t.Error("expected the store to match")
	}
}

// --- multiStoreProvider tests ---

func TestMultiProvider_KnownTokens(t *testing.T) {
	s1 := loadTestStore(t)
	s2 := loadTestStore(t)

	p := NewMultiStoreProvider([]ProviderEntry{
		{Name: "proj-one", Token: "tok-one", Store: s1},
		{Name: "proj-two", Token: "tok-two", Store: s2},
	})

	if got := p.Resolve("tok-one"); got != s1 {
		t.Fatal("expected s1 for tok-one")
	}
	if got := p.Resolve("tok-two"); got != s2 {
		t.Fatal("expected s2 for tok-two")
	}
}

func TestMultiProvider_UnknownToken(t *testing.T) {
	s1 := loadTestStore(t)
	p := NewMultiStoreProvider([]ProviderEntry{
		{Name: "proj-one", Token: "tok-one", Store: s1},
	})

	got := p.Resolve("tok-unknown")
	if got != nil {
		t.Fatal("expected nil for unknown token")
	}
}

func TestMultiProvider_Projects(t *testing.T) {
	s1 := loadTestStore(t)
	s2 := loadTestStore(t)

	p := NewMultiStoreProvider([]ProviderEntry{
		{Name: "alpha", Token: "tok-a", Store: s1},
		{Name: "beta", Token: "tok-b", Store: s2},
	})

	projects := p.Projects()
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Name != "alpha" || projects[0].Store != s1 {
		t.Errorf("projects[0] = {%q, %v}, want {%q, s1}", projects[0].Name, projects[0].Store, "alpha")
	}
	if projects[1].Name != "beta" || projects[1].Store != s2 {
		t.Errorf("projects[1] = {%q, %v}, want {%q, s2}", projects[1].Name, projects[1].Store, "beta")
	}
}

func TestMultiProvider_ProjectsDoesNotExposeToken(t *testing.T) {
	s := loadTestStore(t)
	p := NewMultiStoreProvider([]ProviderEntry{
		{Name: "proj", Token: "secret-tok", Store: s},
	})

	projects := p.Projects()
	// ProjectInfo has no Token field — this is a compile-time guarantee.
	// We verify the returned data only contains Name and Store.
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "proj" {
		t.Errorf("expected name %q, got %q", "proj", projects[0].Name)
	}
	if projects[0].Store != s {
		t.Error("expected the store to match")
	}
}

func TestMultiProvider_EmptyEntries(t *testing.T) {
	p := NewMultiStoreProvider([]ProviderEntry{})

	got := p.Resolve("anything")
	if got != nil {
		t.Fatal("expected nil from empty provider")
	}
}

func TestMultiProvider_IsolatedFromExternalMutation(t *testing.T) {
	s1 := loadTestStore(t)
	s2 := loadTestStore(t)

	entries := []ProviderEntry{
		{Name: "proj-one", Token: "tok-one", Store: s1},
	}
	p := NewMultiStoreProvider(entries)

	// Mutate the original slice after construction.
	entries = append(entries, ProviderEntry{Name: "proj-two", Token: "tok-two", Store: s2})

	if got := p.Resolve("tok-two"); got != nil {
		t.Fatal("provider should not be affected by external slice mutation")
	}
	// Original mapping should still work.
	if got := p.Resolve("tok-one"); got != s1 {
		t.Fatal("expected s1 for tok-one after external mutation")
	}
}
