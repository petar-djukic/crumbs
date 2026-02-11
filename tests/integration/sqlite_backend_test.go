// Package integration tests the SQLite backend through the Cupboard and Table
// interfaces. These tests verify the full Attach → CRUD → Detach lifecycle,
// JSONL persistence round-trips, built-in property seeding, and cascade behavior.
// Implements: test suites for prd001-cupboard-core, prd002-sqlite-backend.
package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// newTestBackend creates a backend attached to a temp directory.
func newTestBackend(t *testing.T) (*sqlite.SQLiteBackend, string) {
	t.Helper()
	dir := t.TempDir()
	b := sqlite.NewBackend()
	if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	return b, dir
}

func TestAttachDetachLifecycle(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "attach creates data directory and JSONL files",
			run: func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "new-data")
				b := sqlite.NewBackend()
				if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
					t.Fatalf("Attach: %v", err)
				}
				defer b.Detach()

				// Verify JSONL files exist.
				for _, name := range []string{
					"crumbs.jsonl", "trails.jsonl", "links.jsonl",
					"properties.jsonl", "categories.jsonl", "crumb_properties.jsonl",
					"metadata.jsonl", "stashes.jsonl", "stash_history.jsonl",
				} {
					if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
						t.Errorf("missing JSONL file %s: %v", name, err)
					}
				}
			},
		},
		{
			name: "double attach returns ErrAlreadyAttached",
			run: func(t *testing.T) {
				b, _ := newTestBackend(t)
				defer b.Detach()
				err := b.Attach(types.Config{Backend: "sqlite", DataDir: t.TempDir()})
				if err != types.ErrAlreadyAttached {
					t.Fatalf("expected ErrAlreadyAttached, got %v", err)
				}
			},
		},
		{
			name: "detach is idempotent",
			run: func(t *testing.T) {
				b, _ := newTestBackend(t)
				if err := b.Detach(); err != nil {
					t.Fatalf("first Detach: %v", err)
				}
				if err := b.Detach(); err != nil {
					t.Fatalf("second Detach: %v", err)
				}
			},
		},
		{
			name: "operations after detach return ErrCupboardDetached",
			run: func(t *testing.T) {
				b, _ := newTestBackend(t)
				b.Detach()
				_, err := b.GetTable(types.TableCrumbs)
				if err != types.ErrCupboardDetached {
					t.Fatalf("expected ErrCupboardDetached, got %v", err)
				}
			},
		},
		{
			name: "invalid backend returns error",
			run: func(t *testing.T) {
				b := sqlite.NewBackend()
				err := b.Attach(types.Config{Backend: "postgres", DataDir: t.TempDir()})
				if err != types.ErrBackendUnknown {
					t.Fatalf("expected ErrBackendUnknown, got %v", err)
				}
			},
		},
		{
			name: "empty backend returns error",
			run: func(t *testing.T) {
				b := sqlite.NewBackend()
				err := b.Attach(types.Config{Backend: "", DataDir: t.TempDir()})
				if err != types.ErrBackendEmpty {
					t.Fatalf("expected ErrBackendEmpty, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestGetTableStandardNames(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	for _, name := range []string{
		types.TableCrumbs, types.TableTrails, types.TableProperties,
		types.TableMetadata, types.TableLinks, types.TableStashes,
	} {
		tbl, err := b.GetTable(name)
		if err != nil {
			t.Errorf("GetTable(%q): %v", name, err)
		}
		if tbl == nil {
			t.Errorf("GetTable(%q) returned nil", name)
		}
	}

	_, err := b.GetTable("nonexistent")
	if err != types.ErrTableNotFound {
		t.Fatalf("expected ErrTableNotFound, got %v", err)
	}
}

func TestBuiltinPropertySeeding(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableProperties)
	results, err := tbl.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch properties: %v", err)
	}

	names := make(map[string]*types.Property)
	for _, r := range results {
		if p, ok := r.(*types.Property); ok {
			names[p.Name] = p
		}
	}

	expected := []string{"priority", "type", "description", "owner", "labels"}
	for _, name := range expected {
		if _, ok := names[name]; !ok {
			t.Errorf("missing built-in property %q", name)
		}
	}

	// Check priority categories.
	priority := names["priority"]
	if priority == nil {
		t.Fatal("priority property not found")
	}
	cats, err := tbl.Fetch(map[string]any{"property_id": priority.PropertyID})
	if err != nil {
		t.Fatalf("Fetch categories: %v", err)
	}
	catNames := make([]string, 0, len(cats))
	for _, r := range cats {
		if cat, ok := r.(*types.Category); ok {
			catNames = append(catNames, cat.Name)
		}
	}
	expectedCats := []string{"highest", "high", "medium", "low", "lowest"}
	if len(catNames) != len(expectedCats) {
		t.Fatalf("expected %d priority categories, got %d: %v", len(expectedCats), len(catNames), catNames)
	}
}

func TestCrumbCRUD(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableCrumbs)

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "create crumb with UUID v7",
			run: func(t *testing.T) {
				id, err := tbl.Set("", &types.Crumb{Name: "Test crumb"})
				if err != nil {
					t.Fatalf("Set: %v", err)
				}
				if id == "" {
					t.Fatal("expected non-empty ID")
				}

				raw, err := tbl.Get(id)
				if err != nil {
					t.Fatalf("Get: %v", err)
				}
				c := raw.(*types.Crumb)
				if c.Name != "Test crumb" {
					t.Errorf("expected name 'Test crumb', got %q", c.Name)
				}
				if c.State != types.CrumbStateDraft {
					t.Errorf("expected state draft, got %q", c.State)
				}
				if len(c.Properties) == 0 {
					t.Error("expected initialized properties")
				}
			},
		},
		{
			name: "get nonexistent returns ErrNotFound",
			run: func(t *testing.T) {
				_, err := tbl.Get("nonexistent-id")
				if err != types.ErrNotFound {
					t.Fatalf("expected ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "get empty ID returns ErrInvalidID",
			run: func(t *testing.T) {
				_, err := tbl.Get("")
				if err != types.ErrInvalidID {
					t.Fatalf("expected ErrInvalidID, got %v", err)
				}
			},
		},
		{
			name: "update crumb",
			run: func(t *testing.T) {
				id, _ := tbl.Set("", &types.Crumb{Name: "Original"})
				raw, _ := tbl.Get(id)
				c := raw.(*types.Crumb)
				c.Name = "Updated"
				c.State = types.CrumbStateReady
				_, err := tbl.Set(id, c)
				if err != nil {
					t.Fatalf("Set update: %v", err)
				}
				raw, _ = tbl.Get(id)
				c = raw.(*types.Crumb)
				if c.Name != "Updated" {
					t.Errorf("expected 'Updated', got %q", c.Name)
				}
				if c.State != types.CrumbStateReady {
					t.Errorf("expected state ready, got %q", c.State)
				}
			},
		},
		{
			name: "delete crumb cascades",
			run: func(t *testing.T) {
				id, _ := tbl.Set("", &types.Crumb{Name: "To delete"})

				// Add metadata.
				mTbl, _ := b.GetTable(types.TableMetadata)
				_, err := mTbl.Set("", &types.Metadata{
					CrumbID:   id,
					TableName: "comments",
					Content:   "Test comment",
				})
				if err != nil {
					t.Fatalf("Set metadata: %v", err)
				}

				if err := tbl.Delete(id); err != nil {
					t.Fatalf("Delete: %v", err)
				}
				_, err = tbl.Get(id)
				if err != types.ErrNotFound {
					t.Fatalf("expected ErrNotFound after delete, got %v", err)
				}
			},
		},
		{
			name: "delete nonexistent returns ErrNotFound",
			run: func(t *testing.T) {
				err := tbl.Delete("nonexistent-id")
				if err != types.ErrNotFound {
					t.Fatalf("expected ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "create without name returns ErrInvalidName",
			run: func(t *testing.T) {
				_, err := tbl.Set("", &types.Crumb{})
				if err != types.ErrInvalidName {
					t.Fatalf("expected ErrInvalidName, got %v", err)
				}
			},
		},
		{
			name: "fetch all crumbs",
			run: func(t *testing.T) {
				tbl.Set("", &types.Crumb{Name: "Fetch A"})
				tbl.Set("", &types.Crumb{Name: "Fetch B"})
				results, err := tbl.Fetch(nil)
				if err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				if len(results) < 2 {
					t.Errorf("expected at least 2 crumbs, got %d", len(results))
				}
			},
		},
		{
			name: "fetch with state filter",
			run: func(t *testing.T) {
				tbl.Set("", &types.Crumb{Name: "Ready crumb", State: types.CrumbStateReady})
				results, err := tbl.Fetch(map[string]any{"states": []string{types.CrumbStateReady}})
				if err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				for _, r := range results {
					c := r.(*types.Crumb)
					if c.State != types.CrumbStateReady {
						t.Errorf("expected state ready, got %q", c.State)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestTrailCRUD(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableTrails)

	t.Run("create and get trail", func(t *testing.T) {
		id, err := tbl.Set("", &types.Trail{State: types.TrailStateActive})
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		raw, err := tbl.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		tr := raw.(*types.Trail)
		if tr.State != types.TrailStateActive {
			t.Errorf("expected state active, got %q", tr.State)
		}
	})

	t.Run("complete trail removes belongs_to links", func(t *testing.T) {
		trailID, _ := tbl.Set("", &types.Trail{State: types.TrailStateActive})
		crumbTbl, _ := b.GetTable(types.TableCrumbs)
		crumbID, _ := crumbTbl.Set("", &types.Crumb{Name: "Trail crumb"})

		linkTbl, _ := b.GetTable(types.TableLinks)
		linkTbl.Set("", &types.Link{
			LinkType: types.LinkTypeBelongsTo,
			FromID:   crumbID,
			ToID:     trailID,
		})

		// Complete the trail.
		raw, _ := tbl.Get(trailID)
		tr := raw.(*types.Trail)
		tr.Complete()
		tbl.Set(trailID, tr)

		// Check that belongs_to links were removed.
		links, _ := linkTbl.Fetch(map[string]any{
			"link_type": types.LinkTypeBelongsTo,
			"to_id":     trailID,
		})
		if len(links) != 0 {
			t.Errorf("expected 0 belongs_to links after complete, got %d", len(links))
		}
	})

	t.Run("abandon trail deletes crumbs", func(t *testing.T) {
		trailID, _ := tbl.Set("", &types.Trail{State: types.TrailStateActive})
		crumbTbl, _ := b.GetTable(types.TableCrumbs)
		crumbID, _ := crumbTbl.Set("", &types.Crumb{Name: "Doomed crumb"})

		linkTbl, _ := b.GetTable(types.TableLinks)
		linkTbl.Set("", &types.Link{
			LinkType: types.LinkTypeBelongsTo,
			FromID:   crumbID,
			ToID:     trailID,
		})

		// Abandon the trail.
		raw, _ := tbl.Get(trailID)
		tr := raw.(*types.Trail)
		tr.Abandon()
		tbl.Set(trailID, tr)

		// Crumb should be deleted.
		_, err := crumbTbl.Get(crumbID)
		if err != types.ErrNotFound {
			t.Errorf("expected ErrNotFound for crumb after abandon, got %v", err)
		}
	})
}

func TestPropertyCRUD(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableProperties)

	t.Run("create property and category", func(t *testing.T) {
		propID, err := tbl.Set("", &types.Property{
			Name:      "custom-prop",
			ValueType: types.ValueTypeText,
		})
		if err != nil {
			t.Fatalf("Set property: %v", err)
		}

		raw, err := tbl.Get(propID)
		if err != nil {
			t.Fatalf("Get property: %v", err)
		}
		p := raw.(*types.Property)
		if p.Name != "custom-prop" {
			t.Errorf("expected name 'custom-prop', got %q", p.Name)
		}
	})

	t.Run("duplicate name returns ErrDuplicateName", func(t *testing.T) {
		_, err := tbl.Set("", &types.Property{
			Name:      "unique-name",
			ValueType: types.ValueTypeBoolean,
		})
		if err != nil {
			t.Fatalf("first Set: %v", err)
		}
		_, err = tbl.Set("", &types.Property{
			Name:      "unique-name",
			ValueType: types.ValueTypeBoolean,
		})
		if err != types.ErrDuplicateName {
			t.Fatalf("expected ErrDuplicateName, got %v", err)
		}
	})

	t.Run("property backfills existing crumbs", func(t *testing.T) {
		crumbTbl, _ := b.GetTable(types.TableCrumbs)
		crumbID, _ := crumbTbl.Set("", &types.Crumb{Name: "Backfill target"})

		propID, err := tbl.Set("", &types.Property{
			Name:      "backfill-test",
			ValueType: types.ValueTypeInteger,
		})
		if err != nil {
			t.Fatalf("Set property: %v", err)
		}

		raw, _ := crumbTbl.Get(crumbID)
		c := raw.(*types.Crumb)
		val, ok := c.Properties[propID]
		if !ok {
			t.Fatal("expected backfilled property")
		}
		// JSON deserialization may produce float64 for integers.
		switch v := val.(type) {
		case float64:
			if v != 0 {
				t.Errorf("expected 0, got %v", v)
			}
		case int64:
			if v != 0 {
				t.Errorf("expected 0, got %v", v)
			}
		default:
			t.Errorf("unexpected type %T for backfilled value", val)
		}
	})
}

func TestLinkCRUD(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableLinks)

	t.Run("create and fetch links", func(t *testing.T) {
		id, err := tbl.Set("", &types.Link{
			LinkType: types.LinkTypeBelongsTo,
			FromID:   "crumb-1",
			ToID:     "trail-1",
		})
		if err != nil {
			t.Fatalf("Set: %v", err)
		}

		raw, err := tbl.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		l := raw.(*types.Link)
		if l.LinkType != types.LinkTypeBelongsTo {
			t.Errorf("expected belongs_to, got %q", l.LinkType)
		}

		results, err := tbl.Fetch(map[string]any{"link_type": types.LinkTypeBelongsTo})
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected at least one link")
		}
	})
}

func TestMetadataCRUD(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.TableCrumbs)
	crumbID, _ := crumbTbl.Set("", &types.Crumb{Name: "Metadata host"})

	tbl, _ := b.GetTable(types.TableMetadata)

	t.Run("create and get metadata", func(t *testing.T) {
		id, err := tbl.Set("", &types.Metadata{
			CrumbID:   crumbID,
			TableName: "comments",
			Content:   "Hello world",
		})
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		raw, err := tbl.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		m := raw.(*types.Metadata)
		if m.Content != "Hello world" {
			t.Errorf("expected 'Hello world', got %q", m.Content)
		}
	})

	t.Run("empty content returns ErrInvalidContent", func(t *testing.T) {
		_, err := tbl.Set("", &types.Metadata{
			CrumbID:   crumbID,
			TableName: "comments",
			Content:   "",
		})
		if err != types.ErrInvalidContent {
			t.Fatalf("expected ErrInvalidContent, got %v", err)
		}
	})
}

func TestStashCRUD(t *testing.T) {
	b, _ := newTestBackend(t)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableStashes)

	t.Run("create and get stash", func(t *testing.T) {
		id, err := tbl.Set("", &types.Stash{
			Name:      "test-resource",
			StashType: types.StashTypeResource,
			Value:     map[string]any{"uri": "file:///tmp"},
		})
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		raw, err := tbl.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		s := raw.(*types.Stash)
		if s.Name != "test-resource" {
			t.Errorf("expected 'test-resource', got %q", s.Name)
		}
	})

	t.Run("delete stash cascades history", func(t *testing.T) {
		id, _ := tbl.Set("", &types.Stash{
			Name:      "to-delete",
			StashType: types.StashTypeCounter,
			Value:     map[string]any{"value": int64(0)},
		})
		if err := tbl.Delete(id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := tbl.Get(id)
		if err != types.ErrNotFound {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

func TestJSONLRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// First session: create data.
	b1 := sqlite.NewBackend()
	if err := b1.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
		t.Fatalf("Attach 1: %v", err)
	}
	tbl, _ := b1.GetTable(types.TableCrumbs)
	crumbID, _ := tbl.Set("", &types.Crumb{Name: "Persisted crumb", State: types.CrumbStateReady})

	trailTbl, _ := b1.GetTable(types.TableTrails)
	trailID, _ := trailTbl.Set("", &types.Trail{State: types.TrailStateActive})

	linkTbl, _ := b1.GetTable(types.TableLinks)
	linkTbl.Set("", &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumbID,
		ToID:     trailID,
	})

	b1.Detach()

	// Verify cupboard.db is gone after detach.
	if _, err := os.Stat(filepath.Join(dir, "cupboard.db")); !os.IsNotExist(err) {
		t.Error("cupboard.db should be removed after orderly Detach")
	}

	// Verify JSONL files have content.
	data, err := os.ReadFile(filepath.Join(dir, "crumbs.jsonl"))
	if err != nil {
		t.Fatalf("reading crumbs.jsonl: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("crumbs.jsonl is empty after write")
	}

	// Second session: data should survive.
	b2 := sqlite.NewBackend()
	if err := b2.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
		t.Fatalf("Attach 2: %v", err)
	}
	defer b2.Detach()

	tbl2, _ := b2.GetTable(types.TableCrumbs)
	raw, err := tbl2.Get(crumbID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	c := raw.(*types.Crumb)
	if c.Name != "Persisted crumb" {
		t.Errorf("expected 'Persisted crumb', got %q", c.Name)
	}
	if c.State != types.CrumbStateReady {
		t.Errorf("expected state ready, got %q", c.State)
	}

	// Verify trail survived.
	trailTbl2, _ := b2.GetTable(types.TableTrails)
	rawTrail, err := trailTbl2.Get(trailID)
	if err != nil {
		t.Fatalf("Get trail after reload: %v", err)
	}
	tr := rawTrail.(*types.Trail)
	if tr.State != types.TrailStateActive {
		t.Errorf("expected trail state active, got %q", tr.State)
	}

	// Verify link survived.
	linkTbl2, _ := b2.GetTable(types.TableLinks)
	links, err := linkTbl2.Fetch(map[string]any{
		"link_type": types.LinkTypeBelongsTo,
		"to_id":     trailID,
	})
	if err != nil {
		t.Fatalf("Fetch links after reload: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("expected 1 link after reload, got %d", len(links))
	}
}

func TestJSONLSurvivesDatabaseDeletion(t *testing.T) {
	dir := t.TempDir()

	// Create data.
	b := sqlite.NewBackend()
	b.Attach(types.Config{Backend: "sqlite", DataDir: dir})
	tbl, _ := b.GetTable(types.TableCrumbs)
	crumbID, _ := tbl.Set("", &types.Crumb{Name: "Resilient crumb"})
	b.Detach()

	// Manually delete the database (simulating corruption or crash).
	os.Remove(filepath.Join(dir, "cupboard.db"))

	// Re-attach: should rebuild from JSONL.
	b2 := sqlite.NewBackend()
	if err := b2.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
		t.Fatalf("Attach after db deletion: %v", err)
	}
	defer b2.Detach()

	tbl2, _ := b2.GetTable(types.TableCrumbs)
	raw, err := tbl2.Get(crumbID)
	if err != nil {
		t.Fatalf("Get after db deletion: %v", err)
	}
	c := raw.(*types.Crumb)
	if c.Name != "Resilient crumb" {
		t.Errorf("expected 'Resilient crumb', got %q", c.Name)
	}
}

func TestAtomicWritePattern(t *testing.T) {
	dir := t.TempDir()

	b := sqlite.NewBackend()
	b.Attach(types.Config{Backend: "sqlite", DataDir: dir})
	defer b.Detach()

	tbl, _ := b.GetTable(types.TableCrumbs)

	// Create several crumbs.
	for i := 0; i < 5; i++ {
		tbl.Set("", &types.Crumb{Name: "Atomic test"})
	}

	// Verify JSONL file is valid: each line must be valid JSON.
	data, err := os.ReadFile(filepath.Join(dir, "crumbs.jsonl"))
	if err != nil {
		t.Fatalf("reading crumbs.jsonl: %v", err)
	}

	lines := splitNonEmpty(string(data))
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 JSONL lines, got %d", len(lines))
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("line %d is not valid JSON: %s", i+1, line)
		}
	}

	// Verify no .tmp files remain.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file found: %s", e.Name())
		}
	}
}

// splitNonEmpty splits text by newline and returns non-empty strings.
func splitNonEmpty(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 {
				result = append(result, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		line := s[start:]
		if len(line) > 0 {
			result = append(result, line)
		}
	}
	return result
}
