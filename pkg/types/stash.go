package types

import "time"

// Stash type constants (prd008-stash-interface R2.1).
const (
	StashTypeResource = "resource"
	StashTypeArtifact = "artifact"
	StashTypeContext  = "context"
	StashTypeCounter  = "counter"
	StashTypeLock     = "lock"
)

// validStashTypes provides O(1) lookup for stash type validation.
var validStashTypes = map[string]bool{
	StashTypeResource: true,
	StashTypeArtifact: true,
	StashTypeContext:  true,
	StashTypeCounter:  true,
	StashTypeLock:     true,
}

// ValidStashType reports whether st is a recognized stash type.
func ValidStashType(st string) bool {
	return validStashTypes[st]
}

// Stash operation constants recorded in history entries
// (prd008-stash-interface R7.3).
const (
	StashOpCreate    = "create"
	StashOpSet       = "set"
	StashOpIncrement = "increment"
	StashOpAcquire   = "acquire"
	StashOpRelease   = "release"
)

// Stash represents shared state scoped to a trail or global
// (prd008-stash-interface R1.1). Entity methods modify the struct in memory;
// the caller must call Table.Set to persist changes.
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

// SetValue updates the stash value (prd008-stash-interface R4.2). Version is
// incremented and LastOperation set to "set". Returns ErrInvalidStashType if
// the stash is a lock (use Acquire/Release instead). The caller must persist
// via Table.Set.
func (s *Stash) SetValue(value any) error {
	if s.StashType == StashTypeLock {
		return ErrInvalidStashType
	}
	s.Value = value
	s.Version++
	s.LastOperation = StashOpSet
	return nil
}

// GetValue returns the current value (prd008-stash-interface R4.3). Returns
// nil if no value has been set.
func (s *Stash) GetValue() any {
	return s.Value
}

// Increment adds delta to a counter stash (prd008-stash-interface R5.2).
// Returns ErrInvalidStashType if the stash is not a counter. The current value
// is extracted as int64, delta is added, Version is incremented, and the new
// counter value is returned. The caller must persist via Table.Set.
func (s *Stash) Increment(delta int64) (int64, error) {
	if s.StashType != StashTypeCounter {
		return 0, ErrInvalidStashType
	}
	var current int64
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

// Acquire obtains the lock (prd008-stash-interface R6.2). Returns
// ErrInvalidStashType if not a lock, ErrInvalidHolder if holder is empty, or
// ErrLockHeld if held by another holder. Reentrant: succeeds if the same
// holder already holds the lock. Version is incremented on success. The caller
// must persist via Table.Set.
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
					return nil
				}
				return ErrLockHeld
			}
		}
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

// Release releases the lock (prd008-stash-interface R6.3). Returns
// ErrInvalidStashType if not a lock or ErrNotLockHolder if the caller is not
// the current holder. Version is incremented on success. The caller must
// persist via Table.Set.
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

// StashHistoryEntry records a single mutation to a stash
// (prd008-stash-interface R7.2). History entries are managed by the backend
// and are append-only.
type StashHistoryEntry struct {
	HistoryID string    `json:"history_id"`
	StashID   string    `json:"stash_id"`
	Version   int64     `json:"version"`
	Value     any       `json:"value"`
	Operation string    `json:"operation"`
	ChangedBy *string   `json:"changed_by"`
	CreatedAt time.Time `json:"created_at"`
}
