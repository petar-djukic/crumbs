package types

import (
	"errors"
	"testing"
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
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				if tr.CompletedAt != nil {
					t.Fatal("CompletedAt should not be set on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr.State != TrailStateCompleted {
				t.Fatalf("expected completed, got %s", tr.State)
			}
			if tr.CompletedAt == nil {
				t.Fatal("CompletedAt should be set")
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
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr.State != TrailStateAbandoned {
				t.Fatalf("expected abandoned, got %s", tr.State)
			}
			if tr.CompletedAt == nil {
				t.Fatal("CompletedAt should be set")
			}
		})
	}
}
