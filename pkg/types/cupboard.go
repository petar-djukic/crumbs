package types

import "errors"

// Cupboard is the top-level storage interface. Callers attach to a backend,
// access tables by name, and detach when done.
// Implements: prd001-cupboard-core R2.
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
