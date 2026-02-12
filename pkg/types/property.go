package types

import "time"

// Property defines a property that can be set on crumbs.
// Implements prd004-properties-interface R1.
type Property struct {
	PropertyID  string    // UUID v7, generated on creation
	Name        string    // Unique human-readable name (required, non-empty)
	Description string    // Optional explanation
	ValueType   string    // Type of values: categorical, text, integer, boolean, timestamp, list
	CreatedAt   time.Time // Timestamp of creation
}

// Category represents one allowed value for a categorical property.
// Implements prd004-properties-interface R2.
type Category struct {
	CategoryID string // UUID v7, generated on creation
	PropertyID string // ID of the categorical property this belongs to
	Name       string // Display name (unique within property)
	Ordinal    int    // Sort order; lower ordinals sort first
}
