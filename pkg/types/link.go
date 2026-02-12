package types

import "time"

// Link type constants (prd007-links-interface R2.2).
const (
	LinkTypeBelongsTo    = "belongs_to"
	LinkTypeChildOf      = "child_of"
	LinkTypeBranchesFrom = "branches_from"
	LinkTypeScopedTo     = "scoped_to"
)

// Link represents a directed edge between two entities in the graph.
// Implements prd007-links-interface R1.
type Link struct {
	LinkID    string    `json:"link_id"`
	LinkType  string    `json:"link_type"`
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	CreatedAt time.Time `json:"created_at"`
}
