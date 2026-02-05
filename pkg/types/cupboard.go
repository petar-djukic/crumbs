// Cupboard interface defines the contract for storage access and lifecycle management.
// Implements: prd-cupboard-core R2, R4, R5, R6, R7;
//
//	docs/ARCHITECTURE § Main Interface.
package types

import "errors"

// Standard table names used by the system.
const (
	CrumbsTable     = "crumbs"
	TrailsTable     = "trails"
	PropertiesTable = "properties"
	MetadataTable   = "metadata"
	LinksTable      = "links"
	StashesTable    = "stashes"
)

// Cupboard lifecycle and table access errors.
var (
	ErrCupboardDetached = errors.New("cupboard is detached")
	ErrAlreadyAttached  = errors.New("cupboard is already attached")
	ErrTableNotFound    = errors.New("table not found")
)

// Cupboard provides storage access and lifecycle management.
// Applications obtain a Cupboard instance, call Attach with configuration,
// access tables via GetTable, and call Detach when done.
type Cupboard interface {
	// GetTable returns a Table interface for the specified table name.
	// Standard table names are CrumbsTable, TrailsTable, PropertiesTable,
	// MetadataTable, LinksTable, and StashesTable.
	// Returns ErrTableNotFound if the table name is not recognized.
	// Returns ErrCupboardDetached if called after Detach.
	GetTable(name string) (Table, error)

	// Attach initializes the backend connection with the given configuration.
	// Config must be valid (call Config.Validate first or Attach validates internally).
	// Returns ErrAlreadyAttached if called on an already-attached cupboard.
	// Returns an error if backend initialization fails.
	Attach(config Config) error

	// Detach releases all resources held by the cupboard.
	// After Detach, all operations return ErrCupboardDetached.
	// Detach is idempotent; calling it multiple times does not error.
	// Detach blocks until in-flight operations complete or timeout.
	Detach() error
}
