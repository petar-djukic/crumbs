# magefiles

Build tooling for the crumbs project, implemented as [Mage](https://magefile.org) targets. Mage requires all target files to live in a single flat directory (no subdirectories for Go source), so we organize by concern: one file per subsystem.

## Files

| File | Purpose |
|------|---------|
| build.go | Top-level targets: `Build`, `Clean`, `Init`, `Reset`, `Install` |
| generator.go | Generation lifecycle: `Start`, `Run`, `Resume`, `Stop`, `List`, `Switch`, `Reset` |
| cobbler.go | Agent orchestrator: config, `runClaude`, `logf`, `beadsCommit`, cobbler reset |
| measure.go | `cobbler:measure` target: proposes tasks by invoking Claude with project state |
| stitch.go | `cobbler:stitch` target: executes tasks in worktrees, merges results |
| beads.go | Beads lifecycle: `Init`, `Reset`, database helpers |
| commands.go | Shell command wrappers: git, beads (`bd`), Go toolchain |
| test.go | Test targets: `Cobbler`, `Generator`, `Resume`, `Unit`, `Integration`, `All` |
| stats.go | `Stats` target: Go LOC and documentation word counts |
| docker.go | Container image build and Claude container execution |
| flags.go | Flag parsing helpers shared across targets |
| lint.go | `Lint` target: runs golangci-lint |

## Directories

| Directory | Contents |
|-----------|----------|
| prompts/ | Go templates (`measure.tmpl`, `stitch.tmpl`) rendered as Claude prompts |
| bin/ | Mage binary cache (gitignored) |

## Other Files

| File | Purpose |
|------|---------|
| Dockerfile.claude | Container image for running Claude with project tooling |
| test-plan.yaml | Test plan for generator lifecycle and isolation tests |

## Architecture

Mage targets are grouped into namespaces using Go struct types. The `Generator`, `Cobbler`, `Beads`, and `Test` structs each use `mg.Namespace` to create `generator:*`, `cobbler:*`, `beads:*`, and `test:*` targets. Top-level targets (`Build`, `Reset`, `Init`) live in build.go as package-level functions.

All external commands (git, bd, go, claude) are wrapped in helper functions in commands.go. The `logf` function in cobbler.go provides timestamped logging with automatic generation name tagging.

For the generation workflow, see [eng02-generation-workflow.md](../docs/engineering/eng02-generation-workflow.md).
