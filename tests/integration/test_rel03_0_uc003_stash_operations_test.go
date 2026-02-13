// Integration tests for stash operations across all five stash types (resource,
// artifact, context, counter, lock). Validates value operations (SetValue/GetValue),
// counter operations (Increment with positive and negative deltas), lock operations
// (Acquire/Release with reentrant and contention semantics), version tracking,
// fetch with filters, deletion, and history tracking through the Table interface.
// Implements: test-rel03.0-uc003-stash-operations;
//             prd008-stash-interface R1-R12 (Stash entity, types, methods, history, filters);
//             prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence).
package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S1: Stash created via Table.Set generates UUID v7 for StashID ---

func TestStashOperations_CreateGeneratesUUIDv7(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "test-stash",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "value"},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Verify UUID v7 format.
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())

	// Verify stash was populated.
	got, err := stashesTbl.Get(id)
	require.NoError(t, err)
	gotStash := got.(*types.Stash)
	assert.Equal(t, id, gotStash.StashID)
	assert.Equal(t, "test-stash", gotStash.Name)
	assert.Equal(t, types.StashTypeContext, gotStash.StashType)
	assert.Equal(t, int64(1), gotStash.Version)
	assert.False(t, gotStash.CreatedAt.IsZero())
}

// --- S2: All five stash types can be created ---

func TestStashOperations_CreateAllStashTypes(t *testing.T) {
	tests := []struct {
		name      string
		stashType string
		value     any
	}{
		{
			name:      "resource stash",
			stashType: types.StashTypeResource,
			value:     map[string]any{"uri": "https://api.example.com", "kind": "http"},
		},
		{
			name:      "artifact stash",
			stashType: types.StashTypeArtifact,
			value:     map[string]any{"path": "/tmp/build.zip", "producer": "build-agent", "checksum": "abc123"},
		},
		{
			name:      "context stash",
			stashType: types.StashTypeContext,
			value:     map[string]any{"timeout": 30, "retries": 3},
		},
		{
			name:      "counter stash",
			stashType: types.StashTypeCounter,
			value:     map[string]any{"value": int64(0)},
		},
		{
			name:      "lock stash",
			stashType: types.StashTypeLock,
			value:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			stashesTbl, err := backend.GetTable(types.TableStashes)
			require.NoError(t, err)

			stash := &types.Stash{
				Name:      "test-" + tt.stashType,
				StashType: tt.stashType,
				Value:     tt.value,
			}
			id, err := stashesTbl.Set("", stash)
			require.NoError(t, err)

			got, err := stashesTbl.Get(id)
			require.NoError(t, err)
			gotStash := got.(*types.Stash)
			assert.Equal(t, tt.stashType, gotStash.StashType)
			assert.Equal(t, int64(1), gotStash.Version)
		})
	}
}

// --- S3: SetValue updates value and increments Version ---

func TestStashOperations_SetValue(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create context stash.
	stash := &types.Stash{
		Name:      "config",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"timeout": 30},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	// Get it back.
	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(1), stash.Version)

	// SetValue and persist.
	err = stash.SetValue(map[string]any{"timeout": 60, "retries": 5})
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	// Verify update.
	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(2), stash.Version)
	val := stash.Value.(map[string]any)
	assert.Equal(t, float64(60), val["timeout"])
	assert.Equal(t, float64(5), val["retries"])
}

func TestStashOperations_SetValueOnLockReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create lock stash.
	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	// SetValue on lock should fail.
	err = stash.SetValue(map[string]any{"invalid": true})
	assert.ErrorIs(t, err, types.ErrInvalidStashType)
}

// --- S4: GetValue returns current value ---

func TestStashOperations_GetValue(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "config",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "original"},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	val := stash.GetValue()
	require.NotNil(t, val)
	valMap := val.(map[string]any)
	assert.Equal(t, "original", valMap["key"])
}

func TestStashOperations_GetValueReturnsNilForEmptyStash(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "lock",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	val := stash.GetValue()
	assert.Nil(t, val)
}

// --- S5, S6: Increment adds delta to counter value ---

func TestStashOperations_IncrementPositiveDelta(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	newVal, err := stash.Increment(5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), newVal)
	assert.Equal(t, int64(2), stash.Version)

	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(2), stash.Version)
	val := stash.Value.(map[string]any)
	// JSON unmarshaling converts int64 to float64
	assert.Equal(t, float64(5), val["value"])
}

func TestStashOperations_IncrementNegativeDelta(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(10)},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	newVal, err := stash.Increment(-3)
	require.NoError(t, err)
	assert.Equal(t, int64(7), newVal)

	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	val := stash.Value.(map[string]any)
	// JSON unmarshaling converts int64 to float64
	assert.Equal(t, float64(7), val["value"])
}

func TestStashOperations_MultipleIncrementsAccumulate(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	v1, err := stash.Increment(10)
	require.NoError(t, err)
	assert.Equal(t, int64(10), v1)

	v2, err := stash.Increment(-3)
	require.NoError(t, err)
	assert.Equal(t, int64(7), v2)

	v3, err := stash.Increment(5)
	require.NoError(t, err)
	assert.Equal(t, int64(12), v3)

	assert.Equal(t, int64(4), stash.Version)

	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	val := stash.Value.(map[string]any)
	// JSON unmarshaling converts int64 to float64
	assert.Equal(t, float64(12), val["value"])
	assert.Equal(t, int64(4), stash.Version)
}

func TestStashOperations_IncrementOnNonCounterReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "config",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"timeout": 30},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	_, err = stash.Increment(1)
	assert.ErrorIs(t, err, types.ErrInvalidStashType)
}

// --- S7, S8, S9: Lock operations ---

func TestStashOperations_AcquireLock(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(1), stash.Version)

	err = stash.Acquire("worker-1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), stash.Version)

	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(2), stash.Version)
	val := stash.Value.(map[string]any)
	assert.Equal(t, "worker-1", val["holder"])
	assert.NotEmpty(t, val["acquired_at"])
}

func TestStashOperations_ReentrantAcquire(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Acquire("worker-1")
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	// Acquire again by same holder should succeed.
	err = stash.Acquire("worker-1")
	require.NoError(t, err)

	val := stash.Value.(map[string]any)
	assert.Equal(t, "worker-1", val["holder"])
}

func TestStashOperations_AcquireByDifferentHolderReturnsErrLockHeld(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Acquire("worker-1")
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	// Different holder should get ErrLockHeld.
	err = stash.Acquire("worker-2")
	assert.ErrorIs(t, err, types.ErrLockHeld)
}

func TestStashOperations_ReleaseLock(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Acquire("worker-1")
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	versionAfterAcquire := stash.Version

	err = stash.Release("worker-1")
	require.NoError(t, err)
	assert.Nil(t, stash.Value)
	assert.Equal(t, versionAfterAcquire+1, stash.Version)

	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Nil(t, stash.Value)
}

func TestStashOperations_ReleaseByWrongHolderReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Acquire("worker-1")
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	// Wrong holder should get ErrNotLockHolder.
	err = stash.Release("worker-2")
	assert.ErrorIs(t, err, types.ErrNotLockHolder)
}

func TestStashOperations_ReleaseOnUnheldLockReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Release("worker-1")
	assert.ErrorIs(t, err, types.ErrNotLockHolder)
}

// --- S10: Version starts at 1 and increments on every mutation ---

func TestStashOperations_VersionTracking(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{
		Name:      "counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
	}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(1), stash.Version)

	stash.Increment(1)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(2), stash.Version)

	stash.Increment(1)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(3), stash.Version)

	stash.Increment(1)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)
	assert.Equal(t, int64(4), stash.Version)
}

// --- S11, S12: Fetch with stash_type and name filters ---

func TestStashOperations_FetchByStashType(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create stashes of different types.
	_, err = stashesTbl.Set("", &types.Stash{Name: "counter-1", StashType: types.StashTypeCounter, Value: map[string]any{"value": int64(0)}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "context-1", StashType: types.StashTypeContext, Value: map[string]any{"timeout": 30}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "lock-1", StashType: types.StashTypeLock, Value: nil})
	require.NoError(t, err)

	results, err := stashesTbl.Fetch(types.Filter{"stash_type": types.StashTypeCounter})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, types.StashTypeCounter, results[0].(*types.Stash).StashType)
}

func TestStashOperations_FetchByName(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	_, err = stashesTbl.Set("", &types.Stash{Name: "shared-config", StashType: types.StashTypeContext, Value: map[string]any{"timeout": 30}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "other-config", StashType: types.StashTypeContext, Value: map[string]any{"timeout": 60}})
	require.NoError(t, err)

	results, err := stashesTbl.Fetch(types.Filter{"name": "shared-config"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "shared-config", results[0].(*types.Stash).Name)
}

func TestStashOperations_FetchAll(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	_, err = stashesTbl.Set("", &types.Stash{Name: "resource-1", StashType: types.StashTypeResource, Value: map[string]any{"uri": "https://api.example.com"}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "artifact-1", StashType: types.StashTypeArtifact, Value: map[string]any{"path": "/tmp/build"}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "context-1", StashType: types.StashTypeContext, Value: map[string]any{"timeout": 30}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "counter-1", StashType: types.StashTypeCounter, Value: map[string]any{"value": int64(0)}})
	require.NoError(t, err)
	_, err = stashesTbl.Set("", &types.Stash{Name: "lock-1", StashType: types.StashTypeLock, Value: nil})
	require.NoError(t, err)

	results, err := stashesTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

// --- S13: Delete removes stash; subsequent Get returns ErrNotFound ---

func TestStashOperations_DeleteStash(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{Name: "temp", StashType: types.StashTypeContext, Value: map[string]any{"key": "value"}}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	err = stashesTbl.Delete(id)
	require.NoError(t, err)

	_, err = stashesTbl.Get(id)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestStashOperations_DeleteNonexistentReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	err = stashesTbl.Delete("nonexistent-uuid-12345")
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestStashOperations_DeleteWithEmptyIDReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	err = stashesTbl.Delete("")
	assert.ErrorIs(t, err, types.ErrInvalidID)
}

// --- S14, S15: History tracking ---

func TestStashOperations_HistoryTracksAllMutations(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create counter stash.
	stash := &types.Stash{Name: "counter", StashType: types.StashTypeCounter, Value: map[string]any{"value": int64(0)}}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	// Perform mutations.
	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	stash.LastOperation = types.StashOpIncrement
	_, err = stash.Increment(5)
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	stash.LastOperation = types.StashOpIncrement
	_, err = stash.Increment(3)
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	// Query history via FetchStashHistory.
	// Type assert to access FetchStashHistory method.
	stashesTable, ok := stashesTbl.(interface {
		FetchStashHistory(string) ([]types.StashHistoryEntry, error)
	})
	require.True(t, ok, "stashes table does not implement FetchStashHistory")

	history, err := stashesTable.FetchStashHistory(id)
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// Verify operations.
	assert.Equal(t, types.StashOpCreate, history[0].Operation)
	assert.Equal(t, types.StashOpIncrement, history[1].Operation)
	assert.Equal(t, types.StashOpIncrement, history[2].Operation)

	// Verify versions.
	assert.Equal(t, int64(1), history[0].Version)
	assert.Equal(t, int64(2), history[1].Version)
	assert.Equal(t, int64(3), history[2].Version)

	// Verify all entries have HistoryID, StashID, CreatedAt.
	for i, entry := range history {
		assert.NotEmpty(t, entry.HistoryID, "history entry %d missing HistoryID", i)
		assert.Equal(t, id, entry.StashID, "history entry %d has wrong StashID", i)
		assert.False(t, entry.CreatedAt.IsZero(), "history entry %d has zero CreatedAt", i)
	}

	// Verify value snapshots.
	val0 := history[0].Value.(map[string]any)
	assert.Equal(t, float64(0), val0["value"])

	val1 := history[1].Value.(map[string]any)
	assert.Equal(t, float64(5), val1["value"])

	val2 := history[2].Value.(map[string]any)
	assert.Equal(t, float64(8), val2["value"])
}

func TestStashOperations_HistoryOrderedByVersionAscending(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create context stash.
	stash := &types.Stash{Name: "config", StashType: types.StashTypeContext, Value: map[string]any{"key": "v0"}}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	// Perform multiple mutations.
	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	stash.LastOperation = types.StashOpSet
	err = stash.SetValue(map[string]any{"key": "v1"})
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	stash.LastOperation = types.StashOpSet
	err = stash.SetValue(map[string]any{"key": "v2"})
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	stash.LastOperation = types.StashOpSet
	err = stash.SetValue(map[string]any{"key": "v3"})
	require.NoError(t, err)
	_, err = stashesTbl.Set(id, stash)
	require.NoError(t, err)

	// Query history.
	stashesTable, ok := stashesTbl.(interface {
		FetchStashHistory(string) ([]types.StashHistoryEntry, error)
	})
	require.True(t, ok, "stashes table does not implement FetchStashHistory")

	history, err := stashesTable.FetchStashHistory(id)
	require.NoError(t, err)
	assert.Len(t, history, 4)

	// Verify ordering by version ascending.
	assert.Equal(t, int64(1), history[0].Version)
	assert.Equal(t, int64(2), history[1].Version)
	assert.Equal(t, int64(3), history[2].Version)
	assert.Equal(t, int64(4), history[3].Version)

	// Verify operations.
	assert.Equal(t, types.StashOpCreate, history[0].Operation)
	assert.Equal(t, types.StashOpSet, history[1].Operation)
	assert.Equal(t, types.StashOpSet, history[2].Operation)
	assert.Equal(t, types.StashOpSet, history[3].Operation)

	// Verify value snapshots progressed.
	val0 := history[0].Value.(map[string]any)
	assert.Equal(t, "v0", val0["key"])

	val1 := history[1].Value.(map[string]any)
	assert.Equal(t, "v1", val1["key"])

	val2 := history[2].Value.(map[string]any)
	assert.Equal(t, "v2", val2["key"])

	val3 := history[3].Value.(map[string]any)
	assert.Equal(t, "v3", val3["key"])
}

// --- Full workflow test ---

func TestStashOperations_FullWorkflow(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create stashes of each type.
	resourceID, err := stashesTbl.Set("", &types.Stash{
		Name:      "api-conn",
		StashType: types.StashTypeResource,
		Value:     map[string]any{"uri": "https://api.example.com", "kind": "http"},
	})
	require.NoError(t, err)

	artifactID, err := stashesTbl.Set("", &types.Stash{
		Name:      "build-out",
		StashType: types.StashTypeArtifact,
		Value:     map[string]any{"path": "/tmp/build", "producer": "agent-1"},
	})
	require.NoError(t, err)

	contextID, err := stashesTbl.Set("", &types.Stash{
		Name:      "shared-cfg",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"timeout": 30},
	})
	require.NoError(t, err)

	counterID, err := stashesTbl.Set("", &types.Stash{
		Name:      "req-counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
	})
	require.NoError(t, err)

	lockID, err := stashesTbl.Set("", &types.Stash{
		Name:      "mutex",
		StashType: types.StashTypeLock,
		Value:     nil,
	})
	require.NoError(t, err)

	// Verify 5 stashes exist.
	all, err := stashesTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, all, 5)

	// SetValue on context stash.
	entity, err := stashesTbl.Get(contextID)
	require.NoError(t, err)
	contextStash := entity.(*types.Stash)
	err = contextStash.SetValue(map[string]any{"timeout": 60, "retries": 3})
	require.NoError(t, err)
	_, err = stashesTbl.Set(contextID, contextStash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(contextID)
	require.NoError(t, err)
	contextStash = entity.(*types.Stash)
	val := contextStash.Value.(map[string]any)
	assert.Equal(t, float64(60), val["timeout"])

	// Increment counter.
	entity, err = stashesTbl.Get(counterID)
	require.NoError(t, err)
	counterStash := entity.(*types.Stash)

	counterStash.Increment(10)
	counterStash.Increment(-3)
	counterStash.Increment(5)
	_, err = stashesTbl.Set(counterID, counterStash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(counterID)
	require.NoError(t, err)
	counterStash = entity.(*types.Stash)
	counterVal := counterStash.Value.(map[string]any)
	// JSON unmarshaling converts int64 to float64
	assert.Equal(t, float64(12), counterVal["value"])

	// Lock operations.
	entity, err = stashesTbl.Get(lockID)
	require.NoError(t, err)
	lockStash := entity.(*types.Stash)

	err = lockStash.Acquire("worker-1")
	require.NoError(t, err)
	_, err = stashesTbl.Set(lockID, lockStash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(lockID)
	require.NoError(t, err)
	lockStash = entity.(*types.Stash)

	// Try acquire with worker-2 (should fail).
	err = lockStash.Acquire("worker-2")
	assert.ErrorIs(t, err, types.ErrLockHeld)

	// Release with worker-1.
	err = lockStash.Release("worker-1")
	require.NoError(t, err)
	_, err = stashesTbl.Set(lockID, lockStash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(lockID)
	require.NoError(t, err)
	lockStash = entity.(*types.Stash)

	// Acquire with worker-2 (should succeed).
	err = lockStash.Acquire("worker-2")
	require.NoError(t, err)
	_, err = stashesTbl.Set(lockID, lockStash)
	require.NoError(t, err)

	entity, err = stashesTbl.Get(lockID)
	require.NoError(t, err)
	lockStash = entity.(*types.Stash)
	lockVal := lockStash.Value.(map[string]any)
	assert.Equal(t, "worker-2", lockVal["holder"])

	// Fetch by type.
	counters, err := stashesTbl.Fetch(types.Filter{"stash_type": types.StashTypeCounter})
	require.NoError(t, err)
	assert.Len(t, counters, 1)

	locks, err := stashesTbl.Fetch(types.Filter{"stash_type": types.StashTypeLock})
	require.NoError(t, err)
	assert.Len(t, locks, 1)

	// Delete context stash.
	err = stashesTbl.Delete(contextID)
	require.NoError(t, err)

	all, err = stashesTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, all, 4)

	// Verify IDs still exist.
	_, err = stashesTbl.Get(resourceID)
	require.NoError(t, err)
	_, err = stashesTbl.Get(artifactID)
	require.NoError(t, err)
	_, err = stashesTbl.Get(counterID)
	require.NoError(t, err)
	_, err = stashesTbl.Get(lockID)
	require.NoError(t, err)
}

// --- Error handling tests ---

func TestStashOperations_GetWithEmptyIDReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	_, err = stashesTbl.Get("")
	assert.ErrorIs(t, err, types.ErrInvalidID)
}

func TestStashOperations_GetNonexistentReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	_, err = stashesTbl.Get("nonexistent-uuid-12345")
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestStashOperations_FetchEmptyTableReturnsEmptySlice(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	results, err := stashesTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- Edge case: Acquire with empty holder ---

func TestStashOperations_AcquireWithEmptyHolderReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{Name: "mutex", StashType: types.StashTypeLock, Value: nil}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Acquire("")
	assert.ErrorIs(t, err, types.ErrInvalidHolder)
}

// --- Edge case: lock/counter operations on wrong types ---

func TestStashOperations_LockOperationsOnNonLockReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{Name: "config", StashType: types.StashTypeContext, Value: map[string]any{"timeout": 30}}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	err = stash.Acquire("worker-1")
	assert.ErrorIs(t, err, types.ErrInvalidStashType)

	err = stash.Release("worker-1")
	assert.ErrorIs(t, err, types.ErrInvalidStashType)
}

// --- Edge case: timestamps are set correctly ---

func TestStashOperations_CreatedAtSet(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	stash := &types.Stash{Name: "test", StashType: types.StashTypeContext, Value: map[string]any{"key": "value"}}
	id, err := stashesTbl.Set("", stash)
	require.NoError(t, err)

	entity, err := stashesTbl.Get(id)
	require.NoError(t, err)
	stash = entity.(*types.Stash)

	// Verify CreatedAt is set and not zero.
	assert.False(t, stash.CreatedAt.IsZero())
	// Verify it's within the last minute (reasonable for a test).
	assert.WithinDuration(t, time.Now().UTC(), stash.CreatedAt, time.Minute)
}
