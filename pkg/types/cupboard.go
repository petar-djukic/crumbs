package types

import "errors"

// Standard table names (prd001-cupboard-core R2.5).
const (
	TableCrumbs     = "crumbs"
	TableTrails     = "trails"
	TableProperties = "properties"
	TableMetadata   = "metadata"
	TableLinks      = "links"
	TableStashes    = "stashes"
)

// Cupboard defines the contract for storage access and lifecycle management.
// Callers use Attach to initialize a backend, GetTable to access a named
// table, and Detach to release resources.
// Implements: prd001-cupboard-core R2.
type Cupboard interface {
	// GetTable returns the Table for the given name. It returns
	// ErrTableNotFound if the name is not a recognized standard table.
	GetTable(name string) (Table, error)

	// Attach validates the config and initializes the backend connection.
	// It returns ErrAlreadyAttached if the cupboard is already attached.
	Attach(config Config) error

	// Detach releases all resources held by the cupboard. It is
	// idempotent and blocks until in-flight operations complete.
	Detach() error
}

// Cupboard lifecycle errors (prd001-cupboard-core R7.1).
var (
	ErrCupboardDetached = errors.New("cupboard is detached")
	ErrAlreadyAttached  = errors.New("cupboard is already attached")
	ErrTableNotFound    = errors.New("table not found")
)
