# API: Bulk Bead Status Endpoint

## Overview

`GET /api/v1/beads/status` returns a batch status lookup for one or more bead IDs. It requires no authentication and searches across all projects.

---

## Endpoint

```
GET /api/v1/beads/status
```

**Authentication:** None required (public endpoint).

---

## Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `ids` | string | Comma-separated list of bead IDs to look up |

---

## Response

**`200 OK`** — always returned (even when `ids` is absent or empty).

Response body is a JSON object mapping each requested ID to its status string:

```json
{
  "bd-a1b2": "open",
  "bd-c3d4": "in_progress",
  "bd-e5f6": "closed",
  "bd-xxxx": "unknown"
}
```

### Status Values

| Value | Meaning |
|-------|---------|
| `open` | Bead exists and is open |
| `not_ready` | Bead exists and is blocked |
| `in_progress` | Bead exists and is being worked on |
| `closed` | Bead exists and is closed |
| `deleted` | Bead exists and has been soft-deleted |
| `unknown` | ID was not found in any project |

---

## Behavior

- **Absent or empty `ids`:** returns `{}` with no error.
- **Duplicates:** deduplicated — each ID appears once in the response.
- **Cross-project:** searches across all project stores; IDs are unique across projects (see [Cross-Project ID Uniqueness](#cross-project-id-uniqueness)).
- **No rate limiting** and **no cap on the number of IDs** per request.

### Example

Request:
```
GET /api/v1/beads/status?ids=bd-a1b2,bd-c3d4,bd-missing
```

Response:
```json
{
  "bd-a1b2": "open",
  "bd-c3d4": "closed",
  "bd-missing": "unknown"
}
```

---

## Cross-Project ID Uniqueness

Bead IDs are unique across all projects. This uniqueness is enforced at creation time: when a new bead is created, the server scans all project stores to exclude any already-existing IDs from the candidate set before assigning an ID.

No migration is applied to beads created before this enforcement was introduced; those beads retain their existing IDs.

---

## Router Ordering

The static route `/api/v1/beads/status` is registered in the unauthenticated router block, before the authenticated group that contains the dynamic route `/api/v1/beads/{id}`. This ensures the literal segment `status` is matched first and is never mistaken for a bead ID.
