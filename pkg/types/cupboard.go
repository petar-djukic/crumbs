package types

import "errors"

// Cupboard is the contract between applications and storage backends.
// Applications call GetTable to obtain a Table for a specific entity type,
// Attach to initialize the backend, and Detach to release resources.
// Implements prd001-cupboard-core R2.
type Cupboard interface {
	GetTable(name string) (Table, error)
	Attach(config Config) error
	Detach() error
}

// Cupboard lifecycle errors (prd001-cupboard-core R7.1).
var (
	ErrCupboardDetached = errors.New("cupboard is detached")
	ErrAlreadyAttached  = errors.New("cupboard is already attached")
	ErrTableNotFound    = errors.New("table not found")
)
