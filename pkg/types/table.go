package types

import "errors"

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
	ErrDuplicateName     = errors.New("duplicate name")
	ErrInvalidValueType  = errors.New("invalid value type")
)

// Filter is the query parameter type for Fetch operations. Keys are field names;
// values are the required field values. An empty filter returns all entities.
type Filter = map[string]any

// Table provides uniform CRUD operations for all entity types
// (prd001-cupboard-core R3.1). Get and Fetch return entity objects (Crumb,
// Trail, etc.) as any; callers type-assert to access entity-specific fields.
type Table interface {
	Get(id string) (any, error)
	Set(id string, data any) (string, error)
	Delete(id string) error
	Fetch(filter Filter) ([]any, error)
}
