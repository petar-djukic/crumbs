# Use Case: Go Install

## Summary

A developer installs the cupboard CLI via `go install` and verifies it works by initializing a cupboard and performing basic operations. This validates that the module is installable from the Go toolchain and the binary lands in `$GOBIN`.

## Actor and Trigger

The actor is a developer or agent with a Go toolchain (1.25+) installed. The trigger is wanting to use cupboard on a new machine or in a new project.

## Flow

1. Install the cupboard binary via `go install`:

```bash
go install github.com/mesh-intelligence/crumbs/cmd/cupboard@latest
```

The Go toolchain downloads the module, builds `cmd/cupboard`, and places the binary in `$GOBIN` (defaults to `$GOPATH/bin`, typically `~/go/bin`).

2. Verify the binary is accessible:

```bash
cupboard --help
```

The CLI prints usage information showing available commands (init, set, get, list, delete).

3. Initialize a cupboard in a project directory:

```bash
cd ~/my-project
cupboard init
```

This creates the data directory and configuration. The JSONL files (crumbs.jsonl, trails.jsonl, etc.) are created empty, ready for use.

4. Create a crumb to verify the installation works end-to-end:

```bash
cupboard set crumbs "" '{"Name":"First crumb","State":"draft"}'
```

The CLI returns the created crumb as JSON with a generated UUID v7.

5. List crumbs to confirm persistence:

```bash
cupboard list crumbs
```

The output includes the crumb created in step 4.

## Architecture Touchpoints

| Component | Role in this use case |
|-----------|----------------------|
| `cmd/cupboard` | CLI binary built by `go install` |
| `pkg/types` | Config struct, entity types |
| `internal/sqlite` | Backend that creates JSONL files and SQLite cache |
| `internal/paths` | Resolves default config and data directories |

## Success / Demo Criteria

- `go install github.com/mesh-intelligence/crumbs/cmd/cupboard@latest` completes without errors
- `cupboard --help` prints usage from `$GOBIN`
- `cupboard init` creates the data directory with JSONL files
- `cupboard set crumbs` + `cupboard list crumbs` round-trips a crumb
- No manual compilation steps required beyond `go install`

## Out of Scope

- Package managers (brew, apt, snap)
- Docker-based installation (see rel99.0-uc002-docker-bootstrap)
- Version management or update mechanisms
- Cross-compilation or release binaries (goreleaser, GitHub releases)

## Dependencies

The `go install` command requires that the module is published to the Go module proxy. While the `replace` directive in go.mod works for local development (`replace github.com/mesh-intelligence/crumbs => ./`), it must be removed before the module can be installed remotely. This use case assumes the module is available on the proxy (i.e., the repository is public and tagged).

For local-only use before publishing, developers can build directly:

```bash
git clone https://github.com/mesh-intelligence/crumbs.git
cd crumbs
go build -o cupboard ./cmd/cupboard
```
