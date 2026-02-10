# Beads Server - Product Requirements Document

## Overview

An issue tracker optimized for coding agent use. A single executable (`bs`) that runs as either a **server** (holds the issue database, exposes an HTTP/JSON API) or a **CLI client** (thin client that talks to the server). Inspired by Steve Yegge's beads, but with a clean client-server architecture that avoids the git-sync and dual-storage complexity of the original.

## Goals

1. **Agent-first**: All output is JSON (pretty-printed). Agents parse JSON; humans can read it directly or pipe through `jq`
2. **Simple and focused**: Only issue tracking. No git sync, no plugins, no bloat
3. **Client-server**: Server is the single source of truth. Works across worktrees, containers, and remote hosts
4. **Easy to run**: Single binary, minimal configuration. Cross-compiled for Linux and Windows
5. **Multi-agent**: Multiple agents can share one tracker, claim work items, and resume aborted tasks

## Architecture

### Single Executable, Two Modes

```
bs serve                     # Start the server (HTTP API)
bs <command>                 # CLI client mode (talks to the server)
```

### Technology

- **Language**: Go
- **HTTP framework**: net/http (standard library) with chi for routing
- **Storage**: Single JSON file on disk (simple, human-readable, sufficient for issue-tracker throughput)
- **CLI parsing**: cobra
- **Serialization**: encoding/json (standard library)
- **Build targets**: Linux (amd64), Windows (amd64)

### Server

- HTTP REST API, JSON in and out for all endpoints
- Listens on `0.0.0.0:9999` by default (configurable via `--port` or `BS_PORT` env var)
- Requires a bearer token on every request. Token is configured via `--token` flag or `BS_TOKEN` env var on the server. Requests without a valid token are rejected with 401. There is no "open" mode — a misconfigured or missing token does not fall through to unauthenticated access. The server refuses to start if no token is configured
- Reads/writes a single `beads.json` file (configurable via `--data-file` or `BS_DATA_FILE` env var)
- File is written atomically on every mutation (write to temp file, then rename). Server can be killed at any time without data loss
- If the data file does not exist on startup, it is created with an empty bead list
- Supports multi-project mode via `--projects` flag or `BS_PROJECTS_FILE` env var. A JSON config file maps each project to its own token and data file. `--projects` and `--token` are mutually exclusive
- An HTML dashboard at `/` shows bead status overview across all projects (unauthenticated)
- Request logging middleware logs each request (method, path, status, duration) to stdout

### CLI Client

- Discovers server via `BS_URL` env var (full URL, e.g. `http://localhost:9999`, `https://beads.example.com`, or an ngrok tunnel URL). Falls back to `http://localhost:9999` if unset
- Authenticates via `BS_TOKEN` env var (bearer token). Exits with error if `BS_TOKEN` is not set
- All commands hit the server's HTTP API
- All output is pretty-printed JSON (indented, multi-line). No `--json` flag needed — JSON is the only output format

### Environment Variables Summary

| Variable | Used by | Description |
|----------|---------|-------------|
| `BS_URL` | Client | Full server URL (default: `http://localhost:9999`) |
| `BS_TOKEN` | Client & Server | Bearer token for authentication (required) |
| `BS_PORT` | Server | Listen port (default: `9999`) |
| `BS_DATA_FILE` | Server | Path to data file (default: `./beads.json`) |
| `BS_USER` | Client | Agent/user identity for `claim` and `comment` author (default: `anonymous`) |
| `BS_PROJECTS_FILE` | Server | Path to multi-project config file (mutually exclusive with `BS_TOKEN` for server) |

## Data Model

### Bead (Issue)

| Field | Type | Description |
|-------|------|-------------|
| `id` | String | Fixed prefix `bd-` + 4–8 char random lowercase alphanumeric (e.g. `bd-a1b2`). Default 4 chars, escalates on collision |
| `title` | String | One-line summary (required) |
| `description` | String | Longer body text (optional, default empty) |
| `status` | Enum | `open`, `in_progress`, `closed`, `deleted` |
| `priority` | Enum | `critical`, `high`, `medium`, `low`, `none` |
| `type` | Enum | `bug`, `feature`, `task`, `epic`, `chore` |
| `tags` | List\<String\> | Free-form labels |
| `blocked_by` | List\<ID\> | IDs of beads this one depends on |
| `assignee` | String | Optional assignee (set via `claim` or `edit`) |
| `comments` | List\<Comment\> | List of comments (see below) |
| `created_at` | ISO 8601 timestamp | Auto-set on creation |
| `updated_at` | ISO 8601 timestamp | Auto-set on every mutation |

### Comment

| Field | Type | Description |
|-------|------|-------------|
| `author` | String | From `BS_USER` env var or `"anonymous"` |
| `text` | String | Comment body |
| `created_at` | ISO 8601 timestamp | Auto-set on creation |

**Notes:**
- `blocks` is not stored; it is computed as the inverse of `blocked_by`
- Default status: `open`
- Default priority: `medium`
- Default type: `task`
- A bead with status `open` whose `blocked_by` list contains any active (`open`/`in_progress`) bead is considered "blocked" (computed, not stored)
- The `bd-` prefix is fixed and not configurable. It disambiguates bead IDs from other tokens (e.g. `bd-ls` is clearly a bead, not a shell command)

## CLI Commands

### Server Management

| Command | Description |
|---------|-------------|
| `bs serve` | Start the server |
| `bs serve --port 8080` | Start on a specific port |
| `bs serve --data-file /path/to/beads.json` | Use a specific data file |
| `bs serve --token <secret>` | Set the bearer token (or use `BS_TOKEN`) |

### Agent Identity

| Command | Description |
|---------|-------------|
| `bs whoami` | Print the current agent identity (from `BS_USER` env var) as JSON. Local-only, does not contact the server |

### Issue CRUD

| Command | Description |
|---------|-------------|
| `bs add "Fix login bug"` | Create a bead with title |
| `bs add "Fix login bug" --type bug --priority high --description "..." --tags "auth,urgent"` | Create with all fields |
| `bs show <id>` | Show full details of a bead |
| `bs edit <id> --title "New title"` | Modify any field(s) |
| `bs edit <id> --status in_progress` | Change status |
| `bs edit <id> --add-tag foo --remove-tag bar` | Incremental tag editing |
| `bs close <id>` | Shorthand for `--status closed` |
| `bs reopen <id>` | Shorthand for `--status open` |
| `bs delete <id>` | Soft-delete: sets status to `deleted` |

### Claiming Work

| Command | Description |
|---------|-------------|
| `bs claim <id>` | Atomically set status to `in_progress` and assignee to `BS_USER`. Fails if already in_progress and assigned to someone else |
| `bs mine` | List beads assigned to the current `BS_USER` that are `in_progress` (useful for resuming after an aborted run) |

The `claim` command enables a multi-agent workflow:
1. Agent starts up, runs `bs mine` to check for previously claimed but unfinished work
2. If found, the agent resumes that work
3. If not, the agent runs `bs list --ready` to find open, unblocked work, then `bs claim <id>` to take ownership
4. When done, the agent runs `bs close <id>`

### Listing and Search

| Command | Description |
|---------|-------------|
| `bs list` | List active beads (status = `open` or `in_progress`). Excludes deleted |
| `bs list --all` | List all beads regardless of status (including deleted) |
| `bs list --ready` | List beads that are `open` and not blocked by any active bead |
| `bs list --status closed` | Filter by status. Accepts comma-separated values: `--status open,in_progress` |
| `bs list --status deleted` | Show soft-deleted beads |
| `bs list --priority high` | Filter by priority |
| `bs list --type bug` | Filter by type |
| `bs list --tag auth` | Filter by tag (OR semantics: `--tag "auth,security"` matches either) |
| `bs list --assignee agent-1` | Filter by assignee |
| `bs list --page 2 --per-page 50` | Pagination (default: page 1, 100 items per page) |
| `bs search "login"` | Substring search across title and description (case-insensitive) |

`list` and `search` return key fields only: `id`, `title`, `status`, `priority`, `type`, `assignee`. Use `show <id>` for full details including description, comments, tags, and timestamps.

### Dependencies

| Command | Description |
|---------|-------------|
| `bs link <id> --blocked-by <other_id>` | Add a dependency |
| `bs unlink <id> --blocked-by <other_id>` | Remove a dependency |
| `bs deps <id>` | Show direct dependencies: active blockers, resolved blockers, and what this bead blocks (flat, non-transitive) |

### Comments

| Command | Description |
|---------|-------------|
| `bs comment <id> "This is a comment"` | Add a comment to a bead. Author is set from `BS_USER` |

### Maintenance

| Command | Description |
|---------|-------------|
| `bs clean` | Purge closed/deleted beads older than 5 days (default). `--days N` overrides threshold; `--days 0` removes all |

## REST API

All endpoints under `/api/v1/`. All requests require `Authorization: Bearer <token>` header (except `/api/v1/health`). All responses are `application/json`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/beads` | List beads (query params for filters, pagination) |
| `POST` | `/api/v1/beads` | Create a bead |
| `GET` | `/api/v1/beads/:id` | Get a bead |
| `PATCH` | `/api/v1/beads/:id` | Update a bead |
| `DELETE` | `/api/v1/beads/:id` | Soft-delete a bead (sets status to `deleted`) |
| `POST` | `/api/v1/beads/:id/claim` | Claim a bead (atomic set in_progress + assignee) |
| `POST` | `/api/v1/beads/:id/comments` | Add a comment |
| `POST` | `/api/v1/beads/:id/link` | Add a dependency |
| `DELETE` | `/api/v1/beads/:id/link/:other_id` | Remove a dependency |
| `GET` | `/api/v1/beads/:id/deps` | Get direct dependencies (active blockers, resolved blockers, blocks) |
| `GET` | `/api/v1/search?q=...` | Substring search |
| `POST` | `/api/v1/clean` | Purge old closed/deleted beads |
| `GET` | `/api/v1/health` | Health check (does not require auth) |

### Pagination

The `GET /api/v1/beads` and `GET /api/v1/search` endpoints support pagination via query parameters:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number (1-indexed) |
| `per_page` | `100` | Items per page |

Response includes pagination metadata:

```json
{
  "beads": [...],
  "page": 1,
  "per_page": 100,
  "total": 247,
  "total_pages": 3
}
```

### Error Responses

All errors return a JSON object: `{"error": "description of what went wrong"}` with an appropriate HTTP status code (400, 401, 404, 409, 500).

## Behavior Details

### ID Format
- All bead IDs use the format `bd-<4–8 lowercase alphanumeric chars>` (e.g. `bd-a1b2`)
- Default random portion is 4 characters. On collision during generation, length escalates (4→5→6→7→8, 3 retries per length)
- The `bd-` prefix is fixed and serves to disambiguate bead IDs from other tokens
- IDs must be specified exactly and in full, including the `bd-` prefix

### Soft Delete
- `delete` sets the bead's status to `deleted`. The bead remains in the data file
- Deleted beads are excluded from `list` (default), `list --ready`, `mine`, and `search`
- Deleted beads are included in `list --all` and `list --status deleted`
- Deleted beads can be restored via `bs reopen <id>`
- `show <id>` works on deleted beads (returns the bead with status `deleted`)

### Dependency Semantics

**`deps` response format:**

```json
{
  "blocked_by": [
    {"id": "bd-a1b2c3d4", "title": "Fix auth", "status": "open", "priority": "high", "type": "bug", "assignee": "agent-1"}
  ],
  "resolved_blockers": [
    {"id": "bd-x9y8z7w6", "title": "Setup DB", "status": "closed", "priority": "medium", "type": "task", "assignee": ""}
  ],
  "blocks": [
    {"id": "bd-e5f6g7h8", "title": "Deploy", "status": "open", "priority": "medium", "type": "task", "assignee": ""}
  ]
}
```

- **`blocked_by`**: Only active blockers (status = `open` or `in_progress`). If this list is empty, the bead is unblocked — no further inspection needed
- **`resolved_blockers`**: Blockers in the storage-layer `blocked_by` list whose status is `closed` or `deleted`. Included for context; agents can ignore this field
- **`blocks`**: Beads that list this bead in their `blocked_by` (computed inverse). Only active beads (not deleted) are shown

The storage layer is unchanged — `blocked_by` stores all dependency IDs regardless of status. The `deps` endpoint and the `--ready` filter both apply the same logic: only `open`/`in_progress` blockers count as active.

**Validation:**
- Circular dependencies are rejected
- Closing/resolving a bead that blocks other beads: allowed. The response includes an `"unblocked"` field listing beads that are now unblocked
- Adding a dependency to a non-existent or deleted bead: rejected

### Claim Semantics
- `claim` atomically sets `status = in_progress` and `assignee = <BS_USER>`
- If the bead is already `in_progress` and assigned to a different user, `claim` returns 409 Conflict
- If the bead is already `in_progress` and assigned to the same user, `claim` succeeds (idempotent)
- If the bead is in a terminal state (`closed`, `deleted`), `claim` returns 409 Conflict

### Ready Filter
- `--ready` shows beads that are `open` AND not blocked by any active (`open`/`in_progress`) bead
- This is the most common query for agents looking for work to pick up

### List Defaults
- `list` with no flags shows beads where status is `open` or `in_progress` (excludes deleted)
- Sorted by priority (critical first), then by creation date (newest first)
- Returns key fields only: `id`, `title`, `status`, `priority`, `type`, `assignee`
- Default pagination: page 1, 100 items per page

### Concurrent Access
- The server holds state in memory and persists to disk on every mutation
- Single-writer model: mutations are serialized via a mutex (sufficient for issue-tracker throughput)
- Reads are served from memory (fast)

### CLI Behavior
- No confirmation prompts. All commands execute immediately
- Exit code 0 for success, 1 for errors
- All output is pretty-printed JSON (indented with 2 spaces) to stdout. Errors are pretty-printed JSON to stderr

## Out of Scope (for now)

- Multi-user permissions / roles
- Bead history / audit log
- File attachments
- Notifications / webhooks
- Import/export from other trackers
- Parent/child hierarchy (epics containing sub-tasks)
- Transitive dependency resolution
- Configurable ID prefix (fixed at `bd-`)
