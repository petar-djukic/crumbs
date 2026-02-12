package types

import "errors"

// Table defines the uniform CRUD interface for all entity tables.
// Callers use Get, Set, Delete, and Fetch with type assertions on the
// returned values.
// Implements prd001-cupboard-core R3.
type Table interface {
	// Get retrieves the entity with the given ID.
	// Returns ErrNotFound if no entity exists with that ID.
	Get(id string) (any, error)

	// Set creates or updates an entity. When id is empty, the backend
	// generates a UUID v7. Returns the actual ID used (generated or provided).
	Set(id string, data any) (string, error)

	// Delete removes the entity with the given ID.
	// Returns ErrNotFound if no entity exists with that ID.
	Delete(id string) error

	// Fetch returns entities matching the filter criteria.
	// Filter keys are ANDed. An empty filter returns all entities.
	Fetch(filter map[string]any) ([]any, error)
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
