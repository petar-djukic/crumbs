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

// Trail represents an exploratory work session that groups crumbs
// (prd006-trails-interface R1.1). Crumb membership is stored via belongs_to
// links in the links table. Entity methods modify the struct in memory; the
// caller must call Table.Set to persist changes and trigger backend cascade
// operations.
type Trail struct {
	TrailID     string     `json:"trail_id"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// Complete marks the trail as finished (prd006-trails-interface R5). The
// current state must be "active"; otherwise ErrInvalidState is returned. On
// success, State is set to "completed" and CompletedAt is set to the current
// time. The caller must persist via Table.Set, at which point the backend
// removes all belongs_to links for crumbs on this trail.
func (t *Trail) Complete() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	now := time.Now()
	t.State = TrailStateCompleted
	t.CompletedAt = &now
	return nil
}

// Abandon marks the trail as discarded (prd006-trails-interface R6). The
// current state must be "active"; otherwise ErrInvalidState is returned. On
// success, State is set to "abandoned" and CompletedAt is set to the current
// time. The caller must persist via Table.Set, at which point the backend
// deletes all crumbs belonging to this trail and their associated data.
func (t *Trail) Abandon() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	now := time.Now()
	t.State = TrailStateAbandoned
	t.CompletedAt = &now
	return nil
}
