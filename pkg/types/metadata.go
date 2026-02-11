package types

import "time"

// Metadata is an extensible entry attached to a crumb.
// Implements: prd005-metadata-interface R1.
type Metadata struct {
	MetadataID string    // UUID v7, generated on creation.
	CrumbID    string    // The crumb this metadata is attached to.
	TableName  string    // The schema this entry belongs to (e.g. "comments").
	Content    string    // The metadata content (text or JSON).
	PropertyID *string   // Optional property ID for property-specific metadata.
	CreatedAt  time.Time // Timestamp of creation.
}

// Schema defines a named metadata format.
// Implements: prd005-metadata-interface R5.
type Schema struct {
	SchemaName  string // Unique name for this schema (e.g. "comments").
	Description string // Optional explanation of the schema's purpose.
	ContentType string // Expected content format: "text" or "json".
}
