package server

import (
	"path/filepath"
	"testing"

	"github.com/yourorg/beads_server/internal/store"
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

// --- multiStoreProvider tests ---

func TestMultiProvider_KnownTokens(t *testing.T) {
	s1 := loadTestStore(t)
	s2 := loadTestStore(t)

	p := NewMultiStoreProvider(map[string]*store.Store{
		"tok-one": s1,
		"tok-two": s2,
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
	p := NewMultiStoreProvider(map[string]*store.Store{
		"tok-one": s1,
	})

	got := p.Resolve("tok-unknown")
	if got != nil {
		t.Fatal("expected nil for unknown token")
	}
}

func TestMultiProvider_EmptyMap(t *testing.T) {
	p := NewMultiStoreProvider(map[string]*store.Store{})

	got := p.Resolve("anything")
	if got != nil {
		t.Fatal("expected nil from empty provider")
	}
}

func TestMultiProvider_IsolatedFromExternalMutation(t *testing.T) {
	s1 := loadTestStore(t)
	s2 := loadTestStore(t)

	m := map[string]*store.Store{"tok-one": s1}
	p := NewMultiStoreProvider(m)

	// Mutate the original map after construction.
	m["tok-two"] = s2

	if got := p.Resolve("tok-two"); got != nil {
		t.Fatal("provider should not be affected by external map mutation")
	}
	// Original mapping should still work.
	if got := p.Resolve("tok-one"); got != s1 {
		t.Fatal("expected s1 for tok-one after external mutation")
	}
}
