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

3. Initialize a cupboard in a project directory (prd-configuration-directories R1, R2):

```bash
cd ~/my-project
cupboard init
```

This creates the data directory (R2.1, R2.5) and configuration directory (R1.6). The JSONL files (crumbs.jsonl, trails.jsonl, etc.) are created empty per prd-sqlite-backend R4.1, R1.4.

4. Create a crumb to verify the installation works end-to-end (prd-cupboard-core R2, R3):

```bash
cupboard set crumbs "" '{"Name":"First crumb","State":"draft"}'
```

The CLI calls `cupboard.GetTable("crumbs").Set()` (R2.3, R3.3). The backend generates a UUID v7 (R8.2) and returns the created crumb as JSON.

5. List crumbs to confirm persistence (prd-cupboard-core R3.5):

```bash
cupboard list crumbs
```

The CLI calls `cupboard.GetTable("crumbs").Fetch()` with an empty filter to return all crumbs. The output includes the crumb created in step 4.

## Architecture Touchpoints

| Component | Role in this use case | PRD reference |
|-----------|----------------------|---------------|
| `cmd/cupboard` | CLI binary built by `go install` | - |
| `pkg/types` | Config struct, entity types | prd-cupboard-core R1 |
| `internal/sqlite` | Backend that creates JSONL files and SQLite cache | prd-sqlite-backend R1, R4 |
| `internal/paths` | Resolves default config and data directories | prd-configuration-directories R1, R2 |

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

### PRD Dependencies

This use case requires the following PRDs to be implemented:

| PRD | Requirements | Purpose |
|-----|--------------|---------|
| prd-cupboard-core | R1, R2, R3, R8 | Config struct, Cupboard and Table interfaces, UUID v7 generation |
| prd-configuration-directories | R1, R2 | CLI config directory, backend data directory |
| prd-sqlite-backend | R1, R4, R5 | Data directory layout, startup sequence, write operations |

### Go Module Dependency

The `go install` command requires that the module is published to the Go module proxy. While the `replace` directive in go.mod works for local development (`replace github.com/mesh-intelligence/crumbs => ./`), it must be removed before the module can be installed remotely. This use case assumes the module is available on the proxy (i.e., the repository is public and tagged).

For local-only use before publishing, developers can build directly:

```bash
git clone https://github.com/mesh-intelligence/crumbs.git
cd crumbs
go build -o cupboard ./cmd/cupboard
```
