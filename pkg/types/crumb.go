package types

import "time"

// Crumb states. A crumb progresses through these states during its lifecycle.
// Implements: prd003-crumbs-interface R2.
const (
	CrumbStateDraft   = "draft"
	CrumbStatePending = "pending"
	CrumbStateReady   = "ready"
	CrumbStateTaken   = "taken"
	CrumbStatePebble  = "pebble"
	CrumbStateDust    = "dust"
)

// validCrumbStates is the set of recognized crumb state values.
var validCrumbStates = map[string]bool{
	CrumbStateDraft:   true,
	CrumbStatePending: true,
	CrumbStateReady:   true,
	CrumbStateTaken:   true,
	CrumbStatePebble:  true,
	CrumbStateDust:    true,
}

// Crumb represents a work item or task.
// Implements: prd003-crumbs-interface R1.
type Crumb struct {
	CrumbID    string         // UUID v7, generated on creation.
	Name       string         // Human-readable name (required, non-empty).
	State      string         // Current state (one of the CrumbState constants).
	CreatedAt  time.Time      // Timestamp of creation.
	UpdatedAt  time.Time      // Timestamp of last modification.
	Properties map[string]any // Property values keyed by property ID.
}

// SetState sets the crumb state to the given value.
// Returns ErrInvalidState if the state is not recognized.
// Idempotent: setting the current state succeeds without error.
// Implements: prd003-crumbs-interface R3.1.
func (c *Crumb) SetState(state string) error {
	if !validCrumbStates[state] {
		return ErrInvalidState
	}
	c.State = state
	c.UpdatedAt = time.Now()
	return nil
}

// Pebble marks the crumb as successfully finished.
// Returns ErrInvalidTransition if the current state is not "taken".
// Pebble is a terminal state; no further transitions are possible.
// Implements: prd003-crumbs-interface R3.2.
func (c *Crumb) Pebble() error {
	if c.State != CrumbStateTaken {
		return ErrInvalidTransition
	}
	c.State = CrumbStatePebble
	c.UpdatedAt = time.Now()
	return nil
}

// Dust marks the crumb as failed or abandoned.
// Can be called from any state. Idempotent.
// Implements: prd003-crumbs-interface R3.3.
func (c *Crumb) Dust() error {
	c.State = CrumbStateDust
	c.UpdatedAt = time.Now()
	return nil
}

// SetProperty sets the value of a property on this crumb.
// Returns ErrPropertyNotFound if the property ID does not exist in the
// Properties map. Validation of value type against property definitions
// is deferred to the backend (prd003 R5.7).
// Implements: prd003-crumbs-interface R4.1.
func (c *Crumb) SetProperty(propertyID string, value any) error {
	if c.Properties == nil {
		return ErrPropertyNotFound
	}
	if _, ok := c.Properties[propertyID]; !ok {
		return ErrPropertyNotFound
	}
	c.Properties[propertyID] = value
	c.UpdatedAt = time.Now()
	return nil
}

// GetProperty returns the value of a property on this crumb.
// Returns ErrPropertyNotFound if the property ID does not exist.
// Implements: prd003-crumbs-interface R4.2.
func (c *Crumb) GetProperty(propertyID string) (any, error) {
	if c.Properties == nil {
		return nil, ErrPropertyNotFound
	}
	v, ok := c.Properties[propertyID]
	if !ok {
		return nil, ErrPropertyNotFound
	}
	return v, nil
}

// GetProperties returns a copy of all property values.
// Returns an empty map (not nil) if no properties are set.
// Implements: prd003-crumbs-interface R4.3.
func (c *Crumb) GetProperties() map[string]any {
	result := make(map[string]any, len(c.Properties))
	for k, v := range c.Properties {
		result[k] = v
	}
	return result
}

// ClearProperty resets a property to its type-based default value.
// The map entry is not deleted; properties are never unset.
// Returns ErrPropertyNotFound if the property ID does not exist.
// Idempotent. Default values depend on the property's value type;
// this method sets the value to nil and the backend applies the
// correct default on persistence.
// Implements: prd003-crumbs-interface R4.4.
func (c *Crumb) ClearProperty(propertyID string) error {
	if c.Properties == nil {
		return ErrPropertyNotFound
	}
	if _, ok := c.Properties[propertyID]; !ok {
		return ErrPropertyNotFound
	}
	c.Properties[propertyID] = nil
	c.UpdatedAt = time.Now()
	return nil
}
