// Trail entity represents an exploratory work session.
// Implements: prd-trails-interface R1, R2 (Trail struct, state values);
//
//	docs/ARCHITECTURE § Main Interface.
package types

import "time"

// Trail state values.
const (
	TrailStateActive    = "active"
	TrailStateCompleted = "completed"
	TrailStateAbandoned = "abandoned"
)

// Trail represents an exploratory work session that groups crumbs.
type Trail struct {
	// TrailID is a UUID v7, generated on creation.
	TrailID string

	// ParentCrumbID is an optional crumb ID this trail deviates from; nil if standalone.
	// TODO(crumbs-wy7.11): Migrate to branches_from link per Decision 10.
	ParentCrumbID *string

	// State is the trail state (active, completed, abandoned).
	State string

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time

	// CompletedAt is the timestamp when completed or abandoned; nil if active.
	CompletedAt *time.Time
}

// Complete marks the trail as completed.
// Returns ErrInvalidState if the trail is not in active state.
// Sets CompletedAt to now. Caller must save via Table.Set.
// When persisted, the backend removes belongs_to links so crumbs become permanent.
func (t *Trail) Complete() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateCompleted
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// Abandon marks the trail as abandoned.
// Returns ErrInvalidState if the trail is not in active state.
// Sets CompletedAt to now. Caller must save via Table.Set.
// When persisted, the backend deletes all crumbs belonging to this trail.
func (t *Trail) Abandon() error {
	if t.State != TrailStateActive {
		return ErrInvalidState
	}
	t.State = TrailStateAbandoned
	now := time.Now()
	t.CompletedAt = &now
	return nil
}
