# Generation Workflow

We use a branch-based workflow to regenerate code from documentation. A generation is an isolated branch where agents delete existing Go source, rebuild from specs, and accumulate work through the measure/stitch loop. When the generation is complete, we merge it to main. Tags preserve the state before and after each generation so any prior version can be retrieved.

This guideline describes the generation lifecycle. For task-level branching within a generation, see eng01-git-integration. For container execution during generation, see eng04-container-execution.

## Lifecycle

A generation moves through three phases: start, run, and stop. We model these as a trail of code generation. Starting opens an exploratory path. Running adds crumbs (generated code) through measure/stitch cycles. Stopping makes the exploration permanent by merging to main. Resetting abandons the trail.

Table 1 Generation lifecycle

| Phase | Mage target | What happens | Git state after |
|-------|-------------|-------------|----------------|
| Start | `mage generator:start` | Tag main, create generation branch, delete Go files, reinitialize module | On generation branch with clean slate committed |
| Run | `mage generator:run` | measure creates tasks, stitch executes them in worktrees off the generation branch | Generation branch accumulates task merges |
| Stop | `mage generator:stop` | Tag generation, delete code from main, merge generation, tag main, delete branch | On main with generation's code and merged docs |

## Start

Starting a generation preserves the current state and creates a clean branch for agents to rebuild on.

1. Tag the current main commit as `generation-YYYY-MM-DD-HH-mm-start`. This tag captures the pre-generation state so it can be retrieved later.
2. Create and check out a branch named `generation-YYYY-MM-DD-HH-mm` from main.
3. Reset the beads database and reinitialize with the generation branch name as prefix.
4. Delete all Go source files (`*.go`), empty source directories, build artifacts, and `go.sum`.
5. Reinitialize `go.mod` and seed minimal source files.
6. Commit the clean state on the generation branch.

After start, the generation branch has documentation and configuration but no Go code. Agents rebuild everything from the specs.

## Run

Running happens on the generation branch through the measure/stitch loop. Each cycle runs `cobbler:measure` to create tasks, then `cobbler:stitch` to execute them. Each task gets a branch namespaced under the generation branch and a corresponding worktree.

Table 2 Task branch naming

| Base branch | Task branch | Example |
|-------------|-------------|---------|
| `generation-YYYY-MM-DD-HH-mm` | `<base>/task/<issue-id>` | `generation-2026-02-08-09-30/task/crumbs-abc` |
| `main` | `main/task/<issue-id>` | `main/task/crumbs-xyz` |

When a task completes, its branch merges back into the base branch (not main) and is deleted. The namespacing makes task branches discoverable: `git branch --list 'generation-2026-02-08-09-30/task/*'` shows all task branches for a generation, including any that were interrupted before completing.

The generation branch accumulates all task merges. At any point you can see the full diff of the generation with `git diff main...HEAD` (from the generation branch) or `git log main..HEAD` for the commit history.

If the process is interrupted, the generation branch persists. Unfinished task branches remain under the `<base>/task/` namespace. Resume by checking out the generation branch and running again.

## Stop

Stopping finishes the current generation and lands the work on main. We delete Go code from main before merging so the generation's code replaces it cleanly. Documentation is preserved on main so that doc changes from the generation merge normally.

1. Tag the current commit as `generation-YYYY-MM-DD-HH-mm-finished`. This marks the final state of the generation before merging.
2. Switch to main.
3. Delete all Go source files, empty source directories, build artifacts, and `go.sum` from main. Reinitialize `go.mod`. Commit this preparation step.
4. Merge the generation branch into main. The generation's code arrives without conflicts because main no longer has competing Go files. Documentation merges normally.
5. Tag main as `generation-YYYY-MM-DD-HH-mm-merged`.
6. Delete the generation branch.

After stop, main contains the generation's code, merged documentation, and three tags are preserved: the pre-generation baseline (start), the completed generation (finished), and the merged result on main (merged).

## Tags

Tags serve as retrieval points. We use the generation branch name as the tag namespace.

Table 3 Tag conventions

| Tag | Points to | Purpose |
|-----|-----------|---------|
| `generation-YYYY-MM-DD-HH-mm-start` | Main commit before generation started | Retrieve the pre-generation state |
| `generation-YYYY-MM-DD-HH-mm-finished` | Final commit on the generation branch | Retrieve the completed generation before merge |
| `generation-YYYY-MM-DD-HH-mm-merged` | Main commit after merge | Retrieve the post-merge state |

To see what a generation produced: `git diff generation-2026-02-08-09-30-start...generation-2026-02-08-09-30-finished`. To see main after the merge: `git checkout generation-2026-02-08-09-30-merged`. Use `mage generator:list` to see all generations (active branches and past generations discoverable through tags).

## Mage Interface

The generation lifecycle spans four mage namespaces and two top-level orchestration targets. Each namespace owns its own artifacts; the top-level targets call them in the correct order.

Table 4 Top-level targets

| Target       | Operation                 | What it calls                                      |
|--------------|---------------------------|----------------------------------------------------|
| `mage init`  | Initialize project state  | `beads:init`                                       |
| `mage reset` | Full reset to clean state | `cobbler:reset`, `generator:reset`, `beads:reset`  |

Table 5 Generator targets

| Target                   | Operation                                          | Precondition                    |
|--------------------------|----------------------------------------------------|---------------------------------|
| `mage generator:start`   | Start a new generation                             | Must be on main                 |
| `mage generator:run`     | Run measure/stitch cycles                          | Must be on a generation branch  |
| `mage generator:stop`    | Stop the generation and merge to main              | Generation branch must exist    |
| `mage generator:list`    | Show active and past generations                   | None                            |
| `mage generator:switch`  | Switch between generation branches                 | Target branch must exist        |
| `mage generator:reset`   | Remove generation branches, worktrees, Go sources  | None (switches to main)         |

Table 6 Cobbler targets

| Target                 | Operation                                | Precondition       |
|------------------------|------------------------------------------|--------------------|
| `mage cobbler:measure` | Propose new tasks via Claude             | Beads initialized  |
| `mage cobbler:stitch`  | Execute ready tasks via Claude           | Beads initialized  |
| `mage cobbler:reset`   | Remove the `.cobbler/` scratch directory | None               |

Table 7 Beads targets

| Target             | Operation                                               | Precondition |
|--------------------|---------------------------------------------------------|--------------|
| `mage beads:init`  | Initialize the beads database (no-op if already exists) | None         |
| `mage beads:reset` | Destroy and reinitialize the beads database             | None         |

Each reset target is independent: `cobbler:reset` only removes `.cobbler/`, `generator:reset` only handles branches, worktrees, and Go source directories, and `beads:reset` only handles the beads database. The top-level `mage reset` calls all three in order. The top-level `mage init` currently delegates to `beads:init`.

The `generator:run` target accepts `--cycles N` to control how many measure/stitch cycles to execute. Each cycle creates tasks with `cobbler:measure` and executes them with `cobbler:stitch`.

## Multiple Generations

Multiple generations can be active simultaneously. Each generation gets its own branch and beads prefix. Use `mage generator:switch` to commit current work and move between generation branches. When multiple generation branches exist, targets that need a specific branch require `--generation-branch` to disambiguate.

Main must not receive direct commits while a generation is in progress. All work flows through the generation branch.

## References

- eng01-git-integration (task-level branching, JSONL merge behavior, commit conventions)
- eng04-container-execution (container runtime, credential handling)
- magefiles/generator.go (generator lifecycle implementation)
- magefiles/measure.go (task creation)
- magefiles/stitch.go (task execution in worktrees)
