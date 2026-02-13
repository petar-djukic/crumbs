// Package sqlite implements the SQLite backend for the Crumbs storage system.
// Implements: prd002-sqlite-backend (R3 SQLite schema, R11 Cupboard interface);
//             docs/ARCHITECTURE § SQLite Backend.
package sqlite

// Schema DDL for all tables (prd002-sqlite-backend R3.2).
const (
	createCrumbs = `CREATE TABLE crumbs (
    crumb_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);`

	createTrails = `CREATE TABLE trails (
    trail_id TEXT PRIMARY KEY,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL,
    completed_at TEXT
);`

	createLinks = `CREATE TABLE links (
    link_id TEXT PRIMARY KEY,
    link_type TEXT NOT NULL,
    from_id TEXT NOT NULL,
    to_id TEXT NOT NULL,
    created_at TEXT NOT NULL
);`

	createProperties = `CREATE TABLE properties (
    property_id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    value_type TEXT NOT NULL,
    created_at TEXT NOT NULL
);`

	createCategories = `CREATE TABLE categories (
    category_id TEXT PRIMARY KEY,
    property_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    FOREIGN KEY (property_id) REFERENCES properties(property_id)
);`

	createCrumbProperties = `CREATE TABLE crumb_properties (
    crumb_id TEXT NOT NULL,
    property_id TEXT NOT NULL,
    value_type TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (crumb_id, property_id),
    FOREIGN KEY (crumb_id) REFERENCES crumbs(crumb_id),
    FOREIGN KEY (property_id) REFERENCES properties(property_id)
);`

	createMetadata = `CREATE TABLE metadata (
    metadata_id TEXT PRIMARY KEY,
    table_name TEXT NOT NULL,
    crumb_id TEXT NOT NULL,
    property_id TEXT,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (crumb_id) REFERENCES crumbs(crumb_id) ON DELETE CASCADE
);`

	createStashes = `CREATE TABLE stashes (
    stash_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    stash_type TEXT NOT NULL,
    value TEXT NOT NULL,
    version INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);`

	createStashHistory = `CREATE TABLE stash_history (
    history_id TEXT PRIMARY KEY,
    stash_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    value TEXT NOT NULL,
    operation TEXT NOT NULL,
    changed_by TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (stash_id) REFERENCES stashes(stash_id),
    FOREIGN KEY (changed_by) REFERENCES crumbs(crumb_id)
);`
)

// Index DDL for common queries (prd002-sqlite-backend R3.3).
const (
	idxCrumbsState            = `CREATE INDEX idx_crumbs_state ON crumbs(state);`
	idxTrailsState            = `CREATE INDEX idx_trails_state ON trails(state);`
	idxLinksUnique            = `CREATE UNIQUE INDEX idx_links_unique ON links(link_type, from_id, to_id);`
	idxLinksTypeFrom          = `CREATE INDEX idx_links_type_from ON links(link_type, from_id);`
	idxLinksTypeTo            = `CREATE INDEX idx_links_type_to ON links(link_type, to_id);`
	idxCrumbPropertiesCrumb   = `CREATE INDEX idx_crumb_properties_crumb ON crumb_properties(crumb_id);`
	idxCrumbPropertiesProperty = `CREATE INDEX idx_crumb_properties_property ON crumb_properties(property_id);`
	idxMetadataCrumb          = `CREATE INDEX idx_metadata_crumb ON metadata(crumb_id);`
	idxMetadataTable          = `CREATE INDEX idx_metadata_table ON metadata(table_name);`
	idxCategoriesProperty     = `CREATE INDEX idx_categories_property ON categories(property_id);`
	idxStashesName            = `CREATE INDEX idx_stashes_name ON stashes(name);`
	idxStashHistoryStash      = `CREATE INDEX idx_stash_history_stash ON stash_history(stash_id);`
	idxStashHistoryVersion    = `CREATE INDEX idx_stash_history_version ON stash_history(stash_id, version);`
)

// schemaDDL lists all CREATE TABLE statements in dependency order.
var schemaDDL = []string{
	createCrumbs,
	createTrails,
	createLinks,
	createProperties,
	createCategories,
	createCrumbProperties,
	createMetadata,
	createStashes,
	createStashHistory,
}

// indexDDL lists all CREATE INDEX statements.
var indexDDL = []string{
	idxCrumbsState,
	idxTrailsState,
	idxLinksUnique,
	idxLinksTypeFrom,
	idxLinksTypeTo,
	idxCrumbPropertiesCrumb,
	idxCrumbPropertiesProperty,
	idxMetadataCrumb,
	idxMetadataTable,
	idxCategoriesProperty,
	idxStashesName,
	idxStashHistoryStash,
	idxStashHistoryVersion,
}
