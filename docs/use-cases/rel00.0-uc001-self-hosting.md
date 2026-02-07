# Use Case: Self-Hosting

## Summary

The cupboard CLI tracks development work on the crumbs repository. The do-work.sh and make-work.sh scripts run end-to-end using cupboard commands. This validates that cupboard works as an issue tracker for real agent workflows.

## Actor and Trigger

The actor is a coding agent (e.g., Claude Code) working on the crumbs codebase via the do-work.sh and make-work.sh scripts. The trigger is the cupboard CLI providing the issue-tracking commands those scripts need.

## Flow

### Phase 1: Issue-Tracking CLI

The cupboard CLI provides issue-tracking commands that operate on crumbs. Under the hood, each command uses the Cupboard library via GetTable (prd-cupboard-core R2) for storage. Table operations (Get, Set, Fetch) follow the Table interface contract (prd-cupboard-core R3). The CLI layers issue-tracking semantics on top of the storage layer.

1. Implement the following commands.

Table 1: Cupboard issue-tracking commands

| Command | Purpose |
|---------|---------|
| `cupboard ready -n N --json --type <type>` | Return up to N open crumbs of the given type, ordered by priority (prd-crumbs-interface R9, R10) |
| `cupboard create --type <type> --title T --description D` | Create a crumb with type, title, description, and optional --parent and --labels (prd-crumbs-interface R3) |
| `cupboard update <id> --status <status>` | Transition a crumb's state (prd-crumbs-interface R4) |
| `cupboard close <id>` | Shortcut for setting state to closed with a timestamp (prd-crumbs-interface R4.3) |
| `cupboard show <id>` | Display a crumb's details in human-readable format (prd-cupboard-core R3.2) |
| `cupboard list --json` | Return all crumbs as JSON (prd-crumbs-interface R10) |
| `cupboard comments add <id> "text"` | Append a comment to a crumb (metadata via prd-sqlite-backend R2.8) |

2. The `--json` flag outputs JSON for script consumption. Without it, output is human-readable.

3. The `cupboard create` command accepts `--type` (task, epic, chore, bug), `--title`, `--description`, `--parent`, and `--labels`. It creates a crumb with the appropriate properties set: type and labels are categorical and list properties (prd-properties-interface R3), while parent creates a child_of link (prd-sqlite-backend R2.7).

4. Status transitions follow crumb state semantics: draft, pending, ready, taken, pebble, dust (prd-crumbs-interface R2).

### Phase 2: Script Integration

5. The do-work.sh script uses cupboard commands to pick a task, claim it, and close it after completion. The script's structure: pick task (Table.Fetch with state filter, prd-crumbs-interface R10), create worktree, run Claude, merge, clean up, close task (state transition via Table.Set, prd-crumbs-interface R4).

6. The make-work.sh script uses cupboard commands to list existing issues and create new ones. The import function creates crumbs via `cupboard create`, which calls Table.Set with an empty ID to generate a UUID v7 (prd-cupboard-core R8, prd-crumbs-interface R3).

7. No explicit sync command is needed. The SQLite backend syncs JSONL on every write using the immediate sync strategy (prd-sqlite-backend R5, R16). Scripts commit JSONL files to git as part of their workflow (see eng01-git-integration).

### Phase 3: Interactive Workflow

8. The agent workflow rule (.claude/rules/) references cupboard commands for interactive use.

```bash
cupboard ready              # Find available work (Table.Fetch, prd-crumbs-interface R10)
cupboard show <id>          # View issue details (Table.Get, prd-cupboard-core R3.2)
cupboard update <id> --status in_progress  # Claim work (SetState, prd-crumbs-interface R4)
cupboard comments add <id> "tokens: <count>"  # Log token usage (metadata, prd-sqlite-backend R2.8)
cupboard close <id>         # Close work (Pebble, prd-crumbs-interface R4.3)
```

9. JSONL files (crumbs.jsonl, trails.jsonl, links.jsonl, etc.) are committed to git per eng01-git-integration. The `git add` in session completion commits JSONL files from the data directory. The directory layout follows prd-sqlite-backend R1.

## Architecture Touchpoints

Table 2: Components exercised by self-hosting

| Component | Role | PRD References |
|-----------|------|----------------|
| cupboard CLI (`cmd/cupboard`) | Issue-tracking commands (ready, create, close, update, show, list, comments) | prd-crumbs-interface R3, R4, R10 |
| Cupboard library (`pkg/types`) | Cupboard interface with GetTable, Table interface for CRUD, Crumb/Property entities | prd-cupboard-core R2, R3; prd-crumbs-interface R1 |
| SQLite backend (`internal/sqlite`) | Storage, JSONL persistence, query engine, entity hydration | prd-sqlite-backend R1–R5, R12–R15 |
| do-work.sh | Task picking (Fetch), worktree management, agent invocation | prd-crumbs-interface R9, R10 |
| make-work.sh | Work creation (Set), issue import | prd-crumbs-interface R3; prd-cupboard-core R3.3 |
| .claude/rules/ | Agent workflow instructions | eng01-git-integration |

We validate:

- Cupboard lifecycle: Attach initializes DataDir and backend; Detach releases resources (prd-cupboard-core R4, R5)
- Table access: GetTable routes to entity-specific tables (prd-cupboard-core R2; prd-sqlite-backend R12)
- Crumb creation: Set with empty ID generates UUID v7, initializes state to draft (prd-crumbs-interface R3)
- State transitions: SetState, Pebble, Dust entity methods (prd-crumbs-interface R4)
- Property values: type, priority, labels stored via Properties map (prd-crumbs-interface R5; prd-properties-interface R9)
- Query operations: Fetch with filter map for state-based queries (prd-crumbs-interface R9, R10)
- JSONL persistence: All writes persist atomically to JSONL files (prd-sqlite-backend R5)

## Success / Demo Criteria

- `cupboard ready` returns available tasks as JSON
- `cupboard create --type task --title "Test" --description "..."` creates a crumb
- `cupboard update <id> --status in_progress` transitions crumb state
- `cupboard close <id>` marks crumb as closed
- `cupboard list --json` returns all crumbs as JSON
- do-work.sh runs a full cycle (pick, worktree, Claude, merge, close) using cupboard
- make-work.sh creates issues via cupboard and commits JSONL changes

## Out of Scope

- Multi-agent coordination (single agent self-hosting)
- Remote backends (SQLite only)
- Trail-based worktree integration in the CLI (trails are worktrees per eng01-git-integration, but the CLI does not manage git worktrees; do-work.sh handles that)

## Dependencies

- Cupboard interface with Attach/Detach/GetTable (prd-cupboard-core R2, R4, R5)
- Table interface with Get/Set/Delete/Fetch (prd-cupboard-core R3)
- Crumb entity with state transitions and property methods (prd-crumbs-interface R1–R5)
- SQLite backend with JSONL persistence (prd-sqlite-backend R1–R5, R11–R15)
- Property definitions for type, priority, labels (prd-properties-interface R1, R9)
- Built-in properties seeded on first startup (prd-sqlite-backend R9)
- Issue-tracking CLI commands (the new work in this use case)

## Risks and Mitigations

Table 3: Risks

| Risk | Mitigation |
|------|------------|
| Bug in cupboard loses work tracking | Commit JSONL frequently; git history provides recovery |
| Missing CLI features block scripts | Implement minimum commands first, iterate on ergonomics after |
| Self-hosting reveals missing features | Treat as validation; file issues for gaps |
