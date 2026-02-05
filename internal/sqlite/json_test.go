// Tests for JSON persistence in SQLite backend.
// Implements: prd-sqlite-backend acceptance criteria (JSON sync unit tests).
package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dukaforge/crumbs/pkg/types"
)

func TestJSONFilesCreatedOnAttach(t *testing.T) {
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

	// Verify all JSON files are created
	files := []string{
		crumbsFile, trailsFile, linksFile, propertiesFile, categoriesFile,
		crumbPropsFile, metadataFile, stashesFile, stashHistoryFile,
	}

	for _, name := range files {
		path := filepath.Join(tmpDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to be created, but it doesn't exist", name)
		}
	}
}

func TestJSONFilesInitializedEmpty(t *testing.T) {
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

	// Verify crumbs.json contains empty array
	path := filepath.Join(tmpDir, crumbsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", crumbsFile, err)
	}

	var crumbs []crumbJSON
	if err := json.Unmarshal(data, &crumbs); err != nil {
		t.Fatalf("failed to parse %s: %v", crumbsFile, err)
	}

	if len(crumbs) != 0 {
		t.Errorf("expected empty array, got %d items", len(crumbs))
	}
}

func TestCrumbPersistedToJSON(t *testing.T) {
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

	// Read JSON file directly
	path := filepath.Join(tmpDir, crumbsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", crumbsFile, err)
	}

	var crumbs []crumbJSON
	if err := json.Unmarshal(data, &crumbs); err != nil {
		t.Fatalf("failed to parse %s: %v", crumbsFile, err)
	}

	if len(crumbs) != 1 {
		t.Fatalf("expected 1 crumb in JSON, got %d", len(crumbs))
	}

	if crumbs[0].CrumbID != id {
		t.Errorf("expected CrumbID=%q, got %q", id, crumbs[0].CrumbID)
	}
	if crumbs[0].Name != "Test Crumb" {
		t.Errorf("expected Name='Test Crumb', got %q", crumbs[0].Name)
	}
	if crumbs[0].State != types.StatePending {
		t.Errorf("expected State='pending', got %q", crumbs[0].State)
	}
}

func TestCrumbUpdatePersistedToJSON(t *testing.T) {
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

	// Read JSON file directly
	path := filepath.Join(tmpDir, crumbsFile)
	data, _ := os.ReadFile(path)

	var crumbs []crumbJSON
	json.Unmarshal(data, &crumbs)

	if len(crumbs) != 1 {
		t.Fatalf("expected 1 crumb in JSON after update, got %d", len(crumbs))
	}

	if crumbs[0].Name != "Updated Name" {
		t.Errorf("expected Name='Updated Name', got %q", crumbs[0].Name)
	}
}

func TestCrumbDeletePersistedToJSON(t *testing.T) {
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

	// Read JSON file directly
	path := filepath.Join(tmpDir, crumbsFile)
	data, _ := os.ReadFile(path)

	var crumbs []crumbJSON
	json.Unmarshal(data, &crumbs)

	if len(crumbs) != 0 {
		t.Errorf("expected 0 crumbs in JSON after delete, got %d", len(crumbs))
	}
}

func TestJSONLoadedOnAttach(t *testing.T) {
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

	// Second session: verify data loaded from JSON
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

func TestTrailPersistedToJSON(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TrailsTable)

	// Create trail
	parentID := "parent-123"
	trail := &types.Trail{
		ParentCrumbID: &parentID,
		State:         types.TrailStateActive,
	}
	id, err := tbl.Set("", trail)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read JSON file directly
	path := filepath.Join(tmpDir, trailsFile)
	data, _ := os.ReadFile(path)

	var trails []trailJSON
	json.Unmarshal(data, &trails)

	if len(trails) != 1 {
		t.Fatalf("expected 1 trail in JSON, got %d", len(trails))
	}

	if trails[0].TrailID != id {
		t.Errorf("expected TrailID=%q, got %q", id, trails[0].TrailID)
	}
	if trails[0].ParentCrumbID == nil || *trails[0].ParentCrumbID != "parent-123" {
		t.Errorf("expected ParentCrumbID='parent-123', got %v", trails[0].ParentCrumbID)
	}
}

func TestLinkPersistedToJSON(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.LinksTable)

	// Create link
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   "crumb-123",
		ToID:     "trail-456",
	}
	id, err := tbl.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read JSON file directly
	path := filepath.Join(tmpDir, linksFile)
	data, _ := os.ReadFile(path)

	var links []linkJSON
	json.Unmarshal(data, &links)

	if len(links) != 1 {
		t.Fatalf("expected 1 link in JSON, got %d", len(links))
	}

	if links[0].LinkID != id {
		t.Errorf("expected LinkID=%q, got %q", id, links[0].LinkID)
	}
	if links[0].LinkType != types.LinkTypeBelongsTo {
		t.Errorf("expected LinkType='belongs_to', got %q", links[0].LinkType)
	}
}

func TestPropertyPersistedToJSON(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.PropertiesTable)

	// Create property
	prop := &types.Property{
		Name:        "priority",
		Description: "Task priority",
		ValueType:   types.ValueTypeCategorical,
	}
	id, err := tbl.Set("", prop)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read JSON file directly
	path := filepath.Join(tmpDir, propertiesFile)
	data, _ := os.ReadFile(path)

	var props []propertyJSON
	json.Unmarshal(data, &props)

	if len(props) != 1 {
		t.Fatalf("expected 1 property in JSON, got %d", len(props))
	}

	if props[0].PropertyID != id {
		t.Errorf("expected PropertyID=%q, got %q", id, props[0].PropertyID)
	}
	if props[0].Name != "priority" {
		t.Errorf("expected Name='priority', got %q", props[0].Name)
	}
}

func TestMetadataPersistedToJSON(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.MetadataTable)

	// Create metadata
	propID := "prop-123"
	meta := &types.Metadata{
		TableName:  "comments",
		CrumbID:    "crumb-123",
		PropertyID: &propID,
		Content:    "Test comment",
	}
	id, err := tbl.Set("", meta)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read JSON file directly
	path := filepath.Join(tmpDir, metadataFile)
	data, _ := os.ReadFile(path)

	var metas []metadataJSON
	json.Unmarshal(data, &metas)

	if len(metas) != 1 {
		t.Fatalf("expected 1 metadata in JSON, got %d", len(metas))
	}

	if metas[0].MetadataID != id {
		t.Errorf("expected MetadataID=%q, got %q", id, metas[0].MetadataID)
	}
	if metas[0].Content != "Test comment" {
		t.Errorf("expected Content='Test comment', got %q", metas[0].Content)
	}
}

func TestStashPersistedToJSON(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.StashesTable)

	// Create stash
	trailID := "trail-123"
	stash := &types.Stash{
		TrailID:   &trailID,
		Name:      "counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(42)},
		Version:   1,
	}
	id, err := tbl.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read JSON file directly
	path := filepath.Join(tmpDir, stashesFile)
	data, _ := os.ReadFile(path)

	var stashes []stashJSON
	json.Unmarshal(data, &stashes)

	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash in JSON, got %d", len(stashes))
	}

	if stashes[0].StashID != id {
		t.Errorf("expected StashID=%q, got %q", id, stashes[0].StashID)
	}
	if stashes[0].Name != "counter" {
		t.Errorf("expected Name='counter', got %q", stashes[0].Name)
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Test that atomic write creates valid JSON
	path := filepath.Join(tmpDir, "test.json")
	data := []map[string]string{
		{"key": "value1"},
		{"key": "value2"},
	}

	err := writeJSONAtomic(path, data)
	if err != nil {
		t.Fatalf("writeJSONAtomic failed: %v", err)
	}

	// Read and verify
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result []map[string]string
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
}

func TestMultipleEntitiesPersistence(t *testing.T) {
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

	// Verify JSON has correct data
	path := filepath.Join(tmpDir, crumbsFile)
	data, _ := os.ReadFile(path)

	var jsonCrumbs []crumbJSON
	json.Unmarshal(data, &jsonCrumbs)

	if len(jsonCrumbs) != 2 {
		t.Fatalf("expected 2 crumbs in JSON after delete, got %d", len(jsonCrumbs))
	}

	// Verify correct crumbs remain
	names := make(map[string]bool)
	for _, c := range jsonCrumbs {
		names[c.Name] = true
	}

	if !names["Crumb A"] {
		t.Error("expected 'Crumb A' to remain")
	}
	if names["Crumb B"] {
		t.Error("expected 'Crumb B' to be deleted")
	}
	if !names["Crumb C"] {
		t.Error("expected 'Crumb C' to remain")
	}
}

func TestJSONPersistenceAcrossRestarts(t *testing.T) {
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

	// Session 2: Verify all data is loaded
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
