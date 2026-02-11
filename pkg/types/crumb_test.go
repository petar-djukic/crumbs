package types

import (
	"testing"
	"time"
)

func TestCrumbSetState(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		wantErr   error
		wantState string
	}{
		{"draft", CrumbStateDraft, nil, CrumbStateDraft},
		{"pending", CrumbStatePending, nil, CrumbStatePending},
		{"ready", CrumbStateReady, nil, CrumbStateReady},
		{"taken", CrumbStateTaken, nil, CrumbStateTaken},
		{"pebble", CrumbStatePebble, nil, CrumbStatePebble},
		{"dust", CrumbStateDust, nil, CrumbStateDust},
		{"invalid state", "bogus", ErrInvalidState, CrumbStateDraft},
		{"empty state", "", ErrInvalidState, CrumbStateDraft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{State: CrumbStateDraft, UpdatedAt: time.Time{}}
			before := c.UpdatedAt

			err := c.SetState(tt.state)

			if err != tt.wantErr {
				t.Errorf("SetState(%q) error = %v, want %v", tt.state, err, tt.wantErr)
			}
			if c.State != tt.wantState {
				t.Errorf("SetState(%q) State = %q, want %q", tt.state, c.State, tt.wantState)
			}
			if err == nil && !c.UpdatedAt.After(before) {
				t.Errorf("SetState(%q) did not update UpdatedAt", tt.state)
			}
		})
	}
}

func TestCrumbSetStateIdempotent(t *testing.T) {
	c := &Crumb{State: CrumbStateReady}
	if err := c.SetState(CrumbStateReady); err != nil {
		t.Errorf("SetState to current state should be idempotent, got %v", err)
	}
}

func TestCrumbPebble(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		wantErr error
	}{
		{"from taken succeeds", CrumbStateTaken, nil},
		{"from draft fails", CrumbStateDraft, ErrInvalidTransition},
		{"from pending fails", CrumbStatePending, ErrInvalidTransition},
		{"from ready fails", CrumbStateReady, ErrInvalidTransition},
		{"from pebble fails", CrumbStatePebble, ErrInvalidTransition},
		{"from dust fails", CrumbStateDust, ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{State: tt.initial, UpdatedAt: time.Time{}}
			before := c.UpdatedAt

			err := c.Pebble()

			if err != tt.wantErr {
				t.Errorf("Pebble() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				if c.State != CrumbStatePebble {
					t.Errorf("Pebble() State = %q, want %q", c.State, CrumbStatePebble)
				}
				if !c.UpdatedAt.After(before) {
					t.Error("Pebble() did not update UpdatedAt")
				}
			}
		})
	}
}

func TestCrumbDust(t *testing.T) {
	states := []string{
		CrumbStateDraft, CrumbStatePending, CrumbStateReady,
		CrumbStateTaken, CrumbStatePebble, CrumbStateDust,
	}
	for _, state := range states {
		t.Run("from "+state, func(t *testing.T) {
			c := &Crumb{State: state, UpdatedAt: time.Time{}}
			before := c.UpdatedAt

			err := c.Dust()

			if err != nil {
				t.Errorf("Dust() error = %v, want nil", err)
			}
			if c.State != CrumbStateDust {
				t.Errorf("Dust() State = %q, want %q", c.State, CrumbStateDust)
			}
			if !c.UpdatedAt.After(before) {
				t.Error("Dust() did not update UpdatedAt")
			}
		})
	}
}

func TestCrumbDustIdempotent(t *testing.T) {
	c := &Crumb{State: CrumbStateDust}
	if err := c.Dust(); err != nil {
		t.Errorf("Dust() on dust crumb should be idempotent, got %v", err)
	}
	if c.State != CrumbStateDust {
		t.Errorf("State should remain dust, got %q", c.State)
	}
}

func TestCrumbSetProperty(t *testing.T) {
	tests := []struct {
		name       string
		props      map[string]any
		propertyID string
		value      any
		wantErr    error
	}{
		{
			"set existing property",
			map[string]any{"p1": "old"},
			"p1",
			"new",
			nil,
		},
		{
			"property not found",
			map[string]any{"p1": "val"},
			"p2",
			"val",
			ErrPropertyNotFound,
		},
		{
			"nil properties map",
			nil,
			"p1",
			"val",
			ErrPropertyNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{Properties: tt.props, UpdatedAt: time.Time{}}
			before := c.UpdatedAt

			err := c.SetProperty(tt.propertyID, tt.value)

			if err != tt.wantErr {
				t.Errorf("SetProperty(%q) error = %v, want %v", tt.propertyID, err, tt.wantErr)
			}
			if err == nil {
				if c.Properties[tt.propertyID] != tt.value {
					t.Errorf("Property value = %v, want %v", c.Properties[tt.propertyID], tt.value)
				}
				if !c.UpdatedAt.After(before) {
					t.Error("SetProperty did not update UpdatedAt")
				}
			}
		})
	}
}

func TestCrumbGetProperty(t *testing.T) {
	tests := []struct {
		name       string
		props      map[string]any
		propertyID string
		wantVal    any
		wantErr    error
	}{
		{
			"existing property",
			map[string]any{"p1": "val1"},
			"p1",
			"val1",
			nil,
		},
		{
			"missing property",
			map[string]any{"p1": "val1"},
			"p2",
			nil,
			ErrPropertyNotFound,
		},
		{
			"nil properties map",
			nil,
			"p1",
			nil,
			ErrPropertyNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{Properties: tt.props}

			val, err := c.GetProperty(tt.propertyID)

			if err != tt.wantErr {
				t.Errorf("GetProperty(%q) error = %v, want %v", tt.propertyID, err, tt.wantErr)
			}
			if val != tt.wantVal {
				t.Errorf("GetProperty(%q) = %v, want %v", tt.propertyID, val, tt.wantVal)
			}
		})
	}
}

func TestCrumbGetProperties(t *testing.T) {
	t.Run("returns copy", func(t *testing.T) {
		c := &Crumb{Properties: map[string]any{"p1": "v1", "p2": int64(42)}}
		result := c.GetProperties()

		if len(result) != 2 {
			t.Fatalf("GetProperties() returned %d entries, want 2", len(result))
		}
		// Modifying the returned map should not affect the original.
		result["p1"] = "changed"
		if c.Properties["p1"] == "changed" {
			t.Error("GetProperties() returned a reference, not a copy")
		}
	})

	t.Run("nil properties returns empty map", func(t *testing.T) {
		c := &Crumb{}
		result := c.GetProperties()
		if result == nil {
			t.Error("GetProperties() returned nil, want empty map")
		}
		if len(result) != 0 {
			t.Errorf("GetProperties() returned %d entries, want 0", len(result))
		}
	})
}

func TestCrumbClearProperty(t *testing.T) {
	tests := []struct {
		name       string
		props      map[string]any
		propertyID string
		wantErr    error
	}{
		{
			"clear existing property",
			map[string]any{"p1": "val"},
			"p1",
			nil,
		},
		{
			"missing property",
			map[string]any{"p1": "val"},
			"p2",
			ErrPropertyNotFound,
		},
		{
			"nil properties map",
			nil,
			"p1",
			ErrPropertyNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{Properties: tt.props, UpdatedAt: time.Time{}}
			before := c.UpdatedAt

			err := c.ClearProperty(tt.propertyID)

			if err != tt.wantErr {
				t.Errorf("ClearProperty(%q) error = %v, want %v", tt.propertyID, err, tt.wantErr)
			}
			if err == nil {
				if c.Properties[tt.propertyID] != nil {
					t.Errorf("ClearProperty(%q) value = %v, want nil", tt.propertyID, c.Properties[tt.propertyID])
				}
				if _, ok := c.Properties[tt.propertyID]; !ok {
					t.Errorf("ClearProperty(%q) removed the key, should keep it", tt.propertyID)
				}
				if !c.UpdatedAt.After(before) {
					t.Error("ClearProperty did not update UpdatedAt")
				}
			}
		})
	}
}
