package types

import "time"

// Metadata attaches supplementary information to a crumb.
// Implements prd005-metadata-interface R1.
type Metadata struct {
	MetadataID string    `json:"metadata_id"`
	CrumbID    string    `json:"crumb_id"`
	TableName  string    `json:"table_name"`
	Content    string    `json:"content"`
	PropertyID *string   `json:"property_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Schema defines a metadata schema for extensible metadata types.
// Implements prd005-metadata-interface R2.
type Schema struct {
	SchemaName  string `json:"schema_name"`
	Description string `json:"description"`
	ContentType string `json:"content_type"`
}
