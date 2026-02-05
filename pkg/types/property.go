// Property and Category entities for extensible attributes on crumbs.
// Implements: prd-properties-interface R1, R2, R3 (Property, Category, value types);
//
//	docs/ARCHITECTURE § Main Interface.
package types

import "time"

// Value type constants.
const (
	ValueTypeCategorical = "categorical"
	ValueTypeText        = "text"
	ValueTypeInteger     = "integer"
	ValueTypeBoolean     = "boolean"
	ValueTypeTimestamp   = "timestamp"
	ValueTypeList        = "list"
)

// Property defines a custom attribute that can be assigned to crumbs.
type Property struct {
	// PropertyID is a UUID v7, generated on creation.
	PropertyID string

	// Name is a unique human-readable name (e.g., "priority", "labels").
	Name string

	// Description is an optional explanation of the property's purpose.
	Description string

	// ValueType is the type of values this property accepts.
	// One of: categorical, text, integer, boolean, timestamp, list.
	ValueType string

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time
}

// Category defines an enumeration value for categorical properties.
type Category struct {
	// CategoryID is a UUID v7, generated on creation.
	CategoryID string

	// PropertyID is the categorical property this category belongs to.
	PropertyID string

	// Name is the display name for this category (e.g., "high", "medium").
	Name string

	// Ordinal determines display order; lower ordinals sort first.
	Ordinal int
}
