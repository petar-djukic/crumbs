// Integration tests for the Cupboard lifecycle: attach, detach, GetTable,
// trail creation, trail state transitions, crumb-trail linking, JSONL
// persistence, and the full lifecycle workflow with cascade operations.
// Implements: test-rel01.0-uc001-cupboard-lifecycle;
//             prd001-cupboard-core R2, R5, R7; prd002-sqlite-backend R4, R11.
package integration

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAttachedBackend creates a fresh SQLiteBackend attached to a temp directory.
// The caller should defer backend.Detach().
func newAttachedBackend(t *testing.T) (*sqlite.Backend, string) {
	t.Helper()
	dataDir := t.TempDir()
	backend := sqlite.NewBackend()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}
	err := backend.Attach(cfg)
	require.NoError(t, err, "Attach must succeed")
	return backend, dataDir
}

func TestAttachWithValidConfig(t *testing.T) {
	dataDir := t.TempDir()
	backend := sqlite.NewBackend()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

	err := backend.Attach(cfg)
	require.NoError(t, err)
	defer backend.Detach()

	// JSONL files must be created on attach.
	expectedFiles := []string{
		"crumbs.jsonl", "trails.jsonl", "links.jsonl",
		"properties.jsonl", "categories.jsonl", "crumb_properties.jsonl",
		"metadata.jsonl", "stashes.jsonl", "stash_history.jsonl",
	}
	for _, name := range expectedFiles {
		_, err := os.Stat(filepath.Join(dataDir, name))
		assert.NoError(t, err, "expected %s to exist", name)
	}
}

func TestAttachWithInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config types.Config
	}{
		{
			name:   "empty backend",
			config: types.Config{Backend: "", DataDir: t.TempDir()},
		},
		{
			name:   "unknown backend",
			config: types.Config{Backend: "postgres", DataDir: t.TempDir()},
		},
		{
			name:   "empty data dir",
			config: types.Config{Backend: "sqlite", DataDir: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := sqlite.NewBackend()
			err := backend.Attach(tt.config)
			assert.Error(t, err, "Attach must fail with invalid config")
		})
	}
}

func TestAttachAlreadyAttached(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	err := backend.Attach(types.Config{Backend: "sqlite", DataDir: t.TempDir()})
	assert.ErrorIs(t, err, types.ErrAlreadyAttached)
}

func TestDetachIdempotent(t *testing.T) {
	backend, _ := newAttachedBackend(t)

	err := backend.Detach()
	require.NoError(t, err)

	// Second detach should also succeed (idempotent).
	err = backend.Detach()
	assert.NoError(t, err)
}

func TestGetTableStandardNames(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	tables := []string{
		types.TableCrumbs, types.TableTrails, types.TableProperties,
		types.TableMetadata, types.TableLinks, types.TableStashes,
	}
	for _, name := range tables {
		t.Run(name, func(t *testing.T) {
			tbl, err := backend.GetTable(name)
			require.NoError(t, err)
			assert.NotNil(t, tbl)
		})
	}
}

func TestGetTableUnknownName(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	_, err := backend.GetTable("nonexistent")
	assert.ErrorIs(t, err, types.ErrTableNotFound)
}

func TestOperationsAfterDetach(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	require.NoError(t, backend.Detach())

	_, err := backend.GetTable(types.TableCrumbs)
	assert.ErrorIs(t, err, types.ErrCupboardDetached)
}

func TestTrailCreation(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)

	t.Run("create single trail", func(t *testing.T) {
		trail := &types.Trail{State: types.TrailStateActive}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		got, err := trailsTbl.Get(id)
		require.NoError(t, err)
		gotTrail := got.(*types.Trail)
		// Backend forces draft on create; we then update to active.
		// Actually, looking at trails_table.go: Set with id="" forces TrailStateDraft.
		// So state will be "draft", not "active". We verify accordingly.
		assert.Equal(t, types.TrailStateDraft, gotTrail.State)
	})

	t.Run("create multiple trails with unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 3; i++ {
			trail := &types.Trail{State: types.TrailStateActive}
			id, err := trailsTbl.Set("", trail)
			require.NoError(t, err)
			assert.NotEmpty(t, id)
			ids[id] = true
		}
		assert.Len(t, ids, 3, "all trail IDs must be unique")
	})
}

func TestTrailLifecycle(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)

	t.Run("complete trail", func(t *testing.T) {
		// Create trail (defaults to draft), transition to active, then complete.
		trail := &types.Trail{}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)

		// Transition to active.
		activeTrail := &types.Trail{TrailID: id, State: types.TrailStateActive, CreatedAt: trail.CreatedAt}
		_, err = trailsTbl.Set(id, activeTrail)
		require.NoError(t, err)

		// Transition to completed.
		completedTrail := &types.Trail{TrailID: id, State: types.TrailStateCompleted, CreatedAt: trail.CreatedAt}
		_, err = trailsTbl.Set(id, completedTrail)
		require.NoError(t, err)

		got, err := trailsTbl.Get(id)
		require.NoError(t, err)
		assert.Equal(t, types.TrailStateCompleted, got.(*types.Trail).State)
	})

	t.Run("abandon trail", func(t *testing.T) {
		trail := &types.Trail{}
		id, err := trailsTbl.Set("", trail)
		require.NoError(t, err)

		activeTrail := &types.Trail{TrailID: id, State: types.TrailStateActive, CreatedAt: trail.CreatedAt}
		_, err = trailsTbl.Set(id, activeTrail)
		require.NoError(t, err)

		abandonedTrail := &types.Trail{TrailID: id, State: types.TrailStateAbandoned, CreatedAt: trail.CreatedAt}
		_, err = trailsTbl.Set(id, abandonedTrail)
		require.NoError(t, err)

		got, err := trailsTbl.Get(id)
		require.NoError(t, err)
		assert.Equal(t, types.TrailStateAbandoned, got.(*types.Trail).State)
	})
}

func TestCrumbTrailLinking(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)
	linksTbl, err := backend.GetTable(types.TableLinks)
	require.NoError(t, err)

	t.Run("link single crumb to trail", func(t *testing.T) {
		crumb := &types.Crumb{Name: "Crumb 1"}
		crumbID, err := crumbsTbl.Set("", crumb)
		require.NoError(t, err)

		trail := &types.Trail{}
		trailID, err := trailsTbl.Set("", trail)
		require.NoError(t, err)

		link := &types.Link{
			LinkType: types.LinkTypeBelongsTo,
			FromID:   crumbID,
			ToID:     trailID,
		}
		linkID, err := linksTbl.Set("", link)
		require.NoError(t, err)
		assert.NotEmpty(t, linkID)

		got, err := linksTbl.Get(linkID)
		require.NoError(t, err)
		gotLink := got.(*types.Link)
		assert.Equal(t, types.LinkTypeBelongsTo, gotLink.LinkType)
		assert.Equal(t, crumbID, gotLink.FromID)
		assert.Equal(t, trailID, gotLink.ToID)
	})

	t.Run("link multiple crumbs to different trails", func(t *testing.T) {
		crumb1 := &types.Crumb{Name: "Crumb A"}
		crumb1ID, err := crumbsTbl.Set("", crumb1)
		require.NoError(t, err)

		crumb2 := &types.Crumb{Name: "Crumb B"}
		crumb2ID, err := crumbsTbl.Set("", crumb2)
		require.NoError(t, err)

		trail1 := &types.Trail{}
		trail1ID, err := trailsTbl.Set("", trail1)
		require.NoError(t, err)

		trail2 := &types.Trail{}
		trail2ID, err := trailsTbl.Set("", trail2)
		require.NoError(t, err)

		_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1ID, ToID: trail1ID})
		require.NoError(t, err)
		_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2ID, ToID: trail2ID})
		require.NoError(t, err)

		allLinks, err := linksTbl.Fetch(nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allLinks), 2)
	})
}

func TestTrailPersistence(t *testing.T) {
	backend, dataDir := newAttachedBackend(t)
	defer backend.Detach()

	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)

	// Create two trails and transition their states.
	trail1 := &types.Trail{}
	id1, err := trailsTbl.Set("", trail1)
	require.NoError(t, err)

	trail2 := &types.Trail{}
	id2, err := trailsTbl.Set("", trail2)
	require.NoError(t, err)

	// Move to active first.
	_, err = trailsTbl.Set(id1, &types.Trail{TrailID: id1, State: types.TrailStateActive, CreatedAt: trail1.CreatedAt})
	require.NoError(t, err)
	_, err = trailsTbl.Set(id2, &types.Trail{TrailID: id2, State: types.TrailStateActive, CreatedAt: trail2.CreatedAt})
	require.NoError(t, err)

	// Now complete trail1 and abandon trail2.
	_, err = trailsTbl.Set(id1, &types.Trail{TrailID: id1, State: types.TrailStateCompleted, CreatedAt: trail1.CreatedAt})
	require.NoError(t, err)
	_, err = trailsTbl.Set(id2, &types.Trail{TrailID: id2, State: types.TrailStateAbandoned, CreatedAt: trail2.CreatedAt})
	require.NoError(t, err)

	// Verify trails.jsonl content.
	content, err := os.ReadFile(filepath.Join(dataDir, "trails.jsonl"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `"state":"completed"`)
	assert.Contains(t, string(content), `"state":"abandoned"`)
}

func TestFullLifecycleWorkflow(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)
	linksTbl, err := backend.GetTable(types.TableLinks)
	require.NoError(t, err)

	// Create 3 crumbs.
	crumb1 := &types.Crumb{Name: "Implement feature X"}
	crumb1ID, err := crumbsTbl.Set("", crumb1)
	require.NoError(t, err)

	crumb2 := &types.Crumb{Name: "Write tests for feature X"}
	crumb2ID, err := crumbsTbl.Set("", crumb2)
	require.NoError(t, err)

	crumb3 := &types.Crumb{Name: "Try approach A"}
	crumb3ID, err := crumbsTbl.Set("", crumb3)
	require.NoError(t, err)

	// Transition crumb1 to pebble (draft -> taken -> pebble via Set).
	_, err = crumbsTbl.Set(crumb1ID, &types.Crumb{CrumbID: crumb1ID, Name: "Implement feature X", State: types.StateTaken, CreatedAt: crumb1.CreatedAt, UpdatedAt: crumb1.UpdatedAt})
	require.NoError(t, err)
	_, err = crumbsTbl.Set(crumb1ID, &types.Crumb{CrumbID: crumb1ID, Name: "Implement feature X", State: types.StatePebble, CreatedAt: crumb1.CreatedAt, UpdatedAt: crumb1.UpdatedAt})
	require.NoError(t, err)

	// Create 2 trails and move them to active.
	trail1 := &types.Trail{}
	trail1ID, err := trailsTbl.Set("", trail1)
	require.NoError(t, err)
	_, err = trailsTbl.Set(trail1ID, &types.Trail{TrailID: trail1ID, State: types.TrailStateActive, CreatedAt: trail1.CreatedAt})
	require.NoError(t, err)

	trail2 := &types.Trail{}
	trail2ID, err := trailsTbl.Set("", trail2)
	require.NoError(t, err)
	_, err = trailsTbl.Set(trail2ID, &types.Trail{TrailID: trail2ID, State: types.TrailStateActive, CreatedAt: trail2.CreatedAt})
	require.NoError(t, err)

	// Link crumb2 -> trail1, crumb3 -> trail2.
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2ID, ToID: trail1ID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb3ID, ToID: trail2ID})
	require.NoError(t, err)

	// Complete trail1 -> removes belongs_to links (crumb2 becomes permanent).
	_, err = trailsTbl.Set(trail1ID, &types.Trail{TrailID: trail1ID, State: types.TrailStateCompleted, CreatedAt: trail1.CreatedAt})
	require.NoError(t, err)

	// Abandon trail2 -> deletes crumb3 and its links.
	_, err = trailsTbl.Set(trail2ID, &types.Trail{TrailID: trail2ID, State: types.TrailStateAbandoned, CreatedAt: trail2.CreatedAt})
	require.NoError(t, err)

	// Verify final state.
	allCrumbs, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allCrumbs, 2, "crumb3 should be deleted by abandon cascade")

	allTrails, err := trailsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allTrails, 2)

	allLinks, err := linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allLinks, 0, "all links should be removed by cascades")

	// Verify crumb states.
	got1, err := crumbsTbl.Get(crumb1ID)
	require.NoError(t, err)
	assert.Equal(t, types.StatePebble, got1.(*types.Crumb).State)

	got2, err := crumbsTbl.Get(crumb2ID)
	require.NoError(t, err)
	assert.Equal(t, types.StateDraft, got2.(*types.Crumb).State)

	// crumb3 should not exist.
	_, err = crumbsTbl.Get(crumb3ID)
	assert.ErrorIs(t, err, types.ErrNotFound)

	// Verify trail states.
	gotT1, err := trailsTbl.Get(trail1ID)
	require.NoError(t, err)
	assert.Equal(t, types.TrailStateCompleted, gotT1.(*types.Trail).State)

	gotT2, err := trailsTbl.Get(trail2ID)
	require.NoError(t, err)
	assert.Equal(t, types.TrailStateAbandoned, gotT2.(*types.Trail).State)
}

// readJSONLLines reads a JSONL file and returns each non-empty line as a map.
func readJSONLLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal(line, &m))
		lines = append(lines, m)
	}
	require.NoError(t, scanner.Err())
	return lines
}
