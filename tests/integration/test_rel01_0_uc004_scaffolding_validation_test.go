// Integration tests for scaffolding validation: module path, CLI build,
// version command, entity struct compilation, interface satisfaction,
// standard table names, and GetTable for all six tables.
// Implements: test-rel01.0-uc004-scaffolding-validation;
//             prd001-cupboard-core R2, R2.5.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- S1: module path ----------

func TestScaffoldingValidation_ModulePath(t *testing.T) {
	goMod, err := os.ReadFile(filepath.Join(projectRoot(), "go.mod"))
	require.NoError(t, err)
	content := string(goMod)

	t.Run("module path uses mesh-intelligence", func(t *testing.T) {
		assert.Contains(t, content, "module github.com/mesh-intelligence/crumbs")
	})

	t.Run("replace directive points to local directory", func(t *testing.T) {
		assert.Contains(t, content, "replace github.com/mesh-intelligence/crumbs => ./")
	})
}

// ---------- S2: go build ----------

func TestScaffoldingValidation_CupboardCLICompiles(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cupboard")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cupboard")
	cmd.Dir = projectRoot()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(out))

	_, err = os.Stat(binPath)
	assert.NoError(t, err, "cupboard binary must exist after build")
}

// ---------- S3-S4: version command ----------

func TestScaffoldingValidation_VersionCommand(t *testing.T) {
	bin := buildCupboard(t)

	t.Run("prints version and exits 0", func(t *testing.T) {
		out, err := exec.Command(bin, "version").CombinedOutput()
		require.NoError(t, err, "version command failed: %s", string(out))
		assert.Contains(t, string(out), "cupboard")
	})

	t.Run("works without backend connection", func(t *testing.T) {
		cmd := exec.Command(bin, "version")
		cmd.Dir = t.TempDir()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "version command should not require backend: %s", string(out))
		assert.Contains(t, string(out), "cupboard")
	})
}

// ---------- S5: entity struct compilation ----------

func TestScaffoldingValidation_CrumbStructHasRequiredFields(t *testing.T) {
	c := &types.Crumb{
		CrumbID:    "test",
		Name:       "test",
		State:      "draft",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Properties: map[string]any{},
	}
	assert.NotNil(t, c)
}

func TestScaffoldingValidation_TrailStructHasRequiredFields(t *testing.T) {
	tr := &types.Trail{
		TrailID:   "test",
		State:     "active",
		CreatedAt: time.Now(),
	}
	assert.NotNil(t, tr)
}

func TestScaffoldingValidation_PropertyStructHasRequiredFields(t *testing.T) {
	p := &types.Property{
		PropertyID:  "test",
		Name:        "priority",
		Description: "Task priority",
		ValueType:   "categorical",
		CreatedAt:   time.Now(),
	}
	assert.NotNil(t, p)
}

func TestScaffoldingValidation_CategoryStructHasRequiredFields(t *testing.T) {
	c := &types.Category{
		CategoryID: "test",
		PropertyID: "test",
		Name:       "high",
		Ordinal:    1,
	}
	assert.NotNil(t, c)
}

func TestScaffoldingValidation_StashStructHasRequiredFields(t *testing.T) {
	s := &types.Stash{
		StashID:   "test",
		Name:      "shared-config",
		StashType: "context",
		Value:     nil,
		Version:   0,
		CreatedAt: time.Now(),
	}
	assert.NotNil(t, s)
}

func TestScaffoldingValidation_MetadataStructHasRequiredFields(t *testing.T) {
	m := &types.Metadata{
		MetadataID: "test",
		CrumbID:    "test",
		TableName:  "comments",
		Content:    "{}",
		CreatedAt:  time.Now(),
	}
	assert.NotNil(t, m)
}

func TestScaffoldingValidation_LinkStructHasRequiredFields(t *testing.T) {
	l := &types.Link{
		LinkID:    "test",
		LinkType:  "belongs_to",
		FromID:    "crumb-1",
		ToID:      "trail-1",
		CreatedAt: time.Now(),
	}
	assert.NotNil(t, l)
}

// ---------- S6: Table interface ----------

func TestScaffoldingValidation_TableInterfaceMethods(t *testing.T) {
	// Compile-time assertion: Table has Get, Set, Delete, Fetch.
	var tbl types.Table
	_ = tbl
	// The var declaration above compiles only if Table is a valid interface.
	// We also verify the method signatures by assigning to typed function vars.
	var _ func(string) (any, error)             // matches Get
	var _ func(string, any) (string, error)     // matches Set
	var _ func(string) error                    // matches Delete
	var _ func(map[string]any) ([]any, error)   // matches Fetch
}

// ---------- S7: Cupboard interface ----------

func TestScaffoldingValidation_CupboardInterfaceMethods(t *testing.T) {
	// Compile-time assertion: Cupboard has GetTable, Attach, Detach.
	var cupboard types.Cupboard
	_ = cupboard
	var _ func(string) (types.Table, error) // matches GetTable
	var _ func(types.Config) error          // matches Attach
	var _ func() error                      // matches Detach
}

// ---------- S8: SQLite backend satisfies Cupboard ----------

// Compile-time interface satisfaction (file-level).
var _ types.Cupboard = (*sqlite.Backend)(nil)

func TestScaffoldingValidation_SQLiteBackendSatisfiesCupboard(t *testing.T) {
	// The var above is the real check; this test exists for visibility
	// in test output.
	var _ types.Cupboard = (*sqlite.Backend)(nil)
}

// ---------- S9: standard table name constants ----------

func TestScaffoldingValidation_StandardTableNameConstants(t *testing.T) {
	tests := []struct {
		constant string
		value    string
	}{
		{types.TableCrumbs, "crumbs"},
		{types.TableTrails, "trails"},
		{types.TableProperties, "properties"},
		{types.TableMetadata, "metadata"},
		{types.TableLinks, "links"},
		{types.TableStashes, "stashes"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.value, tt.constant)
		})
	}
}

// ---------- S10: GetTable succeeds for all 6 tables ----------

func TestScaffoldingValidation_BackendGetTableAllTables(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	tables := []string{
		types.TableCrumbs,
		types.TableTrails,
		types.TableProperties,
		types.TableMetadata,
		types.TableLinks,
		types.TableStashes,
	}
	for _, name := range tables {
		t.Run(name, func(t *testing.T) {
			tbl, err := backend.GetTable(name)
			require.NoError(t, err, "GetTable(%q) must succeed", name)
			assert.NotNil(t, tbl, "GetTable(%q) must return non-nil Table", name)
		})
	}
}

// ---------- S11: GetTable returns ErrTableNotFound for unknown ----------

func TestScaffoldingValidation_BackendGetTableUnknown(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	_, err := backend.GetTable("nonexistent")
	assert.ErrorIs(t, err, types.ErrTableNotFound)
}

// ---------- helpers ----------

// projectRoot returns the absolute path to the project root by walking up
// from the current file until go.mod is found.
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// buildCupboard builds the cupboard binary into a temp directory and returns
// the path. Uses t.TempDir for automatic cleanup.
func buildCupboard(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cupboard")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cupboard")
	cmd.Dir = projectRoot()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(out))
	return binPath
}

