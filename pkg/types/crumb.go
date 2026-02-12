package types

import "time"

// Crumb represents a work item or task in the system.
// Implements prd003-crumbs-interface R1.
type Crumb struct {
	CrumbID    string         // UUID v7, generated on creation
	Name       string         // Human-readable name (required, non-empty)
	State      string         // Current crumb state
	CreatedAt  time.Time      // Timestamp of creation
	UpdatedAt  time.Time      // Timestamp of last modification
	Properties map[string]any // Property values keyed by property ID
}

// Crumb state constants (prd003-crumbs-interface R2).
const (
	StateDraft   = "draft"   // Being written; not yet ready for consideration
	StatePending = "pending" // Created but waiting for dependencies
	StateReady   = "ready"   // Available for assignment
	StateTaken   = "taken"   // Currently being worked on
	StatePebble  = "pebble"  // Successfully finished (permanent)
	StateDust    = "dust"    // Failed or abandoned (swept away)
)
