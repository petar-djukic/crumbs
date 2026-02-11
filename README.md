# Crumbs

A storage system for work items with built-in support for exploratory work sessions. We use the breadcrumb metaphor: the **cupboard** holds all work items (crumbs), and **trails** are exploration paths you can complete or abandon.

## Installation

```bash
go install github.com/mesh-intelligence/crumbs/cmd/cupboard@latest
```

Or build from source:

```bash
git clone https://github.com/mesh-intelligence/crumbs.git
cd crumbs
go build -o bin/cupboard ./cmd/cupboard
```

## Quick Start

```bash
# Initialize a cupboard in the current directory
cupboard init

# Create a crumb (work item)
cupboard set crumbs "" '{"Name":"Implement feature X","State":"draft"}'

# List all crumbs
cupboard list crumbs

# Filter by state
cupboard list crumbs State=ready

# Create a trail for exploration
cupboard set trails "" '{"State":"active"}'

# Get a specific entity
cupboard get crumbs <id>

# Update an entity
cupboard set crumbs <id> '{"CrumbID":"<id>","Name":"Updated name","State":"taken"}'

# Delete an entity
cupboard delete crumbs <id>
```

## Configuration

Create `.crumbs.yaml` in your project root:

```yaml
backend: sqlite
datadir: .crumbs
```

Or place `config.yaml` in `~/.crumbs/` for global configuration.

## CLI Commands

| Command                              | Description                                        |
| ------------------------------------ | -------------------------------------------------- |
| `cupboard init`                      | Initialize the cupboard storage                    |
| `cupboard set <table> <id> <json>`   | Create or update an entity (empty id creates new)  |
| `cupboard get <table> <id>`          | Get an entity by ID                                |
| `cupboard list <table> [filter...]`  | List entities with optional filters                |
| `cupboard delete <table> <id>`       | Delete an entity                                   |
| `cupboard version`                   | Print version                                      |

Tables: `crumbs`, `trails`, `links`, `properties`, `metadata`, `stashes`

## Concepts

| Concept      | Description                                                            |
| ------------ | ---------------------------------------------------------------------- |
| **Crumb**    | A work item with states: draft, pending, ready, taken, pebble, dust    |
| **Pebble**   | A completed crumb (permanent, enduring - like pebbles in the story)    |
| **Dust**     | A failed or abandoned crumb (swept away - like crumbs eaten by birds)  |
| **Trail**    | An exploration session with states: active, completed, abandoned       |
| **Link**     | A relationship between entities (belongs_to, child_of)                 |
| **Stash**    | Shared state scoped to a trail or global                               |
| **Property** | Custom attributes that extend crumbs                                   |
| **Cupboard** | The storage backend that holds everything                              |
| **Cobbler**  | Agent orchestrator (like elves that work while you sleep)              |

## Project Structure

```text
crumbs/
├── cmd/cupboard/        # CLI entry point
├── pkg/types/           # Public API: interfaces and types
├── internal/sqlite/     # SQLite backend implementation
├── docs/                # Documentation (VISION, ARCHITECTURE, PRDs)
├── magefiles/           # Mage build targets (build, test, measure, stitch, generate)
└── .claude/             # Claude Code configuration
```

## Docker

The Dockerfile lives in `magefiles/` and is built automatically by `mage build` when a container runtime (podman or docker) is available.

```bash
mage build    # compiles Go binary + builds container image
mage clean    # removes build artifacts + container image
```

The image includes Go, Claude Code, Mage, Beads (`bd`), and golangci-lint. Mage auto-detects the runtime in this order: podman, docker, direct claude binary.

### Authentication

Place your Claude OAuth token files in `.secrets/` (already gitignored). The default file is `claude.json`. Use the `--token-file` flag to select a different profile.

```bash
# Uses .secrets/claude.json by default
docker run -it \
  -v ./.secrets:/secrets:ro \
  -v $(pwd):/workspace \
  crumbs

# Pick a different token profile
docker run -it \
  -v ./.secrets:/secrets:ro \
  -e CLAUDE_TOKEN_FILE=claude-pro.json \
  -v $(pwd):/workspace \
  crumbs
```

Token files use the `claudeAiOauth` format that Claude Code writes during `claude setup-token`.

### Running a generation

```bash
mage generator:run -- --cycles 3 --token-file claude.json
```

### Smoke test

```bash
mage test:docker
```

## AI-Assisted Development

This project uses [Claude Code](https://claude.ai/claude-code) for AI-assisted development. The `.claude/` directory contains commands and rules that guide the AI agent.

### Commands

Invoke these commands in Claude Code by typing the command name (e.g., `/do-work`).

| Command         | Purpose                                                            |
| --------------- | ------------------------------------------------------------------ |
| `/bootstrap`    | Start a new project: create initial VISION.md and ARCHITECTURE.md  |
| `/make-work`    | Analyze project state and propose new epics and issues             |
| `/do-work`      | Pick up available work and implement it                            |
| `/do-work-docs` | Work on documentation tasks (PRDs, use cases)                      |
| `/do-work-code` | Work on implementation tasks                                       |

### Workflow

1. **Plan work**: Run `/make-work` to see project state and propose next steps
2. **Create issues**: After agreeing on the plan, issues are created via the `bd` CLI
3. **Do work**: Run `/do-work` to pick up and complete available tasks
4. **Track progress**: Issues track LOC metrics and token usage

### /make-work

The `/make-work` command analyzes the project state and proposes new work. It reads the VISION, ARCHITECTURE, ROADMAP, existing PRDs, and use cases to understand what has been built and what remains. It then proposes epics and issues that move the project forward.

**Usage:**

```text
/make-work [optional context or request]
```

**What it does:**

1. Reads project documentation (VISION, ARCHITECTURE, ROADMAP, PRDs, use cases)
2. Checks open and closed issues via `bd` CLI
3. Identifies gaps between the roadmap and current state
4. Proposes epics and child issues with proper structure (per `crumb-format.md`)
5. Creates issues via `bd` after user approval

**Example:**

```text
/make-work I want to implement the trails feature next
```

### /do-work

The `/do-work` command picks up available work from the issue tracker and implements it. It handles both documentation tasks (PRDs, use cases, architecture updates) and code tasks (Go implementation).

**Usage:**

```text
/do-work           # Pick any available issue
/do-work <id>      # Work on a specific issue
/do-work-docs      # Work on documentation issues only
/do-work-code      # Work on code issues only
```

**What it does:**

1. Finds available work via `bd ready`
2. Claims an issue via `bd update <id> --status in_progress`
3. Reads related PRDs and architecture docs before implementing
4. Implements the deliverable (docs or code)
5. Runs quality gates (tests, linters) for code tasks
6. Logs token usage via `bd comments add <id> "tokens: <count>"`
7. Closes the issue via `bd close <id>`
8. Commits changes with stats (LOC, doc words)

**Example session:**

```text
> /do-work

Agent: Found 3 ready issues. Claiming crumbs-42: "Implement CrumbTable.Archive"
       Reading prd003-crumbs-interface.yaml...
       Implementing in internal/sqlite/crumbs.go...
       Tests pass. Committing.

Stats:
  Lines of code (Go, production): 520 (+45)
  Lines of code (Go, tests):      312 (+28)
  Words (documentation):          21032 (+0)
```

### Rules

The `.claude/rules/` directory contains formatting and process rules:

| Rule                               | Governs                                           |
| ---------------------------------- | ------------------------------------------------- |
| `beads-workflow.md`                | Issue tracking, token logging, session completion |
| `documentation-standards.md`       | Writing style, formatting, content quality        |
| `prd-format.md`                    | Product Requirements Document structure           |
| `use-case-format.md`               | Use case document structure                       |
| `crumb-format.md`                  | How to structure crumbs (docs vs code)            |
| `code-prd-architecture-linking.md` | Linking code to PRDs and architecture             |
| `vision-format.md`                 | Vision document structure                         |
| `architecture-format.md`           | Architecture document structure                   |

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

| Document     | Location                                                 |
| ------------ | -------------------------------------------------------- |
| Vision       | [docs/VISION.md](docs/VISION.md)                         |
| Architecture | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)             |
| PRDs         | [docs/specs/product-requirements/](docs/specs/product-requirements/) |
| Use Cases    | [docs/specs/use-cases/](docs/specs/use-cases/)                       |

## License

MIT
