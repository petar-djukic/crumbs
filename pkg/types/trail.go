package types

import "time"

// Trail state constants (prd006-trails-interface R2.1).
const (
	TrailStateDraft     = "draft"
	TrailStatePending   = "pending"
	TrailStateActive    = "active"
	TrailStateCompleted = "completed"
	TrailStateAbandoned = "abandoned"
)

// Trail represents an exploratory session that groups crumbs
// (prd006-trails-interface R1.1). Entity methods modify the struct in
// memory; callers persist via Table.Set, at which point the backend
// performs cascade operations.
type Trail struct {
	TrailID     string     `json:"trail_id"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// Complete marks the trail as finished. The trail must be in the active
// state (prd006-trails-interface R5.2–R5.4). When persisted via Table.Set,
// the backend removes all belongs_to links for crumbs on this trail.
func (t *Trail) Complete() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateCompleted
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// Abandon marks the trail as discarded. The trail must be in the active
// state (prd006-trails-interface R6.2–R6.4). When persisted via Table.Set,
// the backend deletes all crumbs that belong to this trail.
func (t *Trail) Abandon() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateAbandoned
	now := time.Now()
	t.CompletedAt = &now
	return nil
}
