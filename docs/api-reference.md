# API Reference

Base URL: `http://localhost:9999/api/v1` (configurable via `BS_PORT`)

All endpoints except health require the header:

```
Authorization: Bearer <token>
```

All request and response bodies are `application/json`.

---

## Health Check

```
GET /api/v1/health
```

No authentication required.

**Response** `200`:

```json
{"status": "ok"}
```

---

## Create Bead

```
POST /api/v1/beads
```

**Request body:**

```json
{
  "title": "Fix login bug",
  "description": "Users can't log in after password reset",
  "type": "bug",
  "priority": "high",
  "tags": ["auth", "urgent"],
  "assignee": "agent-1",
  "blocked_by": ["bd-x1y2z3w4"]
}
```

Only `title` is required. All other fields use defaults if omitted (see [Data Model](data-model.md)).

**Response** `201`:

```json
{
  "id": "bd-a1b2c3d4",
  "title": "Fix login bug",
  "description": "Users can't log in after password reset",
  "status": "open",
  "priority": "high",
  "type": "bug",
  "tags": ["auth", "urgent"],
  "blocked_by": ["bd-x1y2z3w4"],
  "assignee": "agent-1",
  "comments": [],
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

**Errors:** `400` if title is missing or JSON is invalid.

---

## Get Bead

```
GET /api/v1/beads/:id
```

Requires the exact full ID (e.g., `bd-a1b2`). The `bd-` prefix is required.

**Response** `200`: Full bead object (same format as create response).

**Errors:** `404` if not found.

---

## Update Bead

```
PATCH /api/v1/beads/:id
```

**Request body** (all fields optional, only provided fields are changed):

```json
{
  "title": "Updated title",
  "description": "Updated description",
  "status": "in_progress",
  "priority": "critical",
  "type": "feature",
  "assignee": "agent-2",
  "tags": ["new-tag-list"],
  "add_tags": ["extra"],
  "remove_tags": ["old"]
}
```

`tags` replaces the entire tag list. `add_tags`/`remove_tags` modify incrementally (duplicates are ignored when adding). If both `tags` and `add_tags`/`remove_tags` are provided, `add_tags`/`remove_tags` takes precedence (it operates on the existing tags and overwrites the `tags` field).

**Response** `200`: Updated bead object.

When a status change to a terminal state (`closed`, `deleted`) unblocks other beads, the response includes an `unblocked` field:

```json
{
  "id": "bd-a1b2c3d4",
  "title": "Fix login bug",
  "status": "closed",
  "unblocked": [
    {
      "id": "bd-e5f6g7h8",
      "title": "Deploy auth service",
      "status": "open",
      ...
    }
  ],
  ...
}
```

**Errors:** `404` if not found. `400` for invalid fields.

---

## Delete Bead

```
DELETE /api/v1/beads/:id
```

Soft-deletes the bead (sets status to `deleted`). The bead remains in storage and can be restored with `PATCH` setting status back to `open`.

**Response** `200`: Deleted bead object (with status `deleted`). Includes `unblocked` field if this bead was blocking others.

**Errors:** `404` if not found.

---

## List Beads

```
GET /api/v1/beads
```

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `status` | string | `open,in_progress` | Comma-separated status filter |
| `priority` | string | | Filter by priority |
| `type` | string | | Filter by type |
| `tag` | string | | Comma-separated tags (OR semantics: matches any) |
| `assignee` | string | | Filter by assignee |
| `all` | `true` | | Show all statuses (overrides `status`) |
| `ready` | `true` | | Show only `open` beads with no active blockers |
| `page` | int | `1` | Page number (1-indexed) |
| `per_page` | int | `100` | Items per page |

**Response** `200`:

```json
{
  "beads": [
    {
      "id": "bd-a1b2c3d4",
      "title": "Fix login bug",
      "status": "open",
      "priority": "high",
      "type": "bug",
      "assignee": ""
    }
  ],
  "page": 1,
  "per_page": 100,
  "total": 1,
  "total_pages": 1
}
```

Results are sorted by priority (critical first), then by creation date (newest first). Returns summary fields only — use `GET /api/v1/beads/:id` for full details.

---

## Search

```
GET /api/v1/search?q=<query>
```

Case-insensitive substring search across title and description. Deleted beads are excluded.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | (required) | Search query |
| `page` | int | `1` | Page number |
| `per_page` | int | `100` | Items per page |

**Response** `200`: Same paginated format as list, with summary fields.

**Errors:** `400` if `q` parameter is missing.

---

## Claim Bead

```
POST /api/v1/beads/:id/claim
```

Atomically sets status to `in_progress` and assignee to the specified user.

**Request body:**

```json
{
  "user": "agent-1"
}
```

**Response** `200`: Updated bead object.

**Errors:**
- `400` if `user` is missing
- `404` if bead not found
- `409` if bead is already claimed by a different user or in a terminal state

---

## Add Comment

```
POST /api/v1/beads/:id/comments
```

**Request body:**

```json
{
  "author": "agent-1",
  "text": "Found the root cause"
}
```

**Response** `201`: Full bead object with the new comment appended.

**Errors:** `400` if `author` or `text` is missing. `404` if bead not found.

---

## Add Dependency

```
POST /api/v1/beads/:id/link
```

Adds `blocked_by` to the bead's dependency list.

**Request body:**

```json
{
  "blocked_by": "bd-x1y2z3w4"
}
```

**Response** `200`: Updated bead object.

**Errors:**
- `400` for self-links, duplicates, circular dependencies, or linking to deleted beads
- `404` if either bead not found

---

## Remove Dependency

```
DELETE /api/v1/beads/:id/link/:other_id
```

Removes `other_id` from the bead's `blocked_by` list.

**Response** `200`: Updated bead object.

**Errors:** `400` if the dependency doesn't exist. `404` if either bead not found.

---

## Get Dependencies

```
GET /api/v1/beads/:id/deps
```

**Response** `200`:

```json
{
  "active_blockers": [
    {
      "id": "bd-x1y2z3w4",
      "title": "Setup database",
      "status": "in_progress",
      ...
    }
  ],
  "resolved_blockers": [
    {
      "id": "bd-m9n8o7p6",
      "title": "Design schema",
      "status": "closed",
      ...
    }
  ],
  "blocks": [
    {
      "id": "bd-e5f6g7h8",
      "title": "Deploy service",
      "status": "open",
      ...
    }
  ]
}
```

- `active_blockers` — beads in the `blocked_by` list with status `open` or `in_progress`
- `resolved_blockers` — beads in the `blocked_by` list with any other status
- `blocks` — other beads that list this bead in their `blocked_by` (computed inverse, non-deleted only)

**Errors:** `404` if bead not found.

---

## Clean (Purge Old Beads)

```
POST /api/v1/clean
```

Permanently removes beads with status `closed` or `deleted` whose `updated_at` is older than the specified threshold.

**Request body:**

```json
{
  "days": 5
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `days` | number | `5` | Remove beads last updated more than N days ago. Accepts decimals (e.g., `0.5` for 12 hours). `0` removes all closed/deleted beads regardless of age |

**Response** `200`:

```json
{
  "removed": 3
}
```

**Errors:** `400` if `days` is negative. `401` if not authenticated.

---

## Error Format

All errors return:

```json
{"error": "description of what went wrong"}
```

| Status Code | Meaning |
|-------------|---------|
| `400` | Bad request (missing fields, invalid values) |
| `401` | Missing or invalid bearer token |
| `404` | Bead not found |
| `409` | Conflict (claim rejected) |
| `500` | Internal server error |
