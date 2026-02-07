// Integration tests for JSON persistence in the SQLite backend.
// Tests JSON file format, data loading on startup, and persistence verification.
// Implements: prd-sqlite-backend R2 (JSON file format);
//
//	prd-configuration-directories R3, R4, R5, R6;
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// TestJSON_FileCreationOnAttach verifies that JSONL files are created on first attach.
func TestJSON_FileCreationOnAttach(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	// All required JSONL files should be created
	requiredFiles := []string{
		crumbsJSONL,
		trailsJSONL,
		linksJSONL,
		propertiesJSONL,
		categoriesJSONL,
		crumbPropsJSONL,
		metadataJSONL,
		stashesJSONL,
		stashHistoryJSONL,
	}

	for _, filename := range requiredFiles {
		path := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Required JSONL file %s was not created", filename)
		}
	}

	// SQLite database should also be created
	dbPath := filepath.Join(tmpDir, "cupboard.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("cupboard.db was not created")
	}
}

// TestJSON_CrumbFormat verifies crumb JSON format matches prd-sqlite-backend R2.2.
func TestJSON_CrumbFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.CrumbsTable)

	// Create crumb
	crumb := &types.Crumb{
		Name:  "Format test crumb",
		State: types.StatePending,
	}
	id, _ := table.Set("", crumb)

	// Read and verify JSON format
	record := readJSONLRecord(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)

	// Required fields per R2.2
	requiredFields := []string{"crumb_id", "name", "state", "created_at", "updated_at"}
	for _, field := range requiredFields {
		if _, ok := record[field]; !ok {
			t.Errorf("Crumb JSON missing required field: %s", field)
		}
	}

	// Verify field values
	if record["crumb_id"] != id {
		t.Errorf("crumb_id mismatch: expected %q, got %v", id, record["crumb_id"])
	}
	if record["name"] != "Format test crumb" {
		t.Errorf("name mismatch: expected %q, got %v", "Format test crumb", record["name"])
	}
	if record["state"] != types.StatePending {
		t.Errorf("state mismatch: expected %q, got %v", types.StatePending, record["state"])
	}

	// Timestamps should be RFC3339 format
	_, err := time.Parse(time.RFC3339, record["created_at"].(string))
	if err != nil {
		t.Errorf("created_at is not valid RFC3339: %v", err)
	}
	_, err = time.Parse(time.RFC3339, record["updated_at"].(string))
	if err != nil {
		t.Errorf("updated_at is not valid RFC3339: %v", err)
	}
}

// TestJSON_TrailFormat verifies trail JSON format matches prd-sqlite-backend R2.3.
func TestJSON_TrailFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.TrailsTable)

	// Create trail without parent
	trail := &types.Trail{
		State: types.TrailStateActive,
	}
	id, _ := table.Set("", trail)

	// Read and verify JSON format
	record := readJSONLRecord(t, filepath.Join(tmpDir, trailsJSONL), "trail_id", id)

	// Required fields per R2.3 (ParentCrumbID removed - now uses branches_from links)
	if record["trail_id"] != id {
		t.Errorf("trail_id mismatch: expected %q, got %v", id, record["trail_id"])
	}
	if record["state"] != types.TrailStateActive {
		t.Errorf("state mismatch: expected %q, got %v", types.TrailStateActive, record["state"])
	}
	if record["completed_at"] != nil {
		t.Errorf("completed_at should be null for active trail, got %v", record["completed_at"])
	}
}

// TestJSON_PropertyFormat verifies property JSON format matches prd-sqlite-backend R2.4.
func TestJSON_PropertyFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.PropertiesTable)

	prop := &types.Property{
		Name:        "test_property",
		Description: "Test description",
		ValueType:   types.ValueTypeInteger,
	}
	id, _ := table.Set("", prop)

	record := readJSONLRecord(t, filepath.Join(tmpDir, propertiesJSONL), "property_id", id)

	// Verify fields per R2.4
	if record["property_id"] != id {
		t.Errorf("property_id mismatch: expected %q, got %v", id, record["property_id"])
	}
	if record["name"] != "test_property" {
		t.Errorf("name mismatch: expected %q, got %v", "test_property", record["name"])
	}
	if record["description"] != "Test description" {
		t.Errorf("description mismatch: expected %q, got %v", "Test description", record["description"])
	}
	if record["value_type"] != types.ValueTypeInteger {
		t.Errorf("value_type mismatch: expected %q, got %v", types.ValueTypeInteger, record["value_type"])
	}
}

// TestJSON_LinkFormat verifies link JSON format matches prd-sqlite-backend R2.7.
func TestJSON_LinkFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.LinksTable)

	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   "crumb-abc",
		ToID:     "trail-xyz",
	}
	id, _ := table.Set("", link)

	record := readJSONLRecord(t, filepath.Join(tmpDir, linksJSONL), "link_id", id)

	// Verify fields per R2.7
	if record["link_id"] != id {
		t.Errorf("link_id mismatch: expected %q, got %v", id, record["link_id"])
	}
	if record["link_type"] != types.LinkTypeBelongsTo {
		t.Errorf("link_type mismatch: expected %q, got %v", types.LinkTypeBelongsTo, record["link_type"])
	}
	if record["from_id"] != "crumb-abc" {
		t.Errorf("from_id mismatch: expected %q, got %v", "crumb-abc", record["from_id"])
	}
	if record["to_id"] != "trail-xyz" {
		t.Errorf("to_id mismatch: expected %q, got %v", "trail-xyz", record["to_id"])
	}
}

// TestJSON_MetadataFormat verifies metadata JSON format matches prd-sqlite-backend R2.9.
func TestJSON_MetadataFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.MetadataTable)

	// Metadata without property_id
	meta := &types.Metadata{
		TableName: "comments",
		CrumbID:   "crumb-123",
		Content:   "Test comment content",
	}
	id, _ := table.Set("", meta)

	record := readJSONLRecord(t, filepath.Join(tmpDir, metadataJSONL), "metadata_id", id)

	// Verify fields per R2.9
	if record["metadata_id"] != id {
		t.Errorf("metadata_id mismatch: expected %q, got %v", id, record["metadata_id"])
	}
	if record["table_name"] != "comments" {
		t.Errorf("table_name mismatch: expected %q, got %v", "comments", record["table_name"])
	}
	if record["crumb_id"] != "crumb-123" {
		t.Errorf("crumb_id mismatch: expected %q, got %v", "crumb-123", record["crumb_id"])
	}
	if record["property_id"] != nil {
		t.Errorf("property_id should be null, got %v", record["property_id"])
	}
	if record["content"] != "Test comment content" {
		t.Errorf("content mismatch: expected %q, got %v", "Test comment content", record["content"])
	}

	// Test metadata with property_id
	propID := "prop-456"
	meta2 := &types.Metadata{
		TableName:  "notes",
		CrumbID:    "crumb-789",
		PropertyID: &propID,
		Content:    "Property-specific note",
	}
	id2, _ := table.Set("", meta2)

	record2 := readJSONLRecord(t, filepath.Join(tmpDir, metadataJSONL), "metadata_id", id2)
	if record2["property_id"] != propID {
		t.Errorf("property_id mismatch: expected %q, got %v", propID, record2["property_id"])
	}
}

// TestJSON_StashFormat verifies stash JSON format matches prd-sqlite-backend R2.10.
func TestJSON_StashFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.StashesTable)

	// Global stash (no trail_id)
	stash := &types.Stash{
		Name:      "global_context",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"config": "value", "count": float64(42)},
		Version:   1,
	}
	id, _ := table.Set("", stash)

	record := readJSONLRecord(t, filepath.Join(tmpDir, stashesJSONL), "stash_id", id)

	// Verify fields per R2.10 (TrailID removed - now uses scoped_to links)
	if record["stash_id"] != id {
		t.Errorf("stash_id mismatch: expected %q, got %v", id, record["stash_id"])
	}
	if record["name"] != "global_context" {
		t.Errorf("name mismatch: expected %q, got %v", "global_context", record["name"])
	}
	if record["stash_type"] != types.StashTypeContext {
		t.Errorf("stash_type mismatch: expected %q, got %v", types.StashTypeContext, record["stash_type"])
	}

	// Verify value is a map
	value, ok := record["value"].(map[string]any)
	if !ok {
		t.Errorf("value should be a map, got %T", record["value"])
	} else {
		if value["config"] != "value" {
			t.Errorf("value.config mismatch: expected %q, got %v", "value", value["config"])
		}
	}

	// Version should be numeric
	version, ok := record["version"].(float64) // JSON numbers are float64
	if !ok {
		t.Errorf("version should be numeric, got %T", record["version"])
	} else if int64(version) != 1 {
		t.Errorf("version mismatch: expected 1, got %v", version)
	}
}

// TestJSON_CrumbPropertyFormat verifies crumb property JSON format matches prd-sqlite-backend R2.6.
func TestJSON_CrumbPropertyFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	propTable, _ := backend.GetTable(types.PropertiesTable)
	crumbTable, _ := backend.GetTable(types.CrumbsTable)

	// Define properties of different types
	textProp := &types.Property{Name: "desc", ValueType: types.ValueTypeText}
	intProp := &types.Property{Name: "count", ValueType: types.ValueTypeInteger}
	boolProp := &types.Property{Name: "done", ValueType: types.ValueTypeBoolean}
	listProp := &types.Property{Name: "tags", ValueType: types.ValueTypeList}

	textID, _ := propTable.Set("", textProp)
	intID, _ := propTable.Set("", intProp)
	boolID, _ := propTable.Set("", boolProp)
	listID, _ := propTable.Set("", listProp)

	// Create crumb (properties auto-initialized)
	crumb := &types.Crumb{Name: "Prop format test", State: types.StateDraft}
	crumbID, _ := crumbTable.Set("", crumb)

	// Read crumb_properties.jsonl
	records := readAllJSONLRecords(t, filepath.Join(tmpDir, crumbPropsJSONL))

	// Find records for our crumb
	crumbRecords := filterRecordsByCrumbID(records, crumbID)
	if len(crumbRecords) != 4 {
		t.Errorf("Expected 4 property records for crumb, got %d", len(crumbRecords))
	}

	// Verify format per R2.6
	for _, record := range crumbRecords {
		// Required fields
		if record["crumb_id"] != crumbID {
			t.Errorf("crumb_id mismatch")
		}
		if _, ok := record["property_id"]; !ok {
			t.Error("property_id missing")
		}
		if _, ok := record["value_type"]; !ok {
			t.Error("value_type missing")
		}
		if _, ok := record["value"]; !ok {
			t.Error("value missing")
		}

		// Verify value types match
		propID := record["property_id"].(string)
		valueType := record["value_type"].(string)
		value := record["value"]

		switch propID {
		case textID:
			if valueType != types.ValueTypeText {
				t.Errorf("text property value_type mismatch: got %s", valueType)
			}
			if value != "" {
				t.Errorf("text property default should be empty string, got %v", value)
			}
		case intID:
			if valueType != types.ValueTypeInteger {
				t.Errorf("integer property value_type mismatch: got %s", valueType)
			}
			// JSON numbers are float64
			if v, ok := value.(float64); !ok || int64(v) != 0 {
				t.Errorf("integer property default should be 0, got %v (%T)", value, value)
			}
		case boolID:
			if valueType != types.ValueTypeBoolean {
				t.Errorf("boolean property value_type mismatch: got %s", valueType)
			}
			if value != false {
				t.Errorf("boolean property default should be false, got %v", value)
			}
		case listID:
			if valueType != types.ValueTypeList {
				t.Errorf("list property value_type mismatch: got %s", valueType)
			}
			// List should be an empty array
			if arr, ok := value.([]any); !ok || len(arr) != 0 {
				t.Errorf("list property default should be empty array, got %v", value)
			}
		}
	}
}

// TestJSON_DataLoadingOnRestart verifies data loads correctly from JSON on restart.
func TestJSON_DataLoadingOnRestart(t *testing.T) {
	tmpDir := t.TempDir()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	// First session: create diverse data
	backend1 := NewBackend()
	if err := backend1.Attach(config); err != nil {
		t.Fatalf("First attach failed: %v", err)
	}

	crumbTable, _ := backend1.GetTable(types.CrumbsTable)
	trailTable, _ := backend1.GetTable(types.TrailsTable)
	linkTable, _ := backend1.GetTable(types.LinksTable)

	// Create multiple crumbs
	crumb1 := &types.Crumb{Name: "Crumb 1", State: types.StateDraft}
	crumb2 := &types.Crumb{Name: "Crumb 2", State: types.StateReady}
	crumb3 := &types.Crumb{Name: "Crumb 3", State: types.StatePebble}
	id1, _ := crumbTable.Set("", crumb1)
	id2, _ := crumbTable.Set("", crumb2)
	_, _ = crumbTable.Set("", crumb3) // id3 not used but needed for count

	// Create trail
	trail := &types.Trail{State: types.TrailStateActive}
	trailID, _ := trailTable.Set("", trail)

	// Create links
	link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: id1, ToID: trailID}
	link2 := &types.Link{LinkType: types.LinkTypeChildOf, FromID: id2, ToID: id1}
	linkTable.Set("", link1)
	linkTable.Set("", link2)

	backend1.Detach()

	// Verify JSON files have content
	crumbsPath := filepath.Join(tmpDir, crumbsJSONL)
	info, _ := os.Stat(crumbsPath)
	if info.Size() == 0 {
		t.Fatal("crumbs.jsonl should have content")
	}

	// Second session: verify data loads
	backend2 := NewBackend()
	if err := backend2.Attach(config); err != nil {
		t.Fatalf("Second attach failed: %v", err)
	}
	defer backend2.Detach()

	crumbTable2, _ := backend2.GetTable(types.CrumbsTable)
	trailTable2, _ := backend2.GetTable(types.TrailsTable)
	linkTable2, _ := backend2.GetTable(types.LinksTable)

	// Verify all crumbs loaded
	allCrumbs, _ := crumbTable2.Fetch(nil)
	if len(allCrumbs) != 3 {
		t.Errorf("Expected 3 crumbs after restart, got %d", len(allCrumbs))
	}

	// Verify specific crumb data
	entity, err := crumbTable2.Get(id2)
	if err != nil {
		t.Fatalf("Get crumb 2 failed: %v", err)
	}
	loaded := entity.(*types.Crumb)
	if loaded.Name != "Crumb 2" || loaded.State != types.StateReady {
		t.Errorf("Crumb 2 data mismatch: name=%q, state=%q", loaded.Name, loaded.State)
	}

	// Verify trail loaded
	trailEntity, err := trailTable2.Get(trailID)
	if err != nil {
		t.Fatalf("Get trail failed: %v", err)
	}
	loadedTrail := trailEntity.(*types.Trail)
	if loadedTrail.State != types.TrailStateActive {
		t.Errorf("Trail state mismatch: expected %q, got %q", types.TrailStateActive, loadedTrail.State)
	}

	// Verify links loaded
	allLinks, _ := linkTable2.Fetch(nil)
	if len(allLinks) != 2 {
		t.Errorf("Expected 2 links after restart, got %d", len(allLinks))
	}

	// Verify filter still works after loading
	readyCrumbs, _ := crumbTable2.Fetch(map[string]any{"State": types.StateReady})
	if len(readyCrumbs) != 1 {
		t.Errorf("Expected 1 ready crumb, got %d", len(readyCrumbs))
	}
}

// TestJSON_AtomicWritePreventsCorruption verifies that writes are atomic.
func TestJSON_AtomicWritePreventsCorruption(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	table, _ := backend.GetTable(types.CrumbsTable)

	// Create many crumbs to increase file size
	var ids []string
	for i := 0; i < 100; i++ {
		crumb := &types.Crumb{
			Name:  "Crumb for atomic test",
			State: types.StateDraft,
		}
		id, _ := table.Set("", crumb)
		ids = append(ids, id)
	}

	backend.Detach()

	// Verify JSON file is valid after many writes
	path := filepath.Join(tmpDir, crumbsJSONL)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open crumbs.jsonl failed: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", lineCount+1, err)
		}
		lineCount++
	}

	if lineCount != 100 {
		t.Errorf("Expected 100 records, got %d", lineCount)
	}
}

// TestJSON_DeleteRemovesFromFile verifies that Delete removes records from JSON.
func TestJSON_DeleteRemovesFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.CrumbsTable)

	// Create 5 crumbs
	var ids []string
	for i := 0; i < 5; i++ {
		crumb := &types.Crumb{Name: "Delete test crumb", State: types.StateDraft}
		id, _ := table.Set("", crumb)
		ids = append(ids, id)
	}

	// Verify all 5 are in JSON
	records := readAllJSONLRecords(t, filepath.Join(tmpDir, crumbsJSONL))
	if len(records) != 5 {
		t.Errorf("Expected 5 records before delete, got %d", len(records))
	}

	// Delete 3 crumbs
	table.Delete(ids[1])
	table.Delete(ids[3])
	table.Delete(ids[4])

	// Verify only 2 remain in JSON
	records = readAllJSONLRecords(t, filepath.Join(tmpDir, crumbsJSONL))
	if len(records) != 2 {
		t.Errorf("Expected 2 records after delete, got %d", len(records))
	}

	// Verify the correct ones remain
	remainingIDs := make(map[string]bool)
	for _, r := range records {
		remainingIDs[r["crumb_id"].(string)] = true
	}
	if !remainingIDs[ids[0]] {
		t.Error("ids[0] should remain")
	}
	if !remainingIDs[ids[2]] {
		t.Error("ids[2] should remain")
	}
	if remainingIDs[ids[1]] || remainingIDs[ids[3]] || remainingIDs[ids[4]] {
		t.Error("Deleted IDs should not remain")
	}
}

// TestJSON_UpdateModifiesExistingRecord verifies that Set updates existing records in JSON.
func TestJSON_UpdateModifiesExistingRecord(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.CrumbsTable)

	// Create crumb
	crumb := &types.Crumb{Name: "Original name", State: types.StateDraft}
	id, _ := table.Set("", crumb)

	// Verify original in JSON
	record := readJSONLRecord(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)
	if record["name"] != "Original name" {
		t.Errorf("Original name mismatch: got %v", record["name"])
	}

	// Update crumb multiple times
	crumb.Name = "First update"
	table.Set(id, crumb)

	crumb.Name = "Second update"
	crumb.State = types.StateReady
	table.Set(id, crumb)

	// Verify JSON has only one record with latest data
	records := readAllJSONLRecords(t, filepath.Join(tmpDir, crumbsJSONL))
	if len(records) != 1 {
		t.Errorf("Should have 1 record, got %d", len(records))
	}

	record = records[0]
	if record["name"] != "Second update" {
		t.Errorf("Name should be 'Second update', got %v", record["name"])
	}
	if record["state"] != types.StateReady {
		t.Errorf("State should be 'ready', got %v", record["state"])
	}
}

// TestJSON_EmptyFileHandling verifies behavior with empty JSONL files.
func TestJSON_EmptyFileHandling(t *testing.T) {
	tmpDir := t.TempDir()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	// First attach creates empty files
	backend1 := NewBackend()
	if err := backend1.Attach(config); err != nil {
		t.Fatalf("First attach failed: %v", err)
	}
	backend1.Detach()

	// Verify files exist and are empty (or nearly empty)
	crumbsPath := filepath.Join(tmpDir, crumbsJSONL)
	info, _ := os.Stat(crumbsPath)
	if info.Size() > 0 {
		t.Logf("crumbs.jsonl has %d bytes (may contain whitespace)", info.Size())
	}

	// Second attach should handle empty files gracefully
	backend2 := NewBackend()
	if err := backend2.Attach(config); err != nil {
		t.Fatalf("Second attach with empty files failed: %v", err)
	}
	defer backend2.Detach()

	// Fetch should return empty slice, not error
	table, _ := backend2.GetTable(types.CrumbsTable)
	results, err := table.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch on empty table failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results from empty table, got %d", len(results))
	}
}

// TestJSON_UUIDFormat verifies that IDs are valid UUID format.
func TestJSON_UUIDFormat(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	table, _ := backend.GetTable(types.CrumbsTable)

	crumb := &types.Crumb{Name: "UUID test", State: types.StateDraft}
	id, _ := table.Set("", crumb)

	// UUID format: 8-4-4-4-12 hexadecimal characters
	// Example: 550e8400-e29b-41d4-a716-446655440000
	if len(id) != 36 {
		t.Errorf("UUID should be 36 characters, got %d", len(id))
	}

	// Verify format with hyphens at correct positions
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("UUID format incorrect: %s", id)
	}

	// Verify JSON has the same ID format
	record := readJSONLRecord(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)
	jsonID := record["crumb_id"].(string)
	if jsonID != id {
		t.Errorf("JSON ID mismatch: expected %q, got %q", id, jsonID)
	}

	// UUIDs should be lowercase per prd-sqlite-backend R2.11
	for _, c := range id {
		if c >= 'A' && c <= 'Z' {
			t.Errorf("UUID should be lowercase: %s", id)
			break
		}
	}
}

// Helper function to filter records by crumb_id
func filterRecordsByCrumbID(records []map[string]any, crumbID string) []map[string]any {
	var result []map[string]any
	for _, r := range records {
		if r["crumb_id"] == crumbID {
			result = append(result, r)
		}
	}
	return result
}
