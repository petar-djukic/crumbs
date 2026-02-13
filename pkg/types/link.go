package types

import "time"

// Link type constants (prd007-links-interface R2.2).
const (
	LinkTypeBelongsTo    = "belongs_to"
	LinkTypeChildOf      = "child_of"
	LinkTypeBranchesFrom = "branches_from"
	LinkTypeScopedTo     = "scoped_to"
)

// validLinkTypes provides O(1) lookup for link type validation.
var validLinkTypes = map[string]bool{
	LinkTypeBelongsTo:    true,
	LinkTypeChildOf:      true,
	LinkTypeBranchesFrom: true,
	LinkTypeScopedTo:     true,
}

// ValidLinkType reports whether lt is a recognized link type.
func ValidLinkType(lt string) bool {
	return validLinkTypes[lt]
}

// Link represents a directed edge in the entity graph
// (prd007-links-interface R1.1). Links connect crumbs to trails (belongs_to),
// crumbs to crumbs (child_of), trails to crumbs (branches_from), and stashes
// to trails (scoped_to). Links are immutable after creation.
type Link struct {
	LinkID    string    `json:"link_id"`
	LinkType  string    `json:"link_type"`
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	CreatedAt time.Time `json:"created_at"`
}
