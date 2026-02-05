# Use Case: Configuration and Cupboard Lifecycle

## Summary

An application initializes a Cupboard with a typed Config struct, attaches to a SQLite backend, accesses tables for data operations, and detaches to release resources. This tracer bullet validates the configuration workflow and cupboard lifecycle management defined in prd-cupboard-core.

## Actor and Trigger

The actor is a developer integrating the Crumbs library into their application. The trigger is application startup, where the application must load configuration, establish a backend connection, and prepare for data operations.

## Flow

1. **Create configuration**: Construct a Config struct specifying Backend as "sqlite" and DataDir as the path to the data directory. Leave DoltConfig and DynamoDBConfig nil since we use SQLite.

2. **Validate configuration**: The application may optionally validate the Config before calling Attach. Invalid configurations (empty Backend, unrecognized Backend, missing required backend-specific config) will fail at Attach time with a descriptive error.

3. **Create Cupboard instance**: Instantiate the Cupboard. At this point the cupboard is not attached to any backend.

4. **Attach to backend**: Call `Attach(config)` on the Cupboard. The operation validates the config, creates the DataDir if needed, initializes the SQLite database schema, and seeds built-in data. After Attach returns successfully, the cupboard is ready for use.

5. **Handle attach errors**: If Attach fails (invalid config, I/O error, database corruption), log the error and exit or retry with corrected configuration. If Attach is called on an already-attached cupboard, it returns ErrAlreadyAttached.

6. **Access tables**: Call `GetTable("crumbs")` to obtain the CrumbTable. Call `GetTable("properties")` for PropertyTable. Each GetTable call returns a Table interface ready for Get, Set, Delete, and Fetch operations. Calling GetTable with an unknown name returns ErrTableNotFound.

7. **Verify connection**: Perform a simple operation to confirm the backend is working. For example, call `Fetch(map[string]any{})` on the crumbs table to list all crumbs (initially empty). A successful response confirms the connection.

8. **Perform data operations**: Use the tables for normal application work. All CRUD operations (Get, Set, Delete, Fetch) are available. Entity IDs are UUID v7, generated on Set when the ID field is empty.

9. **Detach from backend**: When the application shuts down or needs to release resources, call `Detach()`. The operation blocks until in-flight operations complete, then closes database connections and file handles.

10. **Verify detach**: After Detach, any Cupboard operation (GetTable, Attach on a new config) returns ErrCupboardDetached. The application must create a new Cupboard instance if it needs to reconnect.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Interface/Component | Operations Used |
|---------------------|-----------------|
| Config | Struct construction with Backend, DataDir fields |
| Cupboard | Attach, Detach, GetTable |
| Table | Fetch (for connection verification) |

The use case validates:

- Config struct construction and field assignment (prd-cupboard-core R1)
- Cupboard interface contract (prd-cupboard-core R2)
- Attach idempotency and validation (prd-cupboard-core R4)
- Detach resource release and idempotency (prd-cupboard-core R5)
- Error handling after detach (prd-cupboard-core R6)
- Standard error types: ErrAlreadyAttached, ErrCupboardDetached, ErrTableNotFound (prd-cupboard-core R7)

## Success Criteria

The demo succeeds when:

- [ ] Config struct is created with Backend="sqlite" and valid DataDir
- [ ] Attach(config) returns nil and initializes the backend
- [ ] Attach on an already-attached cupboard returns ErrAlreadyAttached
- [ ] GetTable("crumbs") returns a valid Table interface
- [ ] GetTable("unknown") returns ErrTableNotFound
- [ ] Fetch on a table returns an empty slice (or populated data if present)
- [ ] Detach() returns nil and releases all resources
- [ ] Detach on an already-detached cupboard returns nil (idempotent)
- [ ] GetTable after Detach returns ErrCupboardDetached

Observable demo script:

```go
// 1. Create config
cfg := Config{
    Backend: "sqlite",
    DataDir: "/tmp/crumbs-demo",
}

// 2. Create and attach cupboard
cupboard := NewCupboard()
err := cupboard.Attach(cfg)
if err != nil {
    log.Fatal("attach failed:", err)
}

// 3. Access tables
crumbs, err := cupboard.GetTable("crumbs")
if err != nil {
    log.Fatal("get table failed:", err)
}

// 4. Verify connection
items, err := crumbs.Fetch(map[string]any{})
fmt.Printf("Found %d crumbs\n", len(items))

// 5. Detach
err = cupboard.Detach()
if err != nil {
    log.Fatal("detach failed:", err)
}

// 6. Verify operations fail after detach
_, err = cupboard.GetTable("crumbs")
if errors.Is(err, ErrCupboardDetached) {
    fmt.Println("Correctly returned ErrCupboardDetached")
}
```

## Out of Scope

This use case does not cover:

- Dolt backend configuration (DoltConfig with DSN and Branch)
- DynamoDB backend configuration (DynamoDBConfig with TableName and Region)
- Multi-backend switching at runtime
- Connection pooling or retry policies
- File-based configuration loading (JSON/YAML parsing)
- Backend-specific features (Dolt versioning, DynamoDB provisioned capacity)

## Dependencies

- prd-cupboard-core must be defined (Config, Cupboard, Table interfaces)
- SQLite backend must implement the Cupboard interface

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| DataDir permissions prevent database creation | Document required permissions; test with non-writable path |
| Attach called concurrently from multiple goroutines | Document that Attach must be called once; use mutex in implementation |
| Detach called while operations are in-flight | Detach blocks until operations complete per R5.3 |
