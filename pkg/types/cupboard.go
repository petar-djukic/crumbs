package types

import "errors"

// Cupboard provides backend-agnostic access to data tables.
// Callers attach a backend with a Config, access tables by name,
// and detach when done.
// Implements: prd001-cupboard-core R2.
type Cupboard interface {
	// GetTable returns the Table for the given name.
	// Returns ErrTableNotFound if the name is not a recognized table.
	// Returns ErrCupboardDetached if the cupboard is not attached.
	GetTable(name string) (Table, error)

	// Attach validates the config and initializes the backend.
	// Returns ErrAlreadyAttached if the cupboard is already attached.
	Attach(config Config) error

	// Detach releases backend resources.
	// Returns ErrCupboardDetached if the cupboard is not attached.
	// Blocks until in-flight operations complete.
	Detach() error
}

// Cupboard lifecycle errors (prd001-cupboard-core R7).
var (
	ErrCupboardDetached = errors.New("cupboard is detached")
	ErrAlreadyAttached  = errors.New("cupboard is already attached")
	ErrTableNotFound    = errors.New("table not found")
)
