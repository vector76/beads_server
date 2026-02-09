package server

import "github.com/yourorg/beads_server/internal/store"

// StoreProvider resolves a bearer token to a Store.
// Returns nil if the token is not recognized.
type StoreProvider interface {
	Resolve(token string) *store.Store
}

// singleStoreProvider maps exactly one token to one store.
type singleStoreProvider struct {
	token string
	store *store.Store
}

// NewSingleStoreProvider returns a StoreProvider that accepts a single token.
func NewSingleStoreProvider(token string, s *store.Store) StoreProvider {
	return &singleStoreProvider{token: token, store: s}
}

func (p *singleStoreProvider) Resolve(token string) *store.Store {
	if token == p.token {
		return p.store
	}
	return nil
}

// multiStoreProvider maps multiple tokens to their respective stores.
type multiStoreProvider struct {
	stores map[string]*store.Store
}

// NewMultiStoreProvider returns a StoreProvider backed by a token-to-store map.
func NewMultiStoreProvider(stores map[string]*store.Store) StoreProvider {
	// Copy the map to prevent external mutation.
	m := make(map[string]*store.Store, len(stores))
	for k, v := range stores {
		m[k] = v
	}
	return &multiStoreProvider{stores: m}
}

func (p *multiStoreProvider) Resolve(token string) *store.Store {
	return p.stores[token]
}
