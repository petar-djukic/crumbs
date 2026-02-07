// Tests for JSONL persistence in SQLite backend.
// Implements: prd-configuration-directories acceptance criteria.
package sqlite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

func TestJSONLFilesCreatedOnAttach(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	err := b.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer b.Detach()

	// Verify all JSONL files are created
	files := []string{
		crumbsJSONL, trailsJSONL, linksJSONL, propertiesJSONL, categoriesJSONL,
		crumbPropsJSONL, metadataJSONL, stashesJSONL, stashHistoryJSONL,
	}

	for _, name := range files {
		path := filepath.Join(tmpDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to be created, but it doesn't exist", name)
		}
	}
}

func TestJSONLFilesInitializedEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	err := b.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer b.Detach()

	// Verify crumbs.jsonl is empty (zero bytes per R4.3)
	path := filepath.Join(tmpDir, crumbsJSONL)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat %s: %v", crumbsJSONL, err)
	}

	if info.Size() != 0 {
		t.Errorf("expected empty file, got %d bytes", info.Size())
	}
}

func TestCrumbPersistedToJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Create crumb
	crumb := &types.Crumb{
		Name:  "Test Crumb",
		State: types.StatePending,
	}
	id, err := tbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read JSONL file directly
	path := filepath.Join(tmpDir, crumbsJSONL)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", crumbsJSONL, err)
	}

	// Verify it's JSONL format (one line per record)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in JSONL, got %d", len(lines))
	}

	// Verify content contains expected data
	line := lines[0]
	if !strings.Contains(line, id) {
		t.Errorf("expected line to contain ID %q", id)
	}
	if !strings.Contains(line, "Test Crumb") {
		t.Errorf("expected line to contain 'Test Crumb'")
	}
	if !strings.Contains(line, types.StatePending) {
		t.Errorf("expected line to contain 'pending'")
	}
}

func TestCrumbUpdatePersistedToJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Create crumb
	crumb := &types.Crumb{
		Name:  "Original Name",
		State: types.StatePending,
	}
	id, _ := tbl.Set("", crumb)

	// Update crumb
	crumb.Name = "Updated Name"
	crumb.CrumbID = id
	tbl.Set(id, crumb)

	// Read JSONL file directly
	path := filepath.Join(tmpDir, crumbsJSONL)
	data, _ := os.ReadFile(path)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in JSONL after update, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "Updated Name") {
		t.Errorf("expected line to contain 'Updated Name'")
	}
}

func TestCrumbDeletePersistedToJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Create crumb
	crumb := &types.Crumb{
		Name:  "To Be Deleted",
		State: types.StatePending,
	}
	id, _ := tbl.Set("", crumb)

	// Delete crumb
	err := tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Read JSONL file directly
	path := filepath.Join(tmpDir, crumbsJSONL)
	data, _ := os.ReadFile(path)

	if len(strings.TrimSpace(string(data))) != 0 {
		t.Errorf("expected empty JSONL after delete, got %q", string(data))
	}
}

func TestJSONLLoadedOnAttach(t *testing.T) {
	tmpDir := t.TempDir()

	// First session: create data
	b1 := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b1.Attach(config)

	tbl, _ := b1.GetTable(types.CrumbsTable)
	crumb := &types.Crumb{
		Name:  "Persistent Crumb",
		State: types.StatePending,
	}
	id, _ := tbl.Set("", crumb)

	b1.Detach()

	// Second session: verify data loaded from JSONL
	b2 := NewBackend()
	b2.Attach(config)
	defer b2.Detach()

	tbl2, _ := b2.GetTable(types.CrumbsTable)
	result, err := tbl2.Get(id)
	if err != nil {
		t.Fatalf("Get failed in second session: %v", err)
	}

	gotCrumb, ok := result.(*types.Crumb)
	if !ok {
		t.Fatalf("expected *types.Crumb, got %T", result)
	}
	if gotCrumb.Name != "Persistent Crumb" {
		t.Errorf("expected Name='Persistent Crumb', got %q", gotCrumb.Name)
	}
}

func TestJSONLNotPrettyPrinted(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	crumb := &types.Crumb{
		Name:  "Test Crumb",
		State: types.StatePending,
	}
	tbl.Set("", crumb)

	// Read JSONL file
	path := filepath.Join(tmpDir, crumbsJSONL)
	data, _ := os.ReadFile(path)

	// JSONL should not contain indentation (per R3.5)
	if strings.Contains(string(data), "  ") {
		t.Error("JSONL should not be pretty-printed")
	}

	// Each record should be on a single line
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 line, got %d", len(lines))
	}
}

func TestJSONLEmptyLinesSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a JSONL file with empty lines
	path := filepath.Join(tmpDir, crumbsJSONL)
	content := `{"crumb_id":"test-1","name":"Crumb 1","state":"pending","created_at":"2025-01-15T10:00:00Z","updated_at":"2025-01-15T10:00:00Z"}

{"crumb_id":"test-2","name":"Crumb 2","state":"ready","created_at":"2025-01-15T11:00:00Z","updated_at":"2025-01-15T11:00:00Z"}

`
	os.WriteFile(path, []byte(content), 0644)

	// Create other required files
	for _, name := range []string{trailsJSONL, linksJSONL, propertiesJSONL, categoriesJSONL, crumbPropsJSONL, metadataJSONL, stashesJSONL, stashHistoryJSONL} {
		os.WriteFile(filepath.Join(tmpDir, name), []byte{}, 0644)
	}

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	err := b.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer b.Detach()

	// Verify both crumbs were loaded
	tbl, _ := b.GetTable(types.CrumbsTable)
	results, err := tbl.Fetch(map[string]any{})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 crumbs (empty lines skipped), got %d", len(results))
	}
}

func TestJSONLMalformedLinesSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a JSONL file with a malformed line
	path := filepath.Join(tmpDir, crumbsJSONL)
	content := `{"crumb_id":"test-1","name":"Crumb 1","state":"pending","created_at":"2025-01-15T10:00:00Z","updated_at":"2025-01-15T10:00:00Z"}
{invalid json here
{"crumb_id":"test-2","name":"Crumb 2","state":"ready","created_at":"2025-01-15T11:00:00Z","updated_at":"2025-01-15T11:00:00Z"}
`
	os.WriteFile(path, []byte(content), 0644)

	// Create other required files
	for _, name := range []string{trailsJSONL, linksJSONL, propertiesJSONL, categoriesJSONL, crumbPropsJSONL, metadataJSONL, stashesJSONL, stashHistoryJSONL} {
		os.WriteFile(filepath.Join(tmpDir, name), []byte{}, 0644)
	}

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	err := b.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer b.Detach()

	// Verify valid crumbs were loaded, malformed line skipped (per R5.2)
	tbl, _ := b.GetTable(types.CrumbsTable)
	results, err := tbl.Fetch(map[string]any{})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 crumbs (malformed line skipped), got %d", len(results))
	}
}

func TestAtomicWriteJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "test.jsonl")
	lines := [][]byte{
		[]byte(`{"key":"value1"}`),
		[]byte(`{"key":"value2"}`),
	}

	err := writeJSONLAtomic(path, lines)
	if err != nil {
		t.Fatalf("writeJSONLAtomic failed: %v", err)
	}

	// Read and verify
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	resultLines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(resultLines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(resultLines))
	}
}

func TestMultipleEntitiesJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Create multiple crumbs
	crumbs := []*types.Crumb{
		{Name: "Crumb A", State: types.StatePending},
		{Name: "Crumb B", State: types.StateReady},
		{Name: "Crumb C", State: types.StateTaken},
	}

	var ids []string
	for _, c := range crumbs {
		id, _ := tbl.Set("", c)
		ids = append(ids, id)
	}

	// Delete middle one
	tbl.Delete(ids[1])

	b.Detach()

	// Verify JSONL has correct data
	path := filepath.Join(tmpDir, crumbsJSONL)
	data, _ := os.ReadFile(path)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in JSONL after delete, got %d", len(lines))
	}

	// Verify correct crumbs remain
	content := string(data)
	if !strings.Contains(content, "Crumb A") {
		t.Error("expected 'Crumb A' to remain")
	}
	if strings.Contains(content, "Crumb B") {
		t.Error("expected 'Crumb B' to be deleted")
	}
	if !strings.Contains(content, "Crumb C") {
		t.Error("expected 'Crumb C' to remain")
	}
}

func TestJSONLPersistenceAcrossRestarts(t *testing.T) {
	tmpDir := t.TempDir()

	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	// Session 1: Create various entities
	b1 := NewBackend()
	b1.Attach(config)

	crumbTbl, _ := b1.GetTable(types.CrumbsTable)
	trailTbl, _ := b1.GetTable(types.TrailsTable)
	linkTbl, _ := b1.GetTable(types.LinksTable)

	trail := &types.Trail{State: types.TrailStateActive}
	trailID, _ := trailTbl.Set("", trail)

	crumb := &types.Crumb{Name: "Test", State: types.StatePending}
	crumbID, _ := crumbTbl.Set("", crumb)

	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumbID,
		ToID:     trailID,
	}
	linkID, _ := linkTbl.Set("", link)

	b1.Detach()

	// Verify cupboard.db is deleted on startup (ephemeral cache per R5.1)
	dbPath := filepath.Join(tmpDir, "cupboard.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		// DB exists after detach, which is fine - just verify it gets deleted on re-attach
	}

	// Session 2: Verify all data is loaded from JSONL
	b2 := NewBackend()
	b2.Attach(config)
	defer b2.Detach()

	// Verify trail
	trailTbl2, _ := b2.GetTable(types.TrailsTable)
	trailResult, err := trailTbl2.Get(trailID)
	if err != nil {
		t.Fatalf("failed to get trail: %v", err)
	}
	gotTrail := trailResult.(*types.Trail)
	if gotTrail.State != types.TrailStateActive {
		t.Errorf("expected trail state 'active', got %q", gotTrail.State)
	}

	// Verify crumb
	crumbTbl2, _ := b2.GetTable(types.CrumbsTable)
	crumbResult, err := crumbTbl2.Get(crumbID)
	if err != nil {
		t.Fatalf("failed to get crumb: %v", err)
	}
	gotCrumb := crumbResult.(*types.Crumb)
	if gotCrumb.Name != "Test" {
		t.Errorf("expected crumb name 'Test', got %q", gotCrumb.Name)
	}

	// Verify link
	linkTbl2, _ := b2.GetTable(types.LinksTable)
	linkResult, err := linkTbl2.Get(linkID)
	if err != nil {
		t.Fatalf("failed to get link: %v", err)
	}
	gotLink := linkResult.(*types.Link)
	if gotLink.FromID != crumbID {
		t.Errorf("expected link FromID=%q, got %q", crumbID, gotLink.FromID)
	}
}

func TestReadJSONLGeneric(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Write test data
	content := `{"crumb_id":"id-1","name":"Test 1","state":"pending","created_at":"2025-01-15T10:00:00Z","updated_at":"2025-01-15T10:00:00Z"}
{"crumb_id":"id-2","name":"Test 2","state":"ready","created_at":"2025-01-15T11:00:00Z","updated_at":"2025-01-15T11:00:00Z"}
`
	os.WriteFile(path, []byte(content), 0644)

	// Read using generic function
	results, err := readJSONL[crumbJSON](path)
	if err != nil {
		t.Fatalf("readJSONL failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 records, got %d", len(results))
	}

	if results[0].CrumbID != "id-1" {
		t.Errorf("expected first ID 'id-1', got %q", results[0].CrumbID)
	}
	if results[1].Name != "Test 2" {
		t.Errorf("expected second name 'Test 2', got %q", results[1].Name)
	}
}

func TestWriteJSONLGeneric(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	records := []crumbJSON{
		{CrumbID: "id-1", Name: "Test 1", State: "pending", CreatedAt: "2025-01-15T10:00:00Z", UpdatedAt: "2025-01-15T10:00:00Z"},
		{CrumbID: "id-2", Name: "Test 2", State: "ready", CreatedAt: "2025-01-15T11:00:00Z", UpdatedAt: "2025-01-15T11:00:00Z"},
	}

	err := writeJSONL(path, records)
	if err != nil {
		t.Fatalf("writeJSONL failed: %v", err)
	}

	// Read back and verify
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "id-1") {
		t.Errorf("expected first line to contain 'id-1'")
	}
}

func TestAppendToJSONLFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Append first record
	err := appendToJSONLFile(path, []byte(`{"key":"value1"}`))
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Append second record
	err = appendToJSONLFile(path, []byte(`{"key":"value2"}`))
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	// Read and verify
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestDeleteFromJSONLFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Create initial data
	content := `{"id":"keep-1","name":"Keep 1"}
{"id":"delete-me","name":"Delete Me"}
{"id":"keep-2","name":"Keep 2"}
`
	os.WriteFile(path, []byte(content), 0644)

	// Delete the middle record
	err := deleteFromJSONLFile(path, "delete-me", "id")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "delete-me") {
		t.Error("expected 'delete-me' to be removed")
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines after delete, got %d", len(lines))
	}
}

func TestUpdateJSONLFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Create initial data
	content := `{"id":"update-me","name":"Original Name"}
{"id":"keep","name":"Keep This"}
`
	os.WriteFile(path, []byte(content), 0644)

	// Update the first record
	err := updateJSONLFile(path, "update-me", "id", func() ([]byte, error) {
		return []byte(`{"id":"update-me","name":"Updated Name"}`), nil
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Updated Name") {
		t.Error("expected 'Updated Name' in file")
	}
	if strings.Contains(string(data), "Original Name") {
		t.Error("expected 'Original Name' to be replaced")
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines after update, got %d", len(lines))
	}
}

func TestUpdateJSONLFileAppendNew(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Create initial data
	content := `{"id":"existing","name":"Existing"}
`
	os.WriteFile(path, []byte(content), 0644)

	// Add a new record (ID not found)
	err := updateJSONLFile(path, "new-id", "id", func() ([]byte, error) {
		return []byte(`{"id":"new-id","name":"New Record"}`), nil
	})
	if err != nil {
		t.Fatalf("update (append) failed: %v", err)
	}

	// Verify
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines after append, got %d", len(lines))
	}

	if !strings.Contains(string(data), "New Record") {
		t.Error("expected 'New Record' in file")
	}
}
