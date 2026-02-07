# Use Case: Crumb Entity Operations

## Summary

A developer creates crumbs and exercises the full crumb state machine, transitioning crumbs through `draft → pending → ready → taken → pebble` (success) and `draft → dust` (failure). This tracer bullet validates the Crumb entity methods (SetState, Pebble, Dust), timestamp behavior on state transitions, and state-based filtering. The focus is on crumbs-specific behavior, not on the generic Table interface (see rel01.0-uc002).

## Actor and Trigger

The actor is a developer or agent managing work items. The trigger is the need to verify that crumbs follow the documented state machine and that entity methods correctly modify state, update timestamps, and interact with Table persistence.

## Flow

1. **Setup**: Attach a Cupboard with SQLite backend and obtain the crumbs table via `cupboard.GetTable("crumbs")`.

2. **Create crumb in draft state**: Call `table.Set("", &Crumb{Name: "Explore approach A"})`. The backend generates a UUID v7 and initializes the crumb with State "draft", CreatedAt, UpdatedAt, and an empty Properties map.

3. **Verify initial state**: Retrieve the crumb via `table.Get(id)`. Confirm State is "draft", CreatedAt and UpdatedAt are set, and Properties is an initialized (possibly empty) map.

4. **Transition draft to pending**: Call `crumb.SetState("pending")`, then `table.Set(id, crumb)`. Retrieve and confirm State is "pending" and UpdatedAt has advanced.

5. **Transition pending to ready**: Call `crumb.SetState("ready")`, then `table.Set(id, crumb)`. Retrieve and confirm State is "ready".

6. **Transition ready to taken**: Call `crumb.SetState("taken")`, then `table.Set(id, crumb)`. Retrieve and confirm State is "taken".

7. **Complete via Pebble**: Call `crumb.Pebble()`, then `table.Set(id, crumb)`. Retrieve and confirm State is "pebble" (terminal success). UpdatedAt must reflect the transition time.

8. **Create a second crumb for the failure path**: Call `table.Set("", &Crumb{Name: "Try approach B"})`. Verify State is "draft".

9. **Fail via Dust**: Call `crumb2.Dust()`, then `table.Set(id2, crumb2)`. Retrieve and confirm State is "dust" (terminal failure).

10. **Filter by state**: Call `table.Fetch(map[string]any{"states": []string{"draft"}})`. Verify the result excludes both the pebble crumb and the dust crumb.

11. **Filter for terminal states**: Call `table.Fetch(map[string]any{"states": []string{"pebble"}})`. Verify only the completed crumb is returned. Repeat for "dust".

12. **Create crumb and transition to dust from any non-terminal state**: Create a third crumb, transition to "ready", then call `Dust()`. Verify the crumb reaches "dust" regardless of its previous state.

13. **Verify UpdatedAt tracks transitions**: For each state transition in the flow, confirm that UpdatedAt changed and is equal to or later than the previous value. CreatedAt must remain unchanged across all transitions.

14. **Detach**: Call `cupboard.Detach()`.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Component | Operations Used |
| --------- | --------------- |
| Table | Get, Set, Fetch (for persistence and retrieval) |
| Crumb entity | SetState, Pebble, Dust |
| Crumb state machine | draft → pending → ready → taken → pebble; any → dust |

We validate:

- Crumb creation defaults to "draft" state with initialized timestamps (prd-crumbs-interface R1, R3)
- SetState transitions: draft → pending → ready → taken (prd-crumbs-interface R2, R4)
- Pebble() transitions taken → pebble as terminal success (prd-crumbs-interface R2)
- Dust() transitions any non-terminal state → dust as terminal failure (prd-crumbs-interface R2, R5)
- UpdatedAt advances on every state transition; CreatedAt remains constant (prd-crumbs-interface R3)
- State-based Fetch filtering correctly includes and excludes crumbs by state (prd-crumbs-interface R9, R10)

## Success Criteria

The use case succeeds when:

- [ ] New crumb starts in "draft" state with CreatedAt and UpdatedAt set
- [ ] SetState("pending") transitions draft → pending
- [ ] SetState("ready") transitions pending → ready
- [ ] SetState("taken") transitions ready → taken
- [ ] Pebble() transitions taken → pebble (terminal)
- [ ] Dust() transitions draft → dust (terminal)
- [ ] Dust() works from any non-terminal state (tested from "ready")
- [ ] UpdatedAt advances on each transition; CreatedAt stays constant
- [ ] Fetch by state returns only crumbs in the requested state
- [ ] Fetch excludes pebble and dust crumbs when filtering for non-terminal states
- [ ] Properties map is initialized on creation

Observable demo:

```go
table, _ := cupboard.GetTable("crumbs")

// Success path: draft → pending → ready → taken → pebble
crumb := &Crumb{Name: "Explore approach A"}
id, _ := table.Set("", crumb)

entity, _ := table.Get(id)
c := entity.(*Crumb)
// c.State == "draft"

c.SetState("pending")
table.Set(id, c)

c.SetState("ready")
table.Set(id, c)

c.SetState("taken")
table.Set(id, c)

c.Pebble()
table.Set(id, c)
// c.State == "pebble"

// Failure path: draft → dust
crumb2 := &Crumb{Name: "Try approach B"}
id2, _ := table.Set("", crumb2)

entity2, _ := table.Get(id2)
c2 := entity2.(*Crumb)
c2.Dust()
table.Set(id2, c2)
// c2.State == "dust"

// Filter: only draft crumbs (excludes pebble and dust)
drafts, _ := table.Fetch(map[string]any{"states": []string{"draft"}})
// len(drafts) == 0
```

## Out of Scope

This use case does not cover:

- Generic Table CRUD operations (create, update, delete, fetch) — see rel01.0-uc002
- Property operations (SetProperty, GetProperty, ClearProperty) — see rel02.0-uc001
- Trail membership (belongs_to links) — see rel03.0-uc001
- Crumb deletion (Table.Delete) — see rel01.0-uc002
- Invalid state transition rejection (not enforced by the Table; agents define rules)
- Concurrent state transitions

## Dependencies

- rel01.0-uc002: Table CRUD must work (Set, Get, Fetch used throughout)
- prd-crumbs-interface: Crumb entity, state machine, entity methods
- prd-sqlite-backend: Persistence of state changes to JSONL

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| State transition rules change | Test against the state machine defined in prd-crumbs-interface; update when PRD changes |
| Dust from unexpected states | Test Dust from multiple starting states to ensure it works universally |
| Timestamp precision | Use time comparison with tolerance; avoid exact equality checks |
