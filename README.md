# Crumbs

A storage system for work items with built-in support for exploratory work sessions. We use the breadcrumb metaphor: the **cupboard** holds all work items (crumbs), and **trails** are exploration paths you can complete or abandon.

## About This Repository

This repository does not contain released application code. The deliverables are **requirements** (PRDs, use cases, architecture docs) and **build tooling** (mage targets). Application code is generated automatically by running `mage generator:start` followed by `mage generator:run`, which invokes Claude to produce Go source code from the specifications.

Each completed generation is tagged and merged into main. To see past generations and their tags, run `mage generator:list --all`. To check out the code from a specific generation, use `git checkout <tag>` with one of the generation tags (e.g. `generation-2026-02-10-15-04-30-merged`).

## Prerequisites

We use [Mage](https://magefile.org) for build automation and [Beads](https://github.com/petardjukic/beads) (`bd` CLI) for local issue tracking. Install both before using the build system.

```bash
go install github.com/magefile/mage@latest
```

## Mage Targets

Targets are organized into namespaces. Use `mage -h <target>` to see help for a specific target.

```bash
# Top-level
mage init                # Initialize project state (beads)
mage reset               # Full reset: cobbler, generator, beads
mage build               # Compile Go binary + build container image
mage clean               # Remove build artifacts + container image
mage install             # Build and copy binary to GOPATH/bin
mage stats               # Print Go LOC and documentation word counts

# Generator (code generation lifecycle)
mage generator:start     # Tag main, create generation branch, delete Go sources
mage generator:run       # Run measure/stitch cycles on the generation branch
mage generator:resume    # Recover from interrupted run, cleanup, continue
mage generator:stop      # Merge generation branch into main
mage generator:list      # Show active and past generations
mage generator:switch    # Switch between generation branches
mage generator:reset     # Remove generation branches, worktrees, Go sources

# Cobbler (agent orchestration)
mage cobbler:measure     # Propose new tasks via Claude
mage cobbler:stitch      # Execute ready tasks via Claude in worktrees
mage cobbler:reset       # Remove the .cobbler/ scratch directory

# Beads (issue tracking)
mage beads:init          # Initialize the beads database
mage beads:reset         # Destroy and reinitialize the beads database

# Testing
mage test:unit           # Run unit tests
mage test:integration    # Build, then run integration tests
mage test:all            # Run all tests
mage test:cobbler        # Cobbler regression suite (measure 3, stitch, verify)
mage test:docker         # Smoke test: build image, run Claude with "Hello World"
mage lint                # Run golangci-lint
```

## Generating Code

A generation is a cycle where Claude reads the project specifications and produces Go implementation code. The workflow is:

1. **Start**: `mage generator:start` tags the current main state, creates a generation branch, and resets Go sources to a minimal scaffold.
2. **Run**: `mage generator:run --cycles N` runs N rounds of measure (propose issues) and stitch (implement issues). Each cycle invokes Claude to analyze project state and produce code.
3. **Stop**: `mage generator:stop` tags the finished generation, merges it into main, and cleans up the generation branch.

Flags for `generator:run`:

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--cycles` | 1 | Number of measure+stitch cycles |
| `--max-issues` | 10 | Issues proposed per measure cycle |
| `--silence-agent` | true | Suppress Claude output |
| `--no-container` | false | Skip container runtime, use local claude binary |
| `--token-file` | claude.json | Token file name in .secrets/ |

### Inspecting Past Generations

```bash
# List all generations (active, merged, and incomplete)
mage generator:list --all

# Check out a specific generation's code
git checkout generation-2026-02-10-15-04-30-merged
```

Generation tags follow the naming convention `generation-YYYY-MM-DD-HH-MM-SS` with lifecycle suffixes:

| Suffix | Meaning |
| ------ | ------- |
| `-start` | State of main before the generation began |
| `-finished` | Final state of the generation branch |
| `-merged` | State of main after the generation was merged |
| `-incomplete` | Generation that was abandoned before merging |

## Docker

The Dockerfile lives in `magefiles/` and is built automatically by `mage build` when a container runtime (podman or docker) is available.

The image includes Go, Claude Code, Mage, Beads (`bd`), and golangci-lint. Mage auto-detects the runtime in this order: podman, docker, direct claude binary. Use `--no-container` on cobbler targets to bypass container detection and run the local `claude` binary directly.

### Authentication

Place your Claude OAuth token files in `.secrets/` (already gitignored). The default file is `claude.json`. Use the `--token-file` flag to select a different profile.

Token files use the `claudeAiOauth` format that Claude Code writes during `claude setup-token`.

## Concepts

| Concept | Description |
| ------- | ----------- |
| **Crumb** | A work item with states: draft, pending, ready, taken, pebble, dust |
| **Pebble** | A completed crumb (permanent, enduring) |
| **Dust** | A failed or abandoned crumb (swept away) |
| **Trail** | An exploration session with states: active, completed, abandoned |
| **Link** | A relationship between entities (belongs_to, child_of) |
| **Stash** | Shared state scoped to a trail or global |
| **Property** | Custom attributes that extend crumbs |
| **Cupboard** | The storage backend that holds everything |
| **Cobbler** | Agent orchestrator (like elves that work while you sleep) |

## Project Structure

```text
crumbs/
├── cmd/cupboard/        # CLI entry point (generated)
├── pkg/types/           # Public API: interfaces and types (generated)
├── internal/sqlite/     # SQLite backend implementation (generated)
├── docs/                # Documentation (VISION, ARCHITECTURE, PRDs)
├── magefiles/           # Mage build targets
├── .beads/              # Beads issue tracker (managed by bd CLI)
├── .cobbler/            # Cobbler scratch directory (gitignored)
├── .secrets/            # Claude credentials (gitignored)
└── .claude/             # Claude Code configuration
```

Directories marked "(generated)" contain code produced by `mage generator:run`. They start empty on main and are populated during a generation cycle.

## AI-Assisted Development

This project uses [Claude Code](https://claude.ai/claude-code) for AI-assisted development. The `.claude/` directory contains commands and rules that guide the AI agent.

### Commands

Invoke these commands in Claude Code by typing the command name (e.g., `/do-work`).

| Command | Purpose |
| ------- | ------- |
| `/bootstrap` | Start a new project: create initial VISION.md and ARCHITECTURE.md |
| `/make-work` | Analyze project state and propose new epics and issues |
| `/do-work` | Pick up available work and implement it |
| `/do-work-docs` | Work on documentation tasks (PRDs, use cases) |
| `/do-work-code` | Work on implementation tasks |

### Workflow

1. **Plan work**: Run `/make-work` to see project state and propose next steps
2. **Create issues**: After agreeing on the plan, issues are created via the `bd` CLI
3. **Do work**: Run `/do-work` to pick up and complete available tasks
4. **Track progress**: Issues track LOC metrics and token usage

### Issue Tracking

This project uses [Beads](https://github.com/petardjukic/beads) (`bd` CLI) for local issue tracking:

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Close completed work
bd sync               # Sync with git
```

## Documentation

| Document | Location |
| -------- | -------- |
| Vision | [docs/VISION.md](docs/VISION.md) |
| Architecture | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| PRDs | [docs/specs/product-requirements/](docs/specs/product-requirements/) |
| Use Cases | [docs/specs/use-cases/](docs/specs/use-cases/) |

## License

MIT
