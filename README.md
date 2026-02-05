# Crumbs

A storage system for work items with built-in support for exploratory work sessions. We use the breadcrumb metaphor: the **cupboard** holds all work items (crumbs), and **trails** are exploration paths you can complete or abandon.

## Installation

```bash
go install github.com/petardjukic/crumbs/cmd/crumbs@latest
```

Or build from source:

```bash
git clone https://github.com/petardjukic/crumbs.git
cd crumbs
go build -o bin/crumbs ./cmd/crumbs
```

## Quick Start

```bash
# Initialize a cupboard in the current directory
crumbs init

# Create a crumb (work item)
crumbs set crumbs "" '{"Name":"Implement feature X","State":"draft"}'

# List all crumbs
crumbs list crumbs

# Filter by state
crumbs list crumbs State=ready

# Create a trail for exploration
crumbs set trails "" '{"State":"active"}'

# Get a specific entity
crumbs get crumbs <id>

# Update an entity
crumbs set crumbs <id> '{"CrumbID":"<id>","Name":"Updated name","State":"taken"}'

# Delete an entity
crumbs delete crumbs <id>
```

## Configuration

Create `.crumbs.yaml` in your project root:

```yaml
backend: sqlite
datadir: .crumbs
```

Or place `config.yaml` in `~/.crumbs/` for global configuration.

## CLI Commands

| Command                            | Description                                        |
| ---------------------------------- | -------------------------------------------------- |
| `crumbs init`                      | Initialize the cupboard storage                    |
| `crumbs set <table> <id> <json>`   | Create or update an entity (empty id creates new)  |
| `crumbs get <table> <id>`          | Get an entity by ID                                |
| `crumbs list <table> [filter...]`  | List entities with optional filters                |
| `crumbs delete <table> <id>`       | Delete an entity                                   |
| `crumbs version`                   | Print version                                      |

Tables: `crumbs`, `trails`, `links`, `properties`, `metadata`, `stashes`

## Concepts

| Concept      | Description                                                                        |
| ------------ | ---------------------------------------------------------------------------------- |
| **Crumb**    | A work item with states: draft, pending, ready, taken, completed, failed, archived |
| **Trail**    | An exploration session with states: active, completed, abandoned                   |
| **Link**     | A relationship between entities (belongs_to, child_of)                             |
| **Stash**    | Shared state scoped to a trail or global                                           |
| **Property** | Custom attributes that extend crumbs                                               |
| **Cupboard** | The storage backend that holds everything                                          |

## Project Structure

```text
crumbs/
├── cmd/crumbs/          # CLI entry point
├── pkg/types/           # Public API: interfaces and types
├── internal/sqlite/     # SQLite backend implementation
├── docs/                # Documentation (VISION, ARCHITECTURE, PRDs)
├── scripts/             # Utility scripts
└── .claude/             # Claude Code configuration
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

### Rules

The `.claude/rules/` directory contains formatting and process rules:

| Rule                               | Governs                                           |
| ---------------------------------- | ------------------------------------------------- |
| `beads-workflow.md`                | Issue tracking, token logging, session completion |
| `documentation-standards.md`       | Writing style, formatting, content quality        |
| `prd-format.md`                    | Product Requirements Document structure           |
| `use-case-format.md`               | Use case document structure                       |
| `issue-format.md`                  | How to structure issues (docs vs code)            |
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
| PRDs         | [docs/product-requirements/](docs/product-requirements/) |
| Use Cases    | [docs/use-cases/](docs/use-cases/)                       |

## Validation

Run the self-hosting validation to verify the system works:

```bash
./scripts/validate-self-hosting.sh
```

## License

MIT
