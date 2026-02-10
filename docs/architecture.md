# Architecture

## Package Structure

```
cmd/bs/main.go             Entry point — delegates to cli.NewRootCmd()
internal/
  model/                   Data types (Bead, Comment, enums)
  store/                   In-memory store with JSON file persistence
  server/                  HTTP server, chi router, handlers
  project/                 Multi-project config loader
  cli/                     Cobra commands, HTTP client
e2e/                       End-to-end tests
```

Dependencies flow in one direction:

```
model  <--  store  <--  server  <--  cli
                   \       \________/ |
                    \                 |
                     project --------/
```

`model` has no internal dependencies. `store` depends only on `model`. `server` depends on `store` and `model`. `project` depends only on the standard library (parses the projects config file). `cli` depends on `server`, `store`, and `project` (the `serve` command loads the store, optionally loads a projects config, and creates the server) and uses the HTTP client to talk to the server for all other commands.

## Package Responsibilities

**`internal/model`** — Pure data types. Defines the `Bead` struct, `Comment` struct, and enums (`Status`, `Priority`, `BeadType`). Provides ID generation helpers (`bd-` + 4–8 random alphanumeric chars) and JSON validation for enum types. No I/O, no state.

**`internal/store`** — The persistence and business logic layer. Holds all beads in a `map[string]model.Bead` protected by a `sync.RWMutex`. Provides CRUD with collision-aware ID generation, exact ID resolution, list/filter/sort/paginate, search, claim, comments, and dependency management (link/unlink/deps with cycle detection). Every mutation writes to disk atomically.

**`internal/project`** — Multi-project configuration. Defines `ProjectEntry` (name, token, data file) and `LoadProjectsFile()` to parse and validate a JSON projects config. No I/O beyond reading the config file.

**`internal/server`** — HTTP layer. Creates a chi router with request logging and bearer token auth middleware. Provides a `StoreProvider` interface that maps a bearer token to the correct store — `singleStoreProvider` for single-project mode, `multiStoreProvider` for multi-project mode. Includes an HTML dashboard at `/` showing bead status across all projects. Maps REST endpoints to store operations. Translates between HTTP request/response formats and store types. No business logic beyond request parsing and response formatting.

**`internal/cli`** — User-facing CLI built with cobra. The `serve` command starts the HTTP server directly (single-project mode with `--token`, or multi-project mode with `--projects`). All other commands are thin HTTP clients: they read `BS_URL`/`BS_TOKEN`/`BS_USER` from environment variables (with `.env` file fallback), call the server's REST API, and print the JSON response to stdout.

## Data Flow

A typical client command (e.g., `bs claim bd-a1b2`):

```
CLI (cobra command)
  → Client.Do("POST", "/api/v1/beads/bd-a1b2/claim", {user: "agent-1"})
    → HTTP request with Authorization: Bearer <token>
      → server.authMiddleware (validates token)
        → server.handleClaimBead (parses request, resolves ID)
          → store.Resolve("bd-a1b2") (exact match)
          → store.Claim(fullID, "agent-1") (acquires mutex, checks conflicts, updates state)
            → store.save() (marshal JSON, write temp file, rename)
          ← returns updated bead
        ← JSON response with 200/409
      ← HTTP response
    ← parsed JSON
  → pretty-print to stdout
```

## Concurrency Model

The store uses a single `sync.RWMutex`:

- **Reads** (`Get`, `Resolve`, `List`, `Search`, `Deps`) acquire `RLock` — concurrent reads are allowed
- **Writes** (`Create`, `Update`, `Delete`, `Claim`, `AddComment`, `Link`, `Unlink`) acquire `Lock` — serialized, one at a time

Every write persists to disk immediately via atomic write (write to temp file in the same directory, then `os.Rename`). If the write fails, the in-memory state is rolled back to the previous value.

This single-writer model is deliberately simple. Issue tracker throughput doesn't justify a database — the mutex serialization is sufficient and the atomic file writes prevent data loss on crashes.

## Storage Format

The data file (`beads.json`) is a single JSON object:

```json
{
  "beads": [
    { "id": "bd-a1b2c3d4", "title": "...", ... },
    { "id": "bd-e5f6g7h8", "title": "...", ... }
  ]
}
```

On startup, the store loads this file into the in-memory map. If the file doesn't exist, an empty store is initialized. The file is human-readable (indented with 2 spaces).

## Test Organization

Tests are organized in four layers, matching the package structure:

**Unit tests (`internal/model/`)** — Validate JSON serialization round-trips, enum validation, ID format, and default values. Fast, no I/O.

**Store tests (`internal/store/`)** — Test all store operations against a real temp file. Cover CRUD, collision-aware ID generation, exact ID resolution, filtering, pagination, search, claim semantics (idempotent, conflict, terminal state), dependency operations (link, unlink, cycle detection), and unblocked computation. Four test files mirror the four source files (`store_test.go`, `list_test.go`, `ops_test.go`, `deps_test.go`).

**Project tests (`internal/project/`)** — Validate project config loading and validation: non-empty fields, no duplicate names or tokens.

**Server tests (`internal/server/`)** — Use `httptest.NewServer` with a real store (temp file). Test each HTTP handler: request parsing, response format, status codes, auth middleware, and store provider routing. Four test files mirror the four source files (`server_test.go`, `handlers_test.go`, `handlers_query_test.go`, `handlers_deps_test.go`), plus `provider_test.go` for provider implementations.

**CLI tests (`internal/cli/`)** — Start a test HTTP server, set environment variables, execute cobra commands, and verify the JSON output. Test the full CLI-to-server round-trip without a real network. Four test files: `cli_test.go` (whoami, help, serve validation), `commands_test.go` (CRUD), `commands_query_test.go` (list, search, claim, comments, dependencies), `dotenv_test.go` (.env file parsing and fallback logic).

**End-to-end tests (`e2e/`)** — Start a real server on a random port, run through complete workflows: create beads, claim work, add comments, manage dependencies, verify multi-agent conflict handling. The most comprehensive integration test.

Run specific layers:

```bash
go test ./internal/model/...
go test ./internal/store/...
go test ./internal/project/...
go test ./internal/server/...
go test ./internal/cli/...
go test ./e2e/...
```

## Design Decisions

**Single binary, two modes.** The `bs` binary is both server and CLI client. `bs serve` starts the HTTP server; every other command is a client that talks to the server via HTTP. This avoids separate installs and keeps deployment simple.

**JSON-only output.** All CLI output is pretty-printed JSON. No tables, no colors, no human-oriented formatting. Agents parse JSON natively; humans can pipe through `jq`. This eliminates an entire class of formatting/parsing bugs.

**Chi for routing, cobra for CLI.** Both are lightweight, widely used Go libraries. Chi adds path parameters and middleware grouping on top of `net/http`. Cobra provides flag parsing, help text, and command grouping. Neither imposes architectural constraints.

**Single JSON file for storage.** A database would add deployment complexity for zero throughput benefit. Issue trackers have low write rates. The JSON file is human-inspectable, trivially backupable, and requires no setup. Atomic writes (temp file + rename) prevent corruption.

**Short IDs with exact matching.** Bead IDs are `bd-` + 4–8 random chars (default 4). IDs are generated at the store layer with collision detection: on collision, the length escalates from 4 up to 8 with retries at each level. IDs must be specified exactly and in full (including the `bd-` prefix). The short default length keeps IDs easy to type while the escalation ensures uniqueness at scale.

**Soft delete.** `delete` sets status to `deleted` rather than removing the bead. This preserves history and enables recovery via `reopen`. Deleted beads are excluded from default queries but visible with `--all` or `--status deleted`.
