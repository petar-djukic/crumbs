package types

import "errors"

// Cupboard provides backend-agnostic access to named tables.
// Callers attach to a backend, access tables by name, and detach
// when finished. Implements prd001-cupboard-core R2.
type Cupboard interface {
	// GetTable returns the Table for the given name.
	// Returns ErrTableNotFound if the name is not a standard table.
	GetTable(name string) (Table, error)

	// Attach connects the Cupboard to the backend described by config.
	// Returns ErrAlreadyAttached if already connected.
	Attach(config Config) error

	// Detach releases backend resources.
	// Returns ErrCupboardDetached if not currently attached.
	Detach() error
}

// Standard table names (prd001-cupboard-core R2.5).
const (
	TableCrumbs     = "crumbs"
	TableTrails     = "trails"
	TableProperties = "properties"
	TableMetadata   = "metadata"
	TableLinks      = "links"
	TableStashes    = "stashes"
)

// Cupboard lifecycle errors (prd001-cupboard-core R7.1).
var (
	ErrCupboardDetached = errors.New("cupboard is detached")
	ErrAlreadyAttached  = errors.New("cupboard is already attached")
	ErrTableNotFound    = errors.New("table not found")
)
