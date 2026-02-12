package types

import "errors"

// Cupboard defines the interface for backend-agnostic storage access.
// Callers attach to a backend, access tables by name, and detach when done.
// Implements prd001-cupboard-core R2.
type Cupboard interface {
	// GetTable returns the Table for the given name.
	// Returns ErrTableNotFound if the name is not a standard table.
	GetTable(name string) (Table, error)

	// Attach connects the Cupboard to the backend described by config.
	// Creates the DataDir if it does not exist. Idempotent on first call;
	// returns ErrAlreadyAttached if called while already attached.
	Attach(config Config) error

	// Detach releases backend resources. Idempotent: multiple calls succeed.
	// After Detach, operations on tables return ErrCupboardDetached.
	Detach() error
}

// Cupboard lifecycle errors (prd001-cupboard-core R7.1).
var (
	ErrCupboardDetached = errors.New("cupboard is detached")
	ErrAlreadyAttached  = errors.New("cupboard is already attached")
	ErrTableNotFound    = errors.New("table not found")
)
