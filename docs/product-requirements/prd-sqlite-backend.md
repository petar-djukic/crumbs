# PRD: SQLite Backend

## Problem

The SQLite backend needs a detailed specification for how JSON files and SQLite interact. prd-cupboard-core establishes that JSON is the source of truth and SQLite serves as a query engine, but it does not specify the JSON file format, SQLite schema, sync lifecycle, or error handling. Without this detail, implementation will make ad-hoc decisions that may not align with project goals.

This PRD specifies the SQLite backend internals: JSON file layout, SQLite schema, startup loading, write persistence, shutdown flushing, error recovery, and concurrency model.

## Graph Model

We store data as a directed acyclic graph (DAG). Crumbs and trails are nodes; relationships are edges stored in link tables. This separates how we store data from how we access it, enabling efficient queries for both.

**Nodes**: Crumbs and trails are stored in separate tables. Both are nodes in the graph.

**Edges (links)**: Relationships between nodes are stored in dedicated link tables:

| Link type | From | To | Cardinality |
|------------|-------|--------|---------------------------------------------|
| belongs_to | crumb | trail | many-to-one (crumb belongs to one trail) |
| child_of | crumb | crumb | many-to-many (DAG of crumbs within a trail) |

**Query patterns**:

- Find all crumbs in a trail: query belongs_to by trail_id
- Find child crumbs of a crumb: query child_of by parent_id
- Find parent crumbs of a crumb: query child_of by child_id
- Traverse the DAG: recursive CTE on child_of

**Integrity**: Audit functions validate the graph (no cycles, valid references, DAG structure).

## Goals

1. Define the JSON file format and directory layout within DataDir
2. Define the SQLite schema that mirrors the JSON structure
3. Specify the startup sequence: loading JSON into SQLite
4. Specify write behavior: updating SQLite and persisting to JSON
5. Specify shutdown behavior: flushing pending writes
6. Define error handling for corrupt files, schema mismatches, and I/O failures
7. Define the concurrency model for safe concurrent access

## Requirements

### R1: Directory Layout

1.1. The SQLite backend operates within a single directory (DataDir from Config).

1.2. DataDir must contain:

| File | Purpose |
|------|---------|
| crumbs.json | All crumbs (source of truth) |
| trails.json | All trails (source of truth) |
| links.json | Graph edges: belongs_to, child_of relationships |
| properties.json | Property definitions (source of truth) |
| categories.json | Category definitions for categorical properties |
| crumb_properties.json | Property values for crumbs |
| metadata.json | All metadata entries |
| cupboard.db | SQLite database (ephemeral cache, regenerated from JSON) |

1.3. If DataDir does not exist, OpenCupboard must create it.

1.4. If JSON files do not exist, OpenCupboard must create empty files with valid JSON (empty arrays).

### R2: JSON File Format

2.1. Each JSON file contains an array of objects. One object per entity.

2.2. crumbs.json format:

```json
[
  {
    "crumb_id": "01945a3b-...",
    "name": "Implement feature X",
    "state": "pending",
    "created_at": "2025-01-15T10:30:00Z",
    "updated_at": "2025-01-15T10:30:00Z"
  }
]
```

Note: trail membership is stored in links.json (belongs_to), not as a field on the crumb.

2.3. trails.json format:

```json
[
  {
    "trail_id": "01945a3c-...",
    "parent_crumb_id": null,
    "state": "active",
    "created_at": "2025-01-15T10:30:00Z",
    "completed_at": null
  }
]
```

2.4. properties.json format:

```json
[
  {
    "property_id": "01945a3d-...",
    "name": "priority",
    "description": "Task priority level",
    "value_type": "categorical",
    "created_at": "2025-01-15T10:30:00Z"
  }
]
```

2.5. categories.json format:

```json
[
  {
    "category_id": "01945a3e-...",
    "property_id": "01945a3d-...",
    "name": "high",
    "ordinal": 0
  }
]
```

2.6. crumb_properties.json format (unified, type in field):

```json
[
  {
    "crumb_id": "01945a3b-...",
    "property_id": "01945a3d-...",
    "value_type": "categorical",
    "value": "01945a3e-..."
  },
  {
    "crumb_id": "01945a3b-...",
    "property_id": "01945a4a-...",
    "value_type": "text",
    "value": "Some description text"
  },
  {
    "crumb_id": "01945a3b-...",
    "property_id": "01945a4b-...",
    "value_type": "integer",
    "value": 42
  },
  {
    "crumb_id": "01945a3b-...",
    "property_id": "01945a4c-...",
    "value_type": "list",
    "value": ["item1", "item2"]
  }
]
```

2.7. links.json format (graph edges):

```json
[
  {
    "link_type": "belongs_to",
    "from_id": "01945a3b-...",
    "to_id": "01945a3c-...",
    "created_at": "2025-01-15T10:30:00Z"
  },
  {
    "link_type": "child_of",
    "from_id": "01945a3d-...",
    "to_id": "01945a3b-...",
    "created_at": "2025-01-15T10:35:00Z"
  }
]
```

Link types:

- `belongs_to`: from_id is crumb_id, to_id is trail_id (crumb belongs to trail)
- `child_of`: from_id is child crumb_id, to_id is parent crumb_id (DAG edge)

2.9. metadata.json format:

```json
[
  {
    "metadata_id": "01945a3f-...",
    "table_name": "comments",
    "crumb_id": "01945a3b-...",
    "property_id": null,
    "content": "Started working on this",
    "created_at": "2025-01-15T11:00:00Z"
  }
]
```

2.10. All timestamps must be RFC 3339 format (ISO 8601 with timezone).

2.11. All UUIDs must be lowercase hyphenated format.

### R3: SQLite Schema

3.1. The SQLite database uses a single file (cupboard.db) in DataDir.

3.2. SQLite schema must mirror JSON structure for direct loading:

```sql
CREATE TABLE crumbs (
    crumb_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE trails (
    trail_id TEXT PRIMARY KEY,
    parent_crumb_id TEXT,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE links (
    link_type TEXT NOT NULL,
    from_id TEXT NOT NULL,
    to_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (link_type, from_id, to_id)
);

CREATE TABLE properties (
    property_id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    value_type TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE categories (
    category_id TEXT PRIMARY KEY,
    property_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    FOREIGN KEY (property_id) REFERENCES properties(property_id)
);

CREATE TABLE crumb_properties (
    crumb_id TEXT NOT NULL,
    property_id TEXT NOT NULL,
    value_type TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (crumb_id, property_id),
    FOREIGN KEY (crumb_id) REFERENCES crumbs(crumb_id),
    FOREIGN KEY (property_id) REFERENCES properties(property_id)
);

CREATE TABLE metadata (
    metadata_id TEXT PRIMARY KEY,
    table_name TEXT NOT NULL,
    crumb_id TEXT NOT NULL,
    property_id TEXT,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (crumb_id) REFERENCES crumbs(crumb_id)
);
```

3.3. Indexes for common queries:

```sql
CREATE INDEX idx_crumbs_state ON crumbs(state);
CREATE INDEX idx_trails_state ON trails(state);
CREATE INDEX idx_links_type_from ON links(link_type, from_id);
CREATE INDEX idx_links_type_to ON links(link_type, to_id);
CREATE INDEX idx_crumb_properties_crumb ON crumb_properties(crumb_id);
CREATE INDEX idx_crumb_properties_property ON crumb_properties(property_id);
CREATE INDEX idx_metadata_crumb ON metadata(crumb_id);
CREATE INDEX idx_metadata_table ON metadata(table_name);
CREATE INDEX idx_categories_property ON categories(property_id);
```

3.4. The value column in crumb_properties stores JSON-encoded values for all types. For categorical properties, it stores the category_id. For lists, it stores a JSON array.

### R4: Startup Sequence

4.1. On OpenCupboard with sqlite backend:

1. Create DataDir if it does not exist
2. Create empty JSON files if they do not exist
3. Delete cupboard.db if it exists (ephemeral cache)
4. Create new cupboard.db with schema (R3)
5. Load each JSON file into corresponding SQLite table
6. Validate foreign key relationships
7. Return ready Cupboard instance

4.2. If any JSON file is malformed (invalid JSON), OpenCupboard must return an error describing which file and the parse error.

4.3. If foreign key validation fails (e.g., crumb references non-existent trail), OpenCupboard must return an error. We do not auto-repair.

4.4. Loading must be transactional: if any load fails, the database remains empty.

### R5: Write Operations

5.1. All write operations follow this pattern:

1. Begin SQLite transaction
2. Execute SQL changes
3. Commit SQLite transaction
4. Persist affected JSON file(s)

5.2. JSON persistence must be atomic: write to temp file, then rename. This prevents corrupt files on crash.

5.3. Write operations must persist immediately (no batching). This ensures JSON files are always current.

5.4. If JSON persistence fails after SQLite commit, the operation must return an error. The next OpenCupboard will reload from JSON (the source of truth), so SQLite and JSON will reconcile.

5.5. Operations that affect multiple tables must persist all affected JSON files:

| Operation | JSON files affected |
|-----------|---------------------|
| DropCrumb | crumbs.json |
| DeleteCrumb | crumbs.json, crumb_properties.json, metadata.json |
| StartTrail | trails.json |
| CompleteTrail | trails.json, crumbs.json |
| AbandonTrail | trails.json, crumbs.json, crumb_properties.json, metadata.json |
| DefineProperty | properties.json |
| DefineCategory | categories.json |
| SetCrumbProperty | crumb_properties.json |
| ClearCrumbProperty | crumb_properties.json |
| AddMetadata | metadata.json |

### R6: Shutdown Sequence

6.1. On Close:

1. Wait for in-flight operations to complete (with timeout)
2. Verify all JSON files are current (no pending writes)
3. Close SQLite connection
4. cupboard.db may be deleted or left for debugging

6.2. Close must be idempotent. Subsequent calls return nil.

6.3. After Close, all operations must return ErrCupboardClosed.

### R7: Error Handling

7.1. Corrupt JSON on startup:

- Return error with file name and parse details
- Do not attempt repair
- User must fix or delete the file

7.2. Schema mismatch (JSON has unknown fields):

- Ignore unknown fields (forward compatibility)
- Log warning if verbose mode enabled

7.3. Missing required fields in JSON:

- Return error identifying the record and missing field

7.4. I/O errors during write:

- Return error to caller
- SQLite may be ahead of JSON temporarily
- Next OpenCupboard reconciles from JSON

7.5. SQLite errors:

- Return error to caller
- Do not corrupt JSON files
- SQLite is regenerated on next startup

### R8: Concurrency Model

8.1. The SQLite backend supports single-writer, multiple-reader within a process.

8.2. Write operations acquire an exclusive lock. Only one write at a time.

8.3. Read operations (GetCrumb, FetchCrumbs, etc.) can run concurrently with each other.

8.4. Read operations block during the write phase but can proceed during JSON persistence.

8.5. Cross-process concurrency is not supported. Only one process should open a DataDir at a time. If a second process attempts to open, behavior is undefined (SQLite may lock, JSON writes may conflict).

8.6. Future: file-based locking (lockfile in DataDir) may be added to detect multi-process access.

### R9: Built-in Properties

9.1. On first startup (empty properties.json), the backend must seed built-in properties:

| property_id | name | value_type | description |
|-------------|------|------------|-------------|
| (generated) | priority | categorical | Task priority (0=highest, 4=lowest) |
| (generated) | type | categorical | Crumb type (task, epic, bug, etc.) |
| (generated) | description | text | Detailed description |
| (generated) | owner | text | Assigned worker/user ID |
| (generated) | labels | list | Capability tags |
| (generated) | dependencies | list | Crumb IDs that must complete first |

9.2. Built-in categories for priority:

| name | ordinal |
|------|---------|
| highest | 0 |
| high | 1 |
| medium | 2 |
| low | 3 |
| lowest | 4 |

9.3. Built-in categories for type:

| name | ordinal |
|------|---------|
| task | 0 |
| epic | 1 |
| bug | 2 |
| chore | 3 |

9.4. Seeding only occurs if properties.json is empty (first run). Existing data is never modified.

### R10: Graph Audit

10.1. The backend must provide audit functions to validate graph integrity:

| Audit | Description |
|-------|-------------|
| ValidateDAG | Ensure no cycles exist in child_of links |
| ValidateReferences | Ensure all link from_id and to_id reference existing entities |
| ValidateBelongsTo | Ensure each crumb belongs to at most one trail |
| ValidateTrailCrumbs | Ensure abandoned trails have no crumbs |

10.2. ValidateDAG must detect cycles using depth-first search or topological sort. If a cycle is found, return an error listing the crumb_ids involved.

10.3. ValidateReferences must check:

- belongs_to links: from_id exists in crumbs, to_id exists in trails
- child_of links: both from_id and to_id exist in crumbs

10.4. Audit functions run on startup after loading JSON. If validation fails, OpenCupboard returns an error.

10.5. Audit functions are also available as Cupboard methods for on-demand validation.

## Non-Goals

1. This PRD does not define the Cupboard interface operations. Those are in prd-cupboard-core and the interface PRDs.

2. This PRD does not define Dolt or DynamoDB backends.

3. This PRD does not define cross-process locking. Single-process access is assumed.

4. This PRD does not define backup or migration utilities.

## Acceptance Criteria

- [ ] JSON file format specified for all entity types (R2)
- [ ] SQLite schema specified with all tables and indexes (R3)
- [ ] Startup sequence specified: create, load, validate (R4)
- [ ] Write operation pattern specified: transaction, persist, atomicity (R5)
- [ ] Shutdown sequence specified (R6)
- [ ] Error handling specified for all failure modes (R7)
- [ ] Concurrency model specified (R8)
- [ ] Built-in properties and categories specified (R9)
- [ ] Graph audit functions specified (R10)
- [ ] File saved at docs/product-requirements/prd-sqlite-backend.md

## Constraints

- modernc.org/sqlite is pure Go; no CGO dependencies
- JSON files must be human-readable (pretty-printed with indentation)
- Timestamps in JSON use RFC 3339 for interoperability
- SQLite database is ephemeral; deleting cupboard.db loses nothing

## References

- prd-cupboard-core (Cupboard interface, configuration, lifecycle)
- prd-crumbs-interface, prd-trails-interface, prd-properties-interface, prd-metadata-interface
- modernc.org/sqlite documentation
