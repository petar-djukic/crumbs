# Use Case: Agent Uses Blazes (Workflow Templates)

## Summary

A coding agent discovers predefined workflow templates (blazes) from a template directory, reads template metadata to understand available workflows, and selects a template based on semantic match to the current task. This tracer bullet validates the template discovery and selection flow that enables agents to reuse common workflow patterns.

## Actor and Trigger

The actor is a coding agent (e.g., Claude Code) that needs to perform a recognized workflow pattern such as bug fixing, feature implementation, or code review. The trigger is the agent encountering a task that matches a known workflow category. Instead of constructing a workflow from scratch, the agent discovers and selects a predefined template.

## Flow

1. **Discover template directory**: The agent locates the template directory (configured via Cupboard or a well-known path such as `.crumbs/blazes/`). The directory contains YAML files, each defining one workflow template.

2. **List available templates**: The agent reads the directory and enumerates all `.yaml` or `.yml` files. Each file represents one blaze.

3. **Read template metadata**: For each template file, the agent parses the YAML and extracts metadata: name, description, tags, and parameters. The agent does not parse the full trail definition at this stage.

4. **Match task to template**: The agent compares the current task context against template metadata. Tags enable semantic matching (e.g., a task described as "fix the login bug" matches templates tagged with `bug-fix`, `debugging`). The agent may also use the description for fuzzy matching.

5. **Select template**: The agent chooses the best-matching template. If multiple templates match, the agent may prompt the user or select based on priority or specificity.

6. **Extract parameters**: The agent reads the parameters section of the selected template. Each parameter has a name, description, type, and optionally a default value. The agent determines which parameters it can infer from context and which require user input.

7. **Present selection**: The agent displays the selected template name, description, and required parameters to the user for confirmation before proceeding.

### Example Template YAML

```yaml
# .crumbs/blazes/bug-fix.yaml
name: Bug Fix
description: Investigate and fix a reported bug with proper testing
tags:
  - bug-fix
  - debugging
  - defect
parameters:
  - name: bug_description
    type: text
    description: Short description of the bug
  - name: reproduction_steps
    type: text
    description: Steps to reproduce the bug
    default: ""
  - name: affected_files
    type: list
    description: Files likely affected by the bug
    default: []
trail:
  # Trail definition structure (out of scope for this use case)
  crumbs:
    - name: "Reproduce bug: {{ bug_description }}"
      dependencies: []
    - name: "Identify root cause"
      dependencies: [0]
    - name: "Implement fix"
      dependencies: [1]
    - name: "Write regression test"
      dependencies: [2]
    - name: "Verify fix resolves original issue"
      dependencies: [2, 3]
```

The `trail` section defines the crumbs and their dependency graph. Crumbs follow the structure defined in prd-crumbs-interface R1 (CrumbID, Name, State, CreatedAt, UpdatedAt, Properties). Crumb dependencies use the `child_of` link type from the links table. Parameter placeholders use mustache-style syntax (`{{ parameter_name }}`). The instantiation mechanism that expands parameters and creates the actual trail is deferred to a future PRD (prd-blazes-instantiation).

## Architecture Touchpoints

This use case exercises the following interfaces and components:

Table 1: Components exercised by this use case

| Component | Role |
|-----------|------|
| Template directory | File system path containing blaze YAML files |
| YAML parser | Reads and parses template files into structured data |
| Template metadata | Name, description, tags, parameters extracted from YAML |
| Parameter extraction | Identifies required inputs before instantiation |

The use case validates:

- Template discovery via directory enumeration
- YAML structure parsing for metadata fields
- Tag-based semantic matching for template selection
- Parameter schema extraction from template definition

This use case does not exercise Cupboard or Table interfaces directly. It operates on the file system and YAML parsing layer that precedes trail instantiation. Once instantiation is implemented (per prd-blazes-instantiation), the flow will continue to:

- Trail creation via Cupboard.GetTable("trails").Set per prd-trails-interface R3
- Crumb creation via Cupboard.GetTable("crumbs").Set per prd-crumbs-interface R3
- Link creation via Cupboard.GetTable("links").Set for belongs_to (prd-trails-interface R7) and child_of relationships

## Success Criteria

The demo succeeds when:

- [ ] Agent discovers template directory at configured or well-known path
- [ ] Agent lists all `.yaml`/`.yml` files in the directory
- [ ] Agent parses template metadata (name, description, tags, parameters) without error
- [ ] Agent matches a task description to template tags
- [ ] Agent selects the best-matching template
- [ ] Agent extracts parameter definitions (name, type, description, default)
- [ ] Agent presents selected template and parameters to user

Observable demo script:

```bash
# 1. List available templates
ls .crumbs/blazes/*.yaml

# 2. Show template metadata (name, description, tags)
yq '.name, .description, .tags' .crumbs/blazes/bug-fix.yaml

# 3. Show required parameters
yq '.parameters[] | .name + ": " + .description' .crumbs/blazes/bug-fix.yaml

# 4. Agent matches task "fix the login crash" to bug-fix template
# (semantic matching logic in agent)

# 5. Agent displays selection
# Selected template: Bug Fix
# Description: Investigate and fix a reported bug with proper testing
# Parameters needed:
#   - bug_description (text): Short description of the bug
#   - reproduction_steps (text): Steps to reproduce the bug
#   - affected_files (list): Files likely affected by the bug
```

## Out of Scope

This use case does not cover:

- Trail instantiation from template (requires prd-blazes-instantiation)
- Parameter value binding and placeholder expansion (requires prd-blazes-instantiation)
- Creating crumbs via Table.Set per prd-crumbs-interface R3 (requires prd-blazes-instantiation)
- Creating trails via Table.Set per prd-trails-interface R3 (requires prd-blazes-instantiation)
- Creating belongs_to links per prd-trails-interface R7 (requires prd-blazes-instantiation)
- Template validation and error handling for malformed YAML (requires prd-blazes-schema)
- Template versioning or inheritance
- Remote template repositories
- Template creation or editing workflows

The instantiation mechanism that transforms a selected template with bound parameters into an actual trail with crumbs will be specified in prd-blazes-instantiation. That PRD will reference prd-trails-interface R3 (trail creation), prd-crumbs-interface R3 (crumb creation), and the links table for establishing crumb membership (belongs_to) and dependencies (child_of).

## Dependencies

### Infrastructure

- File system access to template directory
- YAML parsing capability (standard library or dependency)
- Agent context for semantic matching (task description, tags)

This use case does not depend on Cupboard or Table implementations. It operates at the template discovery layer.

### PRD Coverage

Table 2: PRD requirements referenced by this use case

| PRD | Requirements | Coverage |
|-----|--------------|----------|
| prd-crumbs-interface | R1 (Crumb struct), R2 (State values), R9 (Filter map) | Partial: template defines crumb structure; instantiation deferred |
| prd-trails-interface | R1 (Trail struct), R2 (State values), R3 (Trail creation) | Partial: template defines trail structure; instantiation deferred |
| prd-stash-interface | R2 (Stash types: context, artifact) | Partial: templates may reference shared resources; instantiation deferred |

### PRD Gaps

The following PRDs do not yet exist and are required for full blaze functionality:

| Missing PRD | Purpose |
|-------------|---------|
| prd-blazes-instantiation | Defines how templates are expanded into trails and crumbs, including parameter binding, placeholder expansion, and link creation |
| prd-blazes-schema | Defines the YAML schema for blaze templates, validation rules, and versioning |

Until prd-blazes-instantiation is written, this use case validates only template discovery and selection. Trail and crumb creation from templates remains out of scope.

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| No templates in directory | Return empty list gracefully; agent proceeds without template |
| Malformed YAML in template file | Skip invalid templates with warning; do not fail entire discovery |
| Multiple templates match equally | Define selection heuristics (most specific tags, user preference) |
| Template schema evolves | Version field in template enables migration; validate against schema |
| Large template directory slows discovery | Cache parsed metadata; lazy-load trail definitions |
