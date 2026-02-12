package types

import "time"

// Stash represents shared state scoped to a trail or global.
// Implements prd008-stash-interface R1.
type Stash struct {
	StashID       string    // UUID v7, generated on creation
	Name          string    // Human-readable name, unique within scope
	StashType     string    // Type of stash (resource, artifact, context, counter, lock)
	Value         any       // Current value (JSON blob)
	Version       int64     // Monotonically increasing version number
	CreatedAt     time.Time // Timestamp of creation
	LastOperation string    // Most recent mutation operation
	ChangedBy     *string   // Who performed the last mutation (optional)
}
