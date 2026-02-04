# PRD: Cupboard Core Interface

## Problem

Applications using Crumbs need a consistent way to initialize storage, select backends, and manage the cupboard lifecycle. Without a well-defined core interface, each application must handle backend initialization differently, leading to duplicated setup code and inconsistent error handling. We need a single entry point that accepts configuration, returns a ready-to-use cupboard instance, and cleanly releases resources when done.

This PRD defines the cupboard lifecycle interface: configuration, initialization, and shutdown. It establishes the contract that all backends must implement.

## Goals

1. Define a Config struct that selects backends and provides backend-specific parameters
2. Define the Cupboard interface as the main contract for all storage operations
3. Define OpenCupboard and CloseCupboard lifecycle operations
4. Specify error handling for operations invoked after close
5. Define the Backend interface that all storage implementations must satisfy

## Requirements

### R1: Configuration

1.1. The Config struct must include:

| Field | Type | Description |
|-------|------|-------------|
| Backend | string | Backend type: "json", "dolt", "dynamodb" |
| DataDir | string | Directory for local backends (json, dolt); ignored for cloud backends |
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

2.1. The Cupboard interface must define all storage operations. It is the main contract between applications and storage.

2.2. The Cupboard interface must include lifecycle methods:

```go
type Cupboard interface {
    // Close releases all resources. Idempotent.
    Close() error
}
```

2.3. The Cupboard interface must include crumb operations (defined in prd-crumbs-interface):
- DropCrumb, GetCrumb, DeleteCrumb

2.4. The Cupboard interface must include trail operations (defined in prd-trails-interface):
- StartTrail, GetTrail, GetTrailCrumbs, CompleteTrail, AbandonTrail

2.5. The Cupboard interface must include property operations (defined in prd-properties-interface and prd-crumb-properties-interface):
- DefineProperty, GetProperty, ListProperties, DefineCategory
- SetCrumbProperty, GetCrumbProperty, GetCrumbProperties, ClearCrumbProperty

2.6. The Cupboard interface must include metadata operations (defined in prd-metadata-interface):
- RegisterMetadataTable, AddMetadata, GetMetadata, SearchMetadata

2.7. The Cupboard interface must include query operations (defined in prd-query-interface):
- FetchCrumbs

### R3: OpenCupboard

3.1. OpenCupboard must accept a Config and return a Cupboard instance or error:

```go
func OpenCupboard(cfg Config) (Cupboard, error)
```

3.2. OpenCupboard must validate the config before initializing the backend

3.3. OpenCupboard must return an error if backend initialization fails (e.g., cannot connect to Dolt, cannot access DynamoDB table)

3.4. OpenCupboard must return a ready-to-use Cupboard; callers should not need additional setup

### R4: CloseCupboard (Close method)

4.1. Close must release all resources held by the cupboard (connections, file handles, goroutines)

4.2. Close must be idempotent; calling Close multiple times must not error

4.3. Close must block until all in-flight operations complete or a reasonable timeout elapses

### R5: Error Handling After Close

5.1. All Cupboard operations must return ErrCupboardClosed if invoked after Close

5.2. ErrCupboardClosed must be a sentinel error that callers can check with errors.Is:

```go
var ErrCupboardClosed = errors.New("cupboard is closed")
```

5.3. Backends must track closed state and check it at the start of each operation

### R6: Backend Interface

6.1. The Backend interface must mirror the Cupboard interface; backends implement Backend, and OpenCupboard wraps it in a Cupboard

6.2. The Backend interface allows internal implementation details (e.g., connection pooling) to differ across backends while presenting a uniform Cupboard to applications

6.3. Each backend package must export a constructor:

```go
// json backend
func NewJSONBackend(dataDir string) (Backend, error)

// dolt backend
func NewDoltBackend(cfg DoltConfig) (Backend, error)

// dynamodb backend
func NewDynamoDBBackend(cfg DynamoDBConfig) (Backend, error)
```

## Non-Goals

1. This PRD does not define individual crumb, trail, property, metadata, or query operations. Those are defined in their respective interface PRDs.

2. This PRD does not define backend-specific behavior (e.g., Dolt versioning, DynamoDB single-table design). Backends may add optional methods beyond the interface.

3. This PRD does not define HTTP/RPC wrappers around the Cupboard interface. Applications define their own APIs.

4. This PRD does not define connection pooling or retry policies. Backends may implement these internally.

## Acceptance Criteria

- [ ] Config struct defined with Backend, DataDir, DoltConfig, DynamoDBConfig fields
- [ ] DoltConfig and DynamoDBConfig structs defined with required fields
- [ ] Cupboard interface defined with Close method and references to other interface PRDs
- [ ] OpenCupboard function signature and behavior documented
- [ ] Close method behavior documented (idempotent, blocks until complete)
- [ ] ErrCupboardClosed sentinel error defined
- [ ] Backend interface defined with constructor patterns for each backend
- [ ] All requirements numbered and specific
- [ ] File saved at docs/product-requirements/prd-cupboard-core.md

## Constraints

- Config struct must be serializable to JSON/YAML for file-based configuration
- Backend-specific configs use pointer types to distinguish "not configured" from "configured with defaults"
- ErrCupboardClosed must work with errors.Is for Go 1.13+ error wrapping

## References

- prd-task-storage R8 (Storage Backends), R9 (Cupboard Lifecycle)
- prd-crumbs-interface, prd-trails-interface, prd-properties-interface, prd-crumb-properties-interface, prd-metadata-interface, prd-query-interface
