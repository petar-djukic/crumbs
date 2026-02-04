# Issue Format

Beads issues fall into two deliverable types: **documentation** (markdown in `docs/`) and **code** (implementation). The issue description must make the type and output location explicit so agents know what to produce and where.

## Common Structure (All Issues)

Every issue description should include:

1. **Requirements** – What needs to be built or written (functional requirements, scope)
2. **Design Decisions** – Key technical or structural choices to follow (optional but recommended)
3. **Acceptance Criteria** – How we know it is done (checkable outcomes, tests, or completeness checklist)

For epics, the description can be higher level; child tasks carry the detailed structure.

## Documentation Issues

Documentation issues produce markdown (and optionally diagrams) under `docs/`. The issue must specify **output location** and **which format rule** applies.

### Output Location and Format Rule

| Deliverable type | Output location | Format rule | When to use |
| ----------------- | ---------------- | ----------- | ------------ |
| **ARCHITECTURE / docs** | `docs/ARCHITECTURE.md` or specific doc | documentation-standards | Updating system overview, components, diagrams, design decisions |
| **PRD** | `docs/product-requirements/prd-[feature-name].md` | prd-format | New or updated product requirements; numbered requirements, Problem/Goals/Non-Goals |
| **Use case** | `docs/use-cases/uc-[short-name].md` | use-case-format | Tracer-bullet flows, actor/trigger, demo criteria |

### What to Put in the Issue

- **File or directory path** – e.g. `docs/product-requirements/prd-feature-name.md`, `docs/use-cases/uc-scenario-name.md`
- **Required sections** – List the sections from the format rule (e.g. for PRD: Problem, Goals, Requirements, Non-Goals, Acceptance Criteria)
- **Scope or content hints** – Bullet points or short paragraphs for Problem, Goals, main requirements, and non-goals so the agent does not have to infer them
- **Reference to format rule** – e.g. "Follow .claude/rules/prd-format.md" or "per prd-format rule"
- **Acceptance criteria** – Include checklist items such as "All required sections present", "File saved at [path]", "Requirements numbered and specific"

Example (PRD issue):

```markdown
## PRD Location
docs/product-requirements/prd-feature-name.md

## Required Sections (per prd-format rule)
1. Problem - ...
2. Goals - ...
3. Requirements - R1: ..., R2: ...
4. Non-Goals - ...
5. Acceptance Criteria - ...

## Acceptance Criteria
- [ ] All required sections present
- [ ] File saved as prd-feature-name.md
```

## Code Issues

Code issues produce or change implementation (e.g. Go, Python, config, tests) outside of `docs/`. The issue must specify:

- **Requirements** – Features, behaviors, or changes to implement
- **Design Decisions** – Architecture, patterns, or constraints
- **Acceptance Criteria** – How to verify: tests, CLI behavior, observable outcomes

Optionally:

- **Component or path** – e.g. `pkg/`, `internal/`, `cmd/`
- **References** – PRD or ARCHITECTURE section that defines the contract

Do not put PRD-style "Problem/Goals/Non-Goals" in code issues; use Requirements + Design Decisions + Acceptance Criteria.

### Go Layout (Recommended)

- **pkg/** – Shared public API: types and interfaces. No implementation; importable by other modules.
- **internal/** – Private implementation details. Not importable outside the module.
- **cmd/** – Entry points and executables.

When proposing or implementing code issues, keep implementation in **internal/** not **pkg/**.

## Quick Reference

| Issue type | Output | Key sections in issue |
| ---------- | ------ | ---------------------- |
| Documentation (ARCHITECTURE, general docs) | `docs/*.md`, `docs/**/*.puml` | Context, Requirements, Acceptance Criteria; follow documentation-standards |
| Documentation (PRD) | `docs/product-requirements/prd-*.md` | PRD location, Required sections (Problem, Goals, Requirements, Non-Goals, Acceptance Criteria), Acceptance Criteria; follow prd-format |
| Documentation (use case) | `docs/use-cases/uc-*.md` | File location, Summary, Actor/trigger, Flow, Success criteria; follow use-case-format |
| Code | `pkg/`, `internal/`, `cmd/` | Requirements, Design Decisions, Acceptance Criteria (tests/behavior); see Go layout above |

## When Creating or Editing Issues

1. Set **deliverable type**: documentation vs code.
2. If documentation: set **output path** and **format rule** (PRD, use case, ARCHITECTURE).
3. Include **Requirements** and **Acceptance Criteria** in every issue.
4. For documentation, add **Required sections** and scope bullets so the agent knows what to write and where to save it.
