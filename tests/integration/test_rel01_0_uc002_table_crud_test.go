// Integration tests for the Table interface CRUD operations (Get, Set, Delete,
// Fetch) through the SQLite backend. Covers crumbs and trails tables, UUID v7
// generation, field fidelity, filtering, JSONL persistence, error handling,
// and JSONL roundtrip after database deletion.
// Implements: test-rel01.0-uc002-table-crud;
//             prd001-cupboard-core R3; prd002-sqlite-backend R5, R12-R15;
//             prd003-crumbs-interface R3, R6-R10.
package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S1: Set with empty ID returns UUID v7 and persists to JSONL ---

func TestCRUD_CreateCrumbGeneratesUUIDv7(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Test crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Verify UUID v7 format.
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())

	// Verify state is draft (forced by Set on create).
	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, "Test crumb", gotCrumb.Name)
	assert.Equal(t, types.StateDraft, gotCrumb.State)
}

func TestCRUD_CreatedCrumbPersistsToJSONL(t *testing.T) {
	backend, dataDir := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Persisted crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())

	content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `"name":"Persisted crumb"`)
}

func TestCRUD_TwoCreatesGenerateUniqueUUIDs(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id1, err := crumbsTbl.Set("", &types.Crumb{Name: "First crumb"})
	require.NoError(t, err)
	id2, err := crumbsTbl.Set("", &types.Crumb{Name: "Second crumb"})
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2)

	for _, id := range []string{id1, id2} {
		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		assert.Equal(t, uuid.Version(7), parsed.Version())
	}
}

// --- S2: Get(id) returns entity with all fields matching ---

func TestCRUD_GetRetrievesEntityWithMatchingFields(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Retrieve me"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, id, gotCrumb.CrumbID)
	assert.Equal(t, "Retrieve me", gotCrumb.Name)
	assert.Equal(t, types.StateDraft, gotCrumb.State)
	assert.False(t, gotCrumb.CreatedAt.IsZero())
	assert.False(t, gotCrumb.UpdatedAt.IsZero())
}

func TestCRUD_RoundTripFidelity(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Fidelity test"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, id, gotCrumb.CrumbID)
	assert.Equal(t, "Fidelity test", gotCrumb.Name)
	assert.Equal(t, types.StateDraft, gotCrumb.State)
	assert.False(t, gotCrumb.CreatedAt.IsZero())
	assert.False(t, gotCrumb.UpdatedAt.IsZero())
}

// --- S3: Set(id, entity) updates entity; Get confirms change ---

func TestCRUD_UpdateEntityViaSet(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Original name"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	updated := &types.Crumb{CrumbID: id, Name: "Updated name", State: types.StateDraft, CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC()}
	_, err = crumbsTbl.Set(id, updated)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, "Updated name", gotCrumb.Name)
	assert.Equal(t, types.StateDraft, gotCrumb.State)
}

func TestCRUD_UpdateConfirmedViaGet(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Before update"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	updated := &types.Crumb{CrumbID: id, Name: "After update", State: types.StateReady, CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC()}
	_, err = crumbsTbl.Set(id, updated)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, "After update", gotCrumb.Name)
	assert.Equal(t, types.StateReady, gotCrumb.State)
}

func TestCRUD_UpdatePersistsToJSONL(t *testing.T) {
	backend, dataDir := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "JSONL update test"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	updated := &types.Crumb{CrumbID: id, Name: "JSONL updated", State: types.StateTaken, CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC()}
	_, err = crumbsTbl.Set(id, updated)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `"name":"JSONL updated"`)
	assert.Contains(t, string(content), `"state":"taken"`)
}

func TestCRUD_UpdatedAtChangesOnUpdate(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Timestamp test"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	originalUpdatedAt := got.(*types.Crumb).UpdatedAt

	// Update with a later timestamp.
	newTime := time.Now().UTC().Add(time.Second)
	updated := &types.Crumb{CrumbID: id, Name: "Timestamp updated", State: types.StateDraft, CreatedAt: crumb.CreatedAt, UpdatedAt: newTime}
	_, err = crumbsTbl.Set(id, updated)
	require.NoError(t, err)

	got2, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	// The stored UpdatedAt should be at or after the original.
	assert.True(t, !got2.(*types.Crumb).UpdatedAt.Before(originalUpdatedAt),
		"UpdatedAt should advance on update")
}

// --- S4: Fetch with empty filter returns all entities ---

func TestCRUD_FetchEmptyFilterReturnsAll(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	for _, name := range []string{"Crumb A", "Crumb B", "Crumb C"} {
		_, err := crumbsTbl.Set("", &types.Crumb{Name: name})
		require.NoError(t, err)
	}

	results, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestCRUD_FetchEmptyTableReturnsEmptyList(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	results, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- S5: Fetch with filter returns only matching entities ---

func TestCRUD_FetchWithStateFilter(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Create crumbs in different states.
	c1 := &types.Crumb{Name: "Draft crumb"}
	id1, err := crumbsTbl.Set("", c1)
	require.NoError(t, err)
	_ = id1

	c2 := &types.Crumb{Name: "Ready crumb 1"}
	id2, err := crumbsTbl.Set("", c2)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(id2, &types.Crumb{CrumbID: id2, Name: "Ready crumb 1", State: types.StateReady, CreatedAt: c2.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	c3 := &types.Crumb{Name: "Ready crumb 2"}
	id3, err := crumbsTbl.Set("", c3)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(id3, &types.Crumb{CrumbID: id3, Name: "Ready crumb 2", State: types.StateReady, CreatedAt: c3.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	c4 := &types.Crumb{Name: "Taken crumb"}
	id4, err := crumbsTbl.Set("", c4)
	require.NoError(t, err)
	_, err = crumbsTbl.Set(id4, &types.Crumb{CrumbID: id4, Name: "Taken crumb", State: types.StateTaken, CreatedAt: c4.CreatedAt, UpdatedAt: time.Now().UTC()})
	require.NoError(t, err)

	// Fetch with states filter (crumbs use "states" with []string).
	results, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StateReady}})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, types.StateReady, r.(*types.Crumb).State)
	}
}

func TestCRUD_FetchWithFilterNoMatches(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	_, err = crumbsTbl.Set("", &types.Crumb{Name: "Draft crumb"})
	require.NoError(t, err)

	results, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StatePebble}})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- S6: Delete(id) removes entity; subsequent Get returns error ---

func TestCRUD_DeleteRemovesEntity(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Delete me"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	err = crumbsTbl.Delete(id)
	require.NoError(t, err)

	_, err = crumbsTbl.Get(id)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestCRUD_DeleteRemovesFromJSONL(t *testing.T) {
	backend, dataDir := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "JSONL delete test"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	err = crumbsTbl.Delete(id)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), `"name":"JSONL delete test"`)
}

func TestCRUD_FetchAfterDeleteExcludesDeleted(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id1, err := crumbsTbl.Set("", &types.Crumb{Name: "Keep this"})
	require.NoError(t, err)
	id2, err := crumbsTbl.Set("", &types.Crumb{Name: "Delete this"})
	require.NoError(t, err)

	err = crumbsTbl.Delete(id2)
	require.NoError(t, err)

	results, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, id1, results[0].(*types.Crumb).CrumbID)
}

// --- S7: Get and Delete on nonexistent IDs return errors ---

func TestCRUD_GetNonexistentReturnsError(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
	}{
		{"nonexistent crumb", types.TableCrumbs},
		{"nonexistent trail", types.TableTrails},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			tbl, err := backend.GetTable(tt.tableName)
			require.NoError(t, err)

			_, err = tbl.Get("nonexistent-uuid-12345")
			assert.ErrorIs(t, err, types.ErrNotFound)
		})
	}
}

func TestCRUD_DeleteNonexistentReturnsError(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
	}{
		{"nonexistent crumb", types.TableCrumbs},
		{"nonexistent trail", types.TableTrails},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			tbl, err := backend.GetTable(tt.tableName)
			require.NoError(t, err)

			err = tbl.Delete("nonexistent-uuid-12345")
			assert.ErrorIs(t, err, types.ErrNotFound)
		})
	}
}

// --- S8: Same operations work on crumbs and trails tables ---

func TestCRUD_TrailOperations(t *testing.T) {
	backend, dataDir := newAttachedBackend(t)
	defer backend.Detach()

	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)

	t.Run("create trail generates UUID v7", func(t *testing.T) {
		trail := &types.Trail{State: types.TrailStateActive}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		assert.Equal(t, uuid.Version(7), parsed.Version())

		got, err := trailsTbl.Get(id)
		require.NoError(t, err)
		// Set forces draft on create.
		assert.Equal(t, types.TrailStateDraft, got.(*types.Trail).State)
	})

	t.Run("get trail returns entity with matching fields", func(t *testing.T) {
		trail := &types.Trail{}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)

		got, err := trailsTbl.Get(id)
		require.NoError(t, err)
		gotTrail := got.(*types.Trail)
		assert.Equal(t, id, gotTrail.TrailID)
		assert.Equal(t, types.TrailStateDraft, gotTrail.State)
	})

	t.Run("update trail via Set", func(t *testing.T) {
		trail := &types.Trail{}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)

		updated := &types.Trail{TrailID: id, State: types.TrailStateActive, CreatedAt: trail.CreatedAt}
		_, err = trailsTbl.Set(id, updated)
		require.NoError(t, err)

		got, err := trailsTbl.Get(id)
		require.NoError(t, err)
		assert.Equal(t, types.TrailStateActive, got.(*types.Trail).State)
	})

	t.Run("fetch all trails", func(t *testing.T) {
		// Start fresh for this subtest by counting what we have.
		results, err := trailsTbl.Fetch(nil)
		require.NoError(t, err)
		baseCount := len(results)

		_, err = trailsTbl.Set("", &types.Trail{})
		require.NoError(t, err)
		_, err = trailsTbl.Set("", &types.Trail{})
		require.NoError(t, err)

		results, err = trailsTbl.Fetch(nil)
		require.NoError(t, err)
		assert.Equal(t, baseCount+2, len(results))
	})

	t.Run("fetch trails with state filter", func(t *testing.T) {
		// Create additional trails with specific states.
		t1 := &types.Trail{}
		id1, err := trailsTbl.Set("", t1)
		require.NoError(t, err)
		_, err = trailsTbl.Set(id1, &types.Trail{TrailID: id1, State: types.TrailStateActive, CreatedAt: t1.CreatedAt})
		require.NoError(t, err)

		t2 := &types.Trail{}
		id2, err := trailsTbl.Set("", t2)
		require.NoError(t, err)
		_, err = trailsTbl.Set(id2, &types.Trail{TrailID: id2, State: types.TrailStateActive, CreatedAt: t2.CreatedAt})
		require.NoError(t, err)

		// Trails table uses "state" (singular string) for filter.
		results, err := trailsTbl.Fetch(types.Filter{"state": types.TrailStateActive})
		require.NoError(t, err)
		for _, r := range results {
			assert.Equal(t, types.TrailStateActive, r.(*types.Trail).State)
		}
		assert.GreaterOrEqual(t, len(results), 2)
	})

	t.Run("delete trail", func(t *testing.T) {
		trail := &types.Trail{}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)

		err = trailsTbl.Delete(id)
		require.NoError(t, err)

		_, err = trailsTbl.Get(id)
		assert.ErrorIs(t, err, types.ErrNotFound)
	})

	t.Run("trail persists to JSONL", func(t *testing.T) {
		trail := &types.Trail{}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)
		_ = id

		content, err := os.ReadFile(filepath.Join(dataDir, "trails.jsonl"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"state":"draft"`)
	})
}

// --- S9: JSONL file reflects create, update, delete operations ---

func TestCRUD_JSONLReflectsOperations(t *testing.T) {
	backend, dataDir := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	t.Run("create reflected in JSONL", func(t *testing.T) {
		_, err := crumbsTbl.Set("", &types.Crumb{Name: "Create JSONL test"})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"name":"Create JSONL test"`)
	})

	t.Run("update reflected in JSONL", func(t *testing.T) {
		crumb := &types.Crumb{Name: "Before JSONL update"}
		id, err := crumbsTbl.Set("", crumb)
		require.NoError(t, err)

		_, err = crumbsTbl.Set(id, &types.Crumb{CrumbID: id, Name: "After JSONL update", State: types.StateReady, CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC()})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"name":"After JSONL update"`)
		assert.Contains(t, string(content), `"state":"ready"`)
		assert.NotContains(t, string(content), `"name":"Before JSONL update"`)
	})

	t.Run("delete reflected in JSONL", func(t *testing.T) {
		c1 := &types.Crumb{Name: "To be deleted"}
		id1, err := crumbsTbl.Set("", c1)
		require.NoError(t, err)
		_, err = crumbsTbl.Set("", &types.Crumb{Name: "To be kept"})
		require.NoError(t, err)

		err = crumbsTbl.Delete(id1)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"name":"To be kept"`)
		assert.NotContains(t, string(content), `"name":"To be deleted"`)
	})

	t.Run("multiple operations reflected in JSONL", func(t *testing.T) {
		// Fresh backend for this subtest.
		b2, dd2 := newAttachedBackend(t)
		defer b2.Detach()
		tbl2, err := b2.GetTable(types.TableCrumbs)
		require.NoError(t, err)

		c1 := &types.Crumb{Name: "Multi op 1"}
		id1, err := tbl2.Set("", c1)
		require.NoError(t, err)
		c2 := &types.Crumb{Name: "Multi op 2"}
		id2, err := tbl2.Set("", c2)
		require.NoError(t, err)

		// Update c1, delete c2.
		_, err = tbl2.Set(id1, &types.Crumb{CrumbID: id1, Name: "Multi op 1 updated", State: types.StateReady, CreatedAt: c1.CreatedAt, UpdatedAt: time.Now().UTC()})
		require.NoError(t, err)
		err = tbl2.Delete(id2)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dd2, "crumbs.jsonl"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"name":"Multi op 1 updated"`)
		assert.Contains(t, string(content), `"state":"ready"`)
		assert.NotContains(t, string(content), `"name":"Multi op 2"`)
	})

	t.Run("JSONL lines are valid JSON", func(t *testing.T) {
		b3, dd3 := newAttachedBackend(t)
		defer b3.Detach()
		tbl3, err := b3.GetTable(types.TableCrumbs)
		require.NoError(t, err)

		_, err = tbl3.Set("", &types.Crumb{Name: "Valid JSON 1"})
		require.NoError(t, err)
		_, err = tbl3.Set("", &types.Crumb{Name: "Valid JSON 2"})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dd3, "crumbs.jsonl"))
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		for i, line := range lines {
			assert.True(t, json.Valid([]byte(line)), "line %d is not valid JSON: %s", i, line)
		}
	})
}

// --- S10: Detach prevents further operations ---

func TestCRUD_OperationsAfterDetach(t *testing.T) {
	backend, _ := newAttachedBackend(t)

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Pre-detach crumb"}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	require.NoError(t, backend.Detach())

	// GetTable should fail after detach.
	_, err = backend.GetTable(types.TableCrumbs)
	assert.ErrorIs(t, err, types.ErrCupboardDetached)

	// Direct operations on previously-obtained table references will panic or
	// error at the db layer since the connection is closed. We test GetTable
	// which is the documented gate.
	_ = id
}

// --- JSONL Roundtrip: data survives database deletion ---

func TestCRUD_JSONLRoundtrip(t *testing.T) {
	dataDir := t.TempDir()

	// Phase 1: Attach, write data, detach.
	backend1 := sqlite.NewBackend()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}
	require.NoError(t, backend1.Attach(cfg))

	crumbsTbl, err := backend1.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	trailsTbl, err := backend1.GetTable(types.TableTrails)
	require.NoError(t, err)

	crumb := &types.Crumb{Name: "Roundtrip crumb"}
	crumbID, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	trail := &types.Trail{}
	trailID, err := trailsTbl.Set("", trail)
	require.NoError(t, err)

	require.NoError(t, backend1.Detach())

	// Phase 2: Delete cupboard.db to simulate fresh start from JSONL only.
	dbPath := filepath.Join(dataDir, "cupboard.db")
	_ = os.Remove(dbPath)
	_, err = os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err), "cupboard.db should be deleted")

	// Verify JSONL files still exist and contain data.
	crumbsJSONL, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
	require.NoError(t, err)
	assert.NotEmpty(t, crumbsJSONL)

	trailsJSONL, err := os.ReadFile(filepath.Join(dataDir, "trails.jsonl"))
	require.NoError(t, err)
	assert.NotEmpty(t, trailsJSONL)

	// Phase 3: Re-attach and verify data loaded from JSONL.
	backend2 := sqlite.NewBackend()
	require.NoError(t, backend2.Attach(cfg))
	defer backend2.Detach()

	crumbsTbl2, err := backend2.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	trailsTbl2, err := backend2.GetTable(types.TableTrails)
	require.NoError(t, err)

	// Crumb should be recoverable.
	got, err := crumbsTbl2.Get(crumbID)
	require.NoError(t, err)
	gotCrumb := got.(*types.Crumb)
	assert.Equal(t, crumbID, gotCrumb.CrumbID)
	assert.Equal(t, "Roundtrip crumb", gotCrumb.Name)
	assert.Equal(t, types.StateDraft, gotCrumb.State)

	// Trail should be recoverable.
	gotTrail, err := trailsTbl2.Get(trailID)
	require.NoError(t, err)
	assert.Equal(t, trailID, gotTrail.(*types.Trail).TrailID)
}

// --- Fetch with limit and offset ---

func TestCRUD_FetchWithLimitAndOffset(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Create 5 crumbs.
	for i := 0; i < 5; i++ {
		_, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb"})
		require.NoError(t, err)
	}

	t.Run("limit", func(t *testing.T) {
		results, err := crumbsTbl.Fetch(types.Filter{"limit": 2})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("limit and offset", func(t *testing.T) {
		results, err := crumbsTbl.Fetch(types.Filter{"limit": 2, "offset": 1})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("offset skips first results", func(t *testing.T) {
		results, err := crumbsTbl.Fetch(types.Filter{"limit": 10, "offset": 3})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})
}
