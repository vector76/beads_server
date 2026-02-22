# Data Model

## Bead

A bead is an issue or task in the tracker.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `id` | string | auto-generated | `bd-` + 4–8 random lowercase alphanumeric chars (e.g., `bd-a1b2`) |
| `title` | string | (required) | One-line summary |
| `description` | string | `""` | Longer body text |
| `status` | Status | `open` | Lifecycle state |
| `priority` | Priority | `medium` | Urgency level |
| `type` | BeadType | `task` | Category |
| `tags` | []string | `[]` | Free-form labels |
| `blocked_by` | []string | `[]` | IDs of beads this one depends on |
| `parent_id` | string | `""` | ID of parent epic; empty if top-level |
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

- Format: `bd-` followed by 4–8 random characters from `[a-z0-9]`
- Default length: 4 random chars (e.g., `bd-a1b2`)
- On collision during generation, length escalates: 4→5→6→7→8 (3 retries per length)
- The `bd-` prefix is fixed and disambiguates bead IDs from other tokens
- IDs must be specified exactly and in full (including the `bd-` prefix)
- Existing 8-char IDs (e.g., `bd-a1b2c3d4`) remain valid

## Enums

### Status

Lifecycle state of a bead.

| Value | Description |
|-------|-------------|
| `open` | Default. Available for work; can be claimed |
| `not_ready` | Parked or not yet actionable. Cannot be claimed. Counts as an active blocker |
| `in_progress` | Claimed by an agent, actively being worked on |
| `closed` | Closed (completed, decided, or won't fix) |
| `deleted` | Soft-deleted. Excluded from default queries, recoverable via `reopen` |

**Terminal states:** `closed`, `deleted`. Beads in terminal states cannot be claimed and do not count as active blockers.

**Active states:** `open`, `not_ready`, `in_progress`. These count as active blockers and appear in the default `bs list`.

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
| `chore` | Maintenance or housekeeping |

Note: there is no `epic` type. A bead becomes an epic structurally — by having children — not by type label. See [epics.md](epics.md).

## Stored vs. Computed Fields

**Stored on disk:**
- All bead fields including `blocked_by` (the full list of dependency IDs, regardless of their status) and `parent_id`

**Recomputed on mutation and persisted:**
- Epic status — when a child's status changes, the parent epic's status is recomputed from the children and written to disk immediately (see [epics.md](epics.md) for derivation rules)

**Computed during mutations (returned in response, not stored):**
- `unblocked` — when a bead reaches a terminal state, the server finds beads that referenced it as a blocker and now have no remaining active blockers, and includes them in the mutation response

**Computed at query time:**
- `blocks` — inverse of `blocked_by`, found by scanning all beads. Only non-deleted beads are included
- `is_epic` — true if any bead has this bead's ID as its `parent_id`
- `parent_title` — title of the parent bead, resolved at query time for child beads
- `blocked` — true if the bead has any active blocker (own or inherited from parent epic); included in list/search summaries. Omitted from the response when false (only present when the bead is blocked)
- Active vs. non-active blockers — the `deps` endpoint splits `blocked_by` into active (status `open`/`not_ready`/`in_progress`) and resolved (all other statuses)
- "Ready" status — a bead is ready when it is `open` and has no active blockers (own or inherited). Used by `list --ready`

## Status Transitions

Any status can transition to any other status via `edit --status` (except on epics, whose status is derived). There are no enforced state machine constraints for leaf beads. However, specific commands imply specific transitions:

| Command | Transition |
|---------|------------|
| `claim` | `open` (or `in_progress` by same user) → `in_progress` + sets assignee |
| `close` | any → `closed` |
| `reopen` | any → `open` (including from `deleted`, which restores the bead) |
| `delete` | any → `deleted` |

`claim` has guards: it rejects if the bead is in a terminal state, has status `not_ready`, is an epic, or is already claimed by a different user.

`delete` has a guard on epics: it rejects if any child has status `open`, `in_progress`, or `not_ready`. See [epics.md](epics.md) for details.

## Hard Delete (Clean)

The `clean` command permanently removes beads from the store (hard delete), unlike `delete` which is a soft delete.

- Only beads with status `closed` or `deleted` are eligible
- Cutoff is based on `updated_at`: beads last updated more than N days ago are removed (default: 5 days)
- `--days 0` removes all closed/deleted beads regardless of age
- Adding a comment to a closed/deleted bead resets the clock (updates `updated_at`)
- Removed beads are gone permanently and cannot be recovered
- Epics are cleaned as a unit; see [epics.md](epics.md) for details
