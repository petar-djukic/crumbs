// Metadata and Schema entities for supplementary information on crumbs.
// Implements: prd-metadata-interface R1, R2 (Metadata, Schema);
//
//	docs/ARCHITECTURE § Main Interface.
package types

import "time"

// Content type constants for schema definitions.
const (
	ContentTypeText = "text"
	ContentTypeJSON = "json"
)

// Metadata represents supplementary information attached to a crumb.
type Metadata struct {
	// MetadataID is a UUID v7, generated on creation.
	MetadataID string

	// CrumbID is the crumb this metadata is attached to.
	CrumbID string

	// TableName is the schema this entry belongs to (e.g., "comments").
	TableName string

	// Content is the metadata content (text or JSON).
	Content string

	// PropertyID is an optional property ID for property-specific metadata.
	PropertyID *string

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time
}

// Schema defines a metadata table schema.
type Schema struct {
	// SchemaName is a unique name for this schema (e.g., "comments").
	SchemaName string

	// Description is an optional explanation of the schema's purpose.
	Description string

	// ContentType is the expected content format: "text" or "json".
	ContentType string
}
