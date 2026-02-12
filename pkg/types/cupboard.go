package types

import "errors"

// Cupboard is the top-level storage interface. Applications call Attach to
// initialize a backend, GetTable to access entity tables, and Detach to
// release resources (prd001-cupboard-core R2).
type Cupboard interface {
	GetTable(name string) (Table, error)
	Attach(config Config) error
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
