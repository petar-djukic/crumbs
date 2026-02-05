# Command: Make Work

Read VISION.md, ARCHITECTURE.md, docs/product-requirements/README.md, and docs/use-cases/README.md if they exist.

First, check the current state of work:

1. Run `bd list` to see existing epics and issues
2. Check what's in progress, what's completed, what's pending

Then, summarize:

1. What problem this project solves
2. The high-level architecture (major components and how they fit together)
3. The current state of implementation (what's done, what's in progress)
4. Current repo size: run `./scripts/stats.sh` and include its output (Go production/test LOC, doc words)

Based on this, propose next steps:

1. If epics exist: suggest new issues to add to existing epics, or identify what to work on next
2. If no epics exist: suggest epics to create and initial issues for each
3. Identify dependencies - what should be built first and why?

When proposing issues (per issue-format rule):

1. **Type**: Say whether each issue is **documentation** (markdown in `docs/`) or **code** (implementation).
2. **Required Reading**: List files the agent must read before starting (PRDs, ARCHITECTURE sections, existing code). This is mandatory for all issues.
3. **Files to Create/Modify**: Explicit list of files the issue will produce or change. For docs: output path. For code: packages/files to create or edit.
4. **Structure** (all issues): Requirements, Design Decisions (optional), Acceptance Criteria.
5. **Documentation issues**: Add **format rule** reference and **required sections** (PRD: Problem, Goals, Requirements, Non-Goals, Acceptance Criteria; use case: Summary, Actor/trigger, Flow, Success criteria).
6. **Code issues**: Requirements, Design Decisions, Acceptance Criteria (tests/behavior); no PRD-style Problem/Goals/Non-Goals.

Don't create any issues yet - just propose the breakdown so we can discuss it.

After we agree on the plan and you create epics/issues:

- **Create issues only via the bd CLI** (e.g. `bd create`). Do not edit `.beads/` files directly.
- Run `bd sync`, then commit with a clear message (the commit will include `.beads/` changes produced by bd).

After you implement work:

- Commit your changes with a clear message
- Update issue status and log metrics via bd only (e.g. `bd comments add <id> "tokens: N, loc: X+Y"`, `bd close <id>`). Do not edit `.beads/` files.
- File any new issues via bd; note them for the user if not created in this session
