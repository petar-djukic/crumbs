package types

import (
	"fmt"
	"time"
)

// Stash type constants.
// Implements: prd008-stash-interface R2.
const (
	StashTypeResource = "resource"
	StashTypeArtifact = "artifact"
	StashTypeContext  = "context"
	StashTypeCounter  = "counter"
	StashTypeLock     = "lock"
)

// validStashTypes is the set of recognized stash types.
var validStashTypes = map[string]bool{
	StashTypeResource: true,
	StashTypeArtifact: true,
	StashTypeContext:  true,
	StashTypeCounter:  true,
	StashTypeLock:     true,
}

// Stash operation constants for history tracking.
// Implements: prd008-stash-interface R6.
const (
	StashOpCreate    = "create"
	StashOpSet       = "set"
	StashOpIncrement = "increment"
	StashOpAcquire   = "acquire"
	StashOpRelease   = "release"
)

// Stash represents shared state for trails and crumbs.
// Implements: prd008-stash-interface R1.
type Stash struct {
	StashID       string    // UUID v7, generated on creation.
	Name          string    // Human-readable name, unique within scope.
	StashType     string    // One of the StashType constants.
	Value         any       // Current value (JSON-serializable).
	Version       int64     // Monotonically increasing version number.
	CreatedAt     time.Time // Timestamp of creation.
	LastOperation string    // Most recent mutation operation.
	ChangedBy     *string   // Who performed the last mutation (optional).
}

// StashHistoryEntry records a single change to a stash.
// Implements: prd008-stash-interface R6.
type StashHistoryEntry struct {
	HistoryID string    // UUID v7 of the history entry.
	StashID   string    // ID of the stash.
	Version   int64     // Version number after this change.
	Value     any       // Value after this change.
	Operation string    // Operation that caused this change.
	ChangedBy *string   // Crumb ID that made the change (nullable).
	CreatedAt time.Time // Timestamp of this change.
}

// IsValidStashType reports whether the given string is a recognized stash type.
func IsValidStashType(st string) bool {
	return validStashTypes[st]
}

// SetValue updates the stash value.
// Returns ErrInvalidStashType if called on a lock-type stash.
// The caller must persist via Table.Set after calling this method.
// Implements: prd008-stash-interface R4.1.
func (s *Stash) SetValue(value any) error {
	if s.StashType == StashTypeLock {
		return ErrInvalidStashType
	}
	s.Value = value
	s.Version++
	s.LastOperation = StashOpSet
	return nil
}

// GetValue returns the current stash value. Returns nil if no value is set.
// Implements: prd008-stash-interface R4.2.
func (s *Stash) GetValue() any {
	return s.Value
}

// Increment atomically adds delta to a counter stash and returns the new value.
// Returns ErrInvalidStashType if the stash is not a counter.
// The caller must persist via Table.Set after calling this method.
// Implements: prd008-stash-interface R4.3.
func (s *Stash) Increment(delta int64) (int64, error) {
	if s.StashType != StashTypeCounter {
		return 0, ErrInvalidStashType
	}
	current := counterValue(s.Value)
	newVal := current + delta
	s.Value = map[string]any{"value": newVal}
	s.Version++
	s.LastOperation = StashOpIncrement
	return newVal, nil
}

// Acquire attempts to acquire a lock stash.
// Returns ErrInvalidStashType if the stash is not a lock.
// Returns ErrInvalidHolder if holder is empty.
// Returns ErrLockHeld if the lock is held by a different holder.
// Reentrant: succeeds if already held by the same holder.
// The caller must persist via Table.Set after calling this method.
// Implements: prd008-stash-interface R4.4.
func (s *Stash) Acquire(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}
	if holder == "" {
		return ErrInvalidHolder
	}
	currentHolder := lockHolder(s.Value)
	if currentHolder != "" && currentHolder != holder {
		return ErrLockHeld
	}
	now := time.Now()
	s.Value = map[string]any{
		"holder":      holder,
		"acquired_at": now.Format(time.RFC3339),
	}
	s.Version++
	s.LastOperation = StashOpAcquire
	return nil
}

// Release releases a lock stash held by the specified holder.
// Returns ErrInvalidStashType if the stash is not a lock.
// Returns ErrNotLockHolder if the lock is not held by the specified holder.
// The caller must persist via Table.Set after calling this method.
// Implements: prd008-stash-interface R4.5.
func (s *Stash) Release(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}
	currentHolder := lockHolder(s.Value)
	if currentHolder != holder {
		return ErrNotLockHolder
	}
	s.Value = nil
	s.Version++
	s.LastOperation = StashOpRelease
	return nil
}

// counterValue extracts the int64 counter value from a stash Value.
// Returns 0 if the value is nil or not in the expected format.
func counterValue(v any) int64 {
	if v == nil {
		return 0
	}
	m, ok := v.(map[string]any)
	if !ok {
		return 0
	}
	raw, ok := m["value"]
	if !ok {
		return 0
	}
	switch n := raw.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}

// lockHolder extracts the holder string from a lock stash Value.
// Returns "" if the value is nil or the lock is not held.
func lockHolder(v any) string {
	if v == nil {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	h, ok := m["holder"]
	if !ok {
		return ""
	}
	s, ok := h.(string)
	if !ok {
		return ""
	}
	return s
}

// String returns a human-readable representation of the stash.
func (s *Stash) String() string {
	return fmt.Sprintf("Stash{ID:%s Name:%s Type:%s Version:%d}", s.StashID, s.Name, s.StashType, s.Version)
}
