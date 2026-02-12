package types

import "time"

// Metadata attaches supplementary information to crumbs
// (prd005-metadata-interface R1.1). Entries are append-only by convention.
type Metadata struct {
	MetadataID string    `json:"metadata_id"`
	CrumbID    string    `json:"crumb_id"`
	TableName  string    `json:"table_name"`
	Content    string    `json:"content"`
	PropertyID *string   `json:"property_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// Schema defines a metadata schema for extensible metadata types
// (prd005-metadata-interface R2.1).
type Schema struct {
	SchemaName  string `json:"schema_name"`
	Description string `json:"description"`
	ContentType string `json:"content_type"`
}
