package types

import (
	"errors"
	"testing"
	"time"
)

func TestCrumbSetState(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		target    string
		wantErr   error
		wantState string
	}{
		{"draft to pending", StateDraft, StatePending, nil, StatePending},
		{"pending to ready", StatePending, StateReady, nil, StateReady},
		{"ready to taken", StateReady, StateTaken, nil, StateTaken},
		{"taken to pebble", StateTaken, StatePebble, nil, StatePebble},
		{"taken to dust", StateTaken, StateDust, nil, StateDust},
		{"idempotent same state", StateReady, StateReady, nil, StateReady},
		{"invalid state value", StateDraft, "invalid", ErrInvalidState, StateDraft},
		{"empty state value", StateDraft, "", ErrInvalidState, StateDraft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			c := &Crumb{State: tt.initial, UpdatedAt: before.Add(-time.Hour)}
			err := c.SetState(tt.target)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				if c.State != tt.initial {
					t.Fatalf("state should not change on error, got %s", c.State)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.State != tt.wantState {
				t.Fatalf("expected state %s, got %s", tt.wantState, c.State)
			}
			if c.UpdatedAt.Before(before) {
				t.Fatal("UpdatedAt should be updated")
			}
		})
	}
}

func TestCrumbPebble(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		wantErr error
	}{
		{"from taken succeeds", StateTaken, nil},
		{"from draft fails", StateDraft, ErrInvalidTransition},
		{"from pending fails", StatePending, ErrInvalidTransition},
		{"from ready fails", StateReady, ErrInvalidTransition},
		{"from pebble fails", StatePebble, ErrInvalidTransition},
		{"from dust fails", StateDust, ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Crumb{State: tt.initial}
			err := c.Pebble()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.State != StatePebble {
				t.Fatalf("expected state pebble, got %s", c.State)
			}
		})
	}
}

func TestCrumbDust(t *testing.T) {
	states := []string{StateDraft, StatePending, StateReady, StateTaken, StatePebble, StateDust}
	for _, initial := range states {
		t.Run("from_"+initial, func(t *testing.T) {
			c := &Crumb{State: initial}
			err := c.Dust()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.State != StateDust {
				t.Fatalf("expected state dust, got %s", c.State)
			}
		})
	}
}

func TestCrumbPropertyMethods(t *testing.T) {
	t.Run("set and get property", func(t *testing.T) {
		c := &Crumb{Properties: map[string]any{}}
		if err := c.SetProperty("p1", "hello"); err != nil {
			t.Fatal(err)
		}
		v, err := c.GetProperty("p1")
		if err != nil {
			t.Fatal(err)
		}
		if v != "hello" {
			t.Fatalf("expected hello, got %v", v)
		}
	})

	t.Run("get missing property", func(t *testing.T) {
		c := &Crumb{Properties: map[string]any{}}
		_, err := c.GetProperty("missing")
		if !errors.Is(err, ErrPropertyNotFound) {
			t.Fatalf("expected ErrPropertyNotFound, got %v", err)
		}
	})

	t.Run("get properties returns copy-safe map", func(t *testing.T) {
		c := &Crumb{}
		props := c.GetProperties()
		if props == nil {
			t.Fatal("expected non-nil map")
		}
		if len(props) != 0 {
			t.Fatal("expected empty map")
		}
	})

	t.Run("clear property", func(t *testing.T) {
		c := &Crumb{Properties: map[string]any{"p1": "value"}}
		if err := c.ClearProperty("p1"); err != nil {
			t.Fatal(err)
		}
		v, err := c.GetProperty("p1")
		if err != nil {
			t.Fatal(err)
		}
		if v != nil {
			t.Fatalf("expected nil after clear, got %v", v)
		}
	})

	t.Run("clear missing property", func(t *testing.T) {
		c := &Crumb{Properties: map[string]any{}}
		err := c.ClearProperty("missing")
		if !errors.Is(err, ErrPropertyNotFound) {
			t.Fatalf("expected ErrPropertyNotFound, got %v", err)
		}
	})

	t.Run("set property initializes nil map", func(t *testing.T) {
		c := &Crumb{}
		if err := c.SetProperty("p1", 42); err != nil {
			t.Fatal(err)
		}
		v, err := c.GetProperty("p1")
		if err != nil {
			t.Fatal(err)
		}
		if v != 42 {
			t.Fatalf("expected 42, got %v", v)
		}
	})
}
