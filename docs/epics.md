# Epics: Parent/Child Hierarchy

## Overview

Epics introduce a single level of parent/child containment to the bead model. A bead becomes an epic when it acquires children. This allows large efforts to be decomposed into discrete, independently trackable sub-tasks while preserving the relationship between them.

Epic-ness is a **structural role**, not a type. A bead is an epic because it has children, not because someone labeled it. The `type` field (bug, feature, task, chore) remains orthogonal and still describes *what kind of work* the effort represents. A feature request decomposed into subtasks is still a feature — it just happens to also be an epic.

> **Note:** The `BeadType` enum does not include an `epic` value. Epic-ness is determined solely by the presence of children, not by type label.

## Data Model

The `Bead` struct has one additional optional field:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `parent_id` | string | `""` | ID of the parent bead, empty if top-level |

**Constraints:**

- **Single-parent:** A bead can have at most one parent.
- **Single-level:** A bead that has a parent cannot itself have children. Equivalently, a bead that has children cannot be moved into another bead. This limits the hierarchy to exactly one level: parent (epic) and children.
- **Detection:** A bead is an epic if and only if at least one other bead references it as `parent_id`. This is computed at query time (like `blocks`), not stored as a flag.

### Stored vs. Computed

**Stored:** `parent_id` on each child bead.

**Recomputed on mutation and stored:**
- The epic's lifecycle status — updated whenever a child's status changes, a child is added, or a child is detached. The recomputed value is persisted to disk immediately (consistent with the existing store architecture where every mutation writes atomically). This means the stored `status` field on an epic is always consistent with its children; "derived" means "automatically maintained on every mutation," not "computed on every read."

**Computed at query time:**
- Whether a bead is an epic (has any children) — determined by scanning for beads with matching `parent_id`
- The list of children for a given epic
- The `progress` summary, `is_epic` flag, and `parent_title` convenience field

## Derived Lifecycle State

An epic's lifecycle status is **derived from its children's statuses** and cannot be manually set. The derivation rules:

| Children states | Epic status |
|---|---|
| All terminal (`closed` or `deleted`) | `closed` |
| Any child `in_progress` | `in_progress` |
| Any child `open`, none `in_progress` | `open` |
| All non-terminal children are `not_ready` | `not_ready` |

Spelled out, in priority order:
- If every child is terminal (`closed` or `deleted`), the epic is `closed`.
- Otherwise, if any child is `in_progress`, the epic is `in_progress`.
- Otherwise, if any child is `open`, the epic is `open`.
- Otherwise (all non-terminal children are `not_ready`), the epic is `not_ready`.

The `deleted` status is treated as terminal for derivation purposes.

### What Epics Do Not Have

- **No assignee.** Epics are containers, not work items. Individual children are claimed and assigned. The `claim` command is rejected on epics. The `assignee` field technically exists on the bead struct but has no meaningful role for epics.
- **No manual status control.** Commands like `close`, `reopen`, and `edit --status` are rejected on epics. The status is always a projection of children.

### Auto-Transitions

Any mutation that changes a child's status triggers a recomputation of the parent epic's derived status. Important cases:

- **Closing the last active child** auto-closes the epic. The server recomputes and persists the epic's new `closed` status; the CLI response reflects the updated child, not the epic.
- **Reopening a child** in a fully-closed epic transitions the epic to `open` (because at least one child is now `open` and none are `in_progress`).
- **Adding a new child** to a fully-closed epic transitions the epic to `open` (the new child is `open`, which satisfies the "any open" rule).
- **Deleting a child** triggers recomputation. If after the soft-delete all remaining children are in a terminal state, the epic closes.

## Blocking Semantics

The existing `blocked_by` / `blocks` system extends naturally to epics:

### Task Blocked by Epic

If a standalone task lists an epic in its `blocked_by`, the task cannot proceed until the epic is fully closed — meaning **all of the epic's children must be in a terminal state** (closed or deleted). This is consistent with the general rule: a blocker is resolved when it reaches a terminal state, and an epic reaches terminal state (closed) only when all children are closed or deleted.

### Epic Blocked by Task

If an epic lists a task in its `blocked_by`, **none of the epic's children can proceed** until that task is complete. Children inherit the epic's blocking constraints. When evaluating whether a child is "ready," the system checks both the child's own `blocked_by` list and the parent epic's `blocked_by` list.

### Child Blocking

Children can block each other (within the same epic or across epics), and they can block standalone beads. These relationships work exactly like existing bead-to-bead blocking — no special handling needed.

### Parent-Child Blocking Forbidden

Blocking relationships between an epic and its own children are rejected in **either direction**:

- **Epic blocked by own child:** If epic A has `blocked_by: [childB]` where childB is a child of A, the epic's blockers cascade to all children, so childB would be blocked by itself — a deadlock.
- **Child blocked by own parent epic:** If childB has `blocked_by: [epicA]` where epicA is its parent, then B cannot proceed until A closes, but A cannot close until B (and all siblings) close — also a deadlock.

The existing circular dependency check follows explicit `blocked_by` chains and would not catch these cases because the parent-child closure dependency is implicit (not stored in `blocked_by`). This invariant must be enforced at two points:

1. **At `link` time:** Reject any `blocked_by` link where one bead is the parent of the other.
2. **At `move` time:** Reject `move --into` if any `blocked_by` relationship already exists between the bead and the target parent (in either direction). Without this check, a valid standalone blocking relationship could become a deadlock when the bead is moved into the epic.

## CLI Command Behavior

### Command Applicability

| Command | On Epic | On Leaf Bead |
|---|---|---|
| `bs add --parent <epic-id>` | Creates a child under it | Converts it to an epic (if it has no parent) |
| `bs show` | Shows progress summary + children | Normal detail view |
| `bs claim` | **Rejected** | Normal |
| `bs close` | **Rejected** (derived) | Normal (triggers parent recomputation) |
| `bs reopen` | **Rejected** (derived) | Normal (triggers parent recomputation) |
| `bs edit --status` | **Rejected** (derived) | Normal (triggers parent recomputation) |
| `bs edit` (other fields) | Allowed (title, description, priority, type, assignee, tags) | Normal |
| `bs comment` | Allowed | Allowed |
| `bs link` / `bs unlink` | Allowed | Allowed |
| `bs delete` | **Only if all children are closed or deleted** | Normal (triggers parent recomputation) |
| `bs move --into` | Target only (receives children) | Source only (becomes a child) |
| `bs move --out` | N/A | Detaches from parent |

Rejected operations return an error with a clear message explaining why (e.g., `"cannot close an epic directly; close its children instead"`).

### Creating Children

Two mechanisms for establishing the parent/child relationship:

**Create a new child directly:**
```
bs add --parent <epic-id> "Subtask title"
```
Creates a new bead with `parent_id` set to the given epic. The system validates:
- The target exists and is not deleted
- The target is not itself a child (no nesting)

**Move an existing bead into an epic:**
```
bs move <bead-id> --into <epic-id>
```
Sets `parent_id` on an existing bead. The system validates:
- The bead has no children (cannot nest an epic inside another epic)
- The target is not itself a child (no nesting)
- The bead is not already a child of the target (reject as no-op with error)
- The target exists and is not deleted
- No blocking relationship exists between the bead and the target in either direction (would create a parent-child deadlock; see "Parent-Child Blocking Forbidden")

### Detaching Children

```
bs move <bead-id> --out
```
Clears the `parent_id` field. The bead becomes a standalone top-level bead. If this was the last child, the former parent reverts to a regular bead — its status is no longer automatically maintained and is set to `open` (regardless of what the derived value was). This avoids invalid states like `in_progress` with no assignee that could result from preserving the last derived value.

### Reparenting

```
bs move <bead-id> --into <new-epic-id>
```

If a bead is already a child of one epic and is moved into a different epic, this is a reparenting operation. The same `move --into` command and validation rules apply. Both the old parent (losing a child) and the new parent (gaining a child) have their derived status recomputed. If the old parent loses its last child, it reverts to a regular bead with status set to `open`.

### Deleting

**Deleting a leaf bead that has a parent:** The child is soft-deleted normally. The parent epic's derived status recomputes. No confirmation prompt (consistent with existing CLI behavior — the application is never interactive).

**Deleting an epic:** Only permitted if all children are in a terminal state (`closed` or `deleted`). If any child is `open`, `in_progress`, or `not_ready`, the delete is rejected with an error: `"cannot delete epic with open children; close or delete children first"`.

### Showing an Epic

`bs show <epic-id>` displays the epic's fields plus a progress summary and a listing of all children with their statuses:

```json
{
  "id": "bd-a1b2",
  "title": "Auth rewrite",
  "type": "feature",
  "status": "in_progress",
  "priority": "high",
  "is_epic": true,
  "description": "Rewrite the authentication system to use JWT",
  "progress": {
    "total": 5,
    "open": 1,
    "in_progress": 1,
    "closed": 3,
    "deleted": 0,
    "not_ready": 0
  },
  "children": [
    {"id": "bd-c3d4", "title": "Design token schema", "status": "closed", "priority": "medium", "type": "task", "assignee": ""},
    {"id": "bd-e5f6", "title": "Implement JWT middleware", "status": "closed", "priority": "high", "type": "task", "assignee": ""},
    {"id": "bd-g7h8", "title": "Write migration script", "status": "closed", "priority": "medium", "type": "task", "assignee": ""},
    {"id": "bd-i9j0", "title": "Update login endpoint", "status": "in_progress", "priority": "high", "type": "task", "assignee": "alice"},
    {"id": "bd-k1l2", "title": "Fix token refresh race", "status": "open", "priority": "medium", "type": "bug", "assignee": ""}
  ],
  "tags": [],
  "blocked_by": [],
  "assignee": "",
  "parent_id": "",
  "comments": [],
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-20T14:30:00Z"
}
```

The `progress` and `children` fields are included only when the bead has children (is an epic). The `progress` object always satisfies `total = open + in_progress + not_ready + closed + deleted`.

Children in the show response include only `id`, `title`, `status`, `priority`, `type`, and `assignee`. This is a narrower set than the list response (which uses `BeadSummary` and also includes `updated_at` and `blocked`).

### Deleted Children Visibility

Deleted children are handled differently depending on the view:

- **`bs show`**: Deleted children **are visible** in the `children` array and counted in the `progress` summary (which includes a `deleted` field). This is the full detail view — you want the complete picture of the epic's decomposition, including work that was canceled.
- **`bs list`**: Deleted children **are excluded** from the `children` array, consistent with how `bs list` normally excludes deleted beads. A deleted child represents work that was canceled — in a summary view, that's noise rather than actionable information.

In both cases, deleted children still affect the epic's **derived status** (treated as terminal for derivation purposes). The visibility rules only affect query output, not status computation.

### Showing a Child

`bs show <child-id>` displays the normal bead detail view plus `parent_id` and `parent_title` fields for context:

```json
{
  "id": "bd-k1l2",
  "title": "Fix token refresh race",
  "type": "bug",
  "status": "open",
  "priority": "medium",
  "description": "Race condition when multiple tokens refresh simultaneously",
  "parent_id": "bd-a1b2",
  "parent_title": "Auth rewrite",
  "tags": [],
  "blocked_by": [],
  "assignee": "",
  "comments": [],
  "created_at": "2025-01-16T09:00:00Z",
  "updated_at": "2025-01-16T09:00:00Z"
}
```

`parent_id` is always present in the response (as `""` for top-level beads). `parent_title` is included only when the bead has a parent — it is a convenience field to avoid a second lookup; it is not stored, just resolved at query time.

### Listing

**`bs list`** shows a hierarchical view. Epics and their children are grouped together. Standalone beads (no parent) appear at the top level. Children are nested under their parent epic. The response includes structural indicators so consumers can distinguish epics, children, and standalone beads:

```json
{
  "beads": [
    {
      "id": "bd-a1b2", "title": "Auth rewrite", "status": "in_progress",
      "priority": "high", "type": "feature", "assignee": "",
      "updated_at": "2025-01-20T14:30:00Z",
      "is_epic": true,
      "children": [
        {"id": "bd-c3d4", "title": "Design token schema", "status": "closed", "priority": "medium", "type": "task", "assignee": "", "updated_at": "..."},
        {"id": "bd-i9j0", "title": "Update login endpoint", "status": "in_progress", "priority": "high", "type": "task", "assignee": "alice", "updated_at": "..."},
        {"id": "bd-k1l2", "title": "Fix token refresh race", "status": "open", "priority": "medium", "type": "bug", "assignee": "", "updated_at": "..."}
      ]
    },
    {"id": "bd-m3n4", "title": "Fix CSS on mobile", "status": "open", "priority": "low", "type": "bug", "assignee": "", "updated_at": "2025-01-15T08:00:00Z"}
  ],
  "page": 1,
  "per_page": 100,
  "total": 2,
  "total_pages": 1
}
```

Children do not appear as separate top-level entries — they are nested under their parent. This prevents the flat list from being cluttered with both the epic and its subtasks as independent items. Note that **closed children** of an epic are included in its `children` array even though `bs list` normally excludes closed beads — this is necessary to show the epic's progress. However, **deleted children are excluded** from the `children` array in list output, consistent with how `bs list` excludes deleted beads generally (see "Deleted Children Visibility" above for the full detail).

Pagination counts **top-level items only** (epics and standalone beads). Children nested under an epic do not count toward `total` or affect page boundaries. In the example above, `total` is 2 (one epic, one standalone bead), not 5.

**`bs list --ready`** shows only leaf beads that are `open` and not blocked (either directly or via an inherited epic-level blocker). Epics never appear in ready listings — they are containers, not claimable work items. Ready children appear as **flat top-level items** (not nested under their epic) with `parent_id` and `parent_title` fields for context. This differs from the hierarchical `bs list` format because `--ready` answers "what can I claim right now" — structure is secondary to actionability.

**`bs mine`** shows only leaf beads assigned to the current user that are `in_progress`. Epics never appear — they are containers and are excluded from assignee-filtered views. Like `--ready`, children appear as **flat top-level items** with `parent_id` and `parent_title` fields for context.

### Filtering

Filters (`--type`, `--priority`, `--tag`, `--status`) apply to **top-level items** in the hierarchical listing:

- An epic matches a filter if the **epic itself** matches. If an epic matches, all of its children are included in the nested listing regardless of whether individual children match.
- An epic does **not** match just because one of its children matches. Children are not independently filterable through `bs list` — they exist only as nested items under their parent.
- Standalone beads (no parent) are filtered normally.

The `--assignee` filter is different: it switches to flat mode (like `--ready`), returning only leaf beads that match the assignee. Epics are excluded entirely, and matching children appear as flat top-level items with `parent_id` and `parent_title` for context.

For example, `bs list --type bug` shows standalone bugs and epics whose type is "bug" (with all their children). It does not show a feature epic just because it contains a child of type "bug."

If a use case requires finding specific children across epics, `bs search` is the appropriate tool (see below).

### Search

`bs search` searches across **all non-deleted beads** — epics, children, and standalone beads — matching against title and description. Deleted beads are excluded from search results regardless of whether they are children (even though deleted children are visible in epic `children` arrays, search follows the existing rule of excluding deleted beads). Results are returned as a flat list. Each result includes `parent_id` and `parent_title` when the matching bead is a child, providing context about which epic it belongs to. Epics that match appear with their `is_epic` flag but without their full children listing (use `bs show` for that).

### Cleaning

`bs clean` treats closed epics as a unit:

- A **fully closed epic** (all children closed or deleted) is eligible for cleaning. When cleaned, the epic and all its children are hard-deleted together. The age threshold is evaluated against the **most recent `updated_at`** across the epic and all of its children. If any child was updated recently (e.g., closed yesterday), the entire unit is retained even if the epic itself was last updated long ago.
- A **non-closed epic** (some children still `open`, `in_progress`, or `not_ready`) is never cleaned, even if some of its children are individually closed and old enough. Partially-complete epics retain all their children. This prevents confusing state where an epic references children that no longer exist.
- **Standalone closed beads** (no parent) are cleaned as before — no behavior change.

## REST API

No dedicated endpoints are needed. The existing endpoints handle all epic operations:

- `POST /api/v1/beads` — accepts optional `parent_id` field to create a child directly. Validates nesting constraints (target exists, is not deleted, is not itself a child).
- `GET /api/v1/beads` — response includes hierarchical grouping (children nested under epics) and `is_epic` indicators.
- `GET /api/v1/beads/:id` — response includes `progress` (with `deleted` count), `children` (including deleted children), and `is_epic` fields when the bead is an epic. Includes `parent_title` when the bead is a child. (`parent_id` is always present in the response.)
- `PATCH /api/v1/beads/:id` — rejects status changes on epics (returns 409). Accepts `parent_id` to move a bead into an epic, or `parent_id: ""` to detach from a parent. Validates nesting constraints on `parent_id` changes. The `bs move` CLI command maps to this endpoint.
- `POST /api/v1/beads/:id/claim` — rejects on epics. Returns 409 with error message.
- `DELETE /api/v1/beads/:id` — on epics, rejects if any child is `open`, `in_progress`, or `not_ready`. Returns 409 with error message.
- `POST /api/v1/clean` — applies unit-based cleaning for epics as described above.

## CLI Commands

| Command | Description |
|---|---|
| `bs add --parent <epic-id> "title"` | Create a child bead under an epic |
| `bs move <id> --into <epic-id>` | Move an existing bead into an epic |
| `bs move <id> --out` | Detach a bead from its parent epic |

`bs add --parent` maps to `POST /api/v1/beads` with `parent_id` in the body. `bs move` maps to `PATCH /api/v1/beads/:id` with `parent_id` set to the target epic ID (for `--into`) or empty string (for `--out`). No dedicated API endpoints are needed for these operations.

## Validation Rules Summary

Operations that the system **rejects** with an error:

| Operation | Condition | Error |
|---|---|---|
| `claim` on epic | Bead has children | Cannot claim an epic; claim individual children |
| `close` on epic | Bead has children | Cannot close an epic directly; close its children |
| `reopen` on epic | Bead has children | Cannot reopen an epic directly; reopen individual children |
| `edit --status` on epic | Bead has children | Cannot set status on an epic; status is derived from children |
| `delete` epic with open children | Any child is `open`, `in_progress`, or `not_ready` | Cannot delete epic with open children; close or delete children first |
| `add --parent` targeting a child | Target has a `parent_id` | Cannot nest epics; target is already a child of another bead |
| `move --into` targeting a child | Target has a `parent_id` | Cannot nest epics; target is already a child of another bead |
| `move --into` when source has children | Source bead has children | Cannot nest epics; bead already has children |
| `move --into` when already a child of target | `parent_id` == target | Bead is already a child of this epic |
| `move --into` with existing block | Source and target have a `blocked_by` relationship | Cannot move into an epic that blocks or is blocked by this bead; this creates a deadlock |
| `link` between epic and own child | Target is parent or child of source | Cannot add dependency between an epic and its own children; this creates a deadlock |

No operation prompts for confirmation. Consistent with existing CLI behavior: all commands execute immediately or fail with an error. The application is never interactive.

## Design Rationale

### Why hierarchy instead of just tags or labels?

Tags and type labels annotate work — they add metadata. Hierarchy **structures** work. The value is that the system can answer "what's left to finish X?" by inspecting the epic's children, rather than relying on humans to maintain that mapping via consistent tagging.

### Why derived status instead of manual?

If epic status could be set manually, it would inevitably drift from reality. An epic marked "in progress" with all children closed is a lie. Deriving status from children means the epic always reflects the actual state of the work. There is no synchronization problem to manage.

### Why single-level only?

Recursive hierarchy (epic > epic > task) dramatically increases complexity in queries, derived status computation, blocking semantics, display, and the mental model. For the rare case where sub-grouping is needed, two separate epics with blocking relationships between them achieve the same result without recursive data structures.

### Why single-parent only?

If a task could belong to multiple epics, derived progress becomes ambiguous (does closing it count toward both?), and "show me everything in this epic" becomes a graph traversal instead of a simple filter. If a task is relevant to two efforts, it belongs to one and the other blocks on it directly.

### Why no assignee on epics?

An epic represents an effort, not a work item. Claiming an epic would imply one person is doing all the work, which contradicts the purpose of decomposition. The people doing the work claim individual children. Accountability for the overall effort is established through convention (e.g., the person who created the epic), not through a system-enforced assignee field.
