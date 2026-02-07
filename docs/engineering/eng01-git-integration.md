# Git Integration

This guideline describes how Crumbs integrates with git. Cupboard is git-agnostic: it reads and writes JSONL files to a data directory. The conventions below sit above the storage layer and govern how teams use Crumbs alongside git for version control, branching, and traceability.

## Data Directory in Git

JSONL files are the source of truth for the SQLite backend (see ARCHITECTURE Decision 6). We commit them to git so that task state is versioned alongside code. The SQLite database (`cupboard.db`) is ephemeral and rebuilt from JSONL on every `Attach`. It must not be committed.

| File | Git status |
|------|-----------|
| `*.jsonl` (crumbs, trails, links, properties, etc.) | Committed |
| `cupboard.db` | Gitignored |
| `config.yaml` | Committed (per-repo configuration) |

The `.gitignore` must include `cupboard.db` to prevent accidental commits of the binary database.

## Trails as Worktrees

Each trail maps to a git worktree. A worktree is a separate working directory attached to its own branch, so each trail gets an isolated copy of both code and JSONL task state.

| Trail lifecycle | Git operation |
|----------------|---------------|
| Create trail | `git worktree add` (creates branch and working directory) |
| Work on trail | Commits in the worktree branch (code + JSONL changes together) |
| Complete trail | Merge branch to main, `git worktree remove` |
| Abandon trail | Delete branch, `git worktree remove` |

When a trail branches off main, it inherits the current backlog (crumbs in draft/pending state). As work progresses in the worktree, the trail's JSONL files diverge from main. Completing the trail merges both code and task state back.

## JSONL Merge Behavior

The JSONL sync uses in-place update with stable insertion order: existing records keep their line position, new records append at the end. This makes JSONL files git-merge-friendly.

| Scenario | Git behavior |
|----------|-------------|
| Trail adds new crumbs, main unchanged | New lines appended; auto-merges cleanly |
| Trail modifies a crumb that main did not touch | Changed line in place; auto-merges |
| Two trails modify the same crumb | Merge conflict; must be resolved manually |
| Trail deletes a crumb that main did not touch | Removed line; auto-merges |

Real merge conflicts (two trails modifying the same crumb) surface legitimate coordination problems. These should be resolved by examining which trail's change takes precedence.

## Commit Conventions

Commit messages reference crumb IDs for traceability. This enables bidirectional navigation between task state and code history without storing git hashes inside crumbs.

| Direction | How |
|-----------|-----|
| Commit to crumbs | Read the diff: JSONL changes show which crumbs were affected |
| Crumb to commits | `git log --all --grep="<crumb-id>"` finds all commits that reference the crumb |

A single crumb typically spans multiple commits. The branch history captures the full relationship.

## Main Branch as Backlog

The main branch holds the backlog: crumbs in draft or pending state that no trail has picked up. When a trail starts (worktree branches from main), it inherits this backlog. When a trail completes (branch merges to main), completed crumb states flow back. Abandoned trails discard their changes (branch deleted without merge).

This means main's JSONL files always reflect the current state of the project: completed work merged in, abandoned work gone, pending work waiting for the next trail.
