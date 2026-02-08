// Table interface defines uniform CRUD operations for all entity types.
// Implements: prd-cupboard-core R3;
//
//	docs/ARCHITECTURE § Table Interfaces.
package types

import "errors"

// Table operation errors.
var (
	ErrNotFound    = errors.New("entity not found")
	ErrInvalidID   = errors.New("invalid entity ID")
	ErrInvalidData = errors.New("invalid entity data")
)

// Entity method errors.
var (
	ErrInvalidState      = errors.New("invalid state value")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrInvalidName       = errors.New("invalid name")
	ErrDuplicateName     = errors.New("duplicate name")
	ErrInvalidValueType  = errors.New("invalid value type")
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

// Table provides uniform CRUD operations for all entity types.
// All entity types (Crumb, Trail, Property, etc.) are accessed through
// this interface. Callers use type assertions to access entity-specific fields.
type Table interface {
	// Get retrieves an entity by its ID.
	// Returns the entity object or ErrNotFound if not found.
	// Returns ErrCupboardDetached if the cupboard is detached.
	Get(id string) (any, error)

	// Set persists an entity object.
	// If id is empty, generates a new UUID v7 and creates the entity.
	// If id is provided, updates the existing entity or creates if not found.
	// Returns the actual ID (generated or provided).
	Set(id string, data any) (string, error)

	// Delete removes an entity by ID.
	// Returns ErrNotFound if the entity does not exist.
	// Returns ErrCupboardDetached if the cupboard is detached.
	Delete(id string) error

	// Fetch queries entities matching the filter.
	// Filter keys are field names; values are required field values.
	// An empty filter returns all entities in the table.
	// Returns ErrCupboardDetached if the cupboard is detached.
	Fetch(filter map[string]any) ([]any, error)
}
