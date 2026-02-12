package types

import "time"

// Property value type constants (prd004-properties-interface R3.1).
const (
	ValueTypeCategorical = "categorical"
	ValueTypeText        = "text"
	ValueTypeInteger     = "integer"
	ValueTypeBoolean     = "boolean"
	ValueTypeTimestamp   = "timestamp"
	ValueTypeList        = "list"
)

// Property defines a custom attribute that can be assigned to crumbs
// (prd004-properties-interface R1.1). Properties are created via the
// Table interface and are immutable after creation.
type Property struct {
	PropertyID  string    `json:"property_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ValueType   string    `json:"value_type"`
	CreatedAt   time.Time `json:"created_at"`
}

// Category defines an ordered enum value for categorical properties
// (prd004-properties-interface R2.1).
type Category struct {
	CategoryID string `json:"category_id"`
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	Ordinal    int    `json:"ordinal"`
}
