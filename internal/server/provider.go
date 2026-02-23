package server

import "github.com/vector76/beads_server/internal/store"

// ProjectInfo exposes a project's name and store without auth details.
type ProjectInfo struct {
	Name  string
	Store *store.Store
}

// ProviderEntry is the input to NewMultiStoreProvider: a named project with
// its auth token and backing store.
type ProviderEntry struct {
	Name  string
	Token string
	Store *store.Store
}

// StoreProvider resolves a bearer token to a Store.
// Returns nil if the token is not recognized.
type StoreProvider interface {
	Resolve(token string) *store.Store
	Projects() []ProjectInfo
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

func (p *singleStoreProvider) Projects() []ProjectInfo {
	return []ProjectInfo{{Name: "default", Store: p.store}}
}

// multiStoreProvider maps multiple tokens to their respective stores.
type multiStoreProvider struct {
	stores   map[string]*store.Store
	projects []ProjectInfo
}

// NewMultiStoreProvider returns a StoreProvider backed by a slice of ProviderEntry values.
func NewMultiStoreProvider(entries []ProviderEntry) StoreProvider {
	m := make(map[string]*store.Store, len(entries))
	projects := make([]ProjectInfo, len(entries))
	for i, e := range entries {
		m[e.Token] = e.Store
		projects[i] = ProjectInfo{Name: e.Name, Store: e.Store}
	}
	return &multiStoreProvider{stores: m, projects: projects}
}

func (p *multiStoreProvider) Resolve(token string) *store.Store {
	return p.stores[token]
}

func (p *multiStoreProvider) Projects() []ProjectInfo {
	result := make([]ProjectInfo, len(p.projects))
	copy(result, p.projects)
	return result
}
