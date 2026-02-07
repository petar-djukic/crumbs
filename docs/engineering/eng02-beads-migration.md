# Beads to Cupboard Migration

## Introduction

We migrate issue tracking from the beads (bd) CLI to the cupboard CLI. This guideline maps bd commands to their cupboard equivalents so scripts and workflows can be updated systematically.

## Command Parity

Table 1: bd to cupboard command mapping

| bd command | cupboard equivalent | Used by |
|-----------|-------------------|---------|
| `bd ready -n 1 --json --type task` | `cupboard ready -n 1 --json --type task` | do-work.sh |
| `bd update <id> --status in_progress` | `cupboard update <id> --status in_progress` | do-work.sh, interactive |
| `bd close <id>` | `cupboard close <id>` | do-work.sh, interactive |
| `bd list --json` | `cupboard list --json` | make-work.sh |
| `bd create --type <type> --title --description` | `cupboard create --type <type> --title --description` | make-work.sh |
| `bd show <id>` | `cupboard show <id>` | interactive |
| `bd comments add <id> "text"` | `cupboard comments add <id> "text"` | interactive |
| `bd sync` | Not needed | do-work.sh |

The `bd sync` command has no cupboard equivalent because the SQLite backend syncs JSONL on every write (see eng01-git-integration).

## Data Migration

JSONL files (crumbs.jsonl, trails.jsonl) replace `.beads/issues.jsonl`. The `git add` in session completion commits JSONL files from the data directory instead of `.beads/` files.

## Script Updates

We update do-work.sh and make-work.sh by replacing bd commands with cupboard commands. The scripts retain their structure: do-work.sh picks tasks, creates worktrees, runs Claude, merges, and closes; make-work.sh creates issues and imports them.

## Retirement

Once scripts and the interactive workflow run on cupboard, we remove the bd dependency: delete `.beads/`, update `.gitignore`, and replace `beads-workflow.md` with a cupboard workflow rule.

## References

- eng01-git-integration (JSONL in git, trails as worktrees)
- docs/use-cases/rel00.0-uc001-self-hosting.md (the use case this migration enables)
