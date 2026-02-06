-- SQLite schema for Crumbs storage backend.
-- Implements: prd-sqlite-backend R3 (tables and indexes).

-- Crumbs table: work items
CREATE TABLE IF NOT EXISTS crumbs (
    crumb_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Trails table: exploratory work sessions
-- Trail branching uses branches_from links in the links table (ARCHITECTURE Decision 10)
CREATE TABLE IF NOT EXISTS trails (
    trail_id TEXT PRIMARY KEY,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL,
    completed_at TEXT
);

-- Links table: graph edges between entities
CREATE TABLE IF NOT EXISTS links (
    link_id TEXT PRIMARY KEY,
    link_type TEXT NOT NULL,
    from_id TEXT NOT NULL,
    to_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (link_type, from_id, to_id)
);

-- Properties table: custom attribute definitions
CREATE TABLE IF NOT EXISTS properties (
    property_id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    value_type TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- Categories table: enumeration values for categorical properties
CREATE TABLE IF NOT EXISTS categories (
    category_id TEXT PRIMARY KEY,
    property_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    FOREIGN KEY (property_id) REFERENCES properties(property_id)
);

-- Crumb properties table: property values for crumbs
CREATE TABLE IF NOT EXISTS crumb_properties (
    crumb_id TEXT NOT NULL,
    property_id TEXT NOT NULL,
    value_type TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (crumb_id, property_id),
    FOREIGN KEY (crumb_id) REFERENCES crumbs(crumb_id),
    FOREIGN KEY (property_id) REFERENCES properties(property_id)
);

-- Metadata table: supplementary information attached to crumbs
CREATE TABLE IF NOT EXISTS metadata (
    metadata_id TEXT PRIMARY KEY,
    table_name TEXT NOT NULL,
    crumb_id TEXT NOT NULL,
    property_id TEXT,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (crumb_id) REFERENCES crumbs(crumb_id)
);

-- Stashes table: shared state for trails
-- Stash scoping uses scoped_to links in the links table (ARCHITECTURE Decision 10)
CREATE TABLE IF NOT EXISTS stashes (
    stash_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    stash_type TEXT NOT NULL,
    value TEXT NOT NULL,
    version INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Stash history table: append-only history of stash changes
CREATE TABLE IF NOT EXISTS stash_history (
    history_id TEXT PRIMARY KEY,
    stash_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    value TEXT NOT NULL,
    operation TEXT NOT NULL,
    changed_by TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (stash_id) REFERENCES stashes(stash_id),
    FOREIGN KEY (changed_by) REFERENCES crumbs(crumb_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_crumbs_state ON crumbs(state);
CREATE INDEX IF NOT EXISTS idx_trails_state ON trails(state);
CREATE INDEX IF NOT EXISTS idx_links_type_from ON links(link_type, from_id);
CREATE INDEX IF NOT EXISTS idx_links_type_to ON links(link_type, to_id);
CREATE INDEX IF NOT EXISTS idx_crumb_properties_crumb ON crumb_properties(crumb_id);
CREATE INDEX IF NOT EXISTS idx_crumb_properties_property ON crumb_properties(property_id);
CREATE INDEX IF NOT EXISTS idx_metadata_crumb ON metadata(crumb_id);
CREATE INDEX IF NOT EXISTS idx_metadata_table ON metadata(table_name);
CREATE INDEX IF NOT EXISTS idx_categories_property ON categories(property_id);
CREATE INDEX IF NOT EXISTS idx_stashes_name ON stashes(name);
CREATE INDEX IF NOT EXISTS idx_stash_history_stash ON stash_history(stash_id);
CREATE INDEX IF NOT EXISTS idx_stash_history_version ON stash_history(stash_id, version);
