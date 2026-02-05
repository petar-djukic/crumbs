# Use Case: Self-Hosting (Crumbs Builds Crumbs)

## Summary

A coding agent uses the crumbs system to track the development of crumbs itself. This validates that the system works for real agent workflows and serves as a milestone marker for when crumbs is "ready enough" for production use.

## Actor and Trigger

The actor is a coding agent (e.g., Claude Code) working on the crumbs codebase. The trigger is reaching the minimum viable product: Table interface implemented with Get, Set, Delete, and Fetch operations for crumbs.

## Flow

### Phase 1: Transition from Beads

1. **Verify MVP readiness**: Confirm the Cupboard and Table interfaces work. The cupboard can Attach to an SQLite backend; GetTable("crumbs") returns a Table; the Table supports Get, Set, Delete, and Fetch operations.

2. **Initialize crumbs cupboard**: Attach the cupboard to a local SQLite backend:

```go
cfg := Config{Backend: "sqlite", DataDir: ".crumbs"}
err := cupboard.Attach(cfg)
```

The backend creates the directory, SQLite schema, and seeds built-in properties.

3. **Import existing work**: For any open beads issues, create corresponding crumbs via the Table interface:

```go
crumbsTable, _ := cupboard.GetTable("crumbs")
crumb := &Crumb{Name: "Issue title"}
crumbsTable.Set("", crumb)  // empty ID triggers UUID generation
crumb.SetProperty("priority", int64(2))
crumbsTable.Set(crumb.CrumbID, crumb)
```

4. **Retire beads for this repo**: Once crumbs are tracking all work, stop using `bd` commands for new work in the crumbs repo. Beads remains available for other projects.

### Phase 2: Basic Self-Hosting

5. **Create work items**: When starting new implementation tasks, create crumbs via the ORM pattern:

```go
crumbsTable, _ := cupboard.GetTable("crumbs")
crumb := &Crumb{Name: "Implement PropertyTable.Define"}
crumbsTable.Set("", crumb)  // crumb created in draft state
```

6. **Track progress**: As work progresses, update state via entity methods:

```go
entity, _ := crumbsTable.Get(id)
crumb := entity.(*Crumb)
crumb.SetState("ready")      // ready to implement
crumbsTable.Set(crumb.CrumbID, crumb)

// later...
crumb.SetState("taken")      // actively working
crumbsTable.Set(crumb.CrumbID, crumb)
```

7. **Complete or archive**: When implementation is done, use entity methods:

```go
crumb.Complete()             // transitions to completed
crumbsTable.Set(crumb.CrumbID, crumb)

// or for abandoned work
crumb.Archive()              // transitions to archived
crumbsTable.Set(crumb.CrumbID, crumb)
```

8. **Query work**: Use Table.Fetch with filters to query available work:

```go
readyFilter := map[string]any{"states": []string{"ready"}}
entities, _ := crumbsTable.Fetch(readyFilter)
for _, e := range entities {
    crumb := e.(*Crumb)
    // process ready crumbs
}
```

### Phase 3: Trail-Based Development (after Trail entity)

9. **Explore implementation approaches**: When implementing a complex feature, create an exploration trail:

```go
trailsTable, _ := cupboard.GetTable("trails")
trail := &Trail{ParentCrumbID: nil}
trailsTable.Set("", trail)  // trail created in active state
```

10. **Add exploration crumbs**: Create crumbs and add them to the trail:

```go
crumb := &Crumb{Name: "Try approach A"}
crumbsTable.Set("", crumb)
trail.AddCrumb(cupboard, crumb.CrumbID)
```

11. **Abandon failed approaches**: If the approach fails, abandon the trail. All crumbs on the trail are deleted atomically:

```go
trail.Abandon(cupboard)  // deletes all crumbs, marks trail abandoned
```

12. **Complete successful approaches**: If the approach succeeds, complete the trail. Crumbs become permanent (belongs_to links removed):

```go
trail.Complete(cupboard)  // crumbs become permanent, trail marked completed
```

### Phase 4: Shared State (after Stash entity)

13. **Share context between tasks**: Create a stash for shared configuration via the stashes table.

14. **Track artifacts**: When one task produces output another needs, use artifact stashes to track the handoff.

15. **Coordinate with locks**: For operations that should not run concurrently (e.g., database migrations), use lock stashes.

## Architecture Touchpoints

| Interface | Operations Used |
|-----------|-----------------|
| Cupboard | Attach, Detach, GetTable |
| Table (crumbs) | Get, Set, Delete, Fetch |
| Table (trails) | Get, Set, Delete, Fetch |
| Table (stashes) | Get, Set, Delete, Fetch |
| Crumb entity | SetState, Complete, Archive, Fail, SetProperty, GetProperty |
| Trail entity | AddCrumb, RemoveCrumb, GetCrumbs, Complete, Abandon |

## Success Criteria

The use case succeeds when:

- [ ] Cupboard attaches to SQLite backend (`.crumbs/` directory)
- [ ] All development work tracked via crumbs (not beads)
- [ ] Agent can create, update, and query crumbs via Table interface
- [ ] Agent uses Trail entity methods to explore implementation approaches
- [ ] Abandoned trails delete crumbs atomically (no orphan crumbs)
- [ ] Completed trails make crumbs permanent (remove belongs_to links)
- [ ] System remains stable under self-hosting load

Observable demo:

```go
// Phase 1: Initialize and create work
cfg := Config{Backend: "sqlite", DataDir: ".crumbs"}
cupboard.Attach(cfg)
crumbsTable, _ := cupboard.GetTable("crumbs")
crumb := &Crumb{Name: "Implement Stash operations"}
crumbsTable.Set("", crumb)

// Phase 2: Track progress
entity, _ := crumbsTable.Get(crumb.CrumbID)
crumb = entity.(*Crumb)
crumb.SetState("ready")
crumbsTable.Set(crumb.CrumbID, crumb)
crumb.SetState("taken")
crumbsTable.Set(crumb.CrumbID, crumb)
crumb.Complete()
crumbsTable.Set(crumb.CrumbID, crumb)

// Phase 3: Explore with trails
trailsTable, _ := cupboard.GetTable("trails")
trail := &Trail{}
trailsTable.Set("", trail)
exploreCrumb := &Crumb{Name: "Try optimistic locking"}
crumbsTable.Set("", exploreCrumb)
trail.AddCrumb(cupboard, exploreCrumb.CrumbID)
// ... approach fails ...
trail.Abandon(cupboard)  // deletes exploreCrumb atomically

// Phase 4: Share state (via stashes table)
stashesTable, _ := cupboard.GetTable("stashes")
// ... stash operations ...
```

## Out of Scope

This use case does not cover:

- Multi-agent coordination (single agent self-hosting)
- Remote backends (SQLite only for self-hosting)
- Migration tooling from beads to crumbs (manual import)
- CI/CD integration (local development only)
- Performance benchmarking (correctness first)

## Dependencies

- prd-cupboard-core must be implemented (Cupboard interface: Attach, Detach, GetTable; Table interface: Get, Set, Delete, Fetch)
- prd-crumbs-interface must be implemented (Crumb entity and entity methods)
- For Phase 3: prd-trails-interface must be implemented (Trail entity and entity methods)
- For Phase 4: prd-stash-interface must be implemented (Stash entity)

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
