# PRD: Configuration Directory Structure

## Problem

The current architecture conflates CLI configuration with backend data storage. prd-cupboard-core defines a single DataDir field that serves as both the location for backend data files and implicitly for any CLI settings. This creates several problems.

First, CLI configuration (backend selection, default settings) and backend data (crumbs, trails, etc.) have different lifecycles. Configuration is typically set once and rarely changes; data changes constantly. Mixing them in one directory complicates backup strategies and makes it unclear what to version-control versus what to treat as runtime state.

Second, prd-sqlite-backend specifies JSON arrays for data files (e.g., `[{...}, {...}]` in crumbs.json). JSON arrays require reading and parsing the entire file to add a record. For large datasets or append-heavy workloads, this is inefficient. Line-delimited JSON (JSONL) allows appending without rewriting and enables streaming reads.

Third, the current design lacks clear guidance on where configuration files live on different operating systems. We need a standard location for CLI configuration that follows platform conventions (XDG on Linux, Application Support on macOS, AppData on Windows).

## Goals

1. Define a CLI configuration directory separate from backend data
2. Define the backend data directory structure for SQLite backend
3. Specify JSONL (line-delimited JSON) as the file format for data tables
4. Establish platform-appropriate default locations for both directories
5. Clarify the relationship between configuration and the Cupboard interface

## Requirements

### R1: CLI Configuration Directory

1.1. The CLI configuration directory holds application-level settings, not backend data.

1.2. Default locations by platform:

| Platform | Default path |
|----------|--------------|
| Linux | `$XDG_CONFIG_HOME/crumbs` (falls back to `~/.config/crumbs`) |
| macOS | `~/Library/Application Support/crumbs` |
| Windows | `%APPDATA%\crumbs` |

1.3. The configuration directory can be overridden via:

| Method | Precedence |
|--------|------------|
| `--config-dir` CLI flag | Highest |
| `CRUMBS_CONFIG_DIR` environment variable | Middle |
| Platform default | Lowest |

1.4. The CLI configuration directory must contain:

| File | Purpose |
|------|---------|
| config.yaml | CLI settings: default backend, data directory path, verbosity, etc. |

1.5. config.yaml format:

```yaml
# Backend selection
backend: sqlite

# Data directory (where backend stores data)
data_dir: ~/.local/share/crumbs

# Optional backend-specific settings
sqlite:
  # SQLite-specific options (reserved for future use)

dolt:
  dsn: "dolt://localhost:3306/crumbs"
  branch: main

dynamodb:
  table_name: crumbs
  region: us-east-1
  endpoint: ""  # Optional endpoint override for local testing
```

1.6. If the configuration directory does not exist, the CLI must create it on first run with a default config.yaml.

### R2: Backend Data Directory

2.1. The backend data directory holds all data managed by a Cupboard backend. It is separate from CLI configuration.

2.2. Default locations by platform:

| Platform | Default path |
|----------|--------------|
| Linux | `$XDG_DATA_HOME/crumbs` (falls back to `~/.local/share/crumbs`) |
| macOS | `~/Library/Application Support/crumbs/data` |
| Windows | `%LOCALAPPDATA%\crumbs` |

2.3. The data directory can be overridden via:

| Method | Precedence |
|--------|------------|
| `--data-dir` CLI flag | Highest |
| `data_dir` in config.yaml | Middle |
| Platform default | Lowest |

2.4. The data directory path is passed to Cupboard via Config.DataDir when calling Attach.

2.5. If the data directory does not exist, the backend must create it during Attach.

### R3: JSONL File Format

3.1. The SQLite backend must use JSONL (JSON Lines) format for data files instead of JSON arrays.

3.2. JSONL format rules:

- Each line is a complete, valid JSON object
- Lines are separated by newline (`\n`)
- No commas between lines
- No enclosing array brackets
- Empty lines are ignored
- UTF-8 encoding required

3.3. Example crumbs.jsonl:

```jsonl
{"crumb_id":"01945a3b-...","name":"Implement feature X","state":"pending","created_at":"2025-01-15T10:30:00Z","updated_at":"2025-01-15T10:30:00Z"}
{"crumb_id":"01945a3c-...","name":"Fix bug Y","state":"ready","created_at":"2025-01-15T11:00:00Z","updated_at":"2025-01-15T11:00:00Z"}
```

3.4. Benefits of JSONL over JSON arrays:

| Aspect | JSON array | JSONL |
|--------|------------|-------|
| Appending | Requires rewriting entire file | Append new line |
| Streaming reads | Must parse entire array first | Read line by line |
| Partial failures | Corrupt file unreadable | Only corrupt line lost |
| Tooling | Requires JSON parser | Works with grep, head, tail |
| Merge conflicts | Array brackets conflict | Line-based merging |

3.5. JSONL files must not be pretty-printed. Each record is a single line (no internal newlines in the JSON).

### R4: Data Directory File Layout

4.1. The SQLite backend data directory must contain:

| File | Purpose |
|------|---------|
| crumbs.jsonl | All crumbs (source of truth) |
| trails.jsonl | All trails (source of truth) |
| links.jsonl | Graph edges: belongs_to, child_of relationships |
| properties.jsonl | Property definitions (source of truth) |
| categories.jsonl | Category definitions for categorical properties |
| crumb_properties.jsonl | Property values for crumbs |
| metadata.jsonl | All metadata entries |
| stashes.jsonl | Stash definitions and current values |
| stash_history.jsonl | Append-only history of stash changes |
| cupboard.db | SQLite database (ephemeral cache, regenerated from JSONL) |

4.2. File naming convention: `{table_name}.jsonl` (lowercase, underscores for multi-word names).

4.3. If a JSONL file does not exist, the backend must create an empty file (zero bytes, not an empty array).

### R5: Startup Sequence Updates

5.1. On Attach with SQLite backend:

1. Create data directory if it does not exist
2. Create empty JSONL files if they do not exist
3. Delete cupboard.db if it exists (ephemeral cache)
4. Create new cupboard.db with schema
5. Load each JSONL file into corresponding SQLite table (line by line)
6. Skip empty lines and log warnings for malformed lines
7. Validate foreign key relationships
8. Return ready Cupboard instance

5.2. If a line in a JSONL file is malformed, the backend must log a warning with the file name, line number, and error, then skip that line. The startup continues with remaining valid records.

5.3. If foreign key validation fails, the backend must return an error listing the invalid references.

### R6: Write Operation Updates

6.1. All write operations must persist to JSONL immediately after SQLite commit.

6.2. For updates and deletes, the backend must rewrite the entire JSONL file (read all, modify, write atomically). This is acceptable because JSONL files are small enough to fit in memory for typical workloads.

6.3. For append-only tables (stash_history), the backend may append a new line instead of rewriting.

6.4. Atomic write pattern:

1. Write to temporary file ({filename}.tmp)
2. Sync to disk (fsync)
3. Rename temporary file to target (atomic on POSIX)

### R7: CLI Configuration Loading

7.1. On CLI startup:

1. Determine configuration directory (flag > env > platform default)
2. Load config.yaml if it exists; use defaults otherwise
3. Determine data directory (flag > config > platform default)
4. Pass data directory to Cupboard via Config.DataDir

7.2. The CLI must not require config.yaml to exist. Missing config.yaml uses all defaults.

7.3. The CLI must create config.yaml on first write (e.g., `crumbs config set backend sqlite`).

### R8: Config Struct Updates

8.1. The Config struct in prd-cupboard-core must be updated:

```go
type Config struct {
    Backend        string          // Backend type: "sqlite", "dolt", "dynamodb"
    DataDir        string          // Data directory for backend; ignored for cloud backends
    DoltConfig     *DoltConfig     // Dolt-specific settings; nil if not using Dolt
    DynamoDBConfig *DynamoDBConfig // DynamoDB-specific settings; nil if not using DynamoDB
}
```

8.2. DataDir holds the directory for local backends (sqlite, dolt); ignored for cloud backends.

8.3. CLI configuration (config.yaml) is outside the Cupboard interface. The CLI reads config.yaml and constructs a Config struct to pass to Attach.

## Non-Goals

1. This PRD does not define migration tooling from JSON arrays to JSONL. Manual migration or a separate utility may be needed.

2. This PRD does not define backwards compatibility with existing JSON array files. The new format is JSONL only.

3. This PRD does not define configuration file encryption or secrets management.

4. This PRD does not define multi-workspace support (multiple data directories). One CLI instance operates on one data directory at a time.

5. This PRD does not define network-based configuration (e.g., fetching config from a server).

## Acceptance Criteria

- [ ] CLI configuration directory locations defined for Linux, macOS, Windows
- [ ] Backend data directory locations defined for Linux, macOS, Windows
- [ ] Override mechanisms defined (flags, environment variables, config file)
- [ ] JSONL format specified with examples
- [ ] Data directory file layout updated to use .jsonl extension
- [ ] Startup sequence updated for JSONL loading
- [ ] Write operation pattern updated for JSONL persistence
- [ ] CLI configuration loading sequence defined
- [ ] Config struct relationship to config.yaml documented
- [ ] All requirements numbered and specific
- [ ] File saved at docs/product-requirements/prd-configuration-directories.md

## Constraints

- JSONL files must remain human-readable (no compression)
- Platform-specific paths must follow OS conventions (XDG, Application Support, AppData)
- Atomic write pattern required for data integrity
- SQLite database remains ephemeral (can be deleted and regenerated)

## Open Questions

1. Should we support reading legacy JSON array files during a transition period? (Current answer: No, per Non-Goals)

2. Should stash_history.jsonl use a different compaction strategy to avoid unbounded growth? (Deferred to a future PRD)

## References

- prd-cupboard-core (Cupboard interface, Config struct)
- prd-sqlite-backend (current JSON file layout, to be superseded by this PRD for file format)
- XDG Base Directory Specification
- JSON Lines specification (jsonlines.org)
