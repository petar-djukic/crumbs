// Stash entity for shared state between crumbs in a trail.
// Implements: prd-stash-interface R1, R2, R7 (Stash, stash types, history);
//
//	docs/ARCHITECTURE § Main Interface.
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
// Stash scoping uses scoped_to links in the links table (ARCHITECTURE Decision 10).
// Global stashes have no scoped_to link.
type Stash struct {
	// StashID is a UUID v7, generated on creation.
	StashID string

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

	// LastOperation tracks the operation that caused the last mutation.
	// Used by backend for history tracking. One of StashOp* constants.
	// Set automatically by entity methods (SetValue, Increment, Acquire, Release).
	LastOperation string

	// ChangedBy records the crumb ID that made the last change (optional).
	// Used by backend for history tracking.
	ChangedBy *string
}

// SetValue updates the stash value.
// Returns ErrInvalidStashType if called on a lock-type stash.
// Increments Version. Caller must save via Table.Set.
func (s *Stash) SetValue(value any) error {
	if s.StashType == StashTypeLock {
		return ErrInvalidStashType
	}
	s.Value = value
	s.Version++
	s.LastOperation = StashOpSet
	return nil
}

// GetValue retrieves the current value.
// Returns nil if the stash has no value set.
func (s *Stash) GetValue() any {
	return s.Value
}

// Increment atomically adds delta to the counter value.
// Returns ErrInvalidStashType if the stash is not a counter type.
// Returns the new counter value. Increments Version.
// Caller must save via Table.Set.
func (s *Stash) Increment(delta int64) (int64, error) {
	if s.StashType != StashTypeCounter {
		return 0, ErrInvalidStashType
	}

	var current int64
	if s.Value != nil {
		switch v := s.Value.(type) {
		case map[string]any:
			if val, ok := v["value"]; ok {
				switch n := val.(type) {
				case int64:
					current = n
				case float64:
					current = int64(n)
				case int:
					current = int64(n)
				}
			}
		case int64:
			current = v
		case float64:
			current = int64(v)
		case int:
			current = int64(v)
		}
	}

	newVal := current + delta
	s.Value = map[string]any{"value": newVal}
	s.Version++
	s.LastOperation = StashOpIncrement
	return newVal, nil
}

// Acquire obtains the lock.
// Returns ErrInvalidStashType if the stash is not a lock type.
// Returns ErrInvalidHolder if holder is empty.
// Returns ErrLockHeld if the lock is held by another holder.
// Increments Version. Caller must save via Table.Set.
func (s *Stash) Acquire(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}
	if holder == "" {
		return ErrInvalidHolder
	}

	if s.Value != nil {
		if lockData, ok := s.Value.(map[string]any); ok {
			if currentHolder, ok := lockData["holder"].(string); ok {
				if currentHolder == holder {
					return nil // reentrant
				}
				return ErrLockHeld
			}
		}
	}

	s.Value = map[string]any{
		"holder":      holder,
		"acquired_at": time.Now().Format(time.RFC3339),
	}
	s.Version++
	s.LastOperation = StashOpAcquire
	return nil
}

// Release releases the lock.
// Returns ErrInvalidStashType if the stash is not a lock type.
// Returns ErrNotLockHolder if the lock is not held by the specified holder.
// Increments Version. Caller must save via Table.Set.
func (s *Stash) Release(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}

	if s.Value == nil {
		return ErrNotLockHolder
	}

	lockData, ok := s.Value.(map[string]any)
	if !ok {
		return ErrNotLockHolder
	}

	currentHolder, ok := lockData["holder"].(string)
	if !ok || currentHolder != holder {
		return ErrNotLockHolder
	}

	s.Value = nil
	s.Version++
	s.LastOperation = StashOpRelease
	return nil
}

// StashHistoryEntry records a change to a stash.
type StashHistoryEntry struct {
	// HistoryID is a UUID v7 of the history entry.
	HistoryID string

	// StashID is the ID of the stash this history entry belongs to.
	StashID string

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
