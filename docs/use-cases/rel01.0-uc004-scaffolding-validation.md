# Use Case: Scaffolding Validation

## Summary

A developer builds the cupboard CLI, runs the version command to confirm the binary works, and verifies that all standard tables and entity types are accessible through the Cupboard and Table interfaces. This is the first use case implemented and validates that the type system, interfaces, and CLI entry point compile and link correctly before any backend behavior is exercised.

## Actor and Trigger

The actor is a developer who has cloned the repository and wants to confirm that the project compiles and the foundation is in place. The trigger is running `go build` on the cupboard CLI and executing the version command.

## Flow

1. **Build the cupboard CLI**: Run `go build ./cmd/cupboard`. The command must complete without errors, producing a `cupboard` binary. This confirms that all packages (`pkg/types`, `internal/sqlite`, `cmd/cupboard`) compile and link.

2. **Run the version command**: Execute `cupboard version`. The command prints the version string (e.g., "cupboard v0.1.0") and exits with code 0. The version command does not require a backend connection; it runs without Attach.

3. **Verify entity structs compile**: The build in step 1 transitively compiles all entity types in `pkg/types`. Each struct must have its documented fields and methods:

   | Entity | Struct | Required fields |
   |--------|--------|-----------------|
   | Crumb | Crumb | CrumbID, Name, State, CreatedAt, UpdatedAt, Properties |
   | Trail | Trail | TrailID, State, CreatedAt, CompletedAt |
   | Property | Property | PropertyID, Name, Description, ValueType, CreatedAt |
   | Category | Category | CategoryID, PropertyID, Name, Ordinal |
   | Stash | Stash | StashID, Name, StashType, Value, Version, CreatedAt |
   | Metadata | Metadata | MetadataID, CrumbID, TableName, Content, PropertyID, CreatedAt |
   | Link | Link | LinkID, LinkType, FromID, ToID, CreatedAt |

4. **Verify Table interface compiles**: The Table interface must define Get, Set, Delete, and Fetch methods. A compile-time assertion (e.g., assigning a concrete type to the interface) confirms the contract is met.

5. **Verify Cupboard interface compiles**: The Cupboard interface must define GetTable, Attach, and Detach methods. The SQLite backend must satisfy this interface at compile time.

6. **Verify standard table names are defined**: The constants CrumbsTable, TrailsTable, PropertiesTable, MetadataTable, LinksTable, and StashesTable must exist in `pkg/types` and resolve to the expected string values ("crumbs", "trails", "properties", "metadata", "links", "stashes").

7. **Attach and enumerate tables**: Create a Cupboard, attach with a temporary directory, and call `GetTable` for each of the six standard table names. Each call must return a non-nil Table without error. Detach afterward.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Component | Operations Used |
|-----------|-----------------|
| Cupboard interface | Attach, GetTable, Detach |
| Table interface | Compile-time verification of Get, Set, Delete, Fetch signatures |
| Entity types (all 7) | Compile-time verification of struct fields |
| SQLite backend | NewBackend, Attach (schema creation), Detach |
| CLI (cmd/cupboard) | `version` command |

We validate:

- All packages compile and link without errors
- Cupboard and Table interfaces are implemented by the SQLite backend (prd-cupboard-core R2)
- All entity structs have the fields specified in their PRDs (prd-crumbs-interface R1, prd-trails-interface, prd-properties-interface, prd-stash-interface, prd-metadata-interface, prd-sqlite-backend)
- GetTable returns a Table for all six standard table names (prd-cupboard-core R2.5)
- The version command works without a backend connection

## Success Criteria

The use case succeeds when:

- [ ] `go build ./cmd/cupboard` completes without errors
- [ ] `cupboard version` prints a version string and exits with code 0
- [ ] All seven entity structs (Crumb, Trail, Property, Category, Stash, Metadata, Link) compile with documented fields
- [ ] Table interface compiles with Get, Set, Delete, Fetch methods
- [ ] Cupboard interface compiles with GetTable, Attach, Detach methods
- [ ] SQLite backend satisfies the Cupboard interface at compile time
- [ ] Standard table name constants are defined for all six tables
- [ ] GetTable succeeds for "crumbs", "trails", "properties", "metadata", "links", "stashes"
- [ ] GetTable for an unknown table name returns ErrTableNotFound

Observable demo:

```bash
# Build
go build -o cupboard ./cmd/cupboard

# Version
./cupboard version
# Output: cupboard v0.1.0
```

```go
// Enumerate all tables
cupboard := sqlite.NewBackend()
cfg := Config{Backend: "sqlite", DataDir: tmpDir}
err := cupboard.Attach(cfg)

tables := []string{"crumbs", "trails", "properties", "metadata", "links", "stashes"}
for _, name := range tables {
    tbl, err := cupboard.GetTable(name)
    // err must be nil, tbl must be non-nil
}

// Unknown table
_, err = cupboard.GetTable("nonexistent")
// err must be ErrTableNotFound

cupboard.Detach()
```

## Out of Scope

This use case does not cover:

- Creating, retrieving, or deleting entities (see rel01.0-uc002 and rel01.0-uc003)
- State transitions or entity methods beyond compile-time existence
- JSONL persistence or file creation details (see rel01.0-uc002)
- Property enforcement or backfill (see rel02.0-uc001)
- Trail lifecycle operations (see rel03.0-uc001)

## Dependencies

- prd-cupboard-core: Cupboard and Table interface definitions
- prd-sqlite-backend: Backend implementation of Cupboard interface
- prd-crumbs-interface, prd-trails-interface, prd-properties-interface, prd-metadata-interface, prd-stash-interface: Entity struct definitions

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Struct fields change as PRDs evolve | Test against the fields listed in the current PRDs; update the use case when PRDs change |
| Backend implementation lags behind interface | Compile-time interface satisfaction catches mismatches immediately |
| Version command output format changes | Test for exit code 0 and non-empty output; exact string is secondary |
