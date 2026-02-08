// CLI integration tests for scaffolding validation.
// Validates test-rel01.0-uc004-scaffolding-validation.yaml test cases.
// Implements: docs/specs/test-suites/test-rel01.0-uc004-scaffolding-validation.yaml;
//
//	docs/specs/use-cases/rel01.0-uc004-scaffolding-validation.yaml.
package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// --- Module path validation ---

// TestUC004_ModulePath validates that go.mod declares the correct module path
// and replace directive.
func TestUC004_ModulePath(t *testing.T) {
	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("failed to find project root: %v", err)
	}

	goModPath := projectRoot + "/go.mod"
	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	content := string(data)

	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "module path uses mesh-intelligence",
			contains: "module github.com/mesh-intelligence/crumbs",
		},
		{
			name:     "replace directive points to local directory",
			contains: "replace github.com/mesh-intelligence/crumbs => ./",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(content, tt.contains) {
				t.Errorf("go.mod does not contain %q", tt.contains)
			}
		})
	}
}

// --- Build validation ---

// TestUC004_CupboardCLICompiles validates that go build ./cmd/cupboard succeeds.
// This is implicitly validated by TestMain building the binary, but we make it
// explicit here.
func TestUC004_CupboardCLICompiles(t *testing.T) {
	// The binary is built in TestMain. If we got here, it compiled.
	if buildErr != nil {
		t.Fatalf("cupboard CLI failed to compile: %v", buildErr)
	}
	if cupboardBin == "" {
		t.Fatal("cupboard binary not built")
	}
}

// --- Version command tests ---

// TestUC004_VersionCommand validates the version command behavior.
func TestUC004_VersionCommand(t *testing.T) {
	tests := []struct {
		name           string
		wantExitCode   int
		stdoutContains []string
	}{
		{
			name:         "version command prints version and exits 0",
			wantExitCode: 0,
			stdoutContains: []string{
				"cupboard",
			},
		},
		{
			name:         "version command lists implemented use cases",
			wantExitCode: 0,
			stdoutContains: []string{
				"rel01.0-uc004",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			result := env.RunCupboard("version")

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("exit code = %d, want %d; stderr: %s", result.ExitCode, tt.wantExitCode, result.Stderr)
			}

			for _, want := range tt.stdoutContains {
				if !strings.Contains(result.Stdout, want) {
					t.Errorf("stdout does not contain %q; got: %s", want, result.Stdout)
				}
			}
		})
	}
}

// TestUC004_VersionWorksWithoutBackend validates that version command works
// without backend connection.
func TestUC004_VersionWorksWithoutBackend(t *testing.T) {
	env := NewTestEnv(t)
	// Do not run init - no backend connection
	result := env.RunCupboard("version")

	if result.ExitCode != 0 {
		t.Errorf("version without backend failed with exit code %d; stderr: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "cupboard") {
		t.Errorf("stdout does not contain 'cupboard'; got: %s", result.Stdout)
	}
}

// --- Entity struct compilation tests (compile-time assertions) ---

// TestUC004_CrumbStructHasRequiredFields validates that the Crumb struct has
// all documented fields.
func TestUC004_CrumbStructHasRequiredFields(t *testing.T) {
	// Compile-time test: instantiate Crumb with all documented fields.
	// Using value type (not pointer) to avoid unusedwrite warnings.
	_ = types.Crumb{
		CrumbID:    "test",
		Name:       "test",
		State:      "draft",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Properties: map[string]any{},
	}

	// If this compiles, the struct has the required fields
	t.Log("Crumb struct has required fields: CrumbID, Name, State, CreatedAt, UpdatedAt, Properties")
}

// TestUC004_TrailStructHasRequiredFields validates that the Trail struct has
// all documented fields.
func TestUC004_TrailStructHasRequiredFields(t *testing.T) {
	_ = types.Trail{
		TrailID:   "test",
		State:     "active",
		CreatedAt: time.Now(),
	}

	t.Log("Trail struct has required fields: TrailID, State, CreatedAt")
}

// TestUC004_PropertyStructHasRequiredFields validates that the Property struct
// has all documented fields.
func TestUC004_PropertyStructHasRequiredFields(t *testing.T) {
	_ = types.Property{
		PropertyID:  "test",
		Name:        "priority",
		Description: "Task priority",
		ValueType:   "categorical",
		CreatedAt:   time.Now(),
	}

	t.Log("Property struct has required fields: PropertyID, Name, Description, ValueType, CreatedAt")
}

// TestUC004_CategoryStructHasRequiredFields validates that the Category struct
// has all documented fields.
func TestUC004_CategoryStructHasRequiredFields(t *testing.T) {
	_ = types.Category{
		CategoryID: "test",
		PropertyID: "test",
		Name:       "high",
		Ordinal:    1,
	}

	t.Log("Category struct has required fields: CategoryID, PropertyID, Name, Ordinal")
}

// TestUC004_StashStructHasRequiredFields validates that the Stash struct has
// all documented fields.
func TestUC004_StashStructHasRequiredFields(t *testing.T) {
	_ = types.Stash{
		StashID:   "test",
		Name:      "shared-config",
		StashType: "context",
		Value:     nil,
		Version:   0,
		CreatedAt: time.Now(),
	}

	t.Log("Stash struct has required fields: StashID, Name, StashType, Value, Version, CreatedAt")
}

// TestUC004_MetadataStructHasRequiredFields validates that the Metadata struct
// has all documented fields.
func TestUC004_MetadataStructHasRequiredFields(t *testing.T) {
	_ = types.Metadata{
		MetadataID: "test",
		CrumbID:    "test",
		TableName:  "comments",
		Content:    "{}",
		CreatedAt:  time.Now(),
	}

	t.Log("Metadata struct has required fields: MetadataID, CrumbID, TableName, Content, CreatedAt")
}

// TestUC004_LinkStructHasRequiredFields validates that the Link struct has
// all documented fields.
func TestUC004_LinkStructHasRequiredFields(t *testing.T) {
	_ = types.Link{
		LinkID:    "test",
		LinkType:  "belongs_to",
		FromID:    "crumb-1",
		ToID:      "trail-1",
		CreatedAt: time.Now(),
	}

	t.Log("Link struct has required fields: LinkID, LinkType, FromID, ToID, CreatedAt")
}

// --- Interface satisfaction tests (compile-time assertions) ---

// TestUC004_SQLiteBackendSatisfiesCupboardInterface validates that the SQLite
// backend implements the Cupboard interface.
func TestUC004_SQLiteBackendSatisfiesCupboardInterface(t *testing.T) {
	// Compile-time assertion: sqlite.Backend must implement types.Cupboard
	var _ types.Cupboard = (*sqlite.Backend)(nil)

	t.Log("sqlite.Backend implements types.Cupboard interface")
}

// TestUC004_TableInterfaceDefinesRequiredMethods validates that the Table
// interface has Get, Set, Delete, Fetch methods.
func TestUC004_TableInterfaceDefinesRequiredMethods(t *testing.T) {
	// Compile-time assertion: Table interface must have Get, Set, Delete, Fetch
	// We verify by ensuring the type is usable with these methods
	var tbl types.Table
	_ = func() {
		_, _ = tbl.Get("id")
		_, _ = tbl.Set("id", nil)
		_ = tbl.Delete("id")
		_, _ = tbl.Fetch(map[string]any{})
	}

	t.Log("Table interface defines Get, Set, Delete, Fetch methods")
}

// --- Standard table name constants ---

// TestUC004_StandardTableNameConstantsDefined validates that all six standard
// table name constants are defined.
func TestUC004_StandardTableNameConstantsDefined(t *testing.T) {
	// Compile-time test: reference all table name constants
	names := []string{
		types.CrumbsTable,
		types.TrailsTable,
		types.PropertiesTable,
		types.MetadataTable,
		types.LinksTable,
		types.StashesTable,
	}

	expected := map[string]string{
		"CrumbsTable":     "crumbs",
		"TrailsTable":     "trails",
		"PropertiesTable": "properties",
		"MetadataTable":   "metadata",
		"LinksTable":      "links",
		"StashesTable":    "stashes",
	}

	if len(names) != 6 {
		t.Fatalf("expected 6 table name constants, got %d", len(names))
	}

	// Verify values match expected
	if types.CrumbsTable != expected["CrumbsTable"] {
		t.Errorf("CrumbsTable = %q, want %q", types.CrumbsTable, expected["CrumbsTable"])
	}
	if types.TrailsTable != expected["TrailsTable"] {
		t.Errorf("TrailsTable = %q, want %q", types.TrailsTable, expected["TrailsTable"])
	}
	if types.PropertiesTable != expected["PropertiesTable"] {
		t.Errorf("PropertiesTable = %q, want %q", types.PropertiesTable, expected["PropertiesTable"])
	}
	if types.MetadataTable != expected["MetadataTable"] {
		t.Errorf("MetadataTable = %q, want %q", types.MetadataTable, expected["MetadataTable"])
	}
	if types.LinksTable != expected["LinksTable"] {
		t.Errorf("LinksTable = %q, want %q", types.LinksTable, expected["LinksTable"])
	}
	if types.StashesTable != expected["StashesTable"] {
		t.Errorf("StashesTable = %q, want %q", types.StashesTable, expected["StashesTable"])
	}

	t.Log("All six standard table name constants are defined: crumbs, trails, properties, metadata, links, stashes")
}

// --- GetTable tests for all standard tables ---

// TestUC004_GetTableSucceedsForStandardTables validates that GetTable succeeds
// for all six standard table names.
func TestUC004_GetTableSucceedsForStandardTables(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
	}{
		{"GetTable succeeds for crumbs table", types.CrumbsTable},
		{"GetTable succeeds for trails table", types.TrailsTable},
		{"GetTable succeeds for properties table", types.PropertiesTable},
		{"GetTable succeeds for metadata table", types.MetadataTable},
		{"GetTable succeeds for links table", types.LinksTable},
		{"GetTable succeeds for stashes table", types.StashesTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Probe the table by requesting a nonexistent ID
			// Exit code 1 with "not found" confirms the table exists and is accessible
			result := env.RunCupboard("get", tt.tableName, "nonexistent-probe-id")

			// We expect either:
			// - Exit code 1 with "not found" (table exists, entity not found)
			// - Exit code 0 with empty result (table exists, returns empty)
			// We should NOT get an error about table not found
			if result.ExitCode != 0 && result.ExitCode != 1 {
				t.Errorf("unexpected exit code %d for table %s; stderr: %s",
					result.ExitCode, tt.tableName, result.Stderr)
			}

			// Verify we don't get a "table not found" type error
			stderrLower := strings.ToLower(result.Stderr)
			if strings.Contains(stderrLower, "table not found") ||
				strings.Contains(stderrLower, "unknown table") {
				t.Errorf("table %s should be accessible; got: %s", tt.tableName, result.Stderr)
			}
		})
	}
}

// TestUC004_GetTableFailsForUnknownTable validates that GetTable for an unknown
// table name returns an error.
func TestUC004_GetTableFailsForUnknownTable(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	result := env.RunCupboard("get", "nonexistent", "nonexistent-id")

	if result.ExitCode == 0 {
		t.Error("get for unknown table should fail")
	}

	// The CLI should indicate this table is not valid
	// (could be "unknown table", "table not found", "invalid table", etc.)
	t.Logf("Unknown table error: exit=%d, stderr=%s", result.ExitCode, result.Stderr)
}

// --- Direct backend GetTable tests ---

// TestUC004_BackendGetTableAllTables validates that the backend GetTable works
// for all six standard tables via direct API calls.
func TestUC004_BackendGetTableAllTables(t *testing.T) {
	tmpDir := t.TempDir()

	backend := sqlite.NewBackend()
	config := types.Config{
		Backend: "sqlite",
		DataDir: tmpDir,
	}

	if err := backend.Attach(config); err != nil {
		t.Fatalf("failed to attach: %v", err)
	}
	defer backend.Detach()

	tables := []string{
		types.CrumbsTable,
		types.TrailsTable,
		types.PropertiesTable,
		types.MetadataTable,
		types.LinksTable,
		types.StashesTable,
	}

	for _, tableName := range tables {
		t.Run("GetTable "+tableName, func(t *testing.T) {
			tbl, err := backend.GetTable(tableName)
			if err != nil {
				t.Errorf("GetTable(%q) failed: %v", tableName, err)
			}
			if tbl == nil {
				t.Errorf("GetTable(%q) returned nil table", tableName)
			}
		})
	}
}

// TestUC004_BackendGetTableUnknownReturnsError validates that GetTable for an
// unknown table name returns ErrTableNotFound.
func TestUC004_BackendGetTableUnknownReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	backend := sqlite.NewBackend()
	config := types.Config{
		Backend: "sqlite",
		DataDir: tmpDir,
	}

	if err := backend.Attach(config); err != nil {
		t.Fatalf("failed to attach: %v", err)
	}
	defer backend.Detach()

	_, err := backend.GetTable("nonexistent")
	if err == nil {
		t.Error("GetTable for unknown table should return error")
	}
	if err != types.ErrTableNotFound {
		t.Errorf("GetTable error = %v, want ErrTableNotFound", err)
	}
}

// --- Cupboard interface compilation test ---

// TestUC004_CupboardInterfaceDefinesRequiredMethods validates that the Cupboard
// interface has GetTable, Attach, Detach methods.
func TestUC004_CupboardInterfaceDefinesRequiredMethods(t *testing.T) {
	// Compile-time assertion: Cupboard interface must have GetTable, Attach, Detach
	var cupboard types.Cupboard
	_ = func() {
		_, _ = cupboard.GetTable("name")
		_ = cupboard.Attach(types.Config{})
		_ = cupboard.Detach()
	}

	t.Log("Cupboard interface defines GetTable, Attach, Detach methods")
}
