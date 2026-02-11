package types

import "errors"

// Table provides uniform CRUD operations for any entity type.
// Get and Fetch return concrete entity structs (e.g. *Crumb, *Trail)
// as any; callers type-assert to the appropriate type.
// Implements: prd001-cupboard-core R3.
type Table interface {
	// Get retrieves an entity by ID.
	// Returns ErrNotFound if the entity does not exist.
	// Returns ErrInvalidID if id is empty.
	Get(id string) (any, error)

	// Set creates or updates an entity. If id is empty, a new UUID v7
	// is generated and the entity is created. If id is provided, the
	// entity is updated (or created if not found).
	// Returns the entity ID (generated or provided).
	Set(id string, data any) (string, error)

	// Delete removes an entity by ID.
	// Returns ErrNotFound if the entity does not exist.
	// Returns ErrInvalidID if id is empty.
	Delete(id string) error

	// Fetch returns entities matching the filter. Multiple filter keys
	// are ANDed. An empty or nil filter matches all entities.
	// Returns ErrInvalidFilter if a filter value has the wrong type.
	Fetch(filter map[string]any) ([]any, error)
}

// Table operation errors (prd001-cupboard-core R7).
var (
	ErrNotFound      = errors.New("entity not found")
	ErrInvalidID     = errors.New("invalid entity ID")
	ErrInvalidData   = errors.New("invalid entity data")
	ErrInvalidFilter = errors.New("invalid filter value type")
)

// Entity method errors shared across entity types (prd001-cupboard-core R7).
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
	ErrDuplicateName     = errors.New("name already exists")
	ErrInvalidValueType  = errors.New("invalid value type")
)
