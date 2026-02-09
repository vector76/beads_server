# Multi-Project Support

The beads server supports hosting multiple isolated projects on a single instance. Each project has its own bearer token, data file, and completely independent set of beads. A request authenticated with one project's token cannot see or modify beads belonging to another project.

## Modes

The server operates in one of two mutually exclusive modes:

**Single-project mode** (default) — One token, one data file. This is the original behavior and requires no configuration changes.

```bash
bs serve --token my-secret --data-file beads.json
```

**Multi-project mode** — Multiple projects defined in a JSON config file. Each project gets its own token and data file.

```bash
bs serve --projects projects.json
```

The `--projects` and `--token` flags are mutually exclusive. Providing both is an error.

## Config File Format

The projects config file is a JSON object with a single `projects` array. Each entry has three required fields:

```json
{
  "projects": [
    {
      "name": "webapp",
      "token": "tok-webapp-secret",
      "data_file": "data/webapp.json"
    },
    {
      "name": "backend",
      "token": "tok-backend-secret",
      "data_file": "data/backend.json"
    }
  ]
}
```

| Field       | Type   | Description                                    |
|-------------|--------|------------------------------------------------|
| `name`      | string | Unique human-readable project identifier       |
| `token`     | string | Bearer token for authenticating to this project |
| `data_file` | string | Path to the project's JSON data file            |

### Validation Rules

The config file is validated on startup. The server will refuse to start if any of these rules are violated:

- Every field (`name`, `token`, `data_file`) must be non-empty
- Project names must be unique
- Tokens must be unique (two projects cannot share a token)

## Configuration Sources

Both the projects file and token support flag and environment variable configuration, with flags taking precedence:

| Setting       | Flag          | Environment Variable | Notes                          |
|---------------|---------------|----------------------|--------------------------------|
| Projects file | `--projects`  | `BS_PROJECTS_FILE`   | Enables multi-project mode     |
| Token         | `--token`     | `BS_TOKEN`           | Enables single-project mode    |
| Port          | `--port`      | `BS_PORT`            | Default: 9999                  |
| Data file     | `--data-file` | `BS_DATA_FILE`       | Single-project mode only       |

## How Token-to-Project Mapping Works

The server uses a `StoreProvider` interface to resolve bearer tokens to stores:

```
Request with "Authorization: Bearer tok-webapp-secret"
  → authMiddleware extracts token
    → StoreProvider.Resolve("tok-webapp-secret")
      → returns the webapp project's Store (or nil if unrecognized)
    → store placed in request context
      → handler calls storeFor(r) to retrieve the store
        → all operations (create, list, update, etc.) use this store
```

In single-project mode, the provider accepts exactly one token and always returns the same store. In multi-project mode, the provider looks up the token in a map and returns the corresponding store. An unrecognized token results in a 401 response.

Each store is an independent in-memory instance backed by its own data file. There is no cross-store interaction — beads, dependencies, and comments are fully isolated per project.

## Backward Compatibility

Existing single-project deployments require no changes. The `--token` / `BS_TOKEN` + `--data-file` / `BS_DATA_FILE` configuration continues to work exactly as before. Multi-project mode is opt-in via `--projects` or `BS_PROJECTS_FILE`.

Clients connect to the same server URL regardless of mode. The only client-side difference is which bearer token is used — the token determines which project the client operates on.

## Example: Running Multi-Project

1. Create data directories and config:

```bash
mkdir -p data
```

```json
{
  "projects": [
    {
      "name": "frontend",
      "token": "tok-fe-a8x2k",
      "data_file": "data/frontend.json"
    },
    {
      "name": "backend",
      "token": "tok-be-m4y7p",
      "data_file": "data/backend.json"
    }
  ]
}
```

2. Start the server:

```bash
bs serve --projects projects.json --port 9999
```

3. Use the CLI with different projects by setting `BS_TOKEN`:

```bash
# Work on frontend project
export BS_URL=http://localhost:9999
export BS_TOKEN=tok-fe-a8x2k
export BS_USER=alice
bs add "Fix login page CSS"
bs list

# Work on backend project
export BS_TOKEN=tok-be-m4y7p
export BS_USER=bob
bs add "Optimize database queries"
bs list   # only shows backend beads
```
