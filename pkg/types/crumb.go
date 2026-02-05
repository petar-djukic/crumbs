// Crumb entity represents a work item in the task coordination system.
// Implements: prd-crumbs-interface R1 (Crumb struct);
//             docs/ARCHITECTURE § Main Interface.
package types

import "time"

// Crumb state values.
const (
	StateDraft     = "draft"
	StatePending   = "pending"
	StateReady     = "ready"
	StateTaken     = "taken"
	StateCompleted = "completed"
	StateFailed    = "failed"
	StateArchived  = "archived"
)

// Crumb represents a work item.
type Crumb struct {
	// CrumbID is a UUID v7, generated on creation.
	CrumbID string

	// Name is a human-readable name (required, non-empty).
	Name string

	// State is the crumb state (draft, pending, ready, taken, completed, failed, archived).
	State string

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time

	// UpdatedAt is the timestamp of last modification.
	UpdatedAt time.Time

	// Properties holds property values (property_id to value).
	Properties map[string]any
}
