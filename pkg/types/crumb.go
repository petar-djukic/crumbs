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

// validStates provides O(1) lookup for state validation.
var validStates = map[string]bool{
	StateDraft:   true,
	StatePending: true,
	StateReady:   true,
	StateTaken:   true,
	StatePebble:  true,
	StateDust:    true,
}

// Crumb represents a work item in the Crumbs storage system
// (prd003-crumbs-interface R1.1). Entity methods modify the struct in memory;
// the caller must call Table.Set to persist changes.
type Crumb struct {
	CrumbID    string         `json:"crumb_id"`
	Name       string         `json:"name"`
	State      string         `json:"state"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Properties map[string]any `json:"properties"`
}

// SetState transitions the crumb to the specified state (prd003-crumbs-interface
// R4.2). It validates that state is one of the values defined in R2.1 and returns
// ErrInvalidState if not. The operation is idempotent: setting the current state
// succeeds without error. UpdatedAt is always refreshed.
func (c *Crumb) SetState(state string) error {
	if !validStates[state] {
		return ErrInvalidState
	}
	c.State = state
	c.UpdatedAt = time.Now()
	return nil
}

// Pebble transitions the crumb to the pebble state, marking it as successfully
// completed (prd003-crumbs-interface R4.3). The current state must be "taken";
// otherwise ErrInvalidTransition is returned.
func (c *Crumb) Pebble() error {
	if c.State != StateTaken {
		return ErrInvalidTransition
	}
	c.State = StatePebble
	c.UpdatedAt = time.Now()
	return nil
}

// Dust transitions the crumb to the dust state, marking it as failed or
// abandoned (prd003-crumbs-interface R4.4). Dust can be called from any state
// and is idempotent. UpdatedAt is always refreshed.
func (c *Crumb) Dust() error {
	c.State = StateDust
	c.UpdatedAt = time.Now()
	return nil
}

// SetProperty assigns a value to a property in the Properties map
// (prd003-crumbs-interface R5.2). The method updates the map entry and refreshes
// UpdatedAt. It returns ErrPropertyNotFound if the property does not exist in
// the map. Full type and category validation is deferred to Table.Set per R5.7.
func (c *Crumb) SetProperty(propertyID string, value any) error {
	if _, ok := c.Properties[propertyID]; !ok {
		return ErrPropertyNotFound
	}
	c.Properties[propertyID] = value
	c.UpdatedAt = time.Now()
	return nil
}

// GetProperty retrieves a single property value from the Properties map
// (prd003-crumbs-interface R5.3). Returns ErrPropertyNotFound if the property
// does not exist in the map.
func (c *Crumb) GetProperty(propertyID string) (any, error) {
	v, ok := c.Properties[propertyID]
	if !ok {
		return nil, ErrPropertyNotFound
	}
	return v, nil
}

// GetProperties returns the full Properties map (prd003-crumbs-interface R5.4).
// The map contains an entry for every defined property.
func (c *Crumb) GetProperties() map[string]any {
	return c.Properties
}

// ClearProperty resets a property to its type-based default value
// (prd003-crumbs-interface R5.5). The map entry is set to nil rather than
// deleted; properties are never unset. Full default-value resolution (matching
// the property's ValueType) is deferred to Table.Set per R5.7. Returns
// ErrPropertyNotFound if the property does not exist in the map.
func (c *Crumb) ClearProperty(propertyID string) error {
	if _, ok := c.Properties[propertyID]; !ok {
		return ErrPropertyNotFound
	}
	c.Properties[propertyID] = nil
	c.UpdatedAt = time.Now()
	return nil
}
