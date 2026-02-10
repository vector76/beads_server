# Data Model

## Bead

A bead is an issue or task in the tracker.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `id` | string | auto-generated | `bd-` + 8 random lowercase alphanumeric chars (e.g., `bd-a1b2c3d4`) |
| `title` | string | (required) | One-line summary |
| `description` | string | `""` | Longer body text |
| `status` | Status | `open` | Lifecycle state |
| `priority` | Priority | `medium` | Urgency level |
| `type` | BeadType | `task` | Category |
| `tags` | []string | `[]` | Free-form labels |
| `blocked_by` | []string | `[]` | IDs of beads this one depends on |
| `assignee` | string | `""` | Who is working on this |
| `comments` | []Comment | `[]` | Discussion thread |
| `created_at` | ISO 8601 | auto-set | Creation timestamp (UTC) |
| `updated_at` | ISO 8601 | auto-set | Last modification timestamp (UTC) |

## Comment

| Field | Type | Description |
|-------|------|-------------|
| `author` | string | From `BS_USER` env var or `"anonymous"` |
| `text` | string | Comment body |
| `created_at` | ISO 8601 | Auto-set on creation (UTC) |

## ID Format

- Format: `bd-` followed by 8 random characters from `[a-z0-9]`
- Example: `bd-a1b2c3d4`
- The `bd-` prefix is fixed and disambiguates bead IDs from other tokens
- IDs can be referenced by unique prefix: `bd-a1b` resolves if unambiguous
- The `bd-` prefix can be omitted: `a1b2c3d4` is equivalent to `bd-a1b2c3d4`
- Ambiguous prefixes return an error listing all matching IDs

## Enums

### Status

Lifecycle state of a bead.

| Value | Description |
|-------|-------------|
| `open` | Default. Available for work |
| `in_progress` | Claimed by an agent, actively being worked on |
| `closed` | Closed (completed, decided, or won't fix) |
| `deleted` | Soft-deleted. Excluded from default queries, recoverable via `reopen` |

**Terminal states:** `closed`, `deleted`. Beads in terminal states cannot be claimed. Terminal states do not count as active blockers.

### Priority

Urgency level, with sort rank (lower rank = sorted first).

| Value | Rank | Description |
|-------|------|-------------|
| `critical` | 0 | Highest urgency |
| `high` | 1 | |
| `medium` | 2 | Default |
| `low` | 3 | |
| `none` | 4 | No priority assigned |

List and search results are sorted by priority rank (critical first), then by creation date (newest first).

### BeadType

Category of work.

| Value | Description |
|-------|-------------|
| `bug` | Defect to fix |
| `feature` | New functionality |
| `task` | Default. General work item |
| `epic` | Large body of work |
| `chore` | Maintenance or housekeeping |

## Stored vs. Computed Fields

**Stored on disk:**
- All bead fields including `blocked_by` (the full list of dependency IDs, regardless of their status)

**Computed at query time:**
- `blocks` — inverse of `blocked_by`, found by scanning all beads. Only non-deleted beads are included
- Active vs. non-active blockers — the `deps` endpoint splits `blocked_by` into active (status `open`/`in_progress`) and resolved (all other statuses)
- `unblocked` — when a bead reaches a terminal state, the server finds beads that referenced it as a blocker and now have no remaining active blockers, and includes them in the response
- "Ready" status — a bead is ready when it is `open` and has no active blockers. Used by `list --ready`

## Status Transitions

Any status can transition to any other status via `edit --status`. There are no enforced state machine constraints. However, specific commands imply specific transitions:

| Command | Transition |
|---------|------------|
| `claim` | `open` (or `in_progress` by same user) -> `in_progress` + sets assignee |
| `close` | any -> `closed` |
| `reopen` | any -> `open` (including from `deleted`, which restores the bead) |
| `delete` | any -> `deleted` |

`claim` is the only command with guards: it rejects if the bead is in a terminal state or already claimed by a different user.
