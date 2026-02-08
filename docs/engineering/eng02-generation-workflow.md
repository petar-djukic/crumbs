# Generation Workflow

We use a branch-based workflow to regenerate code from documentation. A generation is an isolated branch where agents delete existing Go source, rebuild from specs, and accumulate work through the make-work/do-work loop. When the generation is complete, we merge it to main. Tags preserve the state before and after each generation so any prior version can be retrieved.

This guideline describes the generation lifecycle. For task-level branching within a generation, see eng01-git-integration.

## Lifecycle

A generation moves through three phases: open, generate, and close.

Table 1 Generation lifecycle

| Phase | What happens | Git state after |
|-------|-------------|----------------|
| Open | Tag main, create generation branch, delete Go files, reinitialize module | On generation branch with clean slate committed |
| Generate | make-work creates tasks, do-work executes them in worktrees off the generation branch | Generation branch accumulates task merges |
| Close | Tag generation branch as closed, merge to main, delete branch | On main with generation merged |

## Open

Opening a generation preserves the current state and creates a clean branch for agents to rebuild on.

1. Tag the current main commit as `generation-YYYY-MM-DD-HH-mm`. This tag captures the pre-generation state so it can be retrieved later.
2. Create and check out a branch named `generation-YYYY-MM-DD-HH-mm` from main.
3. Delete all Go source files (`*.go`), empty source directories, build artifacts, and `go.sum`.
4. Reinitialize `go.mod`.
5. Commit the clean state on the generation branch.

After open, the generation branch has documentation and configuration but no Go code. Agents rebuild everything from the specs.

## Generate

Generation happens on the generation branch through the make-work/do-work loop. `do-work.sh` records the current branch as the base branch at startup. Each task gets a branch namespaced under the base branch and a corresponding worktree.

Table 2 Task branch naming

| Base branch | Task branch | Example |
|-------------|-------------|---------|
| `generation-YYYY-MM-DD-HH-mm` | `<base>/task/<issue-id>` | `generation-2026-02-08-09-30/task/crumbs-abc` |
| `main` | `main/task/<issue-id>` | `main/task/crumbs-xyz` |

When a task completes, its branch merges back into the base branch (not main) and is deleted. The namespacing makes task branches discoverable: `git branch --list 'generation-2026-02-08-09-30/task/*'` shows all task branches for a generation, including any that were interrupted before completing.

The generation branch accumulates all task merges. At any point you can see the full diff of the generation with `git diff main...HEAD` (from the generation branch) or `git log main..HEAD` for the commit history.

If the process is interrupted, the generation branch persists. Unfinished task branches remain under the `<base>/task/` namespace. Resume by checking out the generation branch and running do-work again.

## Close

Closing finishes the current generation and lands the work on main.

1. Verify we are on a `generation-*` branch. Refuse to close if on main or any other branch.
2. Tag the current commit as `generation-YYYY-MM-DD-HH-mm-closed`. This marks the final state of the generation before merging.
3. Switch to main.
4. Merge the generation branch into main.
5. Delete the generation branch.

After close, main contains the regenerated code and both tags (pre-generation baseline and closed generation) are preserved in the history.

## Tags

Tags serve as retrieval points. We use the generation branch name as the tag namespace.

Table 3 Tag conventions

| Tag | Points to | Purpose |
|-----|-----------|---------|
| `generation-YYYY-MM-DD-HH-mm` | Main commit before generation started | Retrieve the pre-generation state |
| `generation-YYYY-MM-DD-HH-mm-closed` | Final commit on the generation branch | Retrieve the completed generation before merge |

To retrieve a previous generation's pre-state: `git checkout generation-2026-02-08-09-30`. To see what a generation produced: `git diff generation-2026-02-08-09-30...generation-2026-02-08-09-30-closed`.

## Script Interface

The generation lifecycle is handled by separate scripts.

Table 4 Generation scripts

| Script | Operation | Precondition |
|--------|-----------|-------------|
| `open-generation.sh` | Open a new generation | Must be on main |
| `generate.sh` | Run make-work/do-work cycles | Works on any branch |
| `close-generation.sh` | Close the current generation | Must be on a `generation-*` branch |
| `do-work.sh` | Drain the task queue | Works on any branch |
| `make-work.sh` | Create new tasks | Works on any branch |

`open-generation.sh` tags main, creates the generation branch, deletes Go files, and commits the clean slate. `generate.sh` runs the generation loop: it calls `make-work.sh` to create tasks then `do-work.sh` to execute them, repeating for the requested number of cycles. `close-generation.sh` tags the generation branch as closed, merges to main, and deletes the branch. `do-work.sh` and `make-work.sh` can also be called independently outside of a generation.

## Constraints

We run one generation at a time. Opening a new generation while another is in progress is not supported. If a generation branch exists, either close it or delete it before opening a new one.

Main must not receive direct commits while a generation is in progress. All work flows through the generation branch.

## References

- eng01-git-integration (task-level branching, JSONL merge behavior, commit conventions)
- open-generation.sh (open a generation)
- generate.sh (make-work/do-work loop)
- close-generation.sh (close a generation)
- do-work.sh (drain task queue)
- make-work.sh (create tasks)
