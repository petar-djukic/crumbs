# Cupboard CLI Usage

This guideline describes how to install, configure, and use the cupboard command-line tool. We cover installation, initialization, configuration, and the full command set for managing crumbs and issue tracking. For the formal specification, see prd-cupboard-cli.yaml.

## Installation

### Via go install

When the module is published, install the binary directly from the Go module proxy.

```bash
go install github.com/mesh-intelligence/crumbs/cmd/cupboard@latest
```

The Go toolchain downloads the module, builds `cmd/cupboard`, and places the binary in `$GOBIN` (typically `~/go/bin`). Ensure `$GOBIN` is in your PATH.

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
cupboard --help
```

### Local development

For development, clone the repository and build locally.

```bash
git clone https://github.com/mesh-intelligence/crumbs.git
cd crumbs
go build -o cupboard ./cmd/cupboard
./cupboard --help
```

Move the binary to a location in your PATH for convenience.

```bash
mv cupboard ~/go/bin/
```

## Initialization

The `cupboard init` command initializes storage and creates the data directory structure.

```bash
cupboard init
```

Expected output:
```
Cupboard initialized successfully
```

Initialization creates the following directory structure (paths vary by platform).

Table 1: Data directory files

| File | Purpose |
|------|---------|
| crumbs.jsonl | All crumbs (source of truth) |
| trails.jsonl | All trails |
| links.jsonl | Graph edges (belongs_to, child_of relationships) |
| properties.jsonl | Property definitions |
| categories.jsonl | Category values for categorical properties |
| crumb_properties.jsonl | Property values for crumbs |
| metadata.jsonl | Metadata entries (comments, etc.) |
| stashes.jsonl | Stash definitions and current values |
| stash_history.jsonl | Append-only history of stash changes |
| cupboard.db | SQLite cache (ephemeral, rebuilt on startup) |

Init is idempotent. Running it multiple times does not duplicate data or return an error.

## Configuration

### config.yaml

The CLI reads configuration from `config.yaml` in the configuration directory.

```yaml
# Backend selection
backend: sqlite

# Data directory (where backend stores data)
data_dir: ~/.local/share/crumbs

# Optional backend-specific settings
sqlite:
  # SQLite-specific options (reserved for future use)
```

### Platform defaults

Table 2: Default directory locations

| Platform | Configuration directory | Data directory |
|----------|------------------------|----------------|
| Linux | `$XDG_CONFIG_HOME/crumbs` (or `~/.config/crumbs`) | `$XDG_DATA_HOME/crumbs` (or `~/.local/share/crumbs`) |
| macOS | `~/Library/Application Support/crumbs` | `~/Library/Application Support/crumbs/data` |
| Windows | `%APPDATA%\crumbs` | `%LOCALAPPDATA%\crumbs` |

### Environment variables

Table 3: Environment variable overrides

| Variable | Purpose |
|----------|---------|
| CRUMBS_CONFIG_DIR | Override configuration directory |
| CRUMBS_DATA_DIR | Override data directory |

### CLI flags

Table 4: Global flags

| Flag | Purpose |
|------|---------|
| --config-dir | Override configuration directory |
| --data-dir | Override data directory |
| --help, -h | Print usage information |
| --version | Print version (on root command) |

### Precedence rules

Configuration resolution follows a precedence order from highest to lowest.

For configuration directory: CLI flag > `CRUMBS_CONFIG_DIR` environment variable > platform default.

For data directory: CLI flag > config.yaml `data_dir` > platform default.

Example using flags to override defaults.

```bash
cupboard --config-dir /path/to/config --data-dir /path/to/data list crumbs
```

## Generic Table Operations

Generic table commands expose the Table interface (Get, Set, Delete, Fetch) for any table. Valid table names are `crumbs`, `trails`, `properties`, `metadata`, `links`, and `stashes`.

### cupboard get

Retrieve an entity by ID.

```bash
cupboard get crumbs 01945a3b-1234-7000-8000-000000000001
```

Expected output (JSON, pretty-printed):
```json
{
  "CrumbID": "01945a3b-1234-7000-8000-000000000001",
  "Name": "Implement feature X",
  "State": "ready",
  "CreatedAt": "2025-01-15T10:30:00Z",
  "UpdatedAt": "2025-01-15T10:30:00Z",
  "Properties": {}
}
```

Error cases:
```bash
cupboard get unknown-table abc123
# Error: unknown table "unknown-table" (valid: crumbs, trails, properties, metadata, links, stashes)

cupboard get crumbs nonexistent-id
# Error: entity "nonexistent-id" not found in table "crumbs"
```

### cupboard set

Create or update an entity. Pass an empty string as ID to create a new entity (the backend generates a UUID v7).

```bash
# Create a new crumb
cupboard set crumbs "" '{"Name":"New task","State":"draft"}'

# Update an existing crumb
cupboard set crumbs 01945a3b-1234-7000-8000-000000000001 '{"Name":"Updated task","State":"ready"}'
```

Expected output (JSON of the saved entity):
```json
{
  "CrumbID": "01945a3b-5678-7000-8000-000000000002",
  "Name": "New task",
  "State": "draft",
  "CreatedAt": "2025-01-15T11:00:00Z",
  "UpdatedAt": "2025-01-15T11:00:00Z",
  "Properties": {}
}
```

### cupboard delete

Remove an entity by ID.

```bash
cupboard delete crumbs 01945a3b-1234-7000-8000-000000000001
```

Expected output:
```
Deleted crumbs/01945a3b-1234-7000-8000-000000000001
```

### cupboard list

Query entities with optional filters. Filters are key=value pairs that are ANDed together.

```bash
# List all crumbs
cupboard list crumbs

# Filter by state
cupboard list crumbs State=ready

# Filter by multiple fields
cupboard list crumbs State=ready Name=MyTask
```

Expected output (JSON array):
```json
[
  {
    "CrumbID": "01945a3b-1234-7000-8000-000000000001",
    "Name": "Implement feature X",
    "State": "ready",
    "CreatedAt": "2025-01-15T10:30:00Z",
    "UpdatedAt": "2025-01-15T10:30:00Z",
    "Properties": {}
  }
]
```

Empty results return an empty JSON array `[]`.

## Crumb Commands

Crumb commands provide entity-specific flags and validation. They are grouped under `cupboard crumb`.

### cupboard crumb add

Create a new crumb with friendly flags.

```bash
cupboard crumb add --name "Implement feature X"
```

Expected output:
```
Created crumb: 01945a3b-1234-7000-8000-000000000001
```

With JSON output:
```bash
cupboard crumb add --name "Fix bug Y" --state pending --json
```

Expected output:
```json
{
  "CrumbID": "01945a3b-5678-7000-8000-000000000002",
  "Name": "Fix bug Y",
  "State": "pending",
  "CreatedAt": "2025-01-15T11:00:00Z",
  "UpdatedAt": "2025-01-15T11:00:00Z",
  "Properties": {}
}
```

Table 5: crumb add flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| --name | yes | - | Human-readable name for the crumb |
| --state | no | draft | Initial state |
| --json | no | false | Output as JSON |

### cupboard crumb get

Retrieve a crumb by ID.

```bash
cupboard crumb get 01945a3b-1234-7000-8000-000000000001
```

Expected output (human-readable):
```
ID:        01945a3b-1234-7000-8000-000000000001
Name:      Implement feature X
State:     ready
Created:   2025-01-15T10:30:00Z
Updated:   2025-01-15T10:30:00Z
Properties:
  priority: medium
  type: task
```

With JSON output:
```bash
cupboard crumb get 01945a3b-1234-7000-8000-000000000001 --json
```

### cupboard crumb list

List crumbs with optional filtering.

```bash
cupboard crumb list
```

Expected output (human-readable):
```
ID        NAME                                      STATE     CREATED
--        ----                                      -----     -------
01945a3b  Implement feature X                       ready     2025-01-15
01945a3c  Fix bug Y                                 pending   2025-01-15
Total: 2 crumb(s)
```

Filter by state:
```bash
cupboard crumb list --state ready
```

With JSON output:
```bash
cupboard crumb list --json
```

### cupboard crumb delete

Delete a crumb by ID.

```bash
cupboard crumb delete 01945a3b-1234-7000-8000-000000000001
```

Expected output:
```
Deleted crumb: 01945a3b-1234-7000-8000-000000000001
```

## Issue-Tracking Commands

Issue-tracking commands support the workflow previously provided by the beads (bd) CLI. They are designed for task tracking in development workflows.

### cupboard ready

List crumbs that are ready for work.

```bash
cupboard ready
```

Expected output (human-readable table of ready crumbs):
```
ID        NAME                                      STATE     CREATED
--        ----                                      -----     -------
01945a3b  Implement feature X                       ready     2025-01-15
Total: 1 crumb(s)
```

Filter by type and limit results:
```bash
cupboard ready -n 1 --type task --json
```

Expected output:
```json
[
  {
    "CrumbID": "01945a3b-1234-7000-8000-000000000001",
    "Name": "Implement feature X",
    "State": "ready",
    "Properties": {
      "type": "task"
    }
  }
]
```

### cupboard create

Create a new crumb with issue-tracking fields.

```bash
cupboard create --type task --title "Implement feature" --description "Add the widget"
```

Expected output:
```
Created task: 01945a3b-1234-7000-8000-000000000001
```

With JSON output:
```bash
cupboard create --type epic --title "Storage layer" --description "Core storage" --labels "code,infra" --json
```

Expected output:
```json
{
  "CrumbID": "01945a3b-5678-7000-8000-000000000002",
  "Name": "Storage layer",
  "State": "draft",
  "Properties": {
    "type": "epic",
    "description": "Core storage",
    "labels": ["code", "infra"]
  }
}
```

Table 6: create flags

| Flag | Required | Description |
|------|----------|-------------|
| --type | yes | Crumb type property (task, epic, bug, etc.) |
| --title | yes | Crumb name |
| --description | no | Description property |
| --labels | no | Comma-separated list of labels |
| --json | no | Output as JSON |

### cupboard show

Display a crumb with full details.

```bash
cupboard show 01945a3b
```

Expected output:
```
ID:          01945a3b-1234-7000-8000-000000000001
Title:       Implement feature
State:       ready
Type:        task
Description: Add the widget
Created:     2025-01-15T10:30:00Z
```

The ID argument accepts full UUIDs or short prefixes.

### cupboard update

Modify crumb fields.

```bash
cupboard update 01945a3b --status in_progress
```

Expected output:
```
Updated 01945a3b-1234-7000-8000-000000000001
```

Table 7: update flags

| Flag | Description |
|------|-------------|
| --status | Set crumb state (draft, pending, ready, taken, pebble, dust) |
| --title | Set crumb name |
| --json | Output as JSON |

### cupboard close

Transition a crumb to completed state.

```bash
cupboard close 01945a3b
```

Expected output:
```
Closed 01945a3b-1234-7000-8000-000000000001
```

The command sets the crumb state to `pebble` (completed). The crumb must be in `taken` state for the transition to succeed.

### cupboard comments add

Add a comment to a crumb.

```bash
cupboard comments add 01945a3b "tokens: 34256"
```

Expected output:
```
Added comment to 01945a3b-1234-7000-8000-000000000001
```

Comments are stored as metadata entries linked to the crumb.

## JSON Output for Scripting

All commands that produce output support the `--json` flag for machine-readable output. JSON output is pretty-printed with 2-space indentation.

### Piping to jq

Extract specific fields using jq.

```bash
# Get the ID of the first ready task
cupboard ready --type task --json | jq -r '.[0].CrumbID'

# Get all task names
cupboard crumb list --json | jq -r '.[].Name'

# Create and capture the ID
CRUMB_ID=$(cupboard create --type task --title "New task" --json | jq -r '.CrumbID')
echo "Created: $CRUMB_ID"
```

### Checking for empty results

```bash
# Count ready tasks
COUNT=$(cupboard ready --type task --json | jq 'length')
if [ "$COUNT" -eq 0 ]; then
  echo "No tasks ready"
fi
```

### Exit codes

Table 8: Exit codes

| Code | Meaning | Examples |
|------|---------|----------|
| 0 | Success | Command completed, including empty results |
| 1 | User error | Invalid arguments, entity not found, validation failure |
| 2 | System error | Backend connection failure, file I/O error |

## Git Integration

JSONL files are the source of truth and should be committed to git. The SQLite database is ephemeral and must be gitignored.

### What to commit

Table 9: Git status of data files

| File | Git status | Reason |
|------|------------|--------|
| `*.jsonl` | Committed | Source of truth, human-readable, mergeable |
| `cupboard.db` | Gitignored | Ephemeral cache, rebuilt on every startup |
| `config.yaml` | Committed | Per-repo configuration |

Add the following to `.gitignore`.

```
cupboard.db
```

### JSONL as source of truth

The SQLite backend loads JSONL files into SQLite on startup. Writes persist to JSONL first, then update SQLite. This design enables line-based merging, grep/tail compatibility, and human readability.

### Commit conventions

Reference crumb IDs in commit messages for traceability.

```bash
git add data/*.jsonl
git commit -m "Complete feature implementation (01945a3b)"
```

Find commits that reference a crumb:
```bash
git log --all --grep="01945a3b"
```

### Merge behavior

JSONL files use in-place update with stable insertion order. New records append at the end. This makes most merges automatic.

Table 10: Merge scenarios

| Scenario | Git behavior |
|----------|-------------|
| Trail adds new crumbs, main unchanged | Auto-merges (new lines appended) |
| Trail modifies crumb that main did not touch | Auto-merges (changed line in place) |
| Two trails modify the same crumb | Merge conflict (resolve manually) |

## Common Patterns

### Create-then-query

Create a crumb and immediately query it to verify.

```bash
ID=$(cupboard create --type task --title "New task" --json | jq -r '.CrumbID')
cupboard show $ID
```

### State transitions

Typical issue-tracking workflow.

```bash
# Pick a ready task
TASK=$(cupboard ready -n 1 --type task --json | jq -r '.[0].CrumbID')

# Claim it
cupboard update $TASK --status in_progress

# Do work...

# Close it
cupboard close $TASK
```

### Filtering

Filter by multiple criteria.

```bash
# Ready tasks of type "bug"
cupboard ready --type bug

# All pending crumbs (generic table command)
cupboard list crumbs State=pending

# Crumbs created by a specific owner (if property is set)
cupboard list crumbs Owner=alice
```

## Beads to Cupboard Migration

For teams migrating from the beads (bd) CLI, the following table maps commands.

Table 11: bd to cupboard command mapping

| bd command | cupboard equivalent |
|------------|-------------------|
| `bd ready -n 1 --json --type task` | `cupboard ready -n 1 --json --type task` |
| `bd update <id> --status in_progress` | `cupboard update <id> --status in_progress` |
| `bd close <id>` | `cupboard close <id>` |
| `bd list --json` | `cupboard list crumbs --json` or `cupboard crumb list --json` |
| `bd create --type <type> --title <title> --description <desc>` | `cupboard create --type <type> --title <title> --description <desc>` |
| `bd show <id>` | `cupboard show <id>` |
| `bd comments add <id> "text"` | `cupboard comments add <id> "text"` |
| `bd sync` | Not needed (cupboard syncs on every write) |

Data migration involves moving from `.beads/issues.jsonl` to the cupboard data directory. The `git add` in session completion commits JSONL files from the data directory instead of `.beads/` files.

## References

- prd-cupboard-cli.yaml (formal CLI specification)
- prd-configuration-directories.yaml (directory structure and config loading)
- prd-cupboard-core.yaml (Cupboard and Table interfaces)
- eng01-git-integration.md (JSONL in git, trails as worktrees)
- eng02-beads-migration.md (beads to cupboard migration details)
