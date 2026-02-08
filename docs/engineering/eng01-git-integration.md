# Git Integration

This guideline describes how Crumbs integrates with git. Cupboard is git-agnostic: it reads and writes JSONL files to a data directory. The conventions below sit above the storage layer and govern how teams use Crumbs alongside git for version control, branching, and traceability.

## Data Directory in Git

JSONL files are the source of truth for the SQLite backend (see ARCHITECTURE Decision 6). We commit them to git so that task state is versioned alongside code. The SQLite database (`cupboard.db`) is ephemeral and rebuilt from JSONL on every `Attach`. It must not be committed.

Table 1 Files in git

| File | Git status |
|------|-----------|
| `*.jsonl` (crumbs, trails, links, properties, etc.) | Committed |
| `cupboard.db` | Gitignored |
| `config.yaml` | Committed (per-repo configuration) |

The `.gitignore` must include `cupboard.db` to prevent accidental commits of the binary database.

## Trails and Git Branches

Trails and git branches serve different purposes. Trails are persistent DAG records of work structure that stay in the cupboard when closed or abandoned. Git branches are ephemeral workspaces for code that are created, merged, and deleted.

Table 2 Trails vs git branches

| Concept | Lifetime | Structure | Purpose |
|---------|----------|-----------|---------|
| Trail | Persistent (closed or abandoned in place) | DAG of crumbs | Record what work happened and how it branched |
| Git branch | Ephemeral (merged and deleted) | Linear commits | Deliver code changes to the base branch |

A trail does not merge into anything. When a trail is completed, its crumbs become permanent records. When a trail is abandoned, its crumbs are deleted. In both cases the trail itself remains in the cupboard as a historical record.

Git branches carry code. A task branch is created for a unit of work, the agent commits code in the branch's worktree, and when done the branch merges back to the base branch and is deleted. The cupboard's JSONL files travel with the code (committed alongside it), so trail state is versioned in git, but the trail lifecycle (complete, abandon) is managed through the Cupboard API, not through git merge.

## Task Branches and Worktrees

Each task gets a git worktree with a branch namespaced under the base branch. The base branch is whatever branch do-work starts on (main or a generation branch).

Table 3 Task branch naming

| Base branch | Task branch | Worktree location |
|-------------|-------------|-------------------|
| `main` | `main/task/<issue-id>` | `/tmp/<project>-worktrees/<issue-id>` |
| `generation-*` | `generation-*/task/<issue-id>` | `/tmp/<project>-worktrees/<issue-id>` |

Table 4 Task branch lifecycle

| Event | Git operation |
|-------|--------------|
| Pick task | Create branch `<base>/task/<id>` from base HEAD |
| Start work | `git worktree add` (creates working directory) |
| Work on task | Commits in the worktree branch (code + JSONL changes together) |
| Complete task | Merge branch to base, `git worktree remove`, delete branch |
| Interrupted | Branch and worktree persist; recovered on next do-work run |

The namespacing makes task branches discoverable: `git branch --list 'main/task/*'` shows all task branches for the current base, including any interrupted ones. On startup, do-work recovers stale branches by removing worktrees, deleting branches, and resetting issue status.

## JSONL Merge Behavior

The JSONL sync uses in-place update with stable insertion order: existing records keep their line position, new records append at the end. This makes JSONL files git-merge-friendly.

Table 5 JSONL merge scenarios

| Scenario | Git behavior |
|----------|-------------|
| Task branch adds new crumbs, base unchanged | New lines appended; auto-merges cleanly |
| Task branch modifies a crumb that base did not touch | Changed line in place; auto-merges |
| Two task branches modify the same crumb | Merge conflict; must be resolved manually |
| Task branch deletes a crumb that base did not touch | Removed line; auto-merges |

Real merge conflicts (two branches modifying the same crumb) surface legitimate coordination problems. These should be resolved by examining which branch's change takes precedence.

## Commit Conventions

Commit messages reference crumb IDs for traceability. This enables bidirectional navigation between task state and code history without storing git hashes inside crumbs.

Table 6 Traceability directions

| Direction | How |
|-----------|-----|
| Commit to crumbs | Read the diff: JSONL changes show which crumbs were affected |
| Crumb to commits | `git log --all --grep="<crumb-id>"` finds all commits that reference the crumb |

A single crumb typically spans multiple commits. The branch history captures the full relationship.

## Main Branch as Backlog

The main branch holds the backlog: crumbs in draft or pending state that no task has picked up. When a task starts (worktree branches from main), it inherits this backlog. When a task completes (branch merges to main), completed crumb states flow back. Interrupted tasks leave stale branches that do-work recovers on the next run.

This means main's JSONL files always reflect the current state of the project: completed work merged in, abandoned work gone, pending work waiting for the next task.

## References

- eng02-generation-workflow (generation lifecycle: open, generate, close)
- do-work.sh (task execution with recovery)
- ARCHITECTURE.md (trail structure, link types)
