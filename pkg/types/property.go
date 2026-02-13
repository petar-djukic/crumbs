package types

import "time"

// Value type constants for property definitions (prd004-properties-interface R3.1).
const (
	ValueTypeCategorical = "categorical"
	ValueTypeText        = "text"
	ValueTypeInteger     = "integer"
	ValueTypeBoolean     = "boolean"
	ValueTypeTimestamp   = "timestamp"
	ValueTypeList        = "list"
)

// validValueTypes provides O(1) lookup for value type validation.
var validValueTypes = map[string]bool{
	ValueTypeCategorical: true,
	ValueTypeText:        true,
	ValueTypeInteger:     true,
	ValueTypeBoolean:     true,
	ValueTypeTimestamp:    true,
	ValueTypeList:        true,
}

// ValidValueType reports whether vt is a recognized value type.
func ValidValueType(vt string) bool {
	return validValueTypes[vt]
}

// Built-in property names seeded on first startup (prd004-properties-interface R9.1).
const (
	PropertyPriority    = "priority"
	PropertyType        = "type"
	PropertyDescription = "description"
	PropertyOwner       = "owner"
	PropertyLabels      = "labels"
)

// Property represents a property definition in the properties table
// (prd004-properties-interface R1.1). Entity methods modify the struct in
// memory; the caller must call Table.Set to persist changes.
type Property struct {
	PropertyID  string    `json:"property_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ValueType   string    `json:"value_type"`
	CreatedAt   time.Time `json:"created_at"`
}

// DefineCategory creates a new category on this property
// (prd004-properties-interface R7). It validates that the property is
// categorical (returns ErrInvalidValueType if not) and that name is non-empty
// (returns ErrInvalidName if empty). Uniqueness checking and persistence are
// performed by the backend when the category is saved via Table.Set.
//
// The cupboard parameter provides access to backend storage for persisting
// the category. Implementations retrieve the categories or properties table
// and call Table.Set to save.
func (p *Property) DefineCategory(cupboard Cupboard, name string, ordinal int) (*Category, error) {
	if p.ValueType != ValueTypeCategorical {
		return nil, ErrInvalidValueType
	}
	if name == "" {
		return nil, ErrInvalidName
	}
	cat := &Category{
		PropertyID: p.PropertyID,
		Name:       name,
		Ordinal:    ordinal,
	}
	// Persist via categories table.
	table, err := cupboard.GetTable(TableCategories)
	if err != nil {
		return nil, err
	}
	id, err := table.Set("", cat)
	if err != nil {
		return nil, err
	}
	cat.CategoryID = id
	return cat, nil
}

// GetCategories retrieves all categories for this property
// (prd004-properties-interface R8). It validates that the property is
// categorical (returns ErrInvalidValueType if not). The cupboard parameter
// provides access to backend storage for querying categories.
func (p *Property) GetCategories(cupboard Cupboard) ([]*Category, error) {
	if p.ValueType != ValueTypeCategorical {
		return nil, ErrInvalidValueType
	}
	// Retrieve categories from backend storage.
	table, err := cupboard.GetTable(TableCategories)
	if err != nil {
		return nil, err
	}
	filter := Filter{"property_id": p.PropertyID}
	entities, err := table.Fetch(filter)
	if err != nil {
		return nil, err
	}
	categories := make([]*Category, len(entities))
	for i, entity := range entities {
		categories[i] = entity.(*Category)
	}
	return categories, nil
}

// Category represents an ordered value within a categorical property
// (prd004-properties-interface R2.1).
type Category struct {
	CategoryID string `json:"category_id"`
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	Ordinal    int    `json:"ordinal"`
}
