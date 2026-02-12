package types

import "errors"

// Table provides uniform CRUD operations for all entity types. Get and Fetch
// return any; callers type-assert to the concrete entity struct (Crumb, Trail,
// Property, etc.). Set generates a UUID v7 when id is empty.
// Implements: prd001-cupboard-core R3.
type Table interface {
	Get(id string) (any, error)
	Set(id string, data any) (string, error)
	Delete(id string) error
	Fetch(filter Filter) ([]any, error)
}

// Filter is a map from field names to required values. An empty filter matches
// all entities in a table.
// Implements: prd003-crumbs-interface R9.1.
type Filter = map[string]any

// Standard table names (prd001-cupboard-core R2.5).
const (
	CrumbsTable     = "crumbs"
	TrailsTable     = "trails"
	PropertiesTable = "properties"
	MetadataTable   = "metadata"
	LinksTable      = "links"
	StashesTable    = "stashes"
)

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
