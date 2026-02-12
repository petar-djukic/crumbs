package types

import (
	"fmt"
	"time"
)

// Stash type constants (prd008-stash-interface R2.1).
const (
	StashTypeResource = "resource"
	StashTypeArtifact = "artifact"
	StashTypeContext  = "context"
	StashTypeCounter  = "counter"
	StashTypeLock     = "lock"
)

// Stash operation constants for history entries
// (prd008-stash-interface R7.3).
const (
	StashOpCreate    = "create"
	StashOpSet       = "set"
	StashOpIncrement = "increment"
	StashOpAcquire   = "acquire"
	StashOpRelease   = "release"
)

// Stash enables crumbs on a trail to share state
// (prd008-stash-interface R1.1). Entity methods modify the struct in
// memory; callers persist via Table.Set.
type Stash struct {
	StashID       string    `json:"stash_id"`
	Name          string    `json:"name"`
	StashType     string    `json:"stash_type"`
	Value         any       `json:"value"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	LastOperation string    `json:"last_operation"`
	ChangedBy     *string   `json:"changed_by"`
}

// SetValue updates the stash value and increments the version
// (prd008-stash-interface R4.2). Cannot be used on lock-type stashes.
func (s *Stash) SetValue(value any) error {
	if s.StashType == StashTypeLock {
		return ErrInvalidStashType
	}
	s.Value = value
	s.Version++
	s.LastOperation = StashOpSet
	return nil
}

// GetValue returns the current stash value (prd008-stash-interface R4.3).
func (s *Stash) GetValue() any {
	return s.Value
}

// Increment atomically adds delta to a counter stash and returns the new
// value (prd008-stash-interface R5.2). Delta may be negative.
func (s *Stash) Increment(delta int64) (int64, error) {
	if s.StashType != StashTypeCounter {
		return 0, ErrInvalidStashType
	}
	current := int64(0)
	if s.Value != nil {
		switch v := s.Value.(type) {
		case map[string]any:
			if raw, ok := v["value"]; ok {
				current = toInt64(raw)
			}
		case int64:
			current = v
		case float64:
			current = int64(v)
		}
	}
	current += delta
	s.Value = map[string]any{"value": current}
	s.Version++
	s.LastOperation = StashOpIncrement
	return current, nil
}

// Acquire obtains a lock stash (prd008-stash-interface R6.2). The lock
// is reentrant for the same holder. Returns ErrLockHeld if held by
// another holder.
func (s *Stash) Acquire(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}
	if holder == "" {
		return ErrInvalidHolder
	}
	if s.Value != nil {
		if m, ok := s.Value.(map[string]any); ok {
			if h, ok := m["holder"].(string); ok && h != "" {
				if h == holder {
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

// Release releases a lock stash (prd008-stash-interface R6.3). Returns
// ErrNotLockHolder if the caller is not the current holder.
func (s *Stash) Release(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}
	if s.Value == nil {
		return ErrNotLockHolder
	}
	m, ok := s.Value.(map[string]any)
	if !ok {
		return ErrNotLockHolder
	}
	h, _ := m["holder"].(string)
	if h != holder {
		return ErrNotLockHolder
	}
	s.Value = nil
	s.Version++
	s.LastOperation = StashOpRelease
	return nil
}

// StashHistoryEntry records a single mutation in a stash's history
// (prd008-stash-interface R7.2). Maintained by the backend, not by
// entity methods.
type StashHistoryEntry struct {
	HistoryID string    `json:"history_id"`
	StashID   string    `json:"stash_id"`
	Version   int64     `json:"version"`
	Value     any       `json:"value"`
	Operation string    `json:"operation"`
	ChangedBy *string   `json:"changed_by"`
	CreatedAt time.Time `json:"created_at"`
}

// toInt64 converts a numeric value to int64.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		panic(fmt.Sprintf("toInt64: unexpected type %T", v))
	}
}
