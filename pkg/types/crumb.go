package types

import "time"

// Crumb state constants (prd003-crumbs-interface R2).
const (
	CrumbStateDraft   = "draft"
	CrumbStatePending = "pending"
	CrumbStateReady   = "ready"
	CrumbStateTaken   = "taken"
	CrumbStatePebble  = "pebble"
	CrumbStateDust    = "dust"
)

// Crumb represents a work item.
// Implements prd003-crumbs-interface R1.
type Crumb struct {
	CrumbID    string         `json:"crumb_id"`
	Name       string         `json:"name"`
	State      string         `json:"state"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Properties map[string]any `json:"properties"`
}

// SetState transitions the crumb to the specified state.
// Returns ErrInvalidState if the state is not recognized.
// Implements prd003-crumbs-interface R4.2.
func (c *Crumb) SetState(state string) error {
	switch state {
	case CrumbStateDraft, CrumbStatePending, CrumbStateReady,
		CrumbStateTaken, CrumbStatePebble, CrumbStateDust:
	default:
		return ErrInvalidState
	}
	c.State = state
	c.UpdatedAt = time.Now()
	return nil
}

// Pebble transitions the crumb to pebble (completed successfully).
// Returns ErrInvalidTransition if current state is not taken.
// Implements prd003-crumbs-interface R4.3.
func (c *Crumb) Pebble() error {
	if c.State != CrumbStateTaken {
		return ErrInvalidTransition
	}
	c.State = CrumbStatePebble
	c.UpdatedAt = time.Now()
	return nil
}

// Dust transitions the crumb to dust (failed or abandoned).
// Can be called from any state. Idempotent.
// Implements prd003-crumbs-interface R4.4.
func (c *Crumb) Dust() error {
	c.State = CrumbStateDust
	c.UpdatedAt = time.Now()
	return nil
}

// SetProperty assigns a value to a property in the Properties map.
// Validation against property definitions is deferred to Table.Set.
// Implements prd003-crumbs-interface R5.2.
func (c *Crumb) SetProperty(propertyID string, value any) error {
	if c.Properties == nil {
		c.Properties = make(map[string]any)
	}
	c.Properties[propertyID] = value
	c.UpdatedAt = time.Now()
	return nil
}

// GetProperty retrieves a single property value.
// Returns ErrPropertyNotFound if the property is not in the Properties map.
// Implements prd003-crumbs-interface R5.3.
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

// GetProperties returns the full Properties map.
// Implements prd003-crumbs-interface R5.4.
func (c *Crumb) GetProperties() map[string]any {
	if c.Properties == nil {
		return map[string]any{}
	}
	return c.Properties
}

// ClearProperty resets a property to its type-based default.
// The default value lookup is deferred to Table.Set; this method
// sets the value to nil as a placeholder for the backend to resolve.
// Implements prd003-crumbs-interface R5.5.
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
