# Multi-Project Support for Beads Server

## Context

Currently the beads server supports a single project: one bearer token, one data file, one store. We want to support multiple projects on a single server instance, where each project has its own bearer token and data file, with complete data isolation. Crucially, when no multi-project config is provided, the server must behave exactly as it does today — no added complexity for single-project users.

## Architecture

**Core idea:** Introduce a `StoreProvider` — a simple interface that maps an HTTP request to the correct `*store.Store`. Two implementations:

1. **`SingleStoreProvider`** — always returns the same store (current behavior, used when `--projects` is not set)
2. **`MultiStoreProvider`** — looks up the bearer token to find the project, attaches the project's store to the request context

The auth middleware and store resolution are unified: in multi-project mode, a valid token simultaneously authenticates the request AND selects the project.

**Projects config file** (`projects.json`):
```json
{
  "projects": [
    { "name": "webapp",  "token": "tok-abc123", "data_file": "webapp.json" },
    { "name": "backend", "token": "tok-def456", "data_file": "backend.json" }
  ]
}
```

## Implementation Plan

### Step 1: Create `internal/project/project.go`

New package with:

```go
type ProjectEntry struct {
    Name     string `json:"name"`
    Token    string `json:"token"`
    DataFile string `json:"data_file"`
}

type projectsFile struct {
    Projects []ProjectEntry `json:"projects"`
}

func LoadProjectsFile(path string) ([]ProjectEntry, error)
```

Simple JSON parsing + validation (no empty names, no empty tokens, no duplicate tokens, no duplicate names). Tests in `project_test.go`.

### Step 2: Create `StoreProvider` interface and implementations

New file `internal/server/provider.go`:

```go
type StoreProvider interface {
    // Resolve authenticates the token and returns the store for this request.
    // Returns nil if the token is invalid.
    Resolve(token string) *store.Store
}
```

Two implementations:
- `singleStoreProvider` — holds one token and one store, returns the store if token matches
- `multiStoreProvider` — holds map[token]*store.Store, returns the matching store

Both live in `internal/server/provider.go` with tests in `provider_test.go`.

### Step 3: Refactor `Server` to use `StoreProvider`

Changes to `internal/server/server.go`:
- `Server` struct: replace `Store *store.Store` with `provider StoreProvider`
- Add `storeFor(r *http.Request) *store.Store` method that retrieves the store from request context
- Auth middleware: call `provider.Resolve(token)`, if nil return 401, otherwise store the result in request context via `context.WithValue`
- `New()` function signature changes: `New(cfg Config, provider StoreProvider)` instead of `New(cfg Config, s *store.Store)`

### Step 4: Update all handlers to use `s.storeFor(r)` instead of `s.Store`

Mechanical change across 3 files:
- `handlers.go`: replace `s.Store` -> `s.storeFor(r)`
- `handlers_query.go`: same
- `handlers_deps.go`: same

### Step 5: Update `internal/cli/serve.go` for multi-project startup

- Add `--projects` flag / `BS_PROJECTS_FILE` env var
- If set: load projects file, create a store per project, build `multiStoreProvider`
- If not set: use existing single-token + single-store logic with `singleStoreProvider`
- Validation: `--projects` and `--token` are mutually exclusive

### Step 6: Update tests

- `server_test.go`: update `testServer()` helper to construct a `singleStoreProvider`
- `e2e/e2e_test.go`: update `startServer()` similarly
- Add new tests:
  - `provider_test.go`: unit tests for both provider implementations
  - `server_test.go`: add test for multi-project auth (two tokens, two stores, verify isolation)
  - `e2e/e2e_test.go`: add multi-project e2e test (two projects, verify beads don't leak between projects)

### Step 7: Documentation

Update `docs/` with a `multi-project.md` describing the feature, config format, and usage.

## Files Modified

| File | Change |
|------|--------|
| `internal/project/project.go` | **NEW** — ProjectEntry struct, LoadProjectsFile |
| `internal/project/project_test.go` | **NEW** — validation tests |
| `internal/server/provider.go` | **NEW** — StoreProvider interface + two implementations |
| `internal/server/provider_test.go` | **NEW** — unit tests for providers |
| `internal/server/server.go` | Refactor: StoreProvider, context-based store lookup, updated auth middleware |
| `internal/server/handlers.go` | `s.Store` -> `s.storeFor(r)` |
| `internal/server/handlers_query.go` | `s.Store` -> `s.storeFor(r)` |
| `internal/server/handlers_deps.go` | `s.Store` -> `s.storeFor(r)` |
| `internal/server/server_test.go` | Update testServer helper, add multi-project auth test |
| `internal/cli/serve.go` | Add --projects flag, dual-mode startup logic |
| `e2e/e2e_test.go` | Update startServer, add multi-project e2e test |
| `docs/multi-project.md` | **NEW** — feature documentation |

## Files NOT Modified

- `internal/store/` — no changes at all (stores are independent, no project awareness needed)
- `internal/model/` — no changes (no project_id on beads)
- `internal/cli/client.go` — no changes (already sends Bearer token)
- `internal/cli/commands.go` — no changes
- `internal/cli/commands_query.go` — no changes

## Verification

1. `go test ./internal/project/...` — project config loading/validation
2. `go test ./internal/server/...` — provider tests, handler tests, auth tests (both modes)
3. `go test ./e2e/...` — full lifecycle in single-project mode + multi-project isolation test
4. `go test ./...` — full suite passes
5. Manual test: start server with `--projects`, use two different tokens, verify isolation

## Task Tracking (Beads Server)

Tasks are tracked in the running beads server. Dependency graph:

```
Step 1: bd-g5r1mres  Create project package          ──┐
Step 2: bd-9g7ua7f6  Create StoreProvider              ─┤
                                                        ▼
Step 3: bd-0dos9rc8  Refactor Server (blocked by 1,2)  ──┐
                                                         │
Step 4: bd-pflwzlzt  Update handlers (blocked by 3)      ├──┐
Step 5: bd-ys7mnign  Update serve cmd (blocked by 1,2,3) ├──┤
                                                         │  ▼
Step 6: bd-yrd39fsv  Tests (blocked by 3,4,5)        ◄──┘──┘
                         │
                         ▼
Step 7: bd-nglctru0  Documentation (blocked by 6)
```

| Bead ID | Step | Title |
|---------|------|-------|
| `bd-g5r1mres` | 1 | Create internal/project package with ProjectEntry and LoadProjectsFile |
| `bd-9g7ua7f6` | 2 | Create StoreProvider interface and implementations |
| `bd-0dos9rc8` | 3 | Refactor Server to use StoreProvider and context-based store lookup |
| `bd-pflwzlzt` | 4 | Update all handlers to use storeFor(r) instead of s.Store |
| `bd-ys7mnign` | 5 | Update serve command for dual-mode startup |
| `bd-yrd39fsv` | 6 | Update existing tests and add multi-project tests |
| `bd-nglctru0` | 7 | Add multi-project documentation |
