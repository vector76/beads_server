# Beads Server

An agent-first issue tracker. A single executable (`bs`) that runs as either an HTTP server or a CLI client, designed for multi-agent workflows where coding agents claim, track, and coordinate work items.

## Quick Start

### Download a release

Pre-built binaries are available on the [Releases page](https://github.com/vector76/beads_server/releases/latest).

**Linux (amd64)**
```bash
curl -L https://github.com/vector76/beads_server/releases/latest/download/bs-linux-amd64 -o bs
chmod +x bs
./bs --version
```

**Windows (amd64)**

Download `bs-windows-amd64.exe` from the Releases page and run it directly:
```
bs-windows-amd64.exe --version
```

**macOS (Apple Silicon / arm64)**
```bash
curl -L https://github.com/vector76/beads_server/releases/latest/download/bs-darwin-arm64 -o bs
chmod +x bs
xattr -d com.apple.quarantine bs   # clear Gatekeeper quarantine flag
./bs --version
```

**macOS (Intel / amd64)**
```bash
curl -L https://github.com/vector76/beads_server/releases/latest/download/bs-darwin-amd64 -o bs
chmod +x bs
xattr -d com.apple.quarantine bs   # clear Gatekeeper quarantine flag
./bs --version
```

> The `xattr` step is required on macOS because the binary is not code-signed. Gatekeeper will block it otherwise.

### Build from source

```bash
go build -o bs ./cmd/bs
```

or on Windows
```
go build -o bs.exe ./cmd/bs
```

Or use the Makefile for cross-compiled binaries:

```bash
make build        # produces build/bs-linux-amd64, build/bs-windows-amd64.exe, build/bs-darwin-arm64, build/bs-darwin-amd64
```

### Start the server

```bash
export BS_TOKEN=my-secret-token
./bs serve
```

or
```
bs serve --token my-secret-token
```

The server listens on port 9999 by default and stores data in `./beads.json`.

### Configure the client

```bash
export BS_TOKEN=my-secret-token
# export BS_USER=agent-1  # optional, only useful if multiple clients are connecting
# export BS_URL=http://localhost:9999   # default, change if server is remote
```

With a client in a docker container and a server on a Windows host, 
```
export BS_TOKEN=my-secret-token
export BS_USER=agent-1
export BS_URL=http://host.docker.internal:9999
```

Note, the `BS_USER` is optional and only necessary if multiple clients are 
claiming work, in which case it can be helpful to distinguish their claimed items.
There is no authentication at the user level, it's only an annotation.

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
| `BS_PROJECTS_FILE` | Server | Path to multi-project config file (mutually exclusive with `BS_TOKEN`) |

The client also reads `BS_TOKEN`, `BS_USER`, and `BS_URL` from a `.env` file in the current directory when the corresponding env var is not set. Env vars take precedence over the file.

## CLI Commands

### Server

| Command | Description |
|---------|-------------|
| `bs serve` | Start the server (`--port`, `--token`, `--data-file`, `--projects` flags available) |

### Client

| Command | Description |
|---------|-------------|
| `bs whoami` | Print current agent identity (local, no server contact) |
| `bs add "title"` | Create a bead (`--type`, `--priority`, `--description`, `--tags`, `--parent <id>`, `--status open\|not_ready`) |
| `bs show <id>` | Show full bead details |
| `bs edit <id>` | Modify fields (`--title`, `--status`, `--priority`, `--type`, `--add-tag`, `--remove-tag`, ...) |
| `bs close <id>` | Set status to `closed` |
| `bs reopen <id>` | Set status to `open` |
| `bs delete <id>` | Soft-delete (sets status to `deleted`, reversible with `reopen`) |
| `bs list` | List active beads — default statuses: `open`, `in_progress`, `not_ready` (`--all`, `--ready`, `--status`, `--priority`, `--type`, `--tag`, `--assignee`) |
| `bs search "query"` | Substring search across title and description |
| `bs claim <id>` | Atomically set status to `in_progress` and assignee to `BS_USER` |
| `bs mine` | List beads assigned to current `BS_USER` that are `in_progress` |
| `bs comment <id> "text"` | Add a comment |
| `bs link <id> --blocked-by <other>` | Add a dependency |
| `bs unlink <id> --blocked-by <other>` | Remove a dependency |
| `bs deps <id>` | Show dependencies (active blockers, resolved blockers, blocks) |
| `bs clean` | Purge old closed/deleted beads (`--days N`, default 5; `--days 0` removes all; `--hours N` alternative) |
| `bs move <id> --into <epic-id>` | Move a bead into an epic (set parent) |
| `bs move <id> --out` | Detach a bead from its parent epic |

All output is pretty-printed JSON. IDs are short by default (`bd-` + 4 chars) and must be specified exactly and in full.

Hidden aliases (not shown in `bs --help`): `bs create` = `bs add`, `bs resolve` = `bs close`.

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
| [Design Goals](docs/design-goals.md) | Project goals, design philosophy, and out-of-scope items |
| [Architecture](docs/architecture.md) | Package structure, data flow, concurrency model, test organization, design decisions |
| [Data Model](docs/data-model.md) | Bead/Comment fields, enums, defaults, ID format, status transitions |
| [API Reference](docs/api-reference.md) | REST endpoints with request/response examples |
| [Multi-Agent Workflow](docs/multi-agent-workflow.md) | Claim/resume patterns, conflict handling, dependency-driven coordination |
| [Multi-Project](docs/multi-project.md) | Hosting multiple isolated projects on a single server instance |
| [Epics](docs/epics.md) | Parent/child epic hierarchy — data model, commands, and behavior |
| [Releasing](docs/releasing.md) | How to cut a release (git tag workflow, versioning conventions) |
