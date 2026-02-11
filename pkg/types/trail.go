package types

import "time"

// Trail states. A trail progresses from draft through active to a terminal state.
// Implements: prd006-trails-interface R2.
const (
	TrailStateDraft     = "draft"
	TrailStatePending   = "pending"
	TrailStateActive    = "active"
	TrailStateCompleted = "completed"
	TrailStateAbandoned = "abandoned"
)

// Trail represents an exploration session grouping related crumbs.
// Implements: prd006-trails-interface R1.
type Trail struct {
	TrailID     string     // UUID v7, generated on creation.
	State       string     // Current state (one of the TrailState constants).
	CreatedAt   time.Time  // Timestamp of creation.
	CompletedAt *time.Time // Timestamp when completed or abandoned; nil if active.
}

// Complete marks the trail as finished. Crumb membership links are
// removed when the backend persists this change.
// Returns ErrInvalidState if the trail is not in "active" state.
// Implements: prd006-trails-interface R3.1.
func (t *Trail) Complete() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateCompleted
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// Abandon discards the trail. All associated crumbs, their properties,
// metadata, and links are deleted when the backend persists this change.
// Returns ErrInvalidState if the trail is not in "active" state.
// Implements: prd006-trails-interface R3.2.
func (t *Trail) Abandon() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateAbandoned
	now := time.Now()
	t.CompletedAt = &now
	return nil
}
