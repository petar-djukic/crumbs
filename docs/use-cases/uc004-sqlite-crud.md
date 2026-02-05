# Use Case: SQLite Backend CRUD Operations

## Summary

A developer attaches a SQLite backend to a cupboard, creates and retrieves crumbs using the Table interface, modifies crumbs via entity methods, saves changes, and deletes a crumb. This tracer bullet validates the ORM pattern where entities are retrieved, modified in memory, and persisted back through the uniform Table interface.

## Actor and Trigger

The actor is a developer or agent integrating with the crumbs system using a SQLite backend. The trigger is the need to create, read, update, and delete crumbs while verifying that all changes persist to JSON files and are queryable via SQLite.

## Flow

1. **Create cupboard and attach backend**: Construct a Cupboard instance and call `Attach(config)` with a Config specifying DataDir. The SQLite backend creates the directory, initializes empty JSON files (crumbs.json, trails.json, links.json, etc.), creates cupboard.db with the schema, and seeds built-in properties.

2. **Get the crumbs table**: Call `cupboard.GetTable("crumbs")` to obtain a Table reference. The returned Table provides Get, Set, Delete, and Fetch operations for crumbs.

3. **Create a crumb**: Construct a new Crumb struct with a Name and call `table.Set("", crumb)`. The backend generates a UUID v7, sets State to "draft", initializes timestamps, populates the Properties map with defaults, inserts into SQLite, and persists to crumbs.json.

4. **Retrieve the crumb**: Call `table.Get(crumbID)` with the generated ID. The backend queries SQLite, hydrates the row into a Crumb struct, and returns it. Cast the result to `*Crumb`.

5. **Modify via entity methods**: Call `crumb.SetState("ready")` to transition the crumb to the ready state. The entity method updates the State field and UpdatedAt timestamp in memory.

6. **Save the crumb**: Call `table.Set(crumbID, crumb)` to persist the modified crumb. The backend updates the SQLite row and writes the change to crumbs.json atomically.

7. **Verify persistence**: Call `table.Fetch(map[string]any{"states": []string{"ready"}})` to query crumbs by state. Confirm the crumb appears in results. Optionally inspect crumbs.json to verify the state change is persisted.

8. **Delete the crumb**: Call `table.Delete(crumbID)` to remove the crumb. The backend deletes the SQLite row, removes associated data (property values, metadata, links), and updates crumbs.json.

9. **Detach the backend**: Call `cupboard.Detach()` to close the SQLite connection. Subsequent calls to GetTable or table operations return ErrCupboardDetached.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Component | Operations Used |
|-----------|-----------------|
| Cupboard | Attach, GetTable, Detach |
| Table | Get, Set, Delete, Fetch |
| Crumb entity | SetState (entity method) |
| SQLite backend | Schema creation, JSON persistence, hydration, dehydration |

We validate:

- Cupboard lifecycle: Attach creates DataDir and initializes files; Detach closes resources (prd-cupboard-core R4, R5)
- Table routing: GetTable("crumbs") returns the crumbs table accessor (prd-sqlite-backend R12)
- Entity creation: Set with empty ID generates UUID v7, initializes state and timestamps (prd-crumbs-interface R3)
- Entity hydration: Get retrieves and hydrates a Crumb from SQLite (prd-sqlite-backend R14)
- ORM pattern: Get entity, modify with entity methods, Set to persist (prd-crumbs-interface R4, R7)
- JSON persistence: All write operations persist atomically to JSON files (prd-sqlite-backend R5)
- Entity deletion: Delete removes entity and cascades to associated data (prd-crumbs-interface R8)

## Success Criteria

The demo succeeds when:

- [ ] Attach creates DataDir with crumbs.json, cupboard.db, and other required files
- [ ] GetTable("crumbs") returns a Table without error
- [ ] Set("", crumb) generates a UUID v7 and persists to crumbs.json
- [ ] Get(crumbID) returns the crumb with correct fields
- [ ] Entity method SetState("ready") updates State and UpdatedAt in memory
- [ ] Set(crumbID, crumb) persists the state change to both SQLite and crumbs.json
- [ ] Fetch with state filter returns only matching crumbs
- [ ] Delete(crumbID) removes the crumb from SQLite and crumbs.json
- [ ] Detach closes resources; subsequent operations return ErrCupboardDetached
- [ ] crumbs.json is human-readable and reflects all operations

Observable demo:

```go
// Attach
cupboard := NewCupboard()
config := Config{DataDir: "/tmp/crumbs-demo", Backend: "sqlite"}
err := cupboard.Attach(config)

// Get table
table, err := cupboard.GetTable("crumbs")

// Create
crumb := &Crumb{Name: "Implement feature X"}
err = table.Set("", crumb)
fmt.Println("Created:", crumb.CrumbID)

// Retrieve
entity, err := table.Get(crumb.CrumbID)
retrieved := entity.(*Crumb)

// Modify and save
retrieved.SetState("ready")
err = table.Set(retrieved.CrumbID, retrieved)

// Query
results, err := table.Fetch(map[string]any{"states": []string{"ready"}})
fmt.Println("Found:", len(results), "ready crumbs")

// Delete
err = table.Delete(crumb.CrumbID)

// Detach
err = cupboard.Detach()
```

## Out of Scope

This use case does not cover:

- Trail operations (creating trails, assigning crumbs to trails, belongs_to links)
- Property operations (defining properties, setting property values, backfill)
- Stash operations (creating stashes, versioning, history)
- Metadata operations (comments, attachments)
- Link operations (child_of relationships, DAG traversal)
- Concurrent access patterns (multiple readers, writer locking)
- Error recovery (corrupt JSON, I/O failures, schema migration)

These are addressed in later use cases (uc005-trails, uc006-properties, uc007-stashes).

## Dependencies

- prd-cupboard-core: Cupboard interface with Attach/Detach/GetTable
- prd-crumbs-interface: Crumb entity struct and methods
- prd-sqlite-backend: SQLite schema, JSON persistence, hydration/dehydration

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| JSON file corruption on crash | Atomic write (temp file + rename) per prd-sqlite-backend R5.2 |
| Type assertion fails on Get | Document that callers must type-assert; consider generic Table[T] in future |
| UUID v7 collision | Timestamp + random bits make collision negligible; no mitigation needed |
