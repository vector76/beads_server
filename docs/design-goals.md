# Design Goals

## Overview

An issue tracker optimized for coding agent use. A single executable (`bs`) that runs as either a **server** (holds the issue database, exposes an HTTP/JSON API) or a **CLI client** (thin client that talks to the server). Inspired by Steve Yegge's beads, but with a clean client-server architecture that avoids the git-sync and dual-storage complexity of the original.

## Goals

1. **Agent-first**: All output is JSON (pretty-printed). Agents parse JSON; humans can read it directly or pipe through `jq`
2. **Simple and focused**: Only issue tracking. No git sync, no plugins, no bloat
3. **Client-server**: Server is the single source of truth. Works across worktrees, containers, and remote hosts
4. **Easy to run**: Single binary, minimal configuration. Cross-compiled for Linux, Windows, and macOS
5. **Multi-agent**: Multiple agents can share one tracker, claim work items, and resume aborted tasks

## Out of Scope

- Multi-user permissions / roles
- Bead history / audit log
- File attachments
- Notifications / webhooks
- Import/export from other trackers
- Recursive epic nesting (hierarchy is limited to one level: epic → children)
- Transitive dependency resolution
- Configurable ID prefix (fixed at `bd-`)
