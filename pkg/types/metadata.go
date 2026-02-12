package types

import "time"

// Metadata represents an extensible metadata entry attached to a crumb.
// Implements prd005-metadata-interface R1.
type Metadata struct {
	MetadataID string    // UUID v7, generated on creation
	CrumbID    string    // The crumb this metadata is attached to
	TableName  string    // The schema this entry belongs to
	Content    string    // The metadata content (text or JSON)
	PropertyID *string   // Optional property ID for property-specific metadata
	CreatedAt  time.Time // Timestamp of creation
}
