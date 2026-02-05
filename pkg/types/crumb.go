// Crumb entity represents a work item in the task coordination system.
// Implements: prd-crumbs-interface R1, R4, R5 (Crumb struct, state methods, property methods);
//
//	docs/ARCHITECTURE § Main Interface.
package types

import (
	"slices"
	"time"
)

// Crumb state values.
const (
	StateDraft     = "draft"
	StatePending   = "pending"
	StateReady     = "ready"
	StateTaken     = "taken"
	StateCompleted = "completed"
	StateFailed    = "failed"
	StateArchived  = "archived"
)

// Crumb represents a work item.
type Crumb struct {
	// CrumbID is a UUID v7, generated on creation.
	CrumbID string

	// Name is a human-readable name (required, non-empty).
	Name string

	// State is the crumb state (draft, pending, ready, taken, completed, failed, archived).
	State string

	// CreatedAt is the timestamp of creation.
	CreatedAt time.Time

	// UpdatedAt is the timestamp of last modification.
	UpdatedAt time.Time

	// Properties holds property values (property_id to value).
	Properties map[string]any
}

// validCrumbStates lists all valid crumb state values.
var validCrumbStates = []string{
	StateDraft, StatePending, StateReady, StateTaken,
	StateCompleted, StateFailed, StateArchived,
}

// SetState transitions the crumb to the specified state.
// Returns ErrInvalidState if the state is not recognized.
// Updates UpdatedAt. Caller must save via Table.Set.
func (c *Crumb) SetState(state string) error {
	if !slices.Contains(validCrumbStates, state) {
		return ErrInvalidState
	}
	c.State = state
	c.UpdatedAt = time.Now()
	return nil
}

// Complete transitions the crumb to the completed state.
// Returns ErrInvalidTransition if current state is not taken.
// Updates UpdatedAt. Caller must save via Table.Set.
func (c *Crumb) Complete() error {
	if c.State != StateTaken {
		return ErrInvalidTransition
	}
	c.State = StateCompleted
	c.UpdatedAt = time.Now()
	return nil
}

// Archive transitions the crumb to the archived state (soft delete).
// Can be called from any state. Idempotent.
// Updates UpdatedAt. Caller must save via Table.Set.
func (c *Crumb) Archive() error {
	c.State = StateArchived
	c.UpdatedAt = time.Now()
	return nil
}

// Fail transitions the crumb to the failed state.
// Returns ErrInvalidTransition if current state is not taken.
// Updates UpdatedAt. Caller must save via Table.Set.
func (c *Crumb) Fail() error {
	if c.State != StateTaken {
		return ErrInvalidTransition
	}
	c.State = StateFailed
	c.UpdatedAt = time.Now()
	return nil
}

// SetProperty assigns a value to a property.
// Initializes Properties map if nil.
// Updates UpdatedAt. Caller must save via Table.Set.
// Note: Type validation is deferred to Table.Set per PRD R5.7.
func (c *Crumb) SetProperty(propertyID string, value any) error {
	if c.Properties == nil {
		c.Properties = make(map[string]any)
	}
	c.Properties[propertyID] = value
	c.UpdatedAt = time.Now()
	return nil
}

// GetProperty retrieves a single property value.
// Returns ErrPropertyNotFound if the property does not exist.
func (c *Crumb) GetProperty(propertyID string) (any, error) {
	if c.Properties == nil {
		return nil, ErrPropertyNotFound
	}
	value, ok := c.Properties[propertyID]
	if !ok {
		return nil, ErrPropertyNotFound
	}
	return value, nil
}

// GetProperties retrieves all property values.
// Returns an empty map if no properties are set.
func (c *Crumb) GetProperties() map[string]any {
	if c.Properties == nil {
		return make(map[string]any)
	}
	return c.Properties
}

// ClearProperty resets a property to nil (removes it from the map).
// Returns ErrPropertyNotFound if the property does not exist.
// Updates UpdatedAt. Caller must save via Table.Set.
// Note: Full default-value semantics require property definition lookup;
// this implementation removes the entry. Table.Set may reinitialize defaults.
func (c *Crumb) ClearProperty(propertyID string) error {
	if c.Properties == nil {
		return ErrPropertyNotFound
	}
	if _, ok := c.Properties[propertyID]; !ok {
		return ErrPropertyNotFound
	}
	delete(c.Properties, propertyID)
	c.UpdatedAt = time.Now()
	return nil
}
