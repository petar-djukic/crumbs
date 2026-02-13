// Integration tests for the crumb entity state machine and lifecycle.
// Validates state transitions (draft→pending→ready→taken→pebble, any→dust),
// timestamp behavior, state-based filtering, full success/failure paths,
// and mixed terminal states.
// Implements: test-rel01.0-uc003-crumb-lifecycle;
//             prd003-crumbs-interface R2-R4 (states, transitions, Pebble, Dust).
package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TestCrumbCreation: S1, S11 ---

func TestCrumbLifecycle_CreateWithDraftState(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "New crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, types.StateDraft, gotCrumb.State)
	assert.False(t, gotCrumb.CreatedAt.IsZero(), "CreatedAt must be set")
	assert.False(t, gotCrumb.UpdatedAt.IsZero(), "UpdatedAt must be set")
}

func TestCrumbLifecycle_CreateWithExplicitState(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Note: Set with empty id forces StateDraft regardless of input.
	// We verify that create always produces draft.
	crumb := &types.Crumb{Name: "Ready crumb", State: types.StateReady}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, types.StateDraft, gotCrumb.State, "create always defaults to draft")
	assert.False(t, gotCrumb.CreatedAt.IsZero())
	assert.False(t, gotCrumb.UpdatedAt.IsZero())
}

// --- TestCrumbStateTransitions: S2-S5 ---

func TestCrumbLifecycle_StateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		fromState string
		toState   string
	}{
		{"draft to pending", types.StateDraft, types.StatePending},
		{"pending to ready", types.StatePending, types.StateReady},
		{"draft to ready", types.StateDraft, types.StateReady},
		{"ready to taken", types.StateReady, types.StateTaken},
		{"taken to pebble", types.StateTaken, types.StatePebble},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			crumbsTbl, err := backend.GetTable(types.TableCrumbs)
			require.NoError(t, err)

			// Create crumb (starts as draft).
			crumb := &types.Crumb{Name: "Test crumb"}
			id, err := crumbsTbl.Set("", crumb)
			require.NoError(t, err)

			// If fromState is not draft, transition to fromState first.
			if tt.fromState != types.StateDraft {
				_, err = crumbsTbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "Test crumb", State: tt.fromState,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)

				got, err := crumbsTbl.Get(id)
				require.NoError(t, err)
				assert.Equal(t, tt.fromState, got.(*types.Crumb).State)
			}

			// Transition to target state.
			_, err = crumbsTbl.Set(id, &types.Crumb{
				CrumbID: id, Name: "Test crumb", State: tt.toState,
				CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
			})
			require.NoError(t, err)

			got, err := crumbsTbl.Get(id)
			require.NoError(t, err)
			assert.Equal(t, tt.toState, got.(*types.Crumb).State)
		})
	}
}

// --- TestCrumbDustTransitions: S6, S7 ---

func TestCrumbLifecycle_DustFromAnyState(t *testing.T) {
	tests := []struct {
		name      string
		fromState string
	}{
		{"draft to dust", types.StateDraft},
		{"pending to dust", types.StatePending},
		{"ready to dust", types.StateReady},
		{"taken to dust", types.StateTaken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			crumbsTbl, err := backend.GetTable(types.TableCrumbs)
			require.NoError(t, err)

			crumb := &types.Crumb{Name: "Dust crumb"}
			id, err := crumbsTbl.Set("", crumb)
			require.NoError(t, err)

			// Move to fromState if needed.
			if tt.fromState != types.StateDraft {
				_, err = crumbsTbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "Dust crumb", State: tt.fromState,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)
			}

			// Transition to dust.
			_, err = crumbsTbl.Set(id, &types.Crumb{
				CrumbID: id, Name: "Dust crumb", State: types.StateDust,
				CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
			})
			require.NoError(t, err)

			got, err := crumbsTbl.Get(id)
			require.NoError(t, err)
			assert.Equal(t, types.StateDust, got.(*types.Crumb).State)
		})
	}
}

// --- TestCrumbTimestampTracking: S8 ---

func TestCrumbLifecycle_TimestampAdvancesOnTransition(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Timestamp crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	originalCreatedAt := got.(*types.Crumb).CreatedAt
	originalUpdatedAt := got.(*types.Crumb).UpdatedAt

	// Wait to ensure timestamp difference.
	time.Sleep(1100 * time.Millisecond)

	// Transition to ready with a new UpdatedAt.
	newTime := time.Now().UTC()
	_, err = crumbsTbl.Set(id, &types.Crumb{
		CrumbID: id, Name: "Timestamp crumb", State: types.StateReady,
		CreatedAt: crumb.CreatedAt, UpdatedAt: newTime,
	})
	require.NoError(t, err)

	got2, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got2.(*types.Crumb)

	assert.Equal(t, types.StateReady, gotCrumb.State)
	assert.True(t, gotCrumb.UpdatedAt.After(originalUpdatedAt),
		"UpdatedAt should advance on state transition")
	// CreatedAt should remain constant (we pass the original).
	assert.Equal(t, originalCreatedAt.Unix(), gotCrumb.CreatedAt.Unix(),
		"CreatedAt must remain constant")
}

// --- TestCrumbFetchByState: S9, S10 ---

func TestCrumbLifecycle_FetchBySingleState(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Create crumbs in different states.
	c1 := &types.Crumb{Name: "Draft crumb"}
	_, err = crumbsTbl.Set("", c1)
	require.NoError(t, err)

	c2 := &types.Crumb{Name: "Ready crumb"}
	id2, err := crumbsTbl.Set("", c2)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(id2, &types.Crumb{CrumbID: id2, Name: "Ready crumb", State: types.StateReady, CreatedAt: c2.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	c3 := &types.Crumb{Name: "Taken crumb"}
	id3, err := crumbsTbl.Set("", c3)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(id3, &types.Crumb{CrumbID: id3, Name: "Taken crumb", State: types.StateTaken, CreatedAt: c3.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	results, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StateReady}})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Ready crumb", results[0].(*types.Crumb).Name)
}

func TestCrumbLifecycle_FilterExcludesTerminalStates(t *testing.T) {
	tests := []struct {
		name         string
		filterState  string
		excludeState string
		excludeName  string
	}{
		{"excludes pebble from draft filter", types.StateDraft, types.StatePebble, "Pebble crumb"},
		{"excludes dust from draft filter", types.StateDraft, types.StateDust, "Dust crumb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			crumbsTbl, err := backend.GetTable(types.TableCrumbs)
			require.NoError(t, err)

			// Create draft crumb.
			_, err = crumbsTbl.Set("", &types.Crumb{Name: "Draft crumb"})
			require.NoError(t, err)

			// Create terminal-state crumb.
			c := &types.Crumb{Name: tt.excludeName}
			id, err := crumbsTbl.Set("", c)
			require.NoError(t, err)
			_, err = crumbsTbl.Set(id, &types.Crumb{CrumbID: id, Name: tt.excludeName, State: tt.excludeState, CreatedAt: c.CreatedAt, UpdatedAt: time.Now().UTC()})
			require.NoError(t, err)

			results, err := crumbsTbl.Fetch(types.Filter{"states": []string{tt.filterState}})
			require.NoError(t, err)
			assert.Len(t, results, 1)
			assert.Equal(t, "Draft crumb", results[0].(*types.Crumb).Name)
		})
	}
}

func TestCrumbLifecycle_FilterForTerminalState(t *testing.T) {
	tests := []struct {
		name        string
		targetState string
		targetName  string
	}{
		{"filter for pebble", types.StatePebble, "Pebble crumb"},
		{"filter for dust", types.StateDust, "Dust crumb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			crumbsTbl, err := backend.GetTable(types.TableCrumbs)
			require.NoError(t, err)

			// Create one of each state type.
			_, err = crumbsTbl.Set("", &types.Crumb{Name: "Draft crumb"})
			require.NoError(t, err)

			cPeb := &types.Crumb{Name: "Pebble crumb"}
			idPeb, err := crumbsTbl.Set("", cPeb)
			require.NoError(t, err)
			_, err = crumbsTbl.Set(idPeb, &types.Crumb{CrumbID: idPeb, Name: "Pebble crumb", State: types.StatePebble, CreatedAt: cPeb.CreatedAt, UpdatedAt: time.Now().UTC()})
			require.NoError(t, err)

			cDust := &types.Crumb{Name: "Dust crumb"}
			idDust, err := crumbsTbl.Set("", cDust)
			require.NoError(t, err)
			_, err = crumbsTbl.Set(idDust, &types.Crumb{CrumbID: idDust, Name: "Dust crumb", State: types.StateDust, CreatedAt: cDust.CreatedAt, UpdatedAt: time.Now().UTC()})
			require.NoError(t, err)

			results, err := crumbsTbl.Fetch(types.Filter{"states": []string{tt.targetState}})
			require.NoError(t, err)
			assert.Len(t, results, 1)
			assert.Equal(t, tt.targetName, results[0].(*types.Crumb).Name)
		})
	}
}

// --- TestCrumbFetchAllStates ---

func TestCrumbLifecycle_ListWithoutFilterReturnsAll(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	names := []string{"Draft crumb", "Ready crumb", "Taken crumb"}
	states := []string{types.StateDraft, types.StateReady, types.StateTaken}

	for i, name := range names {
		c := &types.Crumb{Name: name}
		id, err := crumbsTbl.Set("", c)
		require.NoError(t, err)

		if states[i] != types.StateDraft {
			_, err = crumbsTbl.Set(id, &types.Crumb{CrumbID: id, Name: name, State: states[i], CreatedAt: c.CreatedAt, UpdatedAt: time.Now().UTC()})
			require.NoError(t, err)
		}
	}

	results, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	gotNames := make(map[string]bool)
	for _, r := range results {
		gotNames[r.(*types.Crumb).Name] = true
	}
	for _, name := range names {
		assert.True(t, gotNames[name], "expected to find crumb %q", name)
	}
}

// --- TestFullSuccessPath ---

func TestCrumbLifecycle_FullSuccessPath(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Success path crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	// Verify initial state is draft.
	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	assert.Equal(t, types.StateDraft, got.(*types.Crumb).State)

	// Walk the full success path: draft → pending → ready → taken → pebble.
	transitions := []string{types.StatePending, types.StateReady, types.StateTaken, types.StatePebble}
	for _, state := range transitions {
		_, err = crumbsTbl.Set(id, &types.Crumb{
			CrumbID: id, Name: "Success path crumb", State: state,
			CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
	}

	got, err = crumbsTbl.Get(id)
	require.NoError(t, err)
	assert.Equal(t, types.StatePebble, got.(*types.Crumb).State)
}

// --- TestFullFailurePath ---

func TestCrumbLifecycle_FullFailurePath(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Failure path crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	// Verify initial state is draft.
	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	assert.Equal(t, types.StateDraft, got.(*types.Crumb).State)

	// Transition directly to dust.
	_, err = crumbsTbl.Set(id, &types.Crumb{
		CrumbID: id, Name: "Failure path crumb", State: types.StateDust,
		CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	got, err = crumbsTbl.Get(id)
	require.NoError(t, err)
	assert.Equal(t, types.StateDust, got.(*types.Crumb).State)
}

// --- TestMixedTerminalStates ---

func TestCrumbLifecycle_MixedTerminalStates(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Create crumbs in different states.
	_, err = crumbsTbl.Set("", &types.Crumb{Name: "Draft crumb"})
	require.NoError(t, err)

	cPeb := &types.Crumb{Name: "Pebble crumb"}
	idPeb, err := crumbsTbl.Set("", cPeb)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(idPeb, &types.Crumb{CrumbID: idPeb, Name: "Pebble crumb", State: types.StatePebble, CreatedAt: cPeb.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	cDust := &types.Crumb{Name: "Dust crumb"}
	idDust, err := crumbsTbl.Set("", cDust)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(idDust, &types.Crumb{CrumbID: idDust, Name: "Dust crumb", State: types.StateDust, CreatedAt: cDust.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	// Verify total count.
	all, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Verify per-state counts.
	drafts, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StateDraft}})
	require.NoError(t, err)
	assert.Len(t, drafts, 1)

	pebbles, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StatePebble}})
	require.NoError(t, err)
	assert.Len(t, pebbles, 1)

	dusts, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StateDust}})
	require.NoError(t, err)
	assert.Len(t, dusts, 1)
}

// --- TestPebbleFromNonTaken: entity method validation ---

func TestCrumbLifecycle_PebbleFromNonTakenFails(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"pebble from draft", types.StateDraft},
		{"pebble from pending", types.StatePending},
		{"pebble from ready", types.StateReady},
		{"pebble from dust", types.StateDust},
		{"pebble from pebble", types.StatePebble},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crumb := &types.Crumb{Name: "Test", State: tt.state}
			err := crumb.Pebble()
			assert.ErrorIs(t, err, types.ErrInvalidTransition)
		})
	}
}

func TestCrumbLifecycle_PebbleFromTakenSucceeds(t *testing.T) {
	crumb := &types.Crumb{Name: "Test", State: types.StateTaken}
	err := crumb.Pebble()
	require.NoError(t, err)
	assert.Equal(t, types.StatePebble, crumb.State)
}

func TestCrumbLifecycle_DustFromAnyStateSucceeds(t *testing.T) {
	for _, state := range []string{types.StateDraft, types.StatePending, types.StateReady, types.StateTaken, types.StatePebble, types.StateDust} {
		t.Run("dust from "+state, func(t *testing.T) {
			crumb := &types.Crumb{Name: "Test", State: state}
			err := crumb.Dust()
			require.NoError(t, err)
			assert.Equal(t, types.StateDust, crumb.State)
		})
	}
}

// --- TestCrumbLifecycle_PersistAndReloadState ---

func TestCrumbLifecycle_PersistAndReloadState(t *testing.T) {
	dataDir := t.TempDir()

	// Phase 1: Create crumb and transition to pebble.
	b1 := newBackendWithDir(t, dataDir)
	crumbsTbl, err := b1.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Persist state crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	// Walk to pebble: draft → taken → pebble.
	for _, state := range []string{types.StateTaken, types.StatePebble} {
		_, err = crumbsTbl.Set(id, &types.Crumb{
			CrumbID: id, Name: "Persist state crumb", State: state,
			CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
	}

	require.NoError(t, b1.Detach())

	// Phase 2: Delete db, re-attach, verify state survived via JSONL.
	os.Remove(filepath.Join(dataDir, "cupboard.db"))

	b2 := newBackendWithDir(t, dataDir)
	defer b2.Detach()

	crumbsTbl2, err := b2.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	got, err := crumbsTbl2.Get(id)
	require.NoError(t, err)
	assert.Equal(t, types.StatePebble, got.(*types.Crumb).State)
}

// newBackendWithDir creates a backend attached to a specific directory.
func newBackendWithDir(t *testing.T, dataDir string) *sqlite.Backend {
	t.Helper()
	backend := sqlite.NewBackend()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}
	err := backend.Attach(cfg)
	require.NoError(t, err)
	return backend
}
