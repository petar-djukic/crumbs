// Link entity represents graph edges between entities.
// Implements: prd-sqlite-backend § Graph Model, R14.6 (Link hydration);
//
//	docs/ARCHITECTURE § Main Interface.
package types

import "time"

// Link type constants (see ARCHITECTURE Decision 10).
const (
	LinkTypeBelongsTo    = "belongs_to"    // crumb → trail membership
	LinkTypeChildOf      = "child_of"      // crumb → crumb dependencies
	LinkTypeBranchesFrom = "branches_from" // trail → crumb branch point
	LinkTypeScopedTo     = "scoped_to"     // stash → trail scope
)

// Link represents a directed edge in the entity graph.
type Link struct {
	// LinkID is a UUID v7, generated on creation.
	LinkID string

	// LinkType is the relationship type (belongs_to, child_of, branches_from, scoped_to).
	LinkType string

	// FromID is the source entity ID.
	FromID string

	// ToID is the target entity ID.
	ToID string

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time
}
