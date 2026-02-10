# Beads Server

An agent-first issue tracker. A single executable (`bs`) that runs as either an HTTP server or a CLI client, designed for multi-agent workflows where coding agents claim, track, and coordinate work items.

## Quick Start

### Build

```bash
go build -o bs ./cmd/bs
```

Or use the Makefile for cross-compiled binaries:

```bash
make build        # produces build/bs-linux-amd64 and build/bs-windows-amd64.exe
```

### Start the server

```bash
export BS_TOKEN=my-secret-token
./bs serve
```

The server listens on port 9999 by default and stores data in `./beads.json`.

### Configure the client

```bash
export BS_TOKEN=my-secret-token
export BS_USER=agent-1
# export BS_URL=http://localhost:9999   # default, change if server is remote
```

### Basic workflow

```bash
bs add "Fix login bug" --type bug --priority high
bs list
bs claim bd-a1b2c3d4
bs comment bd-a1b2c3d4 "Root cause identified: session cookie not set"
bs close bd-a1b2c3d4
```

## Environment Variables

| Variable | Used by | Description |
|----------|---------|-------------|
| `BS_TOKEN` | Client & Server | Bearer token for authentication (required) |
| `BS_URL` | Client | Server URL (default: `http://localhost:9999`) |
| `BS_PORT` | Server | Listen port (default: `9999`) |
| `BS_DATA_FILE` | Server | Path to data file (default: `./beads.json`) |
| `BS_USER` | Client | Agent/user identity for `claim` and `comment` (default: `anonymous`) |

## CLI Commands

### Server

| Command | Description |
|---------|-------------|
| `bs serve` | Start the server (`--port`, `--token`, `--data-file` flags available) |

### Client

| Command | Description |
|---------|-------------|
| `bs whoami` | Print current agent identity (local, no server contact) |
| `bs add "title"` | Create a bead (`--type`, `--priority`, `--description`, `--tags`) |
| `bs show <id>` | Show full bead details |
| `bs edit <id>` | Modify fields (`--title`, `--status`, `--priority`, `--type`, `--add-tag`, `--remove-tag`, ...) |
| `bs close <id>` | Set status to `closed` |
| `bs reopen <id>` | Set status to `open` |
| `bs delete <id>` | Soft-delete (sets status to `deleted`, reversible with `reopen`) |
| `bs list` | List active beads (`--all`, `--ready`, `--status`, `--priority`, `--type`, `--tag`, `--assignee`) |
| `bs search "query"` | Substring search across title and description |
| `bs claim <id>` | Atomically set status to `in_progress` and assignee to `BS_USER` |
| `bs mine` | List beads assigned to current `BS_USER` that are `in_progress` |
| `bs comment <id> "text"` | Add a comment |
| `bs link <id> --blocked-by <other>` | Add a dependency |
| `bs unlink <id> --blocked-by <other>` | Remove a dependency |
| `bs deps <id>` | Show dependencies (active blockers, resolved blockers, blocks) |

All output is pretty-printed JSON. IDs can be shortened to any unambiguous prefix (e.g., `bd-a1b` or just `a1b`).

## Running Tests

```bash
go test ./...       # full suite
make test           # same, via Makefile
```

Test subsets by package:

```bash
go test ./internal/model/...     # data model
go test ./internal/store/...     # storage layer
go test ./internal/server/...    # HTTP handlers
go test ./internal/cli/...       # CLI commands
go test ./e2e/...                # end-to-end
```

## Further Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | Package structure, data flow, concurrency model, test organization, design decisions |
| [Data Model](docs/data-model.md) | Bead/Comment fields, enums, defaults, ID format, status transitions |
| [API Reference](docs/api-reference.md) | REST endpoints with request/response examples |
| [Multi-Agent Workflow](docs/multi-agent-workflow.md) | Claim/resume patterns, conflict handling, dependency-driven coordination |
| [PRD](docs/PRD.md) | Original product requirements and specification |
