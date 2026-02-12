package types

import "time"

// Trail state constants (prd006-trails-interface R2).
const (
	TrailStateDraft     = "draft"
	TrailStatePending   = "pending"
	TrailStateActive    = "active"
	TrailStateCompleted = "completed"
	TrailStateAbandoned = "abandoned"
)

// Trail represents an exploration session that groups crumbs.
// Implements prd006-trails-interface R1.
type Trail struct {
	TrailID     string     `json:"trail_id"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Complete marks the trail as finished. Crumbs become permanent when
// the trail is persisted via Table.Set (backend removes belongs_to links).
// Returns ErrInvalidState if the trail is not in active state.
// Implements prd006-trails-interface R5.
func (t *Trail) Complete() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateCompleted
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// Abandon marks the trail as discarded. Crumbs are deleted when
// the trail is persisted via Table.Set (backend cascades deletion).
// Returns ErrInvalidState if the trail is not in active state.
// Implements prd006-trails-interface R6.
func (t *Trail) Abandon() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateAbandoned
	now := time.Now()
	t.CompletedAt = &now
	return nil
}
