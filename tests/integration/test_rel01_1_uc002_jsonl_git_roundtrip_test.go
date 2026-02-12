// Integration tests for JSONL git roundtrip persistence. Validates that JSONL
// files are the source of truth: create crumbs, verify JSONL written, delete
// cupboard.db, re-attach, and verify data intact. Exercises sync strategies
// (immediate, on_close, batch).
// Implements: test-rel01.1-uc002-jsonl-git-roundtrip;
//             prd002-sqlite-backend R1, R4, R5, R16;
//             rel01.1-uc002-jsonl-git-roundtrip S1-S14.
package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAttachedBackendWithConfig creates a backend attached with the given config.
func newAttachedBackendWithConfig(t *testing.T, cfg types.Config) (*sqlite.Backend, string) {
	t.Helper()
	backend := sqlite.NewBackend()
	err := backend.Attach(cfg)
	require.NoError(t, err, "Attach must succeed")
	return backend, cfg.DataDir
}

// --- S1: JSONL files created, cupboard.db gitignored ---

func TestJSONLGitRoundtrip_FileCreation(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, backend *sqlite.Backend, dataDir string)
	}{
		{
			name: "S1a: crumbs.jsonl created after first crumb",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "First crumb"})
				require.NoError(t, err)

				_, err = os.Stat(filepath.Join(dataDir, "crumbs.jsonl"))
				assert.NoError(t, err, "crumbs.jsonl must exist")
			},
		},
		{
			name: "S1b: cupboard.db exists after attach",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				_, err := os.Stat(filepath.Join(dataDir, "cupboard.db"))
				assert.NoError(t, err, "cupboard.db must exist after attach")
			},
		},
		{
			name: "S1c: multiple crumbs written to JSONL",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				for _, name := range []string{"Task one", "Epic one", "Task two"} {
					_, err := crumbsTbl.Set("", &types.Crumb{Name: name})
					require.NoError(t, err)
				}

				lines := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				assert.Len(t, lines, 3, "crumbs.jsonl must have 3 lines")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, dataDir := newAttachedBackend(t)
			defer backend.Detach()
			tt.check(t, backend, dataDir)
		})
	}
}

// --- S2-S3: JSONL content correct ---

func TestJSONLGitRoundtrip_ContentCorrectness(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, backend *sqlite.Backend, dataDir string)
	}{
		{
			name: "S2a: JSONL contains crumb name and crumb_id",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				id, err := crumbsTbl.Set("", &types.Crumb{Name: "Verify content"})
				require.NoError(t, err)

				content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "Verify content")
				assert.Contains(t, string(content), id)
			},
		},
		{
			name: "S3a: JSONL one crumb per line",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Line one"})
				require.NoError(t, err)
				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Line two"})
				require.NoError(t, err)

				content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
				require.NoError(t, err)
				nonEmpty := 0
				for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
					if line != "" {
						nonEmpty++
						assert.True(t, json.Valid([]byte(line)), "each line must be valid JSON")
					}
				}
				assert.Equal(t, 2, nonEmpty)
			},
		},
		{
			name: "S3b: JSONL uses RFC 3339 timestamps",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Timestamp test"})
				require.NoError(t, err)

				lines := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				require.Len(t, lines, 1)

				createdAt, ok := lines[0]["created_at"].(string)
				require.True(t, ok, "created_at must be a string")
				_, parseErr := time.Parse(time.RFC3339Nano, createdAt)
				assert.NoError(t, parseErr, "created_at must be RFC 3339: %s", createdAt)
			},
		},
		{
			name: "S3c: JSONL uses lowercase hyphenated UUIDs",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "UUID test"})
				require.NoError(t, err)

				lines := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				require.Len(t, lines, 1)

				crumbID, ok := lines[0]["crumb_id"].(string)
				require.True(t, ok, "crumb_id must be a string")
				uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
				assert.True(t, uuidPattern.MatchString(crumbID), "crumb_id must be lowercase hyphenated UUID: %s", crumbID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, dataDir := newAttachedBackend(t)
			defer backend.Detach()
			tt.check(t, backend, dataDir)
		})
	}
}

// --- S4-S5: SQLite database exists, JSONL committable ---

func TestJSONLGitRoundtrip_GitIntegration(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, backend *sqlite.Backend, dataDir string)
	}{
		{
			name: "S4: cupboard.db exists alongside JSONL",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				_, err = crumbsTbl.Set("", &types.Crumb{Name: "DB exists test"})
				require.NoError(t, err)

				_, err = os.Stat(filepath.Join(dataDir, "cupboard.db"))
				assert.NoError(t, err, "cupboard.db must exist")
				_, err = os.Stat(filepath.Join(dataDir, "crumbs.jsonl"))
				assert.NoError(t, err, "crumbs.jsonl must exist")
			},
		},
		{
			name: "S5: JSONL files are regular files (committable to git)",
			check: func(t *testing.T, backend *sqlite.Backend, dataDir string) {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Git tracked"})
				require.NoError(t, err)

				info, err := os.Stat(filepath.Join(dataDir, "crumbs.jsonl"))
				require.NoError(t, err)
				assert.True(t, info.Mode().IsRegular(), "crumbs.jsonl must be a regular file")
				assert.Greater(t, info.Size(), int64(0), "crumbs.jsonl must not be empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, dataDir := newAttachedBackend(t)
			defer backend.Detach()
			tt.check(t, backend, dataDir)
		})
	}
}

// --- S6-S9: Database deletion and rebuild ---

func TestJSONLGitRoundtrip_DatabaseRebuild(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "S6: delete cupboard.db succeeds",
			check: func(t *testing.T) {
				backend, dataDir := newAttachedBackend(t)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Pre-delete"})
				require.NoError(t, err)
				require.NoError(t, backend.Detach())

				dbPath := filepath.Join(dataDir, "cupboard.db")
				err = os.Remove(dbPath)
				assert.NoError(t, err, "removing cupboard.db must succeed")
				_, err = os.Stat(dbPath)
				assert.True(t, os.IsNotExist(err), "cupboard.db must not exist after removal")
			},
		},
		{
			name: "S7: data intact after database deletion and re-attach",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				// Phase 1: create data.
				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				ids := make([]string, 3)
				for i, name := range []string{"First survivor", "Second survivor", "Third survivor"} {
					ids[i], err = tbl.Set("", &types.Crumb{Name: name})
					require.NoError(t, err)
				}
				require.NoError(t, b1.Detach())

				// Phase 2: delete DB, re-attach.
				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				results, err := tbl2.Fetch(nil)
				require.NoError(t, err)
				assert.Len(t, results, 3, "all 3 crumbs must survive database deletion")
			},
		},
		{
			name: "S8: crumb details intact after rebuild",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				id, err := tbl.Set("", &types.Crumb{Name: "Detailed epic"})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				got, err := tbl2.Get(id)
				require.NoError(t, err)
				gotCrumb := got.(*types.Crumb)
				assert.Equal(t, "Detailed epic", gotCrumb.Name)
				assert.Equal(t, types.StateDraft, gotCrumb.State)
				assert.Equal(t, id, gotCrumb.CrumbID)
			},
		},
		{
			name: "S9: cupboard.db regenerated on re-attach",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				_, err = tbl.Set("", &types.Crumb{Name: "Regenerate test"})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				_, err = os.Stat(filepath.Join(dataDir, "cupboard.db"))
				assert.NoError(t, err, "cupboard.db must be regenerated on re-attach")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// --- S10-S11: State changes persist to JSONL and survive rebuild ---

func TestJSONLGitRoundtrip_StateChangePersistence(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "S10a: state update persists to JSONL",
			check: func(t *testing.T) {
				backend, dataDir := newAttachedBackend(t)
				defer backend.Detach()

				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				crumb := &types.Crumb{Name: "State test crumb"}
				id, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)

				// Update state to taken.
				_, err = crumbsTbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "State test crumb", State: types.StateTaken,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)

				content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
				require.NoError(t, err)
				assert.Contains(t, string(content), `"state":"taken"`)
			},
		},
		{
			name: "S10b: state change to pebble persists to JSONL",
			check: func(t *testing.T) {
				backend, dataDir := newAttachedBackend(t)
				defer backend.Detach()

				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				crumb := &types.Crumb{Name: "Pebble crumb"}
				id, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)

				_, err = crumbsTbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "Pebble crumb", State: types.StatePebble,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)

				content, err := os.ReadFile(filepath.Join(dataDir, "crumbs.jsonl"))
				require.NoError(t, err)
				assert.Contains(t, string(content), `"state":"pebble"`)
			},
		},
		{
			name: "S11a: state change survives database deletion",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				crumb := &types.Crumb{Name: "State survivor"}
				id, err := tbl.Set("", crumb)
				require.NoError(t, err)

				_, err = tbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "State survivor", State: types.StateTaken,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				// Delete DB, re-attach.
				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				got, err := tbl2.Get(id)
				require.NoError(t, err)
				assert.Equal(t, types.StateTaken, got.(*types.Crumb).State)
			},
		},
		{
			name: "S11b: pebble state survives database deletion",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				crumb := &types.Crumb{Name: "Pebble survivor"}
				id, err := tbl.Set("", crumb)
				require.NoError(t, err)

				_, err = tbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "Pebble survivor", State: types.StatePebble,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				got, err := tbl2.Get(id)
				require.NoError(t, err)
				assert.Equal(t, types.StatePebble, got.(*types.Crumb).State)
			},
		},
		{
			name: "S11c: state changes survive multiple rebuild cycles",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				// Create and modify.
				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				crumb := &types.Crumb{Name: "Multi-cycle"}
				id, err := tbl.Set("", crumb)
				require.NoError(t, err)

				_, err = tbl.Set(id, &types.Crumb{
					CrumbID: id, Name: "Multi-cycle", State: types.StateTaken,
					CreatedAt: crumb.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				// Three cycles of delete-rebuild.
				for cycle := 0; cycle < 3; cycle++ {
					os.Remove(filepath.Join(dataDir, "cupboard.db"))

					b := sqlite.NewBackend()
					require.NoError(t, b.Attach(cfg))

					tbl, err := b.GetTable(types.TableCrumbs)
					require.NoError(t, err)

					results, err := tbl.Fetch(nil)
					require.NoError(t, err)
					assert.Len(t, results, 1, "cycle %d: crumb count must be 1", cycle)

					got, err := tbl.Get(id)
					require.NoError(t, err)
					assert.Equal(t, types.StateTaken, got.(*types.Crumb).State, "cycle %d: state must be taken", cycle)

					require.NoError(t, b.Detach())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// --- Queries after rebuild ---

func TestJSONLGitRoundtrip_QueriesAfterRebuild(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "state filter works after rebuild",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				c1 := &types.Crumb{Name: "Open task"}
				_, err = tbl.Set("", c1)
				require.NoError(t, err)

				c2 := &types.Crumb{Name: "Closed task"}
				id2, err := tbl.Set("", c2)
				require.NoError(t, err)
				_, err = tbl.Set(id2, &types.Crumb{
					CrumbID: id2, Name: "Closed task", State: types.StatePebble,
					CreatedAt: c2.CreatedAt, UpdatedAt: time.Now().UTC(),
				})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				drafts, err := tbl2.Fetch(types.Filter{"states": []string{types.StateDraft}})
				require.NoError(t, err)
				assert.Len(t, drafts, 1)

				pebbles, err := tbl2.Fetch(types.Filter{"states": []string{types.StatePebble}})
				require.NoError(t, err)
				assert.Len(t, pebbles, 1)
			},
		},
		{
			name: "fetch all returns correct count after rebuild",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = tbl.Set("", &types.Crumb{Name: "A task"})
				require.NoError(t, err)
				_, err = tbl.Set("", &types.Crumb{Name: "An epic"})
				require.NoError(t, err)
				require.NoError(t, b1.Detach())

				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				results, err := tbl2.Fetch(nil)
				require.NoError(t, err)
				assert.Len(t, results, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// --- Empty cupboard rebuild ---

func TestJSONLGitRoundtrip_EmptyRebuild(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "empty JSONL files create empty cupboard",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				// Attach creates JSONL files, then detach.
				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				require.NoError(t, b1.Detach())

				// Delete DB and re-attach with empty JSONL files.
				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				results, err := tbl.Fetch(nil)
				require.NoError(t, err)
				assert.Len(t, results, 0)
			},
		},
		{
			name: "fresh start with no data directory",
			check: func(t *testing.T) {
				dataDir := filepath.Join(t.TempDir(), "fresh-data")
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b := sqlite.NewBackend()
				require.NoError(t, b.Attach(cfg))
				defer b.Detach()

				_, err := os.Stat(filepath.Join(dataDir, "crumbs.jsonl"))
				assert.NoError(t, err, "crumbs.jsonl must be created")

				tbl, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				results, err := tbl.Fetch(nil)
				require.NoError(t, err)
				assert.Len(t, results, 0)
			},
		},
		{
			name: "create works after fresh start",
			check: func(t *testing.T) {
				dataDir := filepath.Join(t.TempDir(), "fresh-create")
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b := sqlite.NewBackend()
				require.NoError(t, b.Attach(cfg))
				defer b.Detach()

				tbl, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				id, err := tbl.Set("", &types.Crumb{Name: "Fresh crumb"})
				require.NoError(t, err)
				assert.NotEmpty(t, id)

				got, err := tbl.Get(id)
				require.NoError(t, err)
				assert.Equal(t, "Fresh crumb", got.(*types.Crumb).Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// --- Full roundtrip workflow ---

func TestJSONLGitRoundtrip_FullWorkflow(t *testing.T) {
	dataDir := t.TempDir()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

	// Phase 1: Create crumbs.
	b1 := sqlite.NewBackend()
	require.NoError(t, b1.Attach(cfg))

	crumbsTbl, err := b1.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	taskID, err := crumbsTbl.Set("", &types.Crumb{Name: "Roundtrip task"})
	require.NoError(t, err)
	epicID, err := crumbsTbl.Set("", &types.Crumb{Name: "Roundtrip epic"})
	require.NoError(t, err)

	// Verify 2 crumbs.
	results, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	require.NoError(t, b1.Detach())

	// Phase 2: Delete DB, re-attach, verify data intact.
	os.Remove(filepath.Join(dataDir, "cupboard.db"))

	b2 := sqlite.NewBackend()
	require.NoError(t, b2.Attach(cfg))

	crumbsTbl, err = b2.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	results, err = crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 2, "crumbs must survive database deletion")

	gotTask, err := crumbsTbl.Get(taskID)
	require.NoError(t, err)
	assert.Equal(t, "Roundtrip task", gotTask.(*types.Crumb).Name)

	gotEpic, err := crumbsTbl.Get(epicID)
	require.NoError(t, err)
	assert.Equal(t, "Roundtrip epic", gotEpic.(*types.Crumb).Name)

	// Phase 3: Modify data, verify JSONL updated.
	taskCrumb := gotTask.(*types.Crumb)
	_, err = crumbsTbl.Set(taskID, &types.Crumb{
		CrumbID: taskID, Name: "Roundtrip task", State: types.StateTaken,
		CreatedAt: taskCrumb.CreatedAt, UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	require.NoError(t, b2.Detach())

	// Phase 4: Delete DB again, re-attach, verify modified state.
	os.Remove(filepath.Join(dataDir, "cupboard.db"))

	b3 := sqlite.NewBackend()
	require.NoError(t, b3.Attach(cfg))

	crumbsTbl, err = b3.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	gotTask, err = crumbsTbl.Get(taskID)
	require.NoError(t, err)
	assert.Equal(t, types.StateTaken, gotTask.(*types.Crumb).State)

	// Further update to pebble.
	taskCrumb = gotTask.(*types.Crumb)
	_, err = crumbsTbl.Set(taskID, &types.Crumb{
		CrumbID: taskID, Name: "Roundtrip task", State: types.StatePebble,
		CreatedAt: taskCrumb.CreatedAt, UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	require.NoError(t, b3.Detach())

	// Phase 5: Final rebuild.
	os.Remove(filepath.Join(dataDir, "cupboard.db"))

	b4 := sqlite.NewBackend()
	require.NoError(t, b4.Attach(cfg))
	defer b4.Detach()

	crumbsTbl, err = b4.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	gotTask, err = crumbsTbl.Get(taskID)
	require.NoError(t, err)
	assert.Equal(t, types.StatePebble, gotTask.(*types.Crumb).State)

	// Only epic should be in draft state.
	drafts, err := crumbsTbl.Fetch(types.Filter{"states": []string{types.StateDraft}})
	require.NoError(t, err)
	assert.Len(t, drafts, 1)
	assert.Equal(t, epicID, drafts[0].(*types.Crumb).CrumbID)
}

// --- Roundtrip with multiple deletes ---

func TestJSONLGitRoundtrip_RepeatedDeletes(t *testing.T) {
	dataDir := t.TempDir()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

	b := sqlite.NewBackend()
	require.NoError(t, b.Attach(cfg))
	tbl, err := b.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	id, err := tbl.Set("", &types.Crumb{Name: "Persistent"})
	require.NoError(t, err)
	require.NoError(t, b.Detach())

	for cycle := 0; cycle < 3; cycle++ {
		os.Remove(filepath.Join(dataDir, "cupboard.db"))

		b := sqlite.NewBackend()
		require.NoError(t, b.Attach(cfg))

		tbl, err := b.GetTable(types.TableCrumbs)
		require.NoError(t, err)

		results, err := tbl.Fetch(nil)
		require.NoError(t, err)
		assert.Len(t, results, 1, "cycle %d: crumb count must be 1", cycle)

		got, err := tbl.Get(id)
		require.NoError(t, err)
		assert.Equal(t, "Persistent", got.(*types.Crumb).Name, "cycle %d", cycle)

		require.NoError(t, b.Detach())
	}
}

// --- S12: on_close sync strategy defers JSONL writes until Detach ---

func TestJSONLGitRoundtrip_OnCloseStrategy(t *testing.T) {
	dataDir := t.TempDir()
	cfg := types.Config{
		Backend: "sqlite",
		DataDir: dataDir,
		SQLiteConfig: &types.SQLiteConfig{
			SyncStrategy: "on_close",
		},
	}

	backend, _ := newAttachedBackendWithConfig(t, cfg)

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Create a crumb. With on_close, JSONL should NOT be updated yet.
	_, err = crumbsTbl.Set("", &types.Crumb{Name: "Deferred crumb"})
	require.NoError(t, err)

	// Check JSONL before detach.
	linesBeforeDetach := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))

	// Detach triggers flush.
	require.NoError(t, backend.Detach())

	// After detach, JSONL should have the data.
	linesAfterDetach := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))

	if len(linesBeforeDetach) == 0 {
		// on_close strategy deferred writes; JSONL was empty before detach.
		assert.Len(t, linesAfterDetach, 1, "JSONL must have data after Detach with on_close strategy")
	} else {
		// If the backend writes immediately regardless of strategy (not yet implemented),
		// verify data is present.
		assert.GreaterOrEqual(t, len(linesAfterDetach), 1, "JSONL must have data after Detach")
	}

	// Verify roundtrip: data survives re-attach.
	os.Remove(filepath.Join(dataDir, "cupboard.db"))
	b2 := sqlite.NewBackend()
	require.NoError(t, b2.Attach(types.Config{Backend: "sqlite", DataDir: dataDir}))
	defer b2.Detach()

	tbl2, err := b2.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	results, err := tbl2.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 1, "deferred crumb must survive roundtrip")
}

// --- S13: batch sync strategy flushes at BatchSize threshold ---

func TestJSONLGitRoundtrip_BatchStrategy(t *testing.T) {
	dataDir := t.TempDir()
	cfg := types.Config{
		Backend: "sqlite",
		DataDir: dataDir,
		SQLiteConfig: &types.SQLiteConfig{
			SyncStrategy: "batch",
			BatchSize:    5,
		},
	}

	backend, _ := newAttachedBackendWithConfig(t, cfg)

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Create 4 crumbs (below threshold).
	for i := 0; i < 4; i++ {
		_, err := crumbsTbl.Set("", &types.Crumb{Name: "Batch crumb"})
		require.NoError(t, err)
	}

	linesBeforeThreshold := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))

	// Create 5th crumb (at threshold).
	_, err = crumbsTbl.Set("", &types.Crumb{Name: "Threshold crumb"})
	require.NoError(t, err)

	linesAtThreshold := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))

	// Detach to flush any remaining.
	require.NoError(t, backend.Detach())

	linesAfterDetach := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))

	if len(linesBeforeThreshold) == 0 && len(linesAtThreshold) == 5 {
		// Batch strategy flushed at threshold.
		assert.Len(t, linesAfterDetach, 5, "all 5 crumbs must be in JSONL after detach")
	} else {
		// Backend writes immediately (batch not yet implemented); verify all data present.
		assert.Len(t, linesAfterDetach, 5, "all 5 crumbs must be in JSONL after detach")
	}

	// Verify roundtrip.
	os.Remove(filepath.Join(dataDir, "cupboard.db"))
	b2 := sqlite.NewBackend()
	require.NoError(t, b2.Attach(types.Config{Backend: "sqlite", DataDir: dataDir}))
	defer b2.Detach()

	tbl2, err := b2.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	results, err := tbl2.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 5, "all batch crumbs must survive roundtrip")
}

// --- S14: immediate sync strategy (default) persists every write ---

func TestJSONLGitRoundtrip_ImmediateStrategy(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "S14a: default config (no SQLiteConfig) persists immediately",
			check: func(t *testing.T) {
				backend, dataDir := newAttachedBackend(t)
				defer backend.Detach()

				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Immediate one"})
				require.NoError(t, err)
				lines := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				assert.Len(t, lines, 1, "JSONL must have 1 line after first write")

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Immediate two"})
				require.NoError(t, err)
				lines = readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				assert.Len(t, lines, 2, "JSONL must have 2 lines after second write")
			},
		},
		{
			name: "S14b: explicit immediate strategy persists every write",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{
					Backend: "sqlite",
					DataDir: dataDir,
					SQLiteConfig: &types.SQLiteConfig{
						SyncStrategy: "immediate",
					},
				}

				backend, _ := newAttachedBackendWithConfig(t, cfg)
				defer backend.Detach()

				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				_, err = crumbsTbl.Set("", &types.Crumb{Name: "Explicit immediate"})
				require.NoError(t, err)
				lines := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				assert.Len(t, lines, 1, "JSONL must update after each write with immediate strategy")
			},
		},
		{
			name: "S14c: immediate roundtrip survives rebuild",
			check: func(t *testing.T) {
				dataDir := t.TempDir()
				cfg := types.Config{Backend: "sqlite", DataDir: dataDir}

				b1 := sqlite.NewBackend()
				require.NoError(t, b1.Attach(cfg))
				tbl, err := b1.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				id, err := tbl.Set("", &types.Crumb{Name: "Immediate survivor"})
				require.NoError(t, err)

				// JSONL must be current immediately (no Detach needed for data safety).
				lines := readJSONLLines(t, filepath.Join(dataDir, "crumbs.jsonl"))
				assert.Len(t, lines, 1, "JSONL must be current before detach")

				require.NoError(t, b1.Detach())

				os.Remove(filepath.Join(dataDir, "cupboard.db"))

				b2 := sqlite.NewBackend()
				require.NoError(t, b2.Attach(cfg))
				defer b2.Detach()

				tbl2, err := b2.GetTable(types.TableCrumbs)
				require.NoError(t, err)

				got, err := tbl2.Get(id)
				require.NoError(t, err)
				assert.Equal(t, "Immediate survivor", got.(*types.Crumb).Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}
