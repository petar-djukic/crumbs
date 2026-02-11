# Container Execution

We run Claude Code inside a container during generation. The container provides a reproducible environment with Go tooling, beads, and Claude Code pre-installed. All git operations remain on the host; the container only performs file reads and writes against a mounted directory.

## Runtime Detection

The build system checks for a working container runtime at each invocation. We prefer podman over docker, falling back to the direct `claude` binary when neither is available.

Table 1 Runtime detection order

| Priority | Runtime | Check |
|----------|---------|-------|
| 1 | podman | Binary on PATH and `podman info` succeeds |
| 2 | docker | Binary on PATH and `docker info` succeeds |
| 3 | claude (direct) | Binary on PATH; no container isolation |

A runtime that exists on PATH but cannot connect to its daemon (e.g. podman without a running machine) is skipped with a warning on stderr. When no container runtime is usable, `mage build` still compiles the Go binary and prints a warning; it does not fail.

## Image Structure

The image is defined in `magefiles/Dockerfile.claude` and uses a multi-stage build to keep the final image small.

Table 2 Build stages

| Stage | Base image | Purpose |
|-------|-----------|---------|
| go-tools | golang:1.25.6-alpine | Compile mage and golangci-lint as static binaries (CGO disabled) |
| final | node:20-alpine | Runtime with Node (for Claude Code npm package), Go, git, and bash |

The final image copies the Go runtime from the official Go image and the compiled tool binaries from the go-tools stage. Claude Code is installed globally via npm. Node provides the JavaScript runtime that Claude Code requires.

The container runs as a non-root user (`crumbs`). Claude Code refuses `--dangerously-skip-permissions` when invoked as root, so this is not optional.

## Credential Handling

Claude Code authenticates via a JSON credential file. On the host, credentials live in the `.secrets/` directory at the repository root. The `--token-file` flag (default: `claude.json`) selects which file to use.

At container startup, the credential file is bind-mounted read-only into the location Claude Code expects on Linux:

```
.secrets/claude.json  →  /home/crumbs/.claude/.credentials.json:ro
```

The `--rm` flag ensures the container and its filesystem are destroyed after each run. No credentials persist inside the container.

On macOS, Claude Code stores credentials in the system keychain. To extract them into a file for container use:

```
security find-generic-password -s "Claude Code-credentials" -w > .secrets/claude.json
```

The `.secrets/` directory is gitignored and dockerignored.

## Workspace Mounting

The host directory where Claude should read and write files is bind-mounted as `/workspace` inside the container. During stitch, this is the worktree directory; during measure, it is the repository root.

Table 3 Mount layout

| Host path | Container path | Mode |
|-----------|---------------|------|
| Working directory (repo root or worktree) | /workspace | read-write |
| .secrets/\<token-file\> | /home/crumbs/.claude/.credentials.json | read-only |

Claude sees `/workspace` as its working directory and operates on files there. Changes written by Claude appear immediately on the host because of the bind mount.

## Separation of Concerns

The container handles one thing: running Claude Code with a prompt. Everything else happens on the host.

Table 4 Host vs container responsibilities

| Responsibility | Where |
|----------------|-------|
| Git operations (branch, worktree, merge, commit) | Host (mage) |
| Beads operations (create, update, close, sync) | Host (mage) |
| Claude Code execution | Container (or direct on host) |
| File reads and writes during Claude session | Container via /workspace mount |
| Credential storage | Host (.secrets/) |

This separation matters for worktrees. A git worktree's `.git` file contains an absolute path to the parent repository's `.git` directory. That path is a host path and would be invalid inside the container. Because Claude does not run git commands, this is not a problem. Claude reads source files, writes new code, and exits. The host handles all git operations before and after the container run.

## Mage Integration

Container image management is woven into the standard mage targets.

Table 5 Mage targets and container behavior

| Target | Container action |
|--------|-----------------|
| `mage build` | Compiles Go binary, then builds container image if runtime available |
| `mage clean` | Removes build artifacts, then removes container image if runtime available |
| `mage test:docker` | Builds everything, runs Claude with a "Hello World" prompt as a smoke test |
| `cobbler:measure` | Calls `runClaude` which auto-detects runtime and runs in container if possible |
| `cobbler:stitch` | Same as measure, but runs in the task's worktree directory |

The `runClaude` function is the single entry point for Claude execution. It checks `containerRuntime()` first; if a runtime is available, it delegates to `runClaudeContainer`. Otherwise it falls back to invoking the `claude` binary directly. Callers do not need to know which path is taken.

## Stdout and Stderr Convention

Claude Code outputs stream-json to stdout when invoked with `--output-format stream-json`. To support piping this output through `jq` or other processors, all mage status messages, warnings, and container build output go to stderr. Only Claude's JSON output appears on stdout.

## References

- eng01-git-integration (worktree lifecycle and task branch naming)
- eng02-generation-workflow (generation lifecycle: open, generate, close)
- magefiles/docker.go (runtime detection, container execution)
- magefiles/Dockerfile.claude (image definition)
- magefiles/cobbler.go (runClaude entry point)
