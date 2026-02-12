package types

import "time"

// Link represents a typed relationship between two entities.
// Implements prd007-links-interface R1.
type Link struct {
	LinkID    string    // UUID v7, generated on creation
	LinkType  string    // Relationship type (see link type constants)
	FromID    string    // Source entity ID
	ToID      string    // Target entity ID
	CreatedAt time.Time // Timestamp of creation
}

// Link type constants (prd007-links-interface R2).
const (
	LinkBelongsTo    = "belongs_to"    // Crumb → Trail membership
	LinkChildOf      = "child_of"      // Crumb → Crumb dependency (DAG)
	LinkBranchesFrom = "branches_from" // Trail → Crumb branch point
	LinkScopedTo     = "scoped_to"     // Stash → Trail scope
)
