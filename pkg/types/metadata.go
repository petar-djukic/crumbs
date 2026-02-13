package types

import "time"

// Built-in metadata schema names (prd005-metadata-interface R3.1).
const (
	SchemaComments    = "comments"
	SchemaAttachments = "attachments"
)

// Content type constants for metadata schemas (prd005-metadata-interface R2.1).
const (
	ContentTypeText = "text"
	ContentTypeJSON = "json"
)

// Schema defines a metadata schema for extensible metadata tables
// (prd005-metadata-interface R2.1). SchemaName must be unique across all
// registered schemas and contain only lowercase letters, numbers, and
// underscores.
type Schema struct {
	SchemaName  string `json:"schema_name"`
	Description string `json:"description"`
	ContentType string `json:"content_type"`
}

// Metadata represents a supplementary data entry attached to a crumb
// (prd005-metadata-interface R1.1). Metadata entries are append-only by
// convention; each new entry adds context rather than replacing old entries.
// The caller must call Table.Set to persist changes.
type Metadata struct {
	MetadataID string    `json:"metadata_id"`
	CrumbID    string    `json:"crumb_id"`
	TableName  string    `json:"table_name"`
	Content    string    `json:"content"`
	PropertyID *string   `json:"property_id"`
	CreatedAt  time.Time `json:"created_at"`
}
