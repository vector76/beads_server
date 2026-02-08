# Beads Server - Implementation Plan

This plan breaks the project into tasks of roughly 15-30 minutes each. Tasks are ordered so that each builds on the previous ones. The project follows TDD — each task includes writing tests alongside implementation.

---

## Task 1: Project scaffolding and Go module setup

Initialize the Go module, set up directory structure, and get a minimal `main.go` compiling.

- `go mod init` with module name (e.g., `github.com/yourorg/beads_server`)
- Add dependencies: `chi`, `cobra`
- Create directory layout:
  ```
  cmd/bs/main.go          # Entry point
  internal/model/          # Data types
  internal/store/           # Storage layer
  internal/server/          # HTTP server and routes
  internal/cli/             # CLI commands
  ```
- Minimal `main.go` that prints version and exits
- Verify `go build` produces a `bs` binary
- Add a smoke test that the binary runs without error

---

## Task 2: Data model and serialization

Define the core data types and verify JSON round-tripping.

- `internal/model/bead.go`: `Bead` struct with all fields (id, title, description, status, priority, type, tags, blocked_by, assignee, comments, created_at, updated_at)
- `internal/model/comment.go`: `Comment` struct
- Define enums for status, priority, and type with JSON marshal/unmarshal
- Defaults: status=open, priority=medium, type=task
- ID generation: `bd-` + 8 random lowercase alphanumeric chars
- Tests: JSON serialization/deserialization round-trips, enum validation, ID format validation, defaults applied correctly

---

## Task 3: Storage layer — in-memory store with JSON file persistence

Implement the `Store` that holds beads in memory and persists to a JSON file.

- `internal/store/store.go`: `Store` struct with mutex, bead map, and file path
- `Load(path)` — read and parse `beads.json`, or initialize empty if file doesn't exist
- `Save()` — atomic write (temp file + rename)
- `Create(bead)` — add bead, save, return created bead
- `Get(id)` — exact lookup
- `Update(id, fields)` — partial update, set updated_at, save
- `Delete(id)` — soft-delete (set status=deleted), save
- `All()` — return all beads
- Tests: create/read/update/delete cycle, atomic write (verify file exists after save), load from existing file, load from missing file creates empty store

---

## Task 4: Store — ID prefix resolution

Add the ability to resolve beads by unique ID prefix.

- `Resolve(prefix)` — find bead by exact ID or unique prefix match; error if ambiguous (list matches) or not found
- Accept both `bd-xxxxx` and `xxxxx` forms (auto-prepend `bd-` if missing)
- Tests: exact match, unique prefix match, ambiguous prefix returns error with matching IDs, not found, prefix with/without `bd-` prefix

---

## Task 5: Store — list filtering, sorting, and pagination

Implement the query logic for listing beads.

- `List(filters)` with filter struct: status (multi), priority, type, tag, assignee, all-flag, ready-flag
- Default filter: status in [open, in_progress]
- `--all` flag: no status filter
- `--ready`: status=open AND no active blockers (active = open or in_progress)
- Sorting: priority (critical > high > medium > low > none), then created_at descending
- Pagination: page, per_page, return total count and total_pages
- Return summary fields only: id, title, status, priority, type, assignee
- Tests: filtering by each field, ready filter with blocked/unblocked beads, sorting order, pagination math, default filters

---

## Task 6: Store — search, comments, and claim

Implement remaining store operations.

- `Search(query, page, perPage)` — case-insensitive substring match on title and description; exclude deleted; same pagination and summary fields as list
- `AddComment(beadID, comment)` — append comment, save
- `Claim(beadID, user)` — atomic set status=in_progress + assignee; 409 logic (different assignee, terminal state); idempotent for same user
- Tests: search matches title, search matches description, search excludes deleted, comment added correctly, claim success, claim idempotent, claim conflict (different user), claim conflict (terminal state)

---

## Task 7: Store — dependency management and unblocked computation

Implement link/unlink/deps operations and the cross-cutting "unblocked" logic.

- `Link(beadID, blockedByID)` — add to blocked_by list; reject circular deps, reject non-existent/deleted targets, reject self-links, reject duplicates
- `Unlink(beadID, blockedByID)` — remove from blocked_by list
- `Deps(beadID)` — return active blockers, resolved blockers, and computed blocks (inverse lookup); only include active (non-deleted) beads in blocks
- `ComputeUnblocked(beadID)` — given a bead whose status just changed to closed/resolved/wontfix/deleted, find beads that were blocked only by this one and are now unblocked. This is called by the store's Update and Delete methods and its result is surfaced through the API response's `"unblocked"` field
- Tests: add/remove links, circular detection, non-existent target rejected, deps response structure, unblocked computation on resolve/close

---

## Task 8: HTTP server setup, health endpoint, and auth middleware

Set up the chi router, auth middleware, and health check.

- `internal/server/server.go`: create chi router, configure middleware
- Auth middleware: check `Authorization: Bearer <token>` on all routes except `/api/v1/health`; 401 on missing/invalid token
- `GET /api/v1/health` — returns `{"status": "ok"}`, no auth required
- Server config: port, token, data-file path
- Server refuses to start without a token configured
- Tests: health endpoint returns 200, request without token returns 401, request with wrong token returns 401, request with valid token succeeds

---

## Task 9: API — CRUD endpoints (create, get, update, delete beads)

Wire up the core bead endpoints.

- `POST /api/v1/beads` — create bead from JSON body (title required); return 201 + created bead
- `GET /api/v1/beads/:id` — return full bead (uses ID prefix resolution); 404 if not found
- `PATCH /api/v1/beads/:id` — partial update; handle add-tag/remove-tag in request body; return updated bead; include `unblocked` field when status change unblocks other beads
- `DELETE /api/v1/beads/:id` — soft delete; return deleted bead; include `unblocked` field if applicable
- Error responses as `{"error": "..."}` with appropriate status codes (400, 404)
- Tests: create with minimal fields, create with all fields, get existing, get not found, get by prefix, update fields, update tags, soft delete

---

## Task 10: API — list, search, and claim endpoints

Wire up the query and claim endpoints.

- `GET /api/v1/beads` — list with query params for all filters + pagination; returns paginated response
- `GET /api/v1/search?q=...` — search with pagination
- `POST /api/v1/beads/:id/claim` — claim endpoint; read user from request body; return 409 on conflict
- Tests: list with filters, list pagination, search, claim success/conflict

---

## Task 11: API — comment and dependency endpoints

Wire up the remaining endpoints.

- `POST /api/v1/beads/:id/comments` — add comment; author + text from body
- `POST /api/v1/beads/:id/link` — add dependency; `blocked_by` ID in body
- `DELETE /api/v1/beads/:id/link/:other_id` — remove dependency
- `GET /api/v1/beads/:id/deps` — return deps response
- Tests: add comment, link/unlink, deps

---

## Task 12: CLI framework and `serve` + `whoami` commands

Set up cobra and implement the first two CLI commands.

- `internal/cli/root.go`: root cobra command
- `bs serve` command: start HTTP server with --port, --data-file, --token flags; read from env vars as fallback; refuse to start without token
- `bs whoami` command: print `{"user": "<BS_USER>"}` (local-only, no server contact)
- Wire up in `main.go`
- Tests: whoami output, serve refuses without token

---

## Task 13: CLI HTTP client and CRUD commands

Implement the HTTP client helper and basic bead commands.

- `internal/cli/client.go`: HTTP client that reads BS_URL and BS_TOKEN, sends requests with auth header, returns parsed JSON. Exits with error if BS_TOKEN is not set
- `bs add "title" [--type, --priority, --description, --tags]` — POST to create
- `bs show <id>` — GET bead
- `bs edit <id> [--title, --status, --priority, --type, --description, --assignee, --add-tag, --remove-tag]` — PATCH bead
- `bs close <id>`, `bs resolve <id>`, `bs reopen <id>` — status shorthand commands
- `bs delete <id>` — DELETE bead
- All output as pretty-printed JSON (2-space indent) to stdout; errors to stderr; exit code 0/1
- Tests: integration tests using a test server — add/show/edit/close/resolve/reopen/delete cycle

---

## Task 14: CLI — list, search, claim, mine, comment, and dependency commands

Implement the remaining CLI commands.

- `bs list [--all, --ready, --status, --priority, --type, --tag, --assignee, --page, --per-page]`
- `bs search "query"`
- `bs claim <id>` — POST claim with BS_USER
- `bs mine` — shorthand for `list --assignee=BS_USER --status=in_progress` (no special server endpoint; the CLI composes the query params)
- `bs comment <id> "text"` — POST comment with BS_USER as author
- `bs link <id> --blocked-by <other_id>` — POST link
- `bs unlink <id> --blocked-by <other_id>` — DELETE link
- `bs deps <id>` — GET deps
- Tests: integration tests for each command

---

## Task 15: End-to-end tests and cross-compilation

Validate the full system and set up build targets.

- End-to-end tests: start a real server, run CLI commands against it, verify results
- Multi-agent scenario: two agents claiming work, conflict handling
- Dependency scenario: create chain, resolve blockers, verify unblocked
- Set up `Makefile` or build script for cross-compilation: `GOOS=linux GOARCH=amd64` and `GOOS=windows GOARCH=amd64`
- Verify both binaries build successfully
- Final pass: run full test suite, check for any gaps in coverage

---

## Summary

| Task | Focus | Depends on |
|------|-------|-----------|
| 1 | Project scaffolding | — |
| 2 | Data model | 1 |
| 3 | Store: CRUD + persistence | 2 |
| 4 | Store: ID prefix resolution | 3 |
| 5 | Store: list/filter/sort/paginate | 3 |
| 6 | Store: search, comments, claim | 3 |
| 7 | Store: dependencies + unblocked | 3 |
| 8 | HTTP server, health, auth | 3 |
| 9 | API: CRUD endpoints | 4, 7, 8 |
| 10 | API: list, search, claim | 5, 6, 9 |
| 11 | API: comments, dependencies | 7, 9 |
| 12 | CLI: serve + whoami | 8 |
| 13 | CLI: CRUD commands | 9, 12 |
| 14 | CLI: remaining commands | 10, 11, 13 |
| 15 | E2E tests + cross-compilation | 14 |

Tasks 4-8 can be worked in parallel. Tasks 10-12 can also be parallelized. The critical path is: 1 → 2 → 3 → 8 → 9 → 10 → 13 → 14 → 15.
