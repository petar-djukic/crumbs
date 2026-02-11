package types

import "time"

// Property value types determine what values a property accepts.
// Implements: prd004-properties-interface R2.
const (
	ValueTypeCategorical = "categorical"
	ValueTypeText        = "text"
	ValueTypeInteger     = "integer"
	ValueTypeBoolean     = "boolean"
	ValueTypeTimestamp   = "timestamp"
	ValueTypeList        = "list"
)

// validValueTypes is the set of recognized property value types.
var validValueTypes = map[string]bool{
	ValueTypeCategorical: true,
	ValueTypeText:        true,
	ValueTypeInteger:     true,
	ValueTypeBoolean:     true,
	ValueTypeTimestamp:   true,
	ValueTypeList:        true,
}

// Property defines a named attribute that can be set on crumbs.
// Implements: prd004-properties-interface R1.
type Property struct {
	PropertyID  string    // UUID v7, generated on creation.
	Name        string    // Unique human-readable name (required, non-empty).
	Description string    // Optional explanation of the property's purpose.
	ValueType   string    // One of the ValueType constants.
	CreatedAt   time.Time // Timestamp of creation.
}

// Category is a valid option for a categorical property.
// Implements: prd004-properties-interface R3.
type Category struct {
	CategoryID string // UUID v7, generated on creation.
	PropertyID string // The categorical property this category belongs to.
	Name       string // Display name for this category.
	Ordinal    int    // Sort order; lower ordinals sort first.
}

// DefaultValue returns the type-based default value for a given ValueType.
// Returns nil for categorical and timestamp types, "" for text, 0 for integer,
// false for boolean, and an empty string slice for list.
// Returns nil and ErrInvalidValueType if the type is not recognized.
// Implements: prd004-properties-interface R4.
func DefaultValue(valueType string) (any, error) {
	switch valueType {
	case ValueTypeCategorical:
		return nil, nil
	case ValueTypeText:
		return "", nil
	case ValueTypeInteger:
		return int64(0), nil
	case ValueTypeBoolean:
		return false, nil
	case ValueTypeTimestamp:
		return nil, nil
	case ValueTypeList:
		return []string{}, nil
	default:
		return nil, ErrInvalidValueType
	}
}

// IsValidValueType reports whether the given string is a recognized value type.
func IsValidValueType(vt string) bool {
	return validValueTypes[vt]
}

// DefineCategory creates a new category for this property. The property
// must have ValueType "categorical". The category is not persisted until
// the caller saves it via the backend.
// Returns ErrInvalidValueType if the property is not categorical.
// Returns ErrInvalidName if name is empty.
// Implements: prd004-properties-interface R7.
func (p *Property) DefineCategory(cupboard Cupboard, name string, ordinal int) (*Category, error) {
	if p.ValueType != ValueTypeCategorical {
		return nil, ErrInvalidValueType
	}
	if name == "" {
		return nil, ErrInvalidName
	}
	// Duplicate name validation and persistence happen via the backend.
	// The caller uses cupboard.GetTable(TableProperties) to persist.
	cat := &Category{
		PropertyID: p.PropertyID,
		Name:       name,
		Ordinal:    ordinal,
	}
	return cat, nil
}

// GetCategories retrieves all categories for this property from the backend.
// The property must have ValueType "categorical".
// Returns ErrInvalidValueType if the property is not categorical.
// Categories are ordered by ordinal ascending, then name ascending.
// Returns an empty slice (not nil) if no categories are defined.
// Implements: prd004-properties-interface R8.
func (p *Property) GetCategories(cupboard Cupboard) ([]*Category, error) {
	if p.ValueType != ValueTypeCategorical {
		return nil, ErrInvalidValueType
	}
	tbl, err := cupboard.GetTable(TableProperties)
	if err != nil {
		return nil, err
	}
	results, err := tbl.Fetch(map[string]any{"property_id": p.PropertyID})
	if err != nil {
		return nil, err
	}
	categories := make([]*Category, 0, len(results))
	for _, r := range results {
		if cat, ok := r.(*Category); ok {
			categories = append(categories, cat)
		}
	}
	return categories, nil
}
