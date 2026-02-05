package types

import (
	"errors"
	"testing"
)

func TestTrail_Complete(t *testing.T) {
	tests := []struct {
		name         string
		initialState string
		wantErr      error
		wantState    string
	}{
		{"from active", TrailStateActive, nil, TrailStateCompleted},
		{"from completed", TrailStateCompleted, ErrInvalidState, TrailStateCompleted},
		{"from abandoned", TrailStateAbandoned, ErrInvalidState, TrailStateAbandoned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trail := &Trail{State: tt.initialState}

			err := trail.Complete()

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Complete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if trail.State != tt.wantState {
				t.Errorf("Complete() state = %v, want %v", trail.State, tt.wantState)
			}
			if err == nil && trail.CompletedAt == nil {
				t.Error("Complete() should set CompletedAt")
			}
		})
	}
}

func TestTrail_Abandon(t *testing.T) {
	tests := []struct {
		name         string
		initialState string
		wantErr      error
		wantState    string
	}{
		{"from active", TrailStateActive, nil, TrailStateAbandoned},
		{"from completed", TrailStateCompleted, ErrInvalidState, TrailStateCompleted},
		{"from abandoned", TrailStateAbandoned, ErrInvalidState, TrailStateAbandoned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trail := &Trail{State: tt.initialState}

			err := trail.Abandon()

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Abandon() error = %v, wantErr %v", err, tt.wantErr)
			}
			if trail.State != tt.wantState {
				t.Errorf("Abandon() state = %v, want %v", trail.State, tt.wantState)
			}
			if err == nil && trail.CompletedAt == nil {
				t.Error("Abandon() should set CompletedAt")
			}
		})
	}
}
