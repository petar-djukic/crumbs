# Use Case: Self-Hosting

## Summary

The cupboard CLI tracks development work on the crumbs repository. The do-work.sh and make-work.sh scripts run end-to-end using cupboard commands. This validates that cupboard works as an issue tracker for real agent workflows.

## Actor and Trigger

The actor is a coding agent (e.g., Claude Code) working on the crumbs codebase via the do-work.sh and make-work.sh scripts. The trigger is the cupboard CLI providing the issue-tracking commands those scripts need.

## Flow

### Phase 1: Issue-Tracking CLI

The cupboard CLI provides issue-tracking commands that operate on crumbs. Under the hood, each command uses the Cupboard library (Table.Get, Table.Set, Table.Fetch) for storage. The CLI layers issue-tracking semantics on top of the storage layer.

1. Implement the following commands.

Table 1: Cupboard issue-tracking commands

| Command | Purpose |
|---------|---------|
| `cupboard ready -n N --json --type <type>` | Return up to N open crumbs of the given type, ordered by priority |
| `cupboard create --type <type> --title T --description D` | Create a crumb with type, title, description, and optional --parent and --labels |
| `cupboard update <id> --status <status>` | Transition a crumb's state (open, in_progress, closed) |
| `cupboard close <id>` | Shortcut for setting state to closed with a timestamp |
| `cupboard show <id>` | Display a crumb's details in human-readable format |
| `cupboard list --json` | Return all crumbs as JSON |
| `cupboard comments add <id> "text"` | Append a comment to a crumb |

2. The `--json` flag outputs JSON for script consumption. Without it, output is human-readable.

3. The `cupboard create` command accepts `--type` (task, epic, chore, bug), `--title`, `--description`, `--parent`, and `--labels`. It creates a crumb with the appropriate properties set.

4. Status transitions follow crumb state semantics: open, in_progress, closed.

### Phase 2: Script Integration

5. The do-work.sh script uses cupboard commands to pick a task, claim it, and close it after completion. The script's structure: pick task, create worktree, run Claude, merge, clean up, close task.

6. The make-work.sh script uses cupboard commands to list existing issues and create new ones. The import function creates crumbs via `cupboard create`.

7. No explicit sync command is needed. The SQLite backend syncs JSONL on every write (see eng01-git-integration). Scripts commit JSONL files to git as part of their workflow.

### Phase 3: Interactive Workflow

8. The agent workflow rule (.claude/rules/) references cupboard commands for interactive use.

```bash
cupboard ready              # Find available work
cupboard show <id>          # View issue details
cupboard update <id> --status in_progress  # Claim work
cupboard comments add <id> "tokens: <count>"  # Log token usage
cupboard close <id>         # Close work
```

9. JSONL files (crumbs.jsonl, trails.jsonl) are committed to git per eng01-git-integration. The `git add` in session completion commits JSONL files from the data directory.

## Architecture Touchpoints

Table 2: Components exercised by self-hosting

| Component | Role |
|-----------|------|
| cupboard CLI (`cmd/cupboard`) | Issue-tracking commands (ready, create, close, update, show, list, comments) |
| Cupboard library (`pkg/types`) | Table interface, Crumb/Trail/Property entities |
| SQLite backend (`internal/sqlite`) | Storage, JSONL persistence, query engine |
| do-work.sh | Task picking, worktree management, agent invocation |
| make-work.sh | Work creation, issue import |
| .claude/rules/ | Agent workflow instructions |

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

- Release 01.0 (core storage): Cupboard interface, Table interface, SQLite backend
- Release 02.0 (properties): properties enable type, priority, labels on crumbs
- Issue-tracking CLI commands (the new work in this use case)

## Risks and Mitigations

Table 3: Risks

| Risk | Mitigation |
|------|------------|
| Bug in cupboard loses work tracking | Commit JSONL frequently; git history provides recovery |
| Missing CLI features block scripts | Implement minimum commands first, iterate on ergonomics after |
| Self-hosting reveals missing features | Treat as validation; file issues for gaps |
