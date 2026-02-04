# Use Case: Self-Hosting (Crumbs Builds Crumbs)

## Summary

A coding agent uses the crumbs system to track the development of crumbs itself. This validates that the system works for real agent workflows and serves as a milestone marker for when crumbs is "ready enough" for production use.

## Actor and Trigger

The actor is a coding agent (e.g., Claude Code) working on the crumbs codebase. The trigger is reaching the minimum viable product: CrumbTable implemented with CLI commands for basic operations.

## Flow

### Phase 1: Transition from Beads

1. **Verify MVP readiness**: Confirm CrumbTable operations work (Add, Get, Update, SetState, Archive, Fetch) and CLI commands exist for each.

2. **Initialize crumbs cupboard**: Run `crumbs init --datadir .crumbs` in the crumbs repo root. The backend creates the directory, JSON files, SQLite schema, and seeds built-in properties.

3. **Import existing work**: For any open beads issues, create corresponding crumbs via `crumbs add "Issue title"`. Set properties (priority, type, description) to match beads data.

4. **Retire beads for this repo**: Once crumbs are tracking all work, stop using `bd` commands for new work in the crumbs repo. Beads remains available for other projects.

### Phase 2: Basic Self-Hosting

5. **Create work items**: When starting new implementation tasks, run `crumbs add "Implement PropertyTable.Define"`. The crumb is created in draft state with all properties initialized.

6. **Track progress**: As work progresses, update state via `crumbs state <id> ready` when ready to implement, `crumbs state <id> taken` when actively working.

7. **Complete or archive**: When implementation is done, run `crumbs state <id> completed`. For abandoned work, run `crumbs archive <id>`.

8. **Query work**: Use `crumbs fetch --state ready` to see available work, `crumbs fetch --state taken` to see in-progress items.

### Phase 3: Trail-Based Development (after TrailTable)

9. **Explore implementation approaches**: When implementing a complex feature, run `crumbs trail start` to create an exploration trail.

10. **Add exploration crumbs**: Create crumbs for the exploration approach: `crumbs add "Try approach A"` then `crumbs trail add-crumb <trail-id> <crumb-id>`.

11. **Abandon failed approaches**: If the approach fails, run `crumbs trail abandon <trail-id>`. All crumbs on the trail are deleted atomically.

12. **Complete successful approaches**: If the approach succeeds, run `crumbs trail complete <trail-id>`. Crumbs become permanent.

### Phase 4: Shared State (after StashTable)

13. **Share context between tasks**: Create a stash for shared configuration: `crumbs stash create --trail <id> --name "build-config" --type context`.

14. **Track artifacts**: When one task produces output another needs, use artifact stashes to track the handoff.

15. **Coordinate with locks**: For operations that should not run concurrently (e.g., database migrations), use lock stashes.

## Architecture Touchpoints

| Interface | Operations Used |
|-----------|-----------------|
| Cupboard | OpenCupboard, Close |
| CrumbTable | Add, Get, Update, SetState, Archive, Fetch |
| PropertyTable | List, SetProperty (via CrumbTable) |
| TrailTable | Start, AddCrumb, Complete, Abandon |
| StashTable | Create, Set, GetValue, Acquire, Release |

## Success Criteria

The use case succeeds when:

- [ ] Crumbs cupboard initialized in crumbs repo (`.crumbs/` directory)
- [ ] All development work tracked via crumbs (not beads)
- [ ] Agent can create, update, and query crumbs via CLI
- [ ] Agent uses trails to explore implementation approaches
- [ ] Abandoned trails clean up atomically (no orphan crumbs)
- [ ] Completed trails merge crumbs to permanent record
- [ ] System remains stable under self-hosting load

Observable demo:

```bash
# Phase 1: Initialize and create work
crumbs init --datadir .crumbs
crumbs add "Implement StashTable.Increment"
crumbs fetch --state draft

# Phase 2: Track progress
crumbs state <id> ready
crumbs state <id> taken
crumbs state <id> completed

# Phase 3: Explore with trails
crumbs trail start
crumbs add "Try optimistic locking"
crumbs trail add-crumb <trail-id> <crumb-id>
# ... approach fails ...
crumbs trail abandon <trail-id>

# Phase 4: Share state
crumbs stash create --name "test-db-path" --type resource
crumbs stash set <stash-id> '{"uri": "file:///tmp/test.db"}'
```

## Out of Scope

This use case does not cover:

- Multi-agent coordination (single agent self-hosting)
- Remote backends (SQLite only for self-hosting)
- Migration tooling from beads to crumbs (manual import)
- CI/CD integration (local development only)
- Performance benchmarking (correctness first)

## Dependencies

- prd-cupboard-core must be implemented (OpenCupboard, Close)
- prd-crumbs-interface must be implemented (CrumbTable operations)
- CLI must expose CrumbTable operations
- For Phase 3: prd-trails-interface must be implemented
- For Phase 4: prd-stash-interface must be implemented

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Bug in crumbs loses work tracking | Keep beads as backup until confidence builds; commit frequently |
| CLI ergonomics block adoption | Iterate on CLI UX based on real usage; add shortcuts for common operations |
| Self-hosting reveals missing features | Treat as validation success; file issues for gaps discovered |
| Circular dependency in debugging | Maintain ability to use beads or manual tracking if crumbs is broken |

## Milestone Marker

This use case serves as a milestone marker. When the crumbs system can successfully track its own development:

1. **MVP Validated**: Core concepts work for real agent workflows
2. **Dogfooding Complete**: We eat our own dog food
3. **Ready for Others**: If it works for us, it can work for other projects
