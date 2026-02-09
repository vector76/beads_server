# Multi-Agent Workflow

Beads Server is designed for environments where multiple coding agents share a single issue tracker, claim work independently, and coordinate through dependencies.

## Agent Startup Loop

Every agent should follow this pattern on startup:

```
1. bs mine                    # check for previously claimed, unfinished work
2. if found → resume work     # pick up where you left off
3. if not   → bs list --ready # find open, unblocked beads
4.           → bs claim <id>  # take ownership
5. do the work
6. bs resolve <id>            # mark complete
7. go to step 1
```

This handles the common case where an agent crashes or is restarted mid-task. The `mine` check ensures work isn't lost or duplicated.

## Claiming Work

`bs claim <id>` atomically sets a bead's status to `in_progress` and its assignee to the current `BS_USER`. This is the mechanism for agents to take ownership of work.

**Conflict rules:**

| Bead state | Claim by current assignee | Claim by someone else |
|------------|---------------------------|-----------------------|
| `open` (no assignee) | N/A | Succeeds |
| `in_progress` | Succeeds (idempotent) | 409 Conflict |
| Terminal (`resolved`/`closed`/`wontfix`/`deleted`) | 409 Conflict | 409 Conflict |

When two agents race to claim the same bead, exactly one will succeed. The other receives a 409 and should pick a different bead.

## Finding Work

Several queries help agents discover what to work on:

```bash
bs list --ready                  # open beads with no active blockers (the main work queue)
bs list --priority critical      # high-urgency items
bs list --type bug               # bugs only
bs list --tag backend            # tagged work
bs search "authentication"       # keyword search
```

`list --ready` is the most common starting point. It returns beads that are `open` and not blocked by any `open` or `in_progress` bead, sorted by priority.

## Using Comments for Progress

Agents can log progress through comments, providing visibility for humans and other agents:

```bash
bs claim bd-a1b2c3d4
bs comment bd-a1b2c3d4 "Starting investigation, checking auth module"
bs comment bd-a1b2c3d4 "Root cause: session cookie not set after redirect"
bs comment bd-a1b2c3d4 "Fix applied in commit abc1234, running tests"
bs resolve bd-a1b2c3d4
```

Comments include the author (`BS_USER`) and timestamp, creating an audit trail of what each agent did and when.

## Dependency-Driven Coordination

Dependencies let you express ordering constraints between beads. When agents pick work from `list --ready`, they naturally follow the dependency order.

### Example: three beads in a chain

```bash
# create the work items (IDs are auto-generated)
bs add "Design API schema" --type task --priority high    # → bd-a1b2c3d4
bs add "Implement API endpoints" --type task              # → bd-e5f6g7h8
bs add "Write integration tests" --type task              # → bd-m9n8o7p6

# express ordering (using the generated IDs, or any unambiguous prefix)
bs link bd-e5f6g7h8 --blocked-by bd-a1b2c3d4   # impl waits for design
bs link bd-m9n8o7p6 --blocked-by bd-e5f6g7h8   # tests wait for impl
```

Now:
- `list --ready` shows only "Design API schema" (the other two are blocked)
- Agent-1 claims and completes the design bead
- The resolve response includes `"unblocked": [...]` showing that "Implement API endpoints" is now ready
- `list --ready` now shows "Implement API endpoints"
- Agent-2 claims and works on it
- When it resolves, "Write integration tests" becomes ready

### Checking dependencies

```bash
bs deps bd-m9n8o7p6
```

Returns:
- `active_blockers` — beads that are still `open` or `in_progress` (work must complete first)
- `resolved_blockers` — beads in the `blocked_by` list that are already `closed`/`resolved`/`wontfix`/`deleted`
- `blocks` — beads that are waiting on this bead

## Multi-Agent Example

Two agents working a shared backlog:

```
Agent-1 (BS_USER=agent-1)          Agent-2 (BS_USER=agent-2)
─────────────────────────          ─────────────────────────
bs mine → empty                    bs mine → empty
bs list --ready → [bd-aaa, bd-bbb] bs list --ready → [bd-aaa, bd-bbb]
bs claim bd-aaa → 200 OK          bs claim bd-aaa → 409 Conflict
                                   bs claim bd-bbb → 200 OK
(works on bd-aaa)                  (works on bd-bbb)
bs resolve bd-aaa                  bs resolve bd-bbb
bs list --ready → [bd-ccc]         bs list --ready → [bd-ccc]
bs claim bd-ccc → 200 OK          bs claim bd-ccc → 409 Conflict
                                   bs list --ready → [bd-ddd]
                                   bs claim bd-ddd → 200 OK
```

Each agent independently loops: check for in-progress work, find ready items, claim one, do the work, resolve, repeat. The server's atomic claim ensures no two agents work on the same bead.

## Environment Setup for Multiple Agents

Each agent needs a unique `BS_USER` but shares the same `BS_TOKEN` and `BS_URL`:

```bash
# Agent 1
export BS_TOKEN=shared-secret
export BS_URL=http://beads-server:9999
export BS_USER=agent-1

# Agent 2
export BS_TOKEN=shared-secret
export BS_URL=http://beads-server:9999
export BS_USER=agent-2
```

The `BS_USER` value is used as the assignee in `claim` and the author in `comment`. It should be unique per agent to avoid confusion in the audit trail and to ensure proper claim conflict detection.
