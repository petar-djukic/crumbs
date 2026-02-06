// JSON record structures for SQLite backend persistence.
// These structures define the JSON/JSONL record format for data files.
// Implements: prd-configuration-directories R3;
//
//	prd-sqlite-backend R2;
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

// JSON record structures that mirror the file format per prd-sqlite-backend R2.

// crumbJSON represents a crumb in crumbs.jsonl.
type crumbJSON struct {
	CrumbID   string `json:"crumb_id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// trailJSON represents a trail in trails.jsonl.
// Trail branching uses branches_from links in links.jsonl.
type trailJSON struct {
	TrailID     string  `json:"trail_id"`
	State       string  `json:"state"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`
}

// linkJSON represents a link in links.jsonl.
type linkJSON struct {
	LinkID    string `json:"link_id"`
	LinkType  string `json:"link_type"`
	FromID    string `json:"from_id"`
	ToID      string `json:"to_id"`
	CreatedAt string `json:"created_at"`
}

// propertyJSON represents a property in properties.jsonl.
type propertyJSON struct {
	PropertyID  string `json:"property_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ValueType   string `json:"value_type"`
	CreatedAt   string `json:"created_at"`
}

// categoryJSON represents a category in categories.jsonl.
type categoryJSON struct {
	CategoryID string `json:"category_id"`
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	Ordinal    int    `json:"ordinal"`
}

// crumbPropertyJSON represents a crumb property value in crumb_properties.jsonl.
type crumbPropertyJSON struct {
	CrumbID    string `json:"crumb_id"`
	PropertyID string `json:"property_id"`
	ValueType  string `json:"value_type"`
	Value      any    `json:"value"`
}

// metadataJSON represents metadata in metadata.jsonl.
type metadataJSON struct {
	MetadataID string  `json:"metadata_id"`
	TableName  string  `json:"table_name"`
	CrumbID    string  `json:"crumb_id"`
	PropertyID *string `json:"property_id"`
	Content    string  `json:"content"`
	CreatedAt  string  `json:"created_at"`
}

// stashJSON represents a stash in stashes.jsonl.
// Stash scoping uses scoped_to links in links.jsonl.
type stashJSON struct {
	StashID   string `json:"stash_id"`
	Name      string `json:"name"`
	StashType string `json:"stash_type"`
	Value     any    `json:"value"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// stashHistoryJSON represents a stash history entry in stash_history.jsonl.
type stashHistoryJSON struct {
	HistoryID string  `json:"history_id"`
	StashID   string  `json:"stash_id"`
	Version   int64   `json:"version"`
	Value     any     `json:"value"`
	Operation string  `json:"operation"`
	ChangedBy *string `json:"changed_by"`
	CreatedAt string  `json:"created_at"`
}
