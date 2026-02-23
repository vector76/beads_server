# Releasing

Releases are triggered by pushing a git tag. GitHub Actions handles the rest: it runs tests, cross-compiles binaries for all platforms, and publishes a GitHub Release with the binaries attached.

## How to cut a release

```bash
git tag v0.9.1
git push origin v0.9.1
```

The tag name determines the version string baked into the binaries (`v0.9.1` → `bs --version` reports `0.9.1`).

## What happens automatically

1. CI runs `go test ./...` — if tests fail, the release is aborted
2. Four binaries are built:
   - `bs-linux-amd64`
   - `bs-windows-amd64.exe`
   - `bs-darwin-arm64` (Apple Silicon)
   - `bs-darwin-amd64` (Intel Mac)
3. A GitHub Release named `v0.9.1` is created with all four binaries attached and auto-generated release notes

## Versioning convention

Use [semantic versioning](https://semver.org): `vMAJOR.MINOR.PATCH`

- `PATCH` — bug fixes, no new features (`v0.9.0` → `v0.9.1`)
- `MINOR` — new features, backwards compatible (`v0.9.1` → `v0.10.0`)
- `MAJOR` — breaking changes (`v0.x.x` → `v1.0.0`)

## Non-release commits

Ordinary commits and PRs to `main` run CI (tests only) but do **not** produce a release. Only tagged commits trigger a release build.

## Skipping CI entirely

Add `[skip ci]` anywhere in the commit message to suppress all workflow runs for that push:

```bash
git commit -m "Fix typo in README [skip ci]"
```

Useful for trivial changes where you've already tested locally. Note: this only suppresses `push`-triggered runs — it does not affect PR workflow runs.
