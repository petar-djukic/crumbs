package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCrumbSetState(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		target    string
		wantErr   error
		wantState string
	}{
		{
			name:      "set valid state draft",
			initial:   StateDraft,
			target:    StateDraft,
			wantState: StateDraft,
		},
		{
			name:      "set valid state pending",
			initial:   StateDraft,
			target:    StatePending,
			wantState: StatePending,
		},
		{
			name:      "set valid state ready",
			initial:   StatePending,
			target:    StateReady,
			wantState: StateReady,
		},
		{
			name:      "set valid state taken",
			initial:   StateReady,
			target:    StateTaken,
			wantState: StateTaken,
		},
		{
			name:      "set valid state pebble",
			initial:   StateTaken,
			target:    StatePebble,
			wantState: StatePebble,
		},
		{
			name:      "set valid state dust",
			initial:   StateDraft,
			target:    StateDust,
			wantState: StateDust,
		},
		{
			name:    "invalid state rejected",
			initial: StateDraft,
			target:  "invalid",
			wantErr: ErrInvalidState,
		},
		{
			name:    "empty string rejected",
			initial: StateDraft,
			target:  "",
			wantErr: ErrInvalidState,
		},
		{
			name:      "idempotent set same state",
			initial:   StateReady,
			target:    StateReady,
			wantState: StateReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{
				CrumbID:   "test-id",
				State:     tt.initial,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now().Add(-time.Hour),
			}
			before := c.UpdatedAt

			err := c.SetState(tt.target)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.initial, c.State, "state should not change on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantState, c.State)
				assert.True(t, c.UpdatedAt.After(before) || c.UpdatedAt.Equal(before),
					"UpdatedAt should be refreshed")
			}
		})
	}
}

func TestCrumbSetStateUpdatesTimestamp(t *testing.T) {
	c := &Crumb{
		CrumbID:   "ts-test",
		State:     StateDraft,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
	}
	original := c.CreatedAt

	err := c.SetState(StatePending)
	assert.NoError(t, err)
	assert.Equal(t, original, c.CreatedAt, "CreatedAt must not change")
	assert.True(t, c.UpdatedAt.After(original), "UpdatedAt should advance")
}

func TestCrumbPebble(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		wantErr error
	}{
		{
			name:    "from taken succeeds",
			initial: StateTaken,
		},
		{
			name:    "from draft fails",
			initial: StateDraft,
			wantErr: ErrInvalidTransition,
		},
		{
			name:    "from pending fails",
			initial: StatePending,
			wantErr: ErrInvalidTransition,
		},
		{
			name:    "from ready fails",
			initial: StateReady,
			wantErr: ErrInvalidTransition,
		},
		{
			name:    "from pebble fails",
			initial: StatePebble,
			wantErr: ErrInvalidTransition,
		},
		{
			name:    "from dust fails",
			initial: StateDust,
			wantErr: ErrInvalidTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{
				CrumbID:   "pebble-test",
				State:     tt.initial,
				UpdatedAt: time.Now().Add(-time.Hour),
			}
			before := c.UpdatedAt

			err := c.Pebble()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.initial, c.State, "state should not change on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, StatePebble, c.State)
				assert.True(t, c.UpdatedAt.After(before), "UpdatedAt should advance")
			}
		})
	}
}

func TestCrumbDust(t *testing.T) {
	states := []string{StateDraft, StatePending, StateReady, StateTaken, StatePebble, StateDust}

	for _, initial := range states {
		t.Run("from_"+initial, func(t *testing.T) {
			c := &Crumb{
				CrumbID:   "dust-test",
				State:     initial,
				UpdatedAt: time.Now().Add(-time.Hour),
			}
			before := c.UpdatedAt

			err := c.Dust()

			assert.NoError(t, err)
			assert.Equal(t, StateDust, c.State)
			assert.True(t, c.UpdatedAt.After(before), "UpdatedAt should advance")
		})
	}
}

func TestCrumbDustIdempotent(t *testing.T) {
	c := &Crumb{
		CrumbID:   "dust-idem",
		State:     StateDust,
		UpdatedAt: time.Now().Add(-time.Hour),
	}

	err := c.Dust()
	assert.NoError(t, err)
	assert.Equal(t, StateDust, c.State)

	err = c.Dust()
	assert.NoError(t, err)
	assert.Equal(t, StateDust, c.State)
}

func TestCrumbSetProperty(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]any
		propID     string
		value      any
		wantErr    error
	}{
		{
			name:       "set existing property",
			properties: map[string]any{"priority": "low"},
			propID:     "priority",
			value:      "high",
		},
		{
			name:       "property not found",
			properties: map[string]any{"priority": "low"},
			propID:     "missing",
			value:      "value",
			wantErr:    ErrPropertyNotFound,
		},
		{
			name:       "nil properties map",
			properties: nil,
			propID:     "priority",
			value:      "high",
			wantErr:    ErrPropertyNotFound,
		},
		{
			name:       "empty properties map",
			properties: map[string]any{},
			propID:     "priority",
			value:      "high",
			wantErr:    ErrPropertyNotFound,
		},
		{
			name:       "set property to nil value",
			properties: map[string]any{"priority": "low"},
			propID:     "priority",
			value:      nil,
		},
		{
			name:       "set property with empty key that exists",
			properties: map[string]any{"": "empty-key-value"},
			propID:     "",
			value:      "new-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{
				CrumbID:    "prop-test",
				Properties: tt.properties,
				UpdatedAt:  time.Now().Add(-time.Hour),
			}
			before := c.UpdatedAt

			err := c.SetProperty(tt.propID, tt.value)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.value, c.Properties[tt.propID])
				assert.True(t, c.UpdatedAt.After(before), "UpdatedAt should advance")
			}
		})
	}
}

func TestCrumbGetProperty(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]any
		propID     string
		wantVal    any
		wantErr    error
	}{
		{
			name:       "get existing property",
			properties: map[string]any{"priority": "high"},
			propID:     "priority",
			wantVal:    "high",
		},
		{
			name:       "property not found",
			properties: map[string]any{"priority": "high"},
			propID:     "missing",
			wantErr:    ErrPropertyNotFound,
		},
		{
			name:       "nil properties map",
			properties: nil,
			propID:     "priority",
			wantErr:    ErrPropertyNotFound,
		},
		{
			name:       "get property with nil value",
			properties: map[string]any{"priority": nil},
			propID:     "priority",
			wantVal:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{
				CrumbID:    "get-prop-test",
				Properties: tt.properties,
			}

			val, err := c.GetProperty(tt.propID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, val)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantVal, val)
			}
		})
	}
}

func TestCrumbGetProperties(t *testing.T) {
	props := map[string]any{"priority": "high", "label": "bug"}
	c := &Crumb{
		CrumbID:    "get-props-test",
		Properties: props,
	}

	got := c.GetProperties()
	assert.Equal(t, props, got)
}

func TestCrumbGetPropertiesNilMap(t *testing.T) {
	c := &Crumb{
		CrumbID:    "nil-props-test",
		Properties: nil,
	}

	got := c.GetProperties()
	assert.Nil(t, got)
}

func TestCrumbClearProperty(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]any
		propID     string
		wantErr    error
	}{
		{
			name:       "clear existing property",
			properties: map[string]any{"priority": "high"},
			propID:     "priority",
		},
		{
			name:       "clear already nil property",
			properties: map[string]any{"priority": nil},
			propID:     "priority",
		},
		{
			name:       "property not found",
			properties: map[string]any{"priority": "high"},
			propID:     "missing",
			wantErr:    ErrPropertyNotFound,
		},
		{
			name:       "nil properties map",
			properties: nil,
			propID:     "priority",
			wantErr:    ErrPropertyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{
				CrumbID:    "clear-test",
				Properties: tt.properties,
				UpdatedAt:  time.Now().Add(-time.Hour),
			}
			before := c.UpdatedAt

			err := c.ClearProperty(tt.propID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, c.Properties[tt.propID], "cleared property should be nil")
				assert.True(t, c.UpdatedAt.After(before), "UpdatedAt should advance")
			}
		})
	}
}
