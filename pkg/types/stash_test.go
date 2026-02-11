package types

import (
	"testing"
)

func TestStashSetValue(t *testing.T) {
	tests := []struct {
		name      string
		stashType string
		value     any
		wantErr   error
	}{
		{"resource stash", StashTypeResource, map[string]any{"uri": "http://example.com"}, nil},
		{"artifact stash", StashTypeArtifact, map[string]any{"path": "/tmp/out"}, nil},
		{"context stash", StashTypeContext, map[string]any{"key": "val"}, nil},
		{"counter stash", StashTypeCounter, map[string]any{"value": int64(10)}, nil},
		{"lock stash rejected", StashTypeLock, nil, ErrInvalidStashType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Stash{StashType: tt.stashType, Version: 1}

			err := s.SetValue(tt.value)

			if err != tt.wantErr {
				t.Errorf("SetValue() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				if s.Version != 2 {
					t.Errorf("SetValue() Version = %d, want 2", s.Version)
				}
				if s.LastOperation != StashOpSet {
					t.Errorf("SetValue() LastOperation = %q, want %q", s.LastOperation, StashOpSet)
				}
			}
		})
	}
}

func TestStashGetValue(t *testing.T) {
	t.Run("returns current value", func(t *testing.T) {
		s := &Stash{Value: "hello"}
		if s.GetValue() != "hello" {
			t.Errorf("GetValue() = %v, want %q", s.GetValue(), "hello")
		}
	})

	t.Run("returns nil for unset", func(t *testing.T) {
		s := &Stash{}
		if s.GetValue() != nil {
			t.Errorf("GetValue() = %v, want nil", s.GetValue())
		}
	})
}

func TestStashIncrement(t *testing.T) {
	tests := []struct {
		name      string
		stashType string
		initial   any
		delta     int64
		wantVal   int64
		wantErr   error
	}{
		{
			"increment from zero",
			StashTypeCounter,
			map[string]any{"value": int64(0)},
			5,
			5,
			nil,
		},
		{
			"increment existing",
			StashTypeCounter,
			map[string]any{"value": int64(10)},
			3,
			13,
			nil,
		},
		{
			"decrement",
			StashTypeCounter,
			map[string]any{"value": int64(10)},
			-4,
			6,
			nil,
		},
		{
			"nil value treated as zero",
			StashTypeCounter,
			nil,
			1,
			1,
			nil,
		},
		{
			"wrong stash type",
			StashTypeResource,
			nil,
			1,
			0,
			ErrInvalidStashType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Stash{StashType: tt.stashType, Value: tt.initial, Version: 1}

			result, err := s.Increment(tt.delta)

			if err != tt.wantErr {
				t.Errorf("Increment(%d) error = %v, want %v", tt.delta, err, tt.wantErr)
			}
			if err == nil {
				if result != tt.wantVal {
					t.Errorf("Increment(%d) = %d, want %d", tt.delta, result, tt.wantVal)
				}
				if s.Version != 2 {
					t.Errorf("Increment() Version = %d, want 2", s.Version)
				}
				if s.LastOperation != StashOpIncrement {
					t.Errorf("Increment() LastOperation = %q, want %q", s.LastOperation, StashOpIncrement)
				}
			}
		})
	}
}

func TestStashAcquire(t *testing.T) {
	tests := []struct {
		name      string
		stashType string
		initial   any
		holder    string
		wantErr   error
	}{
		{
			"acquire unheld lock",
			StashTypeLock,
			nil,
			"agent-1",
			nil,
		},
		{
			"reentrant acquire",
			StashTypeLock,
			map[string]any{"holder": "agent-1", "acquired_at": "2024-01-01T00:00:00Z"},
			"agent-1",
			nil,
		},
		{
			"acquire held by another",
			StashTypeLock,
			map[string]any{"holder": "agent-1", "acquired_at": "2024-01-01T00:00:00Z"},
			"agent-2",
			ErrLockHeld,
		},
		{
			"empty holder",
			StashTypeLock,
			nil,
			"",
			ErrInvalidHolder,
		},
		{
			"wrong stash type",
			StashTypeCounter,
			nil,
			"agent-1",
			ErrInvalidStashType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Stash{StashType: tt.stashType, Value: tt.initial, Version: 1}

			err := s.Acquire(tt.holder)

			if err != tt.wantErr {
				t.Errorf("Acquire(%q) error = %v, want %v", tt.holder, err, tt.wantErr)
			}
			if err == nil {
				if s.Version != 2 {
					t.Errorf("Acquire() Version = %d, want 2", s.Version)
				}
				if s.LastOperation != StashOpAcquire {
					t.Errorf("Acquire() LastOperation = %q, want %q", s.LastOperation, StashOpAcquire)
				}
				h := lockHolder(s.Value)
				if h != tt.holder {
					t.Errorf("Acquire() holder = %q, want %q", h, tt.holder)
				}
			}
		})
	}
}

func TestStashRelease(t *testing.T) {
	tests := []struct {
		name      string
		stashType string
		initial   any
		holder    string
		wantErr   error
	}{
		{
			"release by holder",
			StashTypeLock,
			map[string]any{"holder": "agent-1", "acquired_at": "2024-01-01T00:00:00Z"},
			"agent-1",
			nil,
		},
		{
			"release by wrong holder",
			StashTypeLock,
			map[string]any{"holder": "agent-1", "acquired_at": "2024-01-01T00:00:00Z"},
			"agent-2",
			ErrNotLockHolder,
		},
		{
			"release unheld lock",
			StashTypeLock,
			nil,
			"agent-1",
			ErrNotLockHolder,
		},
		{
			"wrong stash type",
			StashTypeCounter,
			nil,
			"agent-1",
			ErrInvalidStashType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Stash{StashType: tt.stashType, Value: tt.initial, Version: 1}

			err := s.Release(tt.holder)

			if err != tt.wantErr {
				t.Errorf("Release(%q) error = %v, want %v", tt.holder, err, tt.wantErr)
			}
			if err == nil {
				if s.Value != nil {
					t.Errorf("Release() Value = %v, want nil", s.Value)
				}
				if s.Version != 2 {
					t.Errorf("Release() Version = %d, want 2", s.Version)
				}
				if s.LastOperation != StashOpRelease {
					t.Errorf("Release() LastOperation = %q, want %q", s.LastOperation, StashOpRelease)
				}
			}
		})
	}
}

func TestStashIncrementValidatesType(t *testing.T) {
	nonCounterTypes := []string{StashTypeResource, StashTypeArtifact, StashTypeContext, StashTypeLock}
	for _, st := range nonCounterTypes {
		t.Run(st, func(t *testing.T) {
			s := &Stash{StashType: st}
			_, err := s.Increment(1)
			if err != ErrInvalidStashType {
				t.Errorf("Increment on %s: error = %v, want %v", st, err, ErrInvalidStashType)
			}
		})
	}
}
