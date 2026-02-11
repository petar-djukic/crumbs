package types

import (
	"testing"
	"time"
)

func TestTrailComplete(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		wantErr error
	}{
		{"from active succeeds", TrailStateActive, nil},
		{"from draft fails", TrailStateDraft, ErrInvalidState},
		{"from pending fails", TrailStatePending, ErrInvalidState},
		{"from completed fails", TrailStateCompleted, ErrInvalidState},
		{"from abandoned fails", TrailStateAbandoned, ErrInvalidState},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Trail{State: tt.initial}

			err := tr.Complete()

			if err != tt.wantErr {
				t.Errorf("Complete() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				if tr.State != TrailStateCompleted {
					t.Errorf("Complete() State = %q, want %q", tr.State, TrailStateCompleted)
				}
				if tr.CompletedAt == nil {
					t.Error("Complete() did not set CompletedAt")
				}
				if time.Since(*tr.CompletedAt) > time.Second {
					t.Error("Complete() set CompletedAt to an unexpected time")
				}
			}
		})
	}
}

func TestTrailAbandon(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		wantErr error
	}{
		{"from active succeeds", TrailStateActive, nil},
		{"from draft fails", TrailStateDraft, ErrInvalidState},
		{"from pending fails", TrailStatePending, ErrInvalidState},
		{"from completed fails", TrailStateCompleted, ErrInvalidState},
		{"from abandoned fails", TrailStateAbandoned, ErrInvalidState},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Trail{State: tt.initial}

			err := tr.Abandon()

			if err != tt.wantErr {
				t.Errorf("Abandon() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				if tr.State != TrailStateAbandoned {
					t.Errorf("Abandon() State = %q, want %q", tr.State, TrailStateAbandoned)
				}
				if tr.CompletedAt == nil {
					t.Error("Abandon() did not set CompletedAt")
				}
				if time.Since(*tr.CompletedAt) > time.Second {
					t.Error("Abandon() set CompletedAt to an unexpected time")
				}
			}
		})
	}
}
