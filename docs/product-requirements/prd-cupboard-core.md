# PRD: Cupboard Core Interface

## Problem

Applications using Crumbs need a consistent way to initialize storage, access data tables, and manage the cupboard lifecycle. Without a well-defined core interface, each application must handle backend initialization differently, leading to duplicated setup code and inconsistent error handling. We need a single entry point that accepts configuration, provides uniform table access, and cleanly releases resources when done.

This PRD defines the cupboard lifecycle interface: configuration, table access, and shutdown. It establishes the contract that all backends must implement.

## Goals

1. Define a Config struct that selects backends and provides backend-specific parameters
2. Define the Cupboard interface with uniform table access via GetTable
3. Define the Table interface that all data tables implement
4. Define Attach and Detach lifecycle operations
5. Specify error handling for operations invoked after detach
6. Document standard table names used by the system

## Requirements

### R1: Configuration

1.1. The Config struct must include:

| Field | Type | Description |
|-------|------|-------------|
| Backend | string | Backend type: "sqlite", "dolt", "dynamodb" |
| DataDir | string | Directory for local backends (sqlite, dolt); ignored for cloud backends |
| DoltConfig | *DoltConfig | Dolt-specific settings (connection string, branch); nil if not using Dolt |
| DynamoDBConfig | *DynamoDBConfig | DynamoDB-specific settings (table name, region); nil if not using DynamoDB |

1.2. DoltConfig must include:

| Field | Type | Description |
|-------|------|-------------|
| DSN | string | Data source name (connection string) |
| Branch | string | Git branch for versioning; defaults to "main" |

1.3. DynamoDBConfig must include:

| Field | Type | Description |
|-------|------|-------------|
| TableName | string | DynamoDB table name |
| Region | string | AWS region |
| Endpoint | string | Optional endpoint override for local testing |

1.4. Config validation must fail if Backend is empty or unrecognized

1.5. Config validation must fail if required backend-specific config is nil (e.g., DoltConfig nil when Backend is "dolt")

### R2: Cupboard Interface

2.1. The Cupboard interface must define the core contract for storage access and lifecycle management

2.2. The Cupboard interface must include:

```go
type Cupboard interface {
    // Table access
    GetTable(name string) (Table, error)

    // Lifecycle
    Attach(config Config) error
    Detach() error
}
```

2.3. GetTable must return a Table interface for the specified table name

2.4. GetTable must return ErrTableNotFound if the table name is not recognized

2.5. Standard table names are:

| Table name | Purpose | Entity type |
|------------|---------|-------------|
| crumbs | Work items and tasks | Crumb |
| trails | Exploration sessions | Trail |
| properties | Property definitions | Property |
| metadata | Extensible metadata entries | Metadata |
| links | Relationships between entities | Link |
| stashes | Shared state for trails | Stash |

2.6. Backends must support all standard table names

### R3: Table Interface

3.1. The Table interface provides uniform CRUD operations for all entity types:

```go
type Table interface {
    Get(id string) (any, error)
    Set(id string, data any) error
    Delete(id string) error
    Fetch(filter map[string]any) ([]any, error)
}
```

3.2. Get retrieves an entity by its ID and returns the entity object or ErrNotFound

3.3. Set persists an entity object. If the entity has an ID field set, it updates the existing entity; if ID is empty or zero, it generates a new ID and creates the entity

3.4. Delete removes an entity by ID. It must return ErrNotFound if the entity does not exist

3.5. Fetch queries entities matching the filter. The filter map keys are field names; values are the required field values. An empty filter returns all entities in the table

3.6. All entity types returned by Get and Fetch are concrete structs (Crumb, Trail, Property, etc.), not interfaces. Callers use type assertions to access entity-specific fields

3.7. Entity structs are defined in their respective interface PRDs:

- Crumb: prd-crumbs-interface
- Trail: prd-trails-interface
- Property: prd-properties-interface
- Metadata: prd-metadata-interface
- Link: prd-sqlite-backend (graph model)
- Stash: prd-stash-interface

### R4: Attach

4.1. Attach must accept a Config and initialize the backend connection:

```go
func (c *Cupboard) Attach(cfg Config) error
```

4.2. Attach must validate the config before initializing the backend

4.3. Attach must return an error if backend initialization fails (e.g., cannot connect to Dolt, cannot access DynamoDB table)

4.4. Attach must be idempotent; calling Attach on an already-attached cupboard must return ErrAlreadyAttached

4.5. After successful Attach, GetTable calls must succeed for standard table names

### R5: Detach

5.1. Detach must release all resources held by the cupboard (connections, file handles, goroutines)

5.2. Detach must be idempotent; calling Detach multiple times must not error

5.3. Detach must block until all in-flight operations complete or a reasonable timeout elapses

### R6: Error Handling After Detach

6.1. All Cupboard operations must return ErrCupboardDetached if invoked after Detach

6.2. ErrCupboardDetached must be a sentinel error that callers can check with errors.Is:

```go
var ErrCupboardDetached = errors.New("cupboard is detached")
```

6.3. Backends must track attached/detached state and check it at the start of each operation

### R7: Standard Error Types

7.1. The following sentinel errors must be defined:

```go
var ErrCupboardDetached = errors.New("cupboard is detached")
var ErrAlreadyAttached = errors.New("cupboard is already attached")
var ErrTableNotFound = errors.New("table not found")
var ErrNotFound = errors.New("entity not found")
```

7.2. Backends may define additional backend-specific errors but must use these standard errors where applicable

### R8: Entity ID Generation

8.1. All entity IDs must be UUID v7 (time-ordered UUIDs per RFC 9562)

8.2. Backends generate UUIDs when Set is called with an entity that has no ID or an empty ID field

8.3. UUID v7 provides sortability by creation time without separate timestamp columns

## Non-Goals

1. This PRD does not define entity-specific schemas or operations. Entity types are defined in their respective interface PRDs (prd-crumbs-interface, prd-trails-interface, etc.).

2. This PRD does not define backend-specific behavior (e.g., Dolt versioning, DynamoDB single-table design). Backends may add optional methods beyond the interface.

3. This PRD does not define HTTP/RPC wrappers around the Cupboard interface. Applications define their own APIs.

4. This PRD does not define connection pooling or retry policies. Backends may implement these internally.

5. This PRD does not define specialized query operations beyond Fetch. Entity-specific PRDs may specify additional query requirements that backends implement via filter conventions.

## Acceptance Criteria

- [ ] Config struct defined with Backend, DataDir, DoltConfig, DynamoDBConfig fields
- [ ] DoltConfig and DynamoDBConfig structs defined with required fields
- [ ] Cupboard interface defined with GetTable, Attach, Detach methods
- [ ] Table interface defined with Get, Set, Delete, Fetch methods
- [ ] Standard table names documented in a table
- [ ] Attach method behavior documented (idempotent, validates config)
- [ ] Detach method behavior documented (idempotent, blocks until complete)
- [ ] Standard error types defined (ErrCupboardDetached, ErrAlreadyAttached, ErrTableNotFound, ErrNotFound)
- [ ] UUID v7 requirement for entity IDs documented
- [ ] All requirements numbered and specific
- [ ] File saved at docs/product-requirements/prd-cupboard-core.md

## Constraints

- Config struct must be serializable to JSON/YAML for file-based configuration
- Backend-specific configs use pointer types to distinguish "not configured" from "configured with defaults"
- All standard error types must work with errors.Is for Go 1.13+ error wrapping
- Table.Get and Table.Fetch return any type; callers must use type assertions to access entity fields

## References

- VISION.md (breadcrumb metaphor, goals, boundaries)
- RFC 9562 (UUID v7 specification)
- prd-sqlite-backend (SQLite backend internals, JSON↔SQLite sync, graph model)
- prd-crumbs-interface, prd-trails-interface, prd-properties-interface, prd-metadata-interface, prd-stash-interface
