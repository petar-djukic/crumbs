package types

import "time"

// Link type constants define the kinds of relationships between entities.
// Implements: prd007-links-interface R2.
const (
	LinkTypeBelongsTo    = "belongs_to"    // Crumb membership in a trail.
	LinkTypeChildOf      = "child_of"      // Crumb dependency (child blocked until parent is pebble).
	LinkTypeBranchesFrom = "branches_from" // Trail branch point from a crumb.
	LinkTypeScopedTo     = "scoped_to"     // Stash scoped to a trail.
)

// Link represents a typed relationship between two entities.
// Implements: prd007-links-interface R1.
type Link struct {
	LinkID    string    // UUID v7, generated on creation.
	LinkType  string    // Relationship type (one of the LinkType constants).
	FromID    string    // Source entity ID.
	ToID      string    // Target entity ID.
	CreatedAt time.Time // Timestamp of creation.
}
