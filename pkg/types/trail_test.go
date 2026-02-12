package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrailComplete(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		wantErr error
	}{
		{
			name:    "from active succeeds",
			initial: TrailStateActive,
		},
		{
			name:    "from draft fails",
			initial: TrailStateDraft,
			wantErr: ErrInvalidState,
		},
		{
			name:    "from pending fails",
			initial: TrailStatePending,
			wantErr: ErrInvalidState,
		},
		{
			name:    "from completed fails",
			initial: TrailStateCompleted,
			wantErr: ErrInvalidState,
		},
		{
			name:    "from abandoned fails",
			initial: TrailStateAbandoned,
			wantErr: ErrInvalidState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trail := &Trail{
				TrailID:   "complete-test",
				State:     tt.initial,
				CreatedAt: time.Now(),
			}

			err := trail.Complete()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.initial, trail.State, "state should not change on error")
				assert.Nil(t, trail.CompletedAt, "CompletedAt should remain nil on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, TrailStateCompleted, trail.State)
				assert.NotNil(t, trail.CompletedAt, "CompletedAt should be set")
				assert.WithinDuration(t, time.Now(), *trail.CompletedAt, time.Second)
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
		{
			name:    "from active succeeds",
			initial: TrailStateActive,
		},
		{
			name:    "from draft fails",
			initial: TrailStateDraft,
			wantErr: ErrInvalidState,
		},
		{
			name:    "from pending fails",
			initial: TrailStatePending,
			wantErr: ErrInvalidState,
		},
		{
			name:    "from completed fails",
			initial: TrailStateCompleted,
			wantErr: ErrInvalidState,
		},
		{
			name:    "from abandoned fails",
			initial: TrailStateAbandoned,
			wantErr: ErrInvalidState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trail := &Trail{
				TrailID:   "abandon-test",
				State:     tt.initial,
				CreatedAt: time.Now(),
			}

			err := trail.Abandon()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.initial, trail.State, "state should not change on error")
				assert.Nil(t, trail.CompletedAt, "CompletedAt should remain nil on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, TrailStateAbandoned, trail.State)
				assert.NotNil(t, trail.CompletedAt, "CompletedAt should be set")
				assert.WithinDuration(t, time.Now(), *trail.CompletedAt, time.Second)
			}
		})
	}
}

func TestTrailCompletePreservesCreatedAt(t *testing.T) {
	created := time.Now().Add(-24 * time.Hour)
	trail := &Trail{
		TrailID:   "preserve-test",
		State:     TrailStateActive,
		CreatedAt: created,
	}

	err := trail.Complete()
	assert.NoError(t, err)
	assert.Equal(t, created, trail.CreatedAt, "CreatedAt must not change")
}

func TestTrailAbandonPreservesCreatedAt(t *testing.T) {
	created := time.Now().Add(-24 * time.Hour)
	trail := &Trail{
		TrailID:   "preserve-test",
		State:     TrailStateActive,
		CreatedAt: created,
	}

	err := trail.Abandon()
	assert.NoError(t, err)
	assert.Equal(t, created, trail.CreatedAt, "CreatedAt must not change")
}
