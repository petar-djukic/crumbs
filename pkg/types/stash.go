package types

import "time"

// Stash type constants (prd008-stash-interface R2).
const (
	StashTypeResource = "resource"
	StashTypeArtifact = "artifact"
	StashTypeContext  = "context"
	StashTypeCounter  = "counter"
	StashTypeLock     = "lock"
)

// Stash operation constants for history tracking (prd008-stash-interface R7.3).
const (
	StashOpCreate    = "create"
	StashOpSet       = "set"
	StashOpIncrement = "increment"
	StashOpAcquire   = "acquire"
	StashOpRelease   = "release"
)

// Stash holds shared state scoped to a trail or globally accessible.
// Implements prd008-stash-interface R1.
type Stash struct {
	StashID       string    `json:"stash_id"`
	Name          string    `json:"name"`
	StashType     string    `json:"stash_type"`
	Value         any       `json:"value"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	LastOperation string    `json:"last_operation"`
	ChangedBy     *string   `json:"changed_by,omitempty"`
}

// SetValue updates the stash value and increments the version.
// Returns ErrInvalidStashType if called on a lock-type stash.
// Implements prd008-stash-interface R4.
func (s *Stash) SetValue(value any) error {
	if s.StashType == StashTypeLock {
		return ErrInvalidStashType
	}
	s.Value = value
	s.Version++
	s.LastOperation = StashOpSet
	return nil
}

// GetValue returns the current stash value.
// Implements prd008-stash-interface R4.3.
func (s *Stash) GetValue() any {
	return s.Value
}

// Increment adds delta to a counter stash and returns the new value.
// Returns ErrInvalidStashType if the stash is not a counter.
// Implements prd008-stash-interface R5.
func (s *Stash) Increment(delta int64) (int64, error) {
	if s.StashType != StashTypeCounter {
		return 0, ErrInvalidStashType
	}
	current := int64(0)
	if m, ok := s.Value.(map[string]any); ok {
		if v, ok := m["value"]; ok {
			switch n := v.(type) {
			case int64:
				current = n
			case float64:
				current = int64(n)
			}
		}
	}
	current += delta
	s.Value = map[string]any{"value": current}
	s.Version++
	s.LastOperation = StashOpIncrement
	return current, nil
}

// Acquire obtains the lock for the given holder.
// Returns ErrInvalidStashType if not a lock, ErrInvalidHolder if holder
// is empty, ErrLockHeld if held by another holder. Reentrant for same holder.
// Implements prd008-stash-interface R6.2.
func (s *Stash) Acquire(holder string) error {
	if s.StashType != StashTypeLock {
		return ErrInvalidStashType
	}
	if holder == "" {
		return ErrInvalidHolder
	}
	if s.Value != nil {
		if m, ok := s.Value.(map[string]any); ok {
			if h, ok := m["holder"].(string); ok && h == holder {
				return nil
			}
			return ErrLockHeld
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

// Release releases the lock held by the given holder.
// Returns ErrInvalidStashType if not a lock, ErrNotLockHolder if
// the lock is not held by the specified holder.
// Implements prd008-stash-interface R6.3.
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
	h, ok := m["holder"].(string)
	if !ok || h != holder {
		return ErrNotLockHolder
	}
	s.Value = nil
	s.Version++
	s.LastOperation = StashOpRelease
	return nil
}

// StashHistoryEntry records a single mutation in a stash's history.
// Implements prd008-stash-interface R7.2.
type StashHistoryEntry struct {
	HistoryID string    `json:"history_id"`
	StashID   string    `json:"stash_id"`
	Version   int64     `json:"version"`
	Value     any       `json:"value"`
	Operation string    `json:"operation"`
	ChangedBy *string   `json:"changed_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
