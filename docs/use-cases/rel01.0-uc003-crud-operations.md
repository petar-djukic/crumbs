# Use Case: Core CRUD Operations

## Summary

A developer creates a SQLite-backed cupboard, adds crumbs with various states, queries and filters crumbs, and cleans up the database. This tracer bullet validates the core CRUD operations across the Cupboard and CrumbTable interfaces without property enforcement.

## Actor and Trigger

The actor is a developer or automated test harness. The trigger is the need to validate that the crumbs system correctly handles the full lifecycle of crumbs: creation, retrieval, state transitions, filtering, archiving, and purging.

## Flow

1. **Create the database**: Call `OpenCupboard` with a SQLite backend configuration specifying a DataDir. The backend creates the directory, initializes empty JSON files, and creates the SQLite schema.

2. **Add first crumb**: Call `Crumbs().Add("Implement login feature")`. The operation generates a UUID v7, sets state to "draft", and initializes CreatedAt and UpdatedAt timestamps.

3. **Retrieve the crumb**: Call `Crumbs().Get(crumbID)` to retrieve the crumb. Verify all fields are populated correctly.

4. **Change crumb state**: Use a state-change operation to transition from "draft" to "ready". Verify UpdatedAt changes.

5. **Add second crumb**: Call `Crumbs().Add("Fix authentication bug")`. Verify it is created with state "draft".

6. **Fetch all crumbs**: Call `Crumbs().Fetch(nil)` or `Crumbs().Fetch(map[string]any{})`. Verify both crumbs are returned.

7. **Fetch with filter**: Call `Crumbs().Fetch({"states": ["ready"]})`. Verify only the first crumb (state "ready") is returned.

8. **Dust a crumb**: Call `crumb.Dust()` to mark as failed/abandoned. Verify the crumb's state becomes "dust" and UpdatedAt changes.

9. **Fetch excludes dust**: Call `Crumbs().Fetch({"states": ["draft", "ready"]})`. Verify the dust crumb is not returned.

10. **Delete dust crumb**: Call `Table.Delete(secondCrumbID)`. Verify the crumb is permanently removed.

11. **Close the cupboard**: Call `Close()`. Verify all resources are released and subsequent operations return ErrCupboardClosed.

12. **Delete the database**: Remove the DataDir to clean up. Verify the directory and all JSON files are gone.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Interface | Operations Used |
|-----------|-----------------|
| Cupboard | OpenCupboard, Close |
| CrumbTable | Add, Get, Dust, Delete, Fetch |

We validate:

- SQLite backend initialization and JSON file creation (prd-sqlite-backend R1, R4)
- Crumb creation with UUID v7 and timestamp initialization (prd-crumbs-interface R3)
- State transitions and dust behavior (prd-crumbs-interface R5)
- Filter-based queries (prd-crumbs-interface R7, R8)
- Purge cascade behavior (prd-crumbs-interface R6)
- Cupboard lifecycle and ErrCupboardClosed (prd-cupboard-core R4, R5)

## Success Criteria

The demo succeeds when:

- [ ] OpenCupboard creates DataDir with all JSON files and cupboard.db
- [ ] Newly added crumbs have UUID v7, state "draft", and timestamps set
- [ ] Get returns the crumb with all fields populated
- [ ] State transitions update the state field and UpdatedAt
- [ ] Fetch returns all crumbs when no filter is applied
- [ ] Fetch correctly filters by state
- [ ] Dust changes state to "dust" without deleting data
- [ ] Delete removes crumb permanently
- [ ] Close prevents further operations with ErrCupboardClosed
- [ ] DataDir removal cleans up all files

Observable demo script:

```bash
# Run the demo binary or test
go test -v ./internal/sqlite -run TestCoreCRUDOperations

# Or run a CLI demo
crumbs demo crud --datadir /tmp/crumbs-demo
```

## Out of Scope

This use case does not cover:

- Property operations (Define, SetProperty, GetProperties, ClearProperty) - see rel02.0-uc001
- Trail operations (Start, Complete, Abandon, GetCrumbs) - see rel03.0-uc001
- Metadata operations (Register, Add, Get, Search)
- Link operations beyond what Purge removes
- Concurrent access patterns
- Error recovery scenarios (corrupt JSON, I/O failures)
- Dolt or DynamoDB backends - see rel04.0-uc001

## Dependencies

- prd-cupboard-core must be implemented (OpenCupboard, Close)
- prd-crumbs-interface must be implemented (CrumbTable operations)
- prd-sqlite-backend must be implemented (JSON persistence, schema)

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| JSON file corruption on crash | Atomic write (temp file + rename) per prd-sqlite-backend R5.2 |
| State transition validation | Test invalid transitions return appropriate errors |
| Filter edge cases (empty, invalid) | Explicit test cases for edge cases |
