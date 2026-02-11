// Tests for rel01.0-uc001-cupboard-lifecycle: configuration, attach/detach,
// GetTable, trail lifecycle, crumb-trail linking, JSONL persistence, and
// full lifecycle workflow with cascade operations.
// Implements: test-rel01.0-uc001-cupboard-lifecycle.yaml.
package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

func TestLifecycleInitialize(t *testing.T) {
	t.Run("initialize creates JSONL files", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "new-data")
		b := sqlite.NewBackend()
		if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
			t.Fatalf("Attach: %v", err)
		}
		defer b.Detach()

		for _, name := range []string{"trails.jsonl", "links.jsonl", "crumbs.jsonl"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("missing %s: %v", name, err)
			}
		}
	})
}

func TestTrailCreation(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "create single trail",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				tbl := mustGetTable(t, b, types.TableTrails)

				id := mustCreateTrail(t, tbl, types.TrailStateActive)
				if id == "" {
					t.Fatal("expected non-empty trail ID")
				}
				if !isUUIDv7(id) {
					t.Errorf("expected UUID v7 format, got %q", id)
				}

				tr := mustGetTrail(t, tbl, id)
				if tr.State != types.TrailStateActive {
					t.Errorf("expected state active, got %q", tr.State)
				}

				all := fetchAll(t, tbl)
				if len(all) != 1 {
					t.Errorf("expected 1 trail, got %d", len(all))
				}
			},
		},
		{
			name: "create multiple trails with unique IDs",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				tbl := mustGetTable(t, b, types.TableTrails)

				ids := make(map[string]bool)
				for i := 0; i < 3; i++ {
					id := mustCreateTrail(t, tbl, types.TrailStateActive)
					if ids[id] {
						t.Errorf("duplicate trail ID: %s", id)
					}
					ids[id] = true
				}

				all := fetchAll(t, tbl)
				if len(all) != 3 {
					t.Errorf("expected 3 trails, got %d", len(all))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestTrailLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		endState string
	}{
		{"complete trail", types.TrailStateCompleted},
		{"abandon trail", types.TrailStateAbandoned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := setupCupboard(t)
			tbl := mustGetTable(t, b, types.TableTrails)

			id := mustCreateTrail(t, tbl, types.TrailStateActive)
			tr := mustGetTrail(t, tbl, id)

			if tt.endState == types.TrailStateCompleted {
				if err := tr.Complete(); err != nil {
					t.Fatalf("Complete: %v", err)
				}
			} else {
				if err := tr.Abandon(); err != nil {
					t.Fatalf("Abandon: %v", err)
				}
			}

			if _, err := tbl.Set(id, tr); err != nil {
				t.Fatalf("Set: %v", err)
			}

			got := mustGetTrail(t, tbl, id)
			if got.State != tt.endState {
				t.Errorf("expected state %q, got %q", tt.endState, got.State)
			}
		})
	}
}

func TestCrumbTrailLinking(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "link single crumb to trail",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				crumbTbl := mustGetTable(t, b, types.TableCrumbs)
				trailTbl := mustGetTable(t, b, types.TableTrails)
				linkTbl := mustGetTable(t, b, types.TableLinks)

				crumbID := mustCreateCrumb(t, crumbTbl, "Crumb 1", types.CrumbStateDraft)
				trailID := mustCreateTrail(t, trailTbl, types.TrailStateActive)
				linkID := mustCreateLink(t, linkTbl, types.LinkTypeBelongsTo, crumbID, trailID)

				if linkID == "" {
					t.Fatal("expected non-empty link ID")
				}

				raw, err := linkTbl.Get(linkID)
				if err != nil {
					t.Fatalf("Get link: %v", err)
				}
				l := raw.(*types.Link)
				if l.LinkType != types.LinkTypeBelongsTo {
					t.Errorf("expected belongs_to, got %q", l.LinkType)
				}

				all := fetchAll(t, linkTbl)
				if len(all) != 1 {
					t.Errorf("expected 1 link, got %d", len(all))
				}
			},
		},
		{
			name: "link multiple crumbs to different trails",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				crumbTbl := mustGetTable(t, b, types.TableCrumbs)
				trailTbl := mustGetTable(t, b, types.TableTrails)
				linkTbl := mustGetTable(t, b, types.TableLinks)

				crumb1 := mustCreateCrumb(t, crumbTbl, "Crumb 1", types.CrumbStateDraft)
				crumb2 := mustCreateCrumb(t, crumbTbl, "Crumb 2", types.CrumbStateDraft)
				trail1 := mustCreateTrail(t, trailTbl, types.TrailStateActive)
				trail2 := mustCreateTrail(t, trailTbl, types.TrailStateActive)

				mustCreateLink(t, linkTbl, types.LinkTypeBelongsTo, crumb1, trail1)
				mustCreateLink(t, linkTbl, types.LinkTypeBelongsTo, crumb2, trail2)

				all := fetchAll(t, linkTbl)
				if len(all) != 2 {
					t.Errorf("expected 2 links, got %d", len(all))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestTrailPersistence(t *testing.T) {
	t.Run("trails persisted to JSONL", func(t *testing.T) {
		b, dir := setupCupboard(t)
		tbl := mustGetTable(t, b, types.TableTrails)

		trail1ID := mustCreateTrail(t, tbl, types.TrailStateActive)
		trail2ID := mustCreateTrail(t, tbl, types.TrailStateActive)

		// Complete trail1.
		tr1 := mustGetTrail(t, tbl, trail1ID)
		tr1.Complete()
		tbl.Set(trail1ID, tr1)

		// Abandon trail2.
		tr2 := mustGetTrail(t, tbl, trail2ID)
		tr2.Abandon()
		tbl.Set(trail2ID, tr2)

		lines := readJSONLFile(t, dir, "trails.jsonl")
		if len(lines) != 2 {
			t.Errorf("expected 2 JSONL lines, got %d", len(lines))
		}

		assertJSONLContains(t, dir, "trails.jsonl", `"state":"completed"`)
		assertJSONLContains(t, dir, "trails.jsonl", `"state":"abandoned"`)
	})
}

func TestFullLifecycleWorkflow(t *testing.T) {
	t.Run("full lifecycle with crumbs and trails", func(t *testing.T) {
		b, _ := setupCupboard(t)
		crumbTbl := mustGetTable(t, b, types.TableCrumbs)
		trailTbl := mustGetTable(t, b, types.TableTrails)
		linkTbl := mustGetTable(t, b, types.TableLinks)

		// Create 3 crumbs.
		crumb1ID := mustCreateCrumb(t, crumbTbl, "Implement feature X", types.CrumbStateDraft)
		crumb2ID := mustCreateCrumb(t, crumbTbl, "Write tests for feature X", types.CrumbStateDraft)
		crumb3ID := mustCreateCrumb(t, crumbTbl, "Try approach A", types.CrumbStateDraft)

		// Transition crumb1 to pebble (via taken first, since Pebble requires taken).
		c1 := mustGetCrumb(t, crumbTbl, crumb1ID)
		c1.State = types.CrumbStatePebble
		if _, err := crumbTbl.Set(crumb1ID, c1); err != nil {
			t.Fatalf("Set crumb1 to pebble: %v", err)
		}

		// Create 2 trails.
		trail1ID := mustCreateTrail(t, trailTbl, types.TrailStateActive)
		trail2ID := mustCreateTrail(t, trailTbl, types.TrailStateActive)

		// Link crumb2 to trail1, crumb3 to trail2.
		mustCreateLink(t, linkTbl, types.LinkTypeBelongsTo, crumb2ID, trail1ID)
		mustCreateLink(t, linkTbl, types.LinkTypeBelongsTo, crumb3ID, trail2ID)

		// Complete trail1: removes belongs_to links, crumb2 becomes permanent.
		tr1 := mustGetTrail(t, trailTbl, trail1ID)
		tr1.Complete()
		if _, err := trailTbl.Set(trail1ID, tr1); err != nil {
			t.Fatalf("Complete trail1: %v", err)
		}

		// Abandon trail2: deletes crumb3 and its links.
		tr2 := mustGetTrail(t, trailTbl, trail2ID)
		tr2.Abandon()
		if _, err := trailTbl.Set(trail2ID, tr2); err != nil {
			t.Fatalf("Abandon trail2: %v", err)
		}

		// Verify final state.
		crumbs := fetchAll(t, crumbTbl)
		if len(crumbs) != 2 {
			t.Errorf("expected 2 crumbs, got %d", len(crumbs))
		}

		trails := fetchAll(t, trailTbl)
		if len(trails) != 2 {
			t.Errorf("expected 2 trails, got %d", len(trails))
		}

		links := fetchAll(t, linkTbl)
		if len(links) != 0 {
			t.Errorf("expected 0 links, got %d", len(links))
		}

		// crumb3 should be deleted by abandon cascade.
		_, err := crumbTbl.Get(crumb3ID)
		if err != types.ErrNotFound {
			t.Errorf("expected crumb3 deleted (ErrNotFound), got %v", err)
		}

		// crumb2 should still exist (permanent after complete).
		c2 := mustGetCrumb(t, crumbTbl, crumb2ID)
		if c2.Name != "Write tests for feature X" {
			t.Errorf("expected crumb2 name preserved, got %q", c2.Name)
		}

		// Count states.
		pebbles := fetchByStates(t, crumbTbl, []string{types.CrumbStatePebble})
		if len(pebbles) != 1 {
			t.Errorf("expected 1 pebble, got %d", len(pebbles))
		}

		drafts := fetchByStates(t, crumbTbl, []string{types.CrumbStateDraft})
		if len(drafts) != 1 {
			t.Errorf("expected 1 draft, got %d", len(drafts))
		}

		completed := fetchByStates(t, trailTbl, []string{types.TrailStateCompleted})
		if len(completed) != 1 {
			t.Errorf("expected 1 completed trail, got %d", len(completed))
		}

		abandoned := fetchByStates(t, trailTbl, []string{types.TrailStateAbandoned})
		if len(abandoned) != 1 {
			t.Errorf("expected 1 abandoned trail, got %d", len(abandoned))
		}
	})
}

func TestCupboardLifecycleErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "attach creates data directory",
			run: func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "subdir", "data")
				b := sqlite.NewBackend()
				if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
					t.Fatalf("Attach: %v", err)
				}
				defer b.Detach()

				if _, err := os.Stat(dir); err != nil {
					t.Errorf("data dir not created: %v", err)
				}
			},
		},
		{
			name: "double attach returns ErrAlreadyAttached",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				err := b.Attach(types.Config{Backend: "sqlite", DataDir: t.TempDir()})
				if err != types.ErrAlreadyAttached {
					t.Fatalf("expected ErrAlreadyAttached, got %v", err)
				}
			},
		},
		{
			name: "GetTable for standard names succeeds",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
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
			},
		},
		{
			name: "GetTable for unknown name returns ErrTableNotFound",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				_, err := b.GetTable("nonexistent")
				if err != types.ErrTableNotFound {
					t.Fatalf("expected ErrTableNotFound, got %v", err)
				}
			},
		},
		{
			name: "Fetch on empty table returns empty slice",
			run: func(t *testing.T) {
				b, _ := setupCupboard(t)
				tbl := mustGetTable(t, b, types.TableCrumbs)
				results := fetchAll(t, tbl)
				if len(results) != 0 {
					t.Errorf("expected 0 crumbs, got %d", len(results))
				}
			},
		},
		{
			name: "detach is idempotent",
			run: func(t *testing.T) {
				dir := t.TempDir()
				b := sqlite.NewBackend()
				if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
					t.Fatalf("Attach: %v", err)
				}
				if err := b.Detach(); err != nil {
					t.Fatalf("first Detach: %v", err)
				}
				if err := b.Detach(); err != nil {
					t.Fatalf("second Detach: %v", err)
				}
			},
		},
		{
			name: "GetTable after detach returns ErrCupboardDetached",
			run: func(t *testing.T) {
				dir := t.TempDir()
				b := sqlite.NewBackend()
				if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
					t.Fatalf("Attach: %v", err)
				}
				b.Detach()
				_, err := b.GetTable(types.TableCrumbs)
				if err != types.ErrCupboardDetached {
					t.Fatalf("expected ErrCupboardDetached, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
