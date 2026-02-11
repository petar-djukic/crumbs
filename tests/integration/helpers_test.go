// Package integration provides shared test helpers for integration tests.
// Implements: test suites for rel01.0-uc001, rel01.0-uc002, rel01.0-uc003.
package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// setupCupboard creates a backend attached to an isolated temp directory.
// Returns the backend, data directory, and a cleanup function.
// Each test case gets its own cupboard instance for isolation.
func setupCupboard(t *testing.T) (*sqlite.SQLiteBackend, string) {
	t.Helper()
	dir := t.TempDir()
	b := sqlite.NewBackend()
	if err := b.Attach(types.Config{Backend: "sqlite", DataDir: dir}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	t.Cleanup(func() { b.Detach() })
	return b, dir
}

// mustGetTable retrieves a table by name or fails the test.
func mustGetTable(t *testing.T, b *sqlite.SQLiteBackend, name string) types.Table {
	t.Helper()
	tbl, err := b.GetTable(name)
	if err != nil {
		t.Fatalf("GetTable(%q): %v", name, err)
	}
	return tbl
}

// mustCreateCrumb creates a crumb and returns its ID.
func mustCreateCrumb(t *testing.T, tbl types.Table, name, state string) string {
	t.Helper()
	c := &types.Crumb{Name: name, State: state}
	id, err := tbl.Set("", c)
	if err != nil {
		t.Fatalf("Set crumb %q: %v", name, err)
	}
	return id
}

// mustGetCrumb retrieves a crumb by ID and returns it.
func mustGetCrumb(t *testing.T, tbl types.Table, id string) *types.Crumb {
	t.Helper()
	raw, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get crumb %q: %v", id, err)
	}
	c, ok := raw.(*types.Crumb)
	if !ok {
		t.Fatalf("expected *types.Crumb, got %T", raw)
	}
	return c
}

// mustCreateTrail creates a trail and returns its ID.
func mustCreateTrail(t *testing.T, tbl types.Table, state string) string {
	t.Helper()
	tr := &types.Trail{State: state}
	id, err := tbl.Set("", tr)
	if err != nil {
		t.Fatalf("Set trail: %v", err)
	}
	return id
}

// mustGetTrail retrieves a trail by ID and returns it.
func mustGetTrail(t *testing.T, tbl types.Table, id string) *types.Trail {
	t.Helper()
	raw, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get trail %q: %v", id, err)
	}
	tr, ok := raw.(*types.Trail)
	if !ok {
		t.Fatalf("expected *types.Trail, got %T", raw)
	}
	return tr
}

// mustCreateLink creates a link and returns its ID.
func mustCreateLink(t *testing.T, tbl types.Table, linkType, fromID, toID string) string {
	t.Helper()
	l := &types.Link{LinkType: linkType, FromID: fromID, ToID: toID}
	id, err := tbl.Set("", l)
	if err != nil {
		t.Fatalf("Set link: %v", err)
	}
	return id
}

// fetchAll calls Fetch with nil filter and returns the results.
func fetchAll(t *testing.T, tbl types.Table) []any {
	t.Helper()
	results, err := tbl.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return results
}

// fetchByStates calls Fetch with a states filter.
func fetchByStates(t *testing.T, tbl types.Table, states []string) []any {
	t.Helper()
	results, err := tbl.Fetch(map[string]any{"states": states})
	if err != nil {
		t.Fatalf("Fetch with states %v: %v", states, err)
	}
	return results
}

// readJSONLFile reads a JSONL file and returns its lines.
func readJSONLFile(t *testing.T, dir, filename string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("reading %s: %v", filename, err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// assertJSONLContains checks that a JSONL file contains a substring.
func assertJSONLContains(t *testing.T, dir, filename, substr string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("reading %s: %v", filename, err)
	}
	if !strings.Contains(string(data), substr) {
		t.Errorf("%s does not contain %q", filename, substr)
	}
}

// assertJSONLNotContains checks that a JSONL file does not contain a substring.
func assertJSONLNotContains(t *testing.T, dir, filename, substr string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("reading %s: %v", filename, err)
	}
	if strings.Contains(string(data), substr) {
		t.Errorf("%s should not contain %q", filename, substr)
	}
}

// isUUIDv7 checks if a string looks like a UUID (basic format check).
func isUUIDv7(s string) bool {
	if len(s) != 36 {
		return false
	}
	// UUID format: 8-4-4-4-12 with hyphens at positions 8, 13, 18, 23.
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	// Version 7: character at position 14 should be '7'.
	return s[14] == '7'
}
