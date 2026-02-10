# Planned Updates

## 1. Request Logging Middleware

**Goal:** Every API request produces one line on stdout indicating activity.

**Approach:** Add a custom chi middleware in `internal/server/server.go` that logs a single line per request. It wraps the response writer to capture the status code, then prints a line after the handler completes.

**Output format:**
```
2026/02/09 14:30:00 GET /api/v1/beads 200 12ms
```

**Implementation:**
- Add a `requestLogger` middleware function on `*Server` (or a standalone func) in `server.go`
- Use a `wrapResponseWriter` that captures the status code written by the handler
- Use `log.New(w, "", log.LstdFlags)` for output — the `log` package provides thread-safe writes via an internal mutex (important under concurrent requests), gives timestamps for free, and is idiomatic Go. The logger is created once in `New()` and stored on the `Server` struct.
- Add a `LogOutput io.Writer` field to `Config`. Production code (`serve.go`) sets it to `os.Stdout`. If nil, `New()` defaults to `os.Stdout`. Test helpers pass `io.Discard` to suppress the 57+ log lines that would otherwise pollute test output. A test for the logging middleware itself can pass a `bytes.Buffer` to assert on the output format.
- Insert it in the middleware chain in `New()`, right after `middleware.Recoverer` and before any route registration — this way it covers all routes including `/api/v1/health` and the new `/` dashboard
- No external logging library needed; only `log`, `os`, and `io` from the standard library

**Files changed:**
- `internal/server/server.go` — add logger field to Server, add `LogOutput` to Config, add middleware, wire it into the router
- `internal/server/handlers_test.go`, `server_test.go`, `handlers_query_test.go`, `handlers_deps_test.go` — update test helpers to pass `LogOutput: io.Discard` in Config
- `internal/cli/commands_test.go`, `e2e/e2e_test.go` — same

## 2. HTML Dashboard at `/`

**Goal:** Hitting `GET /` in a browser shows a rudimentary HTML page with bead status overview (in-progress, open, and other statuses).

**Approach:** Add an unauthenticated `GET /` handler that queries all stores via the provider, renders counts and bead lists grouped by status using Go's `html/template`.

**Key design decision — accessing stores without auth:**
The current `StoreProvider` interface only has `Resolve(token)`. The dashboard is unauthenticated and needs read access to all stores. We extend the interface:

```go
type StoreProvider interface {
    Resolve(token string) *store.Store
    Projects() []ProjectInfo
}

type ProjectInfo struct {
    Name  string
    Store *store.Store
}
```

- `singleStoreProvider.Projects()` returns one entry with name `"default"`
- `multiStoreProvider` needs to be updated to retain project names (currently it only maps token->store). The constructor `NewMultiStoreProvider` changes to accept a `[]ProviderEntry` struct that carries name, token, and store together:

```go
type ProviderEntry struct {
    Name  string
    Token string
    Store *store.Store
}

func NewMultiStoreProvider(entries []ProviderEntry) StoreProvider
```

The provider internally builds a `map[string]*store.Store` (token->store) for `Resolve()` and a `[]ProjectInfo` (name+store) for `Projects()`. The `Token` field is not exposed through the `ProjectInfo` return type, keeping auth details out of the dashboard path.

The serve command already has `project.ProjectEntry` with all three fields (name, token, data_file), so constructing `[]ProviderEntry` is straightforward.

**Callers that need updating for `NewMultiStoreProvider` signature change:**
- `internal/cli/serve.go:72` — production
- `internal/server/server_test.go:158` — test
- `internal/server/provider_test.go:57, 72, 83` — tests
- `e2e/e2e_test.go:307` — test

**HTML rendering:**
- Define an inline Go template as a `const` or `var` in a new file `internal/server/dashboard.go`
- The template shows:
  - Summary counts per status (in_progress, open, closed, etc.)
  - A table of in-progress beads (id, title, assignee, priority)
  - A table of open beads
  - A collapsible or separate section for other statuses
- In multi-project mode, show a section per project
- Minimal inline CSS for readability (no external assets)
- The handler calls `s.provider.Projects()`, iterates stores, calls `store.List(...)` for each status group, and feeds the data into the template

**Files changed:**
- `internal/server/provider.go` — add `ProjectInfo` type, `ProviderEntry` type, `Projects()` to interface, update both providers
- `internal/server/dashboard.go` (new) — template + `handleDashboard` handler
- `internal/server/server.go` — register `GET /` route (unauthenticated, alongside health)
- `internal/cli/serve.go` — construct `[]ProviderEntry` from project entries
- `internal/server/server_test.go`, `provider_test.go`, `e2e/e2e_test.go` — update `NewMultiStoreProvider` calls

## 3. Cleanup: Update `Server.Store` Comment

The `Server.Store` field (`server.go:29`) has a stale comment: "retained for backward compatibility; handlers will migrate to storeFor(r)". All handlers have already migrated to `storeFor(r)`. However, the field is still actively used in tests (4 test files) for direct store access during setup and assertions — it is NOT dead code.

**Action:** Update the comment to reflect reality (used by tests for direct store access). Do not remove the field.

**Files changed:**
- `internal/server/server.go` — update comment

## 4. Cleanup: Introduce `NotFoundError` for Error Matching

There are 11 instances across handlers of `strings.Contains(err.Error(), "not found")` for distinguishing 404 vs 400 responses. This is fragile — if the error message text ever changes, these checks break silently. The codebase already uses this pattern properly with `ConflictError` in `store/ops.go`.

**Action:** Add a `NotFoundError` type to the store package (alongside `ConflictError`), return it from `Resolve`, `Get`, and other store methods that produce "not found" errors, and update handlers to use `errors.As(err, &notFoundErr)`.

**Files changed:**
- `internal/store/ops.go` (or `store.go`) — add `NotFoundError` type
- `internal/store/store.go` — return `NotFoundError` from `Get`, `Resolve`, `Update` (not-found case)
- `internal/store/ops.go` — return `NotFoundError` from `AddComment`, `Claim` (not-found case)
- `internal/store/deps.go` — return `NotFoundError` from `Link`, `Unlink`, `Deps` (not-found case)
- `internal/server/handlers.go` — replace string matching with `errors.As`
- `internal/server/handlers_query.go` — same
- `internal/server/handlers_deps.go` — same

## Implementation Order

1. Introduce `NotFoundError` + update store methods and handlers (foundational cleanup, reduces risk of later changes breaking error handling)
2. Update stale `Server.Store` comment (trivial, do it while editing `server.go`)
3. Request logging middleware + configurable log output in Config + update test helpers to use `io.Discard`
4. Extend `StoreProvider` with `Projects()` + update providers + update callers (including `serve.go`)
5. Dashboard handler, template, and route wiring
6. Tests for all changes
