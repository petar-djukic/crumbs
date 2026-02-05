# Use Case: Trail-Based Exploration

## Summary

A developer creates a trail to explore an implementation approach, adds crumbs to the trail, and either completes or abandons the trail. This tracer bullet validates the trail lifecycle and the atomic cleanup behavior when abandoning exploratory work.

## Actor and Trigger

The actor is a coding agent or developer exploring alternative implementation approaches. The trigger is uncertainty about the best approach to a task, requiring exploratory work that may be discarded.

## Flow

1. **Open cupboard**: Call `OpenCupboard` with a SQLite backend. Ensure the cupboard is attached and tables are accessible.

2. **Create a parent crumb (optional)**: Call `Crumbs().Add("Implement caching layer")` to create a task that the trail will explore approaches for. This is optional; trails can exist without a parent.

3. **Create a trail**: Call `Trails().Add(parentCrumbID)` or `Trails().Add(nil)` to create a new trail. The trail starts in "active" state.

4. **Verify trail state**: Call `Trails().Get(trailID)`. Confirm the trail exists with state "active" and the correct parent (or nil).

5. **Add exploration crumbs**: Create crumbs for the exploration:
   - Call `Crumbs().Add("Try approach A: in-memory cache")`
   - Call `Trails().AddCrumb(trailID, crumbID)` to associate the crumb with the trail

6. **Add more crumbs to trail**: Repeat step 5 for additional exploration:
   - `Crumbs().Add("Try approach B: Redis cache")`
   - `Trails().AddCrumb(trailID, crumbID)`

7. **List crumbs on trail**: Call `Trails().GetCrumbs(trailID)`. Verify both exploration crumbs are returned.

8. **Remove a crumb from trail**: Call `Trails().RemoveCrumb(trailID, crumbID)` to remove approach B. The crumb is removed from the trail but not deleted.

9. **Abandon the trail**: Call `Trails().Abandon(trailID)`. The operation:
   - Changes trail state to "abandoned"
   - Deletes all crumbs that belong exclusively to this trail
   - Leaves crumbs that were removed (step 8) intact

10. **Verify cleanup**: Call `Crumbs().Get()` for the deleted crumb IDs. Verify they return ErrNotFound. The removed crumb (approach B) should still exist.

11. **Create another trail and complete it**: Create a new trail, add crumbs, then call `Trails().Complete(trailID)`. The operation:
    - Changes trail state to "completed"
    - Removes belongs_to links (crumbs become permanent)
    - Crumbs remain in the database

12. **Verify completion**: Crumbs from the completed trail should still exist and be queryable via `Crumbs().Fetch()`.

13. **Filter crumbs by trail**: Call `Crumbs().Fetch({"trail_id": trailID})`. Verify only crumbs associated with that trail are returned.

14. **Close the cupboard**: Call `Close()`.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Interface | Operations Used |
|-----------|-----------------|
| Cupboard | OpenCupboard, Close |
| CrumbTable | Add, Get, Fetch (with trail filter) |
| TrailTable | Add, Get, AddCrumb, RemoveCrumb, GetCrumbs, Complete, Abandon |

We validate:

- Trail creation with optional parent crumb (prd-trails-interface R1, R2)
- Adding and removing crumbs from trails (prd-trails-interface R3, R4)
- Trail completion makes crumbs permanent (prd-trails-interface R5)
- Trail abandonment deletes associated crumbs atomically (prd-trails-interface R6)
- Trail filtering in Fetch (prd-crumbs-interface R8)
- belongs_to link management (prd-links-interface)

## Success Criteria

The demo succeeds when:

- [ ] Trails().Add() creates a trail in "active" state
- [ ] Trails().AddCrumb() associates a crumb with the trail
- [ ] Trails().GetCrumbs() returns all crumbs on the trail
- [ ] Trails().RemoveCrumb() disassociates without deleting the crumb
- [ ] Trails().Abandon() deletes trail's crumbs atomically
- [ ] Removed crumbs survive trail abandonment
- [ ] Trails().Complete() makes crumbs permanent (removes belongs_to)
- [ ] Completed trail's crumbs remain queryable
- [ ] Fetch with trail_id filter returns only that trail's crumbs
- [ ] No orphan crumbs or links after abandonment

Observable demo script:

```bash
# Run the demo binary or test
go test -v ./internal/sqlite -run TestTrailExploration

# Or run a CLI demo
crumbs demo trails --datadir /tmp/crumbs-demo
```

## Out of Scope

This use case does not cover:

- Nested trails (trails containing trails)
- Trail merging or splitting
- Trail history or versioning
- Stash operations - see rel03.0-uc002 (if created)
- Concurrent trail operations
- Trail templates or predefined workflows

## Dependencies

- rel01.0-uc001 (Cupboard lifecycle) must pass
- rel01.0-uc003 (Core CRUD) must pass
- prd-trails-interface must be implemented
- prd-links-interface (belongs_to) must be implemented

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Orphan crumbs after failed abandonment | Use transaction; rollback on failure |
| belongs_to links not cleaned up | Verify link cleanup in tests |
| Performance with large trails | Test with 100+ crumbs on a single trail |
| Race condition on concurrent trail ops | Document single-writer expectation for trails |
