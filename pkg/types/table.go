package types

import "errors"

// Filter is a map of field names to required values used by Table.Fetch
// to select matching entities. An empty filter returns all entities.
type Filter = map[string]any

// Table provides uniform CRUD operations for all entity types stored in
// a cupboard. Callers use type assertions on the returned values to
// access entity-specific fields.
// Implements: prd001-cupboard-core R3.
type Table interface {
	// Get retrieves an entity by ID. It returns ErrNotFound if no entity
	// with that ID exists.
	Get(id string) (any, error)

	// Set persists an entity. When id is empty a new UUID v7 is generated.
	// When id is provided the entity is updated or created. The returned
	// string is the actual ID used (generated or provided).
	Set(id string, data any) (string, error)

	// Delete removes an entity by ID. It returns ErrNotFound if the
	// entity does not exist.
	Delete(id string) error

	// Fetch queries entities matching the filter. Filter keys are field
	// names and values are the required field values. An empty filter
	// returns all entities in the table.
	Fetch(filter Filter) ([]any, error)
}

// Table operation errors (prd001-cupboard-core R7.2).
var (
	ErrNotFound    = errors.New("entity not found")
	ErrInvalidID   = errors.New("invalid entity ID")
	ErrInvalidData = errors.New("invalid entity data")
)

// Entity method errors (prd001-cupboard-core R7.3).
var (
	ErrInvalidState      = errors.New("invalid state value")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrInvalidName       = errors.New("invalid name")
	ErrPropertyNotFound  = errors.New("property not found")
	ErrTypeMismatch      = errors.New("type mismatch")
	ErrInvalidCategory   = errors.New("invalid category")
	ErrInvalidStashType  = errors.New("invalid stash type or operation")
	ErrLockHeld          = errors.New("lock is held")
	ErrNotLockHolder     = errors.New("caller is not the lock holder")
	ErrInvalidHolder     = errors.New("holder cannot be empty")
	ErrAlreadyInTrail    = errors.New("crumb already belongs to a trail")
	ErrNotInTrail        = errors.New("crumb does not belong to the trail")
	ErrSchemaNotFound    = errors.New("schema not found")
	ErrInvalidContent    = errors.New("content must not be empty")
	ErrInvalidFilter     = errors.New("invalid filter value type")
)
