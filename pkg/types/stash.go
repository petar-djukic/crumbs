// Stash entity for shared state between crumbs in a trail.
// Implements: prd-stash-interface R1, R2, R7 (Stash, stash types, history);
//             docs/ARCHITECTURE § Main Interface.
package types

import "time"

// Stash type constants.
const (
	StashTypeResource = "resource"
	StashTypeArtifact = "artifact"
	StashTypeContext  = "context"
	StashTypeCounter  = "counter"
	StashTypeLock     = "lock"
)

// Stash history operation constants.
const (
	StashOpCreate    = "create"
	StashOpSet       = "set"
	StashOpIncrement = "increment"
	StashOpAcquire   = "acquire"
	StashOpRelease   = "release"
)

// Stash represents shared state scoped to a trail or global.
type Stash struct {
	// StashID is a UUID v7, generated on creation.
	StashID string

	// TrailID is the trail scope; nil for global stashes.
	TrailID *string

	// Name is a human-readable name, unique within scope.
	Name string

	// StashType is the type of stash (resource, artifact, context, counter, lock).
	StashType string

	// Value is the current value (JSON blob); structure depends on StashType.
	Value any

	// Version is a monotonically increasing version number.
	Version int64

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time
}

// StashHistoryEntry records a change to a stash.
type StashHistoryEntry struct {
	// HistoryID is a UUID v7 of the history entry.
	HistoryID string

	// Version is the version number after this change.
	Version int64

	// Value is the value after this change.
	Value any

	// Operation is the operation that caused this change.
	// One of: create, set, increment, acquire, release.
	Operation string

	// ChangedBy is the crumb ID that made the change (nullable).
	ChangedBy *string

	// CreatedAt is the timestamp of this change.
	CreatedAt time.Time
}
