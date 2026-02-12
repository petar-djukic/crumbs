package types

import "time"

// Crumb state constants (prd003-crumbs-interface R2.1).
const (
	StateDraft   = "draft"
	StatePending = "pending"
	StateReady   = "ready"
	StateTaken   = "taken"
	StatePebble  = "pebble"
	StateDust    = "dust"
)

// validCrumbStates is the set of recognized crumb states.
var validCrumbStates = map[string]bool{
	StateDraft:   true,
	StatePending: true,
	StateReady:   true,
	StateTaken:   true,
	StatePebble:  true,
	StateDust:    true,
}

// Crumb represents a work item (prd003-crumbs-interface R1.1).
// Entity methods modify the struct in memory; callers persist via Table.Set.
type Crumb struct {
	CrumbID    string         `json:"crumb_id"`
	Name       string         `json:"name"`
	State      string         `json:"state"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Properties map[string]any `json:"properties"`
}

// SetState transitions the crumb to the specified state
// (prd003-crumbs-interface R4.2). It validates that the state is one of
// the recognized values and updates UpdatedAt. Setting to the current
// state is idempotent.
func (c *Crumb) SetState(state string) error {
	if !validCrumbStates[state] {
		return ErrInvalidState
	}
	c.State = state
	c.UpdatedAt = time.Now()
	return nil
}

// Pebble transitions the crumb to the pebble state (completed
// successfully). The crumb must be in the taken state
// (prd003-crumbs-interface R4.3).
func (c *Crumb) Pebble() error {
	if c.State != StateTaken {
		return ErrInvalidTransition
	}
	c.State = StatePebble
	c.UpdatedAt = time.Now()
	return nil
}

// Dust transitions the crumb to the dust state (failed or abandoned).
// Can be called from any state and is idempotent
// (prd003-crumbs-interface R4.4).
func (c *Crumb) Dust() error {
	c.State = StateDust
	c.UpdatedAt = time.Now()
	return nil
}

// SetProperty assigns a value to a property in the Properties map
// (prd003-crumbs-interface R5.2). Validation against property definitions
// is deferred to Table.Set per R5.7.
func (c *Crumb) SetProperty(propertyID string, value any) error {
	if c.Properties == nil {
		c.Properties = make(map[string]any)
	}
	c.Properties[propertyID] = value
	c.UpdatedAt = time.Now()
	return nil
}

// GetProperty retrieves a single property value from the Properties map
// (prd003-crumbs-interface R5.3).
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

// GetProperties returns the entire Properties map
// (prd003-crumbs-interface R5.4).
func (c *Crumb) GetProperties() map[string]any {
	if c.Properties == nil {
		return map[string]any{}
	}
	return c.Properties
}

// ClearProperty resets a property to its type-based default. Because the
// types package does not hold property definitions, this sets the value
// to nil as a placeholder; the backend replaces nil with the real default
// on persist (prd003-crumbs-interface R5.5).
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
