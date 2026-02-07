# Use Case: Table Interface CRUD Operations

## Summary

A developer attaches a SQLite backend, obtains a Table reference, and exercises the four uniform Table operations (Get, Set, Delete, Fetch) to create, retrieve, update, and remove entities. This tracer bullet validates the ORM-style pattern where callers create entity structs, persist them through `Table.Set`, retrieve them with `Table.Get`, query with `Table.Fetch`, and remove with `Table.Delete`. The focus is on the Table interface contract, not on any entity-specific behavior.

## Actor and Trigger

The actor is a developer or agent working through the Cupboard API. The trigger is the need to verify that the Table interface provides correct CRUD behavior across all standard tables, with JSONL persistence and UUID v7 generation.

## Flow

1. **Attach backend**: Construct a Cupboard, call `Attach(config)` with a SQLite backend configuration. The backend creates the DataDir, initializes JSONL files for each table, creates `cupboard.db`, and seeds built-in data.

2. **Get a table**: Call `cupboard.GetTable("crumbs")` to obtain a Table. The returned Table provides Get, Set, Delete, and Fetch operations. The same interface is returned for any standard table name.

3. **Create an entity (Set with empty ID)**: Construct a Crumb struct and call `table.Set("", crumb)`. The backend generates a UUID v7, initializes timestamps, inserts the entity into SQLite, and persists to JSONL. The returned string is the generated ID.

4. **Retrieve the entity (Get)**: Call `table.Get(id)` with the generated ID. The backend queries SQLite, hydrates the row into the entity struct, and returns it as `any`. The caller type-asserts to `*Crumb`.

5. **Verify round-trip fidelity**: Compare the retrieved entity fields against what was created. The CrumbID, Name, State, CreatedAt, and UpdatedAt fields must match.

6. **Update the entity (Set with existing ID)**: Modify a field on the struct (e.g., change Name) and call `table.Set(id, crumb)`. The backend updates the SQLite row and writes the change to JSONL. The returned ID matches the input ID.

7. **Verify the update persists**: Call `table.Get(id)` again. The modified field must reflect the change. UpdatedAt must be equal to or later than the previous value.

8. **Create a second entity**: Call `table.Set("", secondCrumb)` to create another entity. The generated ID must differ from the first.

9. **Fetch all entities**: Call `table.Fetch(map[string]any{})` with an empty filter. The result must contain both entities.

10. **Fetch with filter**: Call `table.Fetch(map[string]any{"states": []string{"draft"}})`. The result must include only entities matching the filter.

11. **Delete an entity**: Call `table.Delete(id)`. The backend removes the entity from SQLite and JSONL. A subsequent `table.Get(id)` must return an error.

12. **Get nonexistent entity**: Call `table.Get("nonexistent-id")`. The operation must return an error indicating the entity was not found.

13. **Delete nonexistent entity**: Call `table.Delete("nonexistent-id")`. The operation must return an error.

14. **Verify JSONL persistence**: Inspect the JSONL file (e.g., `crumbs.jsonl`) to confirm that create, update, and delete operations are reflected. Each line is a valid JSON object. The file is human-readable.

15. **Repeat on another table**: Call `cupboard.GetTable("trails")`, create a Trail, retrieve it, and delete it. The same Table interface contract holds across entity types.

16. **Detach**: Call `cupboard.Detach()` to release resources. Subsequent Table operations return `ErrCupboardDetached`.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Component | Operations Used |
|-----------|-----------------|
| Cupboard | Attach, GetTable, Detach |
| Table | Get, Set, Delete, Fetch |
| SQLite backend | UUID v7 generation, hydration, dehydration, JSONL persistence |

We validate:

- Table.Set with empty ID generates UUID v7 and creates entity (prd-cupboard-core R2, prd-sqlite-backend R5)
- Table.Get retrieves and hydrates entity from SQLite (prd-sqlite-backend R14)
- Table.Set with existing ID updates entity and persists to JSONL (prd-sqlite-backend R5)
- Table.Fetch returns matching entities; empty filter returns all (prd-cupboard-core R2)
- Table.Delete removes entity from SQLite and JSONL (prd-sqlite-backend R5)
- Error handling: Get and Delete on nonexistent ID return errors
- The same Table interface works across different entity types (crumbs, trails, etc.)
- JSONL files are human-readable and reflect all write operations (prd-sqlite-backend R5, R16)

## Success Criteria

The use case succeeds when:

- [ ] Set("", entity) returns a UUID v7 and persists to JSONL
- [ ] Get(id) returns the entity with all fields matching what was created
- [ ] Set(id, entity) updates the entity; Get confirms the change
- [ ] Fetch with empty filter returns all entities in the table
- [ ] Fetch with filter returns only matching entities
- [ ] Delete(id) removes the entity; subsequent Get returns an error
- [ ] Get and Delete on nonexistent IDs return errors
- [ ] The same operations work on the crumbs table and the trails table
- [ ] JSONL file reflects all create, update, and delete operations
- [ ] Detach prevents further operations with ErrCupboardDetached

Observable demo:

```go
// Attach
cupboard := sqlite.NewBackend()
cfg := Config{Backend: "sqlite", DataDir: tmpDir}
cupboard.Attach(cfg)

// Get table
table, _ := cupboard.GetTable("crumbs")

// Create
crumb := &Crumb{Name: "Implement feature X"}
id, _ := table.Set("", crumb)

// Retrieve
entity, _ := table.Get(id)
retrieved := entity.(*Crumb)

// Update
retrieved.Name = "Implement feature X (revised)"
table.Set(id, retrieved)

// Fetch all
all, _ := table.Fetch(map[string]any{})
// len(all) == 1

// Delete
table.Delete(id)

// Get after delete returns error
_, err := table.Get(id)
// err != nil

// Same pattern on trails table
trails, _ := cupboard.GetTable("trails")
trail := &Trail{State: "active"}
tid, _ := trails.Set("", trail)
trails.Get(tid)
trails.Delete(tid)

cupboard.Detach()
```

## Out of Scope

This use case does not cover:

- Crumb state machine transitions (SetState, Pebble, Dust) — see rel01.0-uc003
- Entity method behavior beyond field assignment — see rel01.0-uc003
- Property operations (SetProperty, GetProperty, ClearProperty) — see rel02.0-uc001
- Trail lifecycle (Complete, Abandon, cascade behavior) — see rel03.0-uc001
- Concurrent access patterns
- Error recovery (corrupt JSONL, I/O failures)

## Dependencies

- prd-cupboard-core: Cupboard and Table interface definitions
- prd-sqlite-backend: SQLite backend, JSONL persistence, hydration/dehydration
- prd-crumbs-interface: Crumb entity struct (used as example entity)
- prd-trails-interface: Trail entity struct (used to verify cross-table behavior)

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| JSONL file corruption on crash | Atomic write (temp file + rename) per prd-sqlite-backend R5.2 |
| Type assertion fails on Get | Callers must type-assert; document the pattern |
| UUID v7 collision | Timestamp + random bits make collision negligible |
