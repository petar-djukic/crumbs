# PRD: Crumb Storage

## Problem

Applications need a general-purpose storage system for work items that is independent of any specific coordination framework. The storage should support personal tracking, workflow systems, or coordination layers like Task Fountain. Without a well-defined storage abstraction, applications couple directly to a specific database, making it difficult to switch backends or run in different environments.

This PRD defines a crumb storage system with an asynchronous API, extensible properties, trails for work sessions, and pluggable backends (local JSON, Dolt, DynamoDB, etc.).

## Why "Crumb" and the Breadcrumb Metaphor

We use "crumb" instead of "task", "item", or "record" because the breadcrumb metaphor (Hansel and Gretel) naturally captures how work flows:

**Crumbs** are individual pieces of work. You drop crumbs as you go. Each crumb can depend on other crumbs—forming a path to follow.

**Trails** are sequences of crumbs connected by time and dependency. A trail is the path you're exploring. You follow a trail of crumbs to complete work. Trails can:
- **Grow anywhere.** You can add crumbs in the middle of a trail, not just at the end.
- **Deviate.** You can leave the main path, drop new crumbs, and connect them however you want.
- **Dead end.** Sometimes a trail leads nowhere. You abandon it, backtrack to where you started, and take a different path.
- **Merge back.** A successful trail's crumbs become part of the permanent record.

**Cupboard** is where we keep the bread—the storage system that holds all crumbs and trails.

**Vocabulary:**

| Term | Meaning |
|------|---------|
| Cupboard | The storage system |
| Crumb | A single work item |
| Drop a crumb | Create a new crumb |
| Trail | A sequence of crumbs (work session, exploration path) |
| Follow the trail | Resolve dependencies, complete work |
| Deviate | Start a new trail from an existing crumb |
| Dead end | A trail that failed; must be abandoned |
| Backtrack | Abandon a dead-end trail and start fresh |
| Sweep up | Complete remaining crumbs |

## Goals

1. Define a general-purpose crumb storage system independent of any coordination layer
2. Support trails for work sessions with deviation, completion, and abandonment
3. Support multiple storage backends: local JSON, Dolt, service-based (DynamoDB, etc.)
4. Provide an asynchronous API for all operations
5. Use UUID v7 for all identifiers (time-ordered, sortable)
6. Define a normalized schema with extensible properties and metadata
7. Enable schema evolution: new properties and metadata tables can be added at runtime

## Requirements

### R1: Core Concepts and Identifiers

1.1. All identifiers (crumb IDs, trail IDs, property IDs, category IDs, metadata IDs) must be UUID v7 (time-ordered UUIDs per RFC 9562)

1.2. The API must be asynchronous; all operations return futures/promises or use async/await patterns

1.3. The storage system is called a **Cupboard**

### R2: Crumbs Table

The crumbs table stores core crumb data indexed by crumb ID.

2.1. Each crumb has a unique crumb_id (UUID v7)

2.2. Basic properties stored directly in the crumbs table:

| Column | Type | Description |
|--------|------|-------------|
| crumb_id | UUID v7 | Primary key, unique crumb identifier |
| name | string | Crumb name/title |
| state | string | Crumb state (e.g., pending, ready, running, completed, failed) |
| trail_id | UUID v7, nullable | Trail this crumb belongs to; null = permanent/main path |
| created_at | timestamp | Creation time (derived from UUID v7) |
| updated_at | timestamp | Last modification time |

2.3. DropCrumb(name, trail_id?) must create a crumb with a new UUID v7 crumb_id. If trail_id is provided, the crumb belongs to that trail; otherwise it's on the main path.

2.4. GetCrumb(crumb_id) must return the crumb with basic properties

2.5. DeleteCrumb(crumb_id) must remove the crumb and all associated properties and metadata

### R3: Trails Table

Trails represent work sessions—sequences of crumbs being explored. A trail can complete (crumbs become permanent) or be abandoned (dead end, crumbs cleaned up).

3.1. Each trail has:

| Column | Type | Description |
|--------|------|-------------|
| trail_id | UUID v7 | Primary key, unique trail identifier |
| parent_crumb_id | UUID v7, nullable | Crumb where this trail deviates from; null = independent trail |
| state | string | active, completed, abandoned |
| created_at | timestamp | When trail was started |
| completed_at | timestamp, nullable | When trail completed or was abandoned |

3.2. StartTrail(parent_crumb_id?) must create a new trail. If parent_crumb_id is provided, this trail is a deviation from that crumb's path.

3.3. GetTrail(trail_id) must return the trail with its state

3.4. GetTrailCrumbs(trail_id) must return all crumbs on a trail

3.5. CompleteTrail(trail_id) must:
- Mark the trail as completed
- Clear trail_id from all crumbs on the trail (they become permanent)
- Crumbs are now visible to everyone, part of the main path

3.6. AbandonTrail(trail_id) must:
- Mark the trail as abandoned (dead end)
- Delete or mark as abandoned all crumbs on the trail
- This is backtracking—the exploration failed

3.7. Active queries exclude crumbs on abandoned trails by default

### R4: Properties Table (Property Definitions)

Properties are first-class entities with their own identifiers. New properties can be defined at runtime.

4.1. Each property definition has:

| Column | Type | Description |
|--------|------|-------------|
| property_id | UUID v7 | Primary key, unique property identifier |
| name | string | Property name (e.g., "priority", "description", "labels") |
| description | string | Human-readable description |
| value_type | string | Type: "categorical", "text", "integer", "boolean", "timestamp", "list" |
| created_at | timestamp | When property was defined |

4.2. Some properties exist when the system starts (built-in):

| Property name | value_type | Description |
|---------------|------------|-------------|
| priority | categorical | 0=highest, 4=lowest |
| type | categorical | crumb type (e.g., task, epic, bug) |
| description | text | Detailed requirements |
| owner | text | Worker/user ID |
| labels | list | Capability tags |
| dependencies | list | Crumb IDs that must complete first (following the trail) |

4.3. DefineProperty(name, description, value_type) must create a new property definition with UUID v7 property_id

4.4. GetProperty(property_id) must return the property definition

4.5. ListProperties() must return all property definitions

### R5: Crumb Properties (Property Values by Type)

Crumb properties link crumbs to property values. Values are stored in type-specific tables.

#### Categorical properties

5.1. Categories table:

| Column | Type | Description |
|--------|------|-------------|
| category_id | UUID v7 | Primary key |
| property_id | UUID v7 | FK to properties table |
| name | string | Category name (e.g., "high", "medium", "low" for priority) |
| ordinal | int | Sort order within property |

5.2. Crumb categorical properties table:

| Column | Type | Description |
|--------|------|-------------|
| crumb_id | UUID v7 | FK to crumbs |
| property_id | UUID v7 | FK to properties |
| category_id | UUID v7 | FK to categories |

5.3. DefineCategory(property_id, name, ordinal) must create a new category for a categorical property

#### Textual properties

5.4. Crumb text properties table:

| Column | Type | Description |
|--------|------|-------------|
| crumb_id | UUID v7 | FK to crumbs |
| property_id | UUID v7 | FK to properties |
| value | text | Text value |

#### Integer properties

5.5. Crumb integer properties table:

| Column | Type | Description |
|--------|------|-------------|
| crumb_id | UUID v7 | FK to crumbs |
| property_id | UUID v7 | FK to properties |
| value | int64 | Integer value |

#### List properties

5.6. Crumb list properties table (multi-value):

| Column | Type | Description |
|--------|------|-------------|
| crumb_id | UUID v7 | FK to crumbs |
| property_id | UUID v7 | FK to properties |
| value | text | Single list item (one row per item) |

#### Property operations

5.7. SetCrumbProperty(crumb_id, property_id, value) must set the property value for a crumb. The caller knows the property type and provides the appropriate value.

5.8. GetCrumbProperty(crumb_id, property_id) must return the property value. The caller knows the expected type.

5.9. GetCrumbProperties(crumb_id) must return all properties for a crumb

5.10. ClearCrumbProperty(crumb_id, property_id) must remove the property value for a crumb

### R6: Metadata Tables

Metadata provides extensible storage for crumb-related data (comments, attachments, logs, etc.). The API supports adding new metadata table types.

6.1. Each metadata table has:
- crumb_id (UUID v7) – FK to crumbs
- metadata_id (UUID v7) – unique identifier for the metadata entry
- property_id (UUID v7, optional) – FK to properties for categorization
- content (text) – metadata content
- created_at (timestamp)

6.2. Built-in metadata tables:

| Table | Purpose |
|-------|---------|
| comments | Progress notes, collaboration |
| attachments | File references, links |

6.3. RegisterMetadataTable(table_name, schema) must allow adding new metadata table types at runtime

6.4. AddMetadata(table_name, crumb_id, content, property_id?) must add a metadata entry. Storage generates metadata_id (UUID v7) and created_at.

6.5. GetMetadata(table_name, crumb_id) must return all metadata entries for a crumb in the specified table

6.6. SearchMetadata(table_name, filter) must support search by:
- crumb_id – entries for a specific crumb
- property_id – entries tagged with a property
- text – free text search on content

### R7: Query Operations

7.1. FetchCrumbs(filter) must return crumbs matching the filter. Filter specifies properties the crumb must possess:

```
FetchCrumbs({
  "state": "ready",
  "priority": category_id_for_high,
  "label": "planning"  // crumb has this label
})
```

7.2. The filter is extensible: any property_id or property name can be used

7.3. By default, FetchCrumbs excludes crumbs on abandoned trails

7.4. Optional filter: `trail_id` to fetch crumbs on a specific trail; `include_abandoned: true` to include abandoned

7.5. Results should be paginated for large result sets

### R8: Storage Backends (Cupboard Implementations)

The system must support pluggable storage backends.

8.1. Backend interface must implement all operations from R2–R7

8.2. Built-in backends:

| Backend | Use case |
|---------|----------|
| JSON file | Local development, testing, personal use |
| Dolt | Version-controlled SQL, audit trails |
| DynamoDB | Serverless, scalable cloud storage |

8.3. Configuration selects the backend and provides backend-specific parameters

8.4. Backends may implement optional features (e.g., version history, TTL)

### R9: Cupboard Lifecycle

9.1. OpenCupboard(config) must initialize the storage backend

9.2. CloseCupboard() must release resources; idempotent

9.3. All operations must return errors if called after CloseCupboard()

## Non-Goals

1. This PRD does not define coordination semantics (claiming, timeout, announcements). Those belong to Task Fountain or similar coordination layers that build on this storage.

2. This PRD does not define specific HTTP/RPC APIs. Those are defined by applications using this storage.

3. This PRD does not define replication or multi-region. Backends may provide these features natively (e.g., DynamoDB global tables).

## Acceptance Criteria

- [ ] All identifiers use UUID v7
- [ ] API is asynchronous
- [ ] Crumbs table with basic properties (crumb_id, name, state, trail_id, timestamps)
- [ ] Trails table with StartTrail, CompleteTrail, AbandonTrail
- [ ] Crumbs on completed trails become permanent (trail_id cleared)
- [ ] Crumbs on abandoned trails are cleaned up or excluded by default
- [ ] Properties table with property definitions; DefineProperty, ListProperties
- [ ] Categories table for categorical properties; DefineCategory
- [ ] Type-specific property value tables (categorical, text, integer, list)
- [ ] SetCrumbProperty, GetCrumbProperty, GetCrumbProperties, ClearCrumbProperty work across types
- [ ] Metadata tables with RegisterMetadataTable, AddMetadata, GetMetadata, SearchMetadata
- [ ] FetchCrumbs with property-based filter and trail filtering
- [ ] JSON backend for local/testing
- [ ] Dolt backend with version control
- [ ] Backend selection via configuration
- [ ] Cupboard lifecycle (OpenCupboard, CloseCupboard)

## Constraints

- UUID v7 requires time synchronization for ordering guarantees
- Free text search on metadata depends on backend capabilities (may require indexing)
- Schema changes (new properties, categories, metadata tables) must be backward compatible
- AbandonTrail may delete crumbs or mark them; behavior may vary by backend

## References

- RFC 9562 – UUID v7 specification
- [PRD: Task Fountain Interface](prd-task-fountain-interface.md) – Coordination layer that may use this storage
- [PRD: Technology Stack](prd-technology-stack.md) – Dolt schema patterns
