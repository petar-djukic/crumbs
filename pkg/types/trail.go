package types

import "time"

// Trail represents an exploration session that groups crumbs.
// Implements prd006-trails-interface R1.
type Trail struct {
	TrailID     string     // UUID v7, generated on creation
	State       string     // Current trail state
	CreatedAt   time.Time  // Timestamp of creation
	CompletedAt *time.Time // Timestamp when completed or abandoned; nil if active
}

// Trail state constants (prd006-trails-interface R2).
const (
	TrailDraft     = "draft"     // Being planned; crumbs not yet committed
	TrailPending   = "pending"   // Defined but waiting for precondition
	TrailActive    = "active"    // Open for work; crumbs can be added
	TrailCompleted = "completed" // Finished; crumbs made permanent
	TrailAbandoned = "abandoned" // Discarded; crumbs deleted
)
