// Integration test for uc001-crud-operations tracer bullet.
// Validates the full CRUD lifecycle from ARCHITECTURE § Usage Pattern.
// Implements: uc001-crud-operations (end-to-end validation of Cupboard, Table, and Crumb operations);
//
//	prd-cupboard-core R2, R4, R5;
//	prd-crumbs-interface R1, R3, R5;
//	docs/ARCHITECTURE § Main Interface, § Usage Pattern.
package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dukaforge/crumbs/pkg/types"
)

// TestUC001_CRUDOperations validates the tracer bullet for uc001-crud-operations.
// This test runs through the full CRUD lifecycle:
// 1. Create cupboard and attach with config
// 2. Create crumb via Table.Set (empty ID)
// 3. Fetch crumb via Table.Get
// 4. Update crumb via Table.Set (existing ID)
// 5. Delete crumb via Table.Delete
// 6. Query crumbs via Table.Fetch
// 7. Detach cupboard and verify operations fail
func TestUC001_CRUDOperations(t *testing.T) {
	// Step 1: Create cupboard (backend) and attach with config
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	err := cupboard.Attach(config)
	if err != nil {
		t.Fatalf("Cupboard.Attach failed: %v", err)
	}

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, "cupboard.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("cupboard.db was not created after Attach")
	}

	// Step 2: Get the crumbs table
	table, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		t.Fatalf("GetTable(crumbs) failed: %v", err)
	}

	// Step 3: Create a new crumb via Table.Set with empty ID
	crumb := &types.Crumb{
		Name:  "Implement feature X",
		State: types.StateDraft,
	}

	id, err := table.Set("", crumb)
	if err != nil {
		t.Fatalf("Table.Set (create) failed: %v", err)
	}
	if id == "" {
		t.Error("Table.Set should return generated ID")
	}
	if crumb.CrumbID != id {
		t.Errorf("Crumb.CrumbID should be set to %q, got %q", id, crumb.CrumbID)
	}
	if crumb.CreatedAt.IsZero() {
		t.Error("Crumb.CreatedAt should be set on creation")
	}
	if crumb.UpdatedAt.IsZero() {
		t.Error("Crumb.UpdatedAt should be set on creation")
	}

	// Step 4: Retrieve crumb via Table.Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Table.Get failed: %v", err)
	}

	retrieved, ok := entity.(*types.Crumb)
	if !ok {
		t.Fatalf("Expected *types.Crumb, got %T", entity)
	}
	if retrieved.CrumbID != id {
		t.Errorf("Retrieved CrumbID mismatch: expected %q, got %q", id, retrieved.CrumbID)
	}
	if retrieved.Name != "Implement feature X" {
		t.Errorf("Retrieved Name mismatch: expected %q, got %q", "Implement feature X", retrieved.Name)
	}
	if retrieved.State != types.StateDraft {
		t.Errorf("Retrieved State mismatch: expected %q, got %q", types.StateDraft, retrieved.State)
	}

	// Step 5: Update crumb via Table.Set with existing ID
	// Use entity method to change state (per ARCHITECTURE § Usage Pattern)
	err = crumb.SetState(types.StateReady)
	if err != nil {
		t.Fatalf("Crumb.SetState failed: %v", err)
	}
	crumb.Name = "Implement feature X (updated)"

	_, err = table.Set(id, crumb)
	if err != nil {
		t.Fatalf("Table.Set (update) failed: %v", err)
	}

	// Verify update
	entity, err = table.Get(id)
	if err != nil {
		t.Fatalf("Table.Get after update failed: %v", err)
	}
	updated := entity.(*types.Crumb)
	if updated.Name != "Implement feature X (updated)" {
		t.Errorf("Update did not persist name: expected %q, got %q", "Implement feature X (updated)", updated.Name)
	}
	if updated.State != types.StateReady {
		t.Errorf("Update did not persist state: expected %q, got %q", types.StateReady, updated.State)
	}

	// Step 6: Create a second crumb for Fetch testing
	crumb2 := &types.Crumb{
		Name:  "Fix authentication bug",
		State: types.StateDraft,
	}
	id2, err := table.Set("", crumb2)
	if err != nil {
		t.Fatalf("Table.Set (create second crumb) failed: %v", err)
	}

	// Step 7: Query crumbs via Table.Fetch (no filter - returns all)
	entities, err := table.Fetch(nil)
	if err != nil {
		t.Fatalf("Table.Fetch (all) failed: %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("Table.Fetch (all) expected 2 crumbs, got %d", len(entities))
	}

	// Step 8: Query with filter (state = draft)
	entities, err = table.Fetch(map[string]any{"State": types.StateDraft})
	if err != nil {
		t.Fatalf("Table.Fetch (filter) failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Table.Fetch (filter State=draft) expected 1 crumb, got %d", len(entities))
	}
	if len(entities) > 0 {
		filtered := entities[0].(*types.Crumb)
		if filtered.CrumbID != id2 {
			t.Errorf("Filtered crumb should be second crumb: expected %q, got %q", id2, filtered.CrumbID)
		}
	}

	// Step 9: Delete crumb via Table.Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Table.Delete failed: %v", err)
	}

	// Verify deletion
	_, err = table.Get(id)
	if err != types.ErrNotFound {
		t.Errorf("Table.Get after delete expected ErrNotFound, got %v", err)
	}

	// Verify second crumb still exists
	_, err = table.Get(id2)
	if err != nil {
		t.Errorf("Second crumb should still exist after first is deleted: %v", err)
	}

	// Clean up second crumb
	err = table.Delete(id2)
	if err != nil {
		t.Fatalf("Table.Delete (second crumb) failed: %v", err)
	}

	// Step 10: Detach cupboard and verify operations fail
	err = cupboard.Detach()
	if err != nil {
		t.Fatalf("Cupboard.Detach failed: %v", err)
	}

	// Verify operations return ErrCupboardDetached after Detach
	_, err = cupboard.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("GetTable after Detach expected ErrCupboardDetached, got %v", err)
	}

	// Verify Detach is idempotent
	err = cupboard.Detach()
	if err != nil {
		t.Errorf("Second Detach should be idempotent, got %v", err)
	}
}

// TestUC001_CupboardLifecycle validates the cupboard attach/detach lifecycle.
// Per prd-cupboard-core R4, R5, R6.
func TestUC001_CupboardLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()

	// Verify operations fail before Attach
	_, err := cupboard.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("GetTable before Attach expected ErrCupboardDetached, got %v", err)
	}

	// Attach
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	err = cupboard.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	// Verify double Attach returns ErrAlreadyAttached
	err = cupboard.Attach(config)
	if err != types.ErrAlreadyAttached {
		t.Errorf("Double Attach expected ErrAlreadyAttached, got %v", err)
	}

	// Operations should work after Attach
	table, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		t.Fatalf("GetTable after Attach failed: %v", err)
	}
	if table == nil {
		t.Error("GetTable returned nil table")
	}

	// Detach
	err = cupboard.Detach()
	if err != nil {
		t.Fatalf("Detach failed: %v", err)
	}

	// Operations should fail after Detach
	_, err = cupboard.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("GetTable after Detach expected ErrCupboardDetached, got %v", err)
	}
}

// TestUC001_AllTableTypes validates that all table types are accessible.
// Per prd-cupboard-core R2.5.
func TestUC001_AllTableTypes(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	err := cupboard.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer cupboard.Detach()

	// All standard table names per prd-cupboard-core R2.5
	tableNames := []string{
		types.CrumbsTable,
		types.TrailsTable,
		types.PropertiesTable,
		types.MetadataTable,
		types.LinksTable,
		types.StashesTable,
	}

	for _, name := range tableNames {
		table, err := cupboard.GetTable(name)
		if err != nil {
			t.Errorf("GetTable(%q) failed: %v", name, err)
			continue
		}
		if table == nil {
			t.Errorf("GetTable(%q) returned nil", name)
		}
	}

	// Unknown table should return ErrTableNotFound
	_, err = cupboard.GetTable("unknown_table")
	if err != types.ErrTableNotFound {
		t.Errorf("GetTable(unknown) expected ErrTableNotFound, got %v", err)
	}
}

// TestUC001_CrumbStateMethods validates crumb state transitions using entity methods.
// Per prd-crumbs-interface R4, R5.
func TestUC001_CrumbStateMethods(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	cupboard.Attach(config)
	defer cupboard.Detach()

	table, _ := cupboard.GetTable(types.CrumbsTable)

	// Create crumb in draft state
	crumb := &types.Crumb{
		Name:  "Test state transitions",
		State: types.StateDraft,
	}
	id, err := table.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	// Test SetState (draft -> ready)
	err = crumb.SetState(types.StateReady)
	if err != nil {
		t.Fatalf("SetState(ready) failed: %v", err)
	}
	if crumb.State != types.StateReady {
		t.Errorf("Expected state ready, got %q", crumb.State)
	}

	// Persist and verify
	table.Set(id, crumb)
	entity, _ := table.Get(id)
	persisted := entity.(*types.Crumb)
	if persisted.State != types.StateReady {
		t.Errorf("Persisted state mismatch: expected ready, got %q", persisted.State)
	}

	// Test SetState (ready -> taken)
	err = crumb.SetState(types.StateTaken)
	if err != nil {
		t.Fatalf("SetState(taken) failed: %v", err)
	}
	table.Set(id, crumb)

	// Test Complete (taken -> completed)
	err = crumb.Complete()
	if err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}
	if crumb.State != types.StateCompleted {
		t.Errorf("Expected state completed, got %q", crumb.State)
	}
	table.Set(id, crumb)

	// Test Archive (can be called from any state)
	err = crumb.Archive()
	if err != nil {
		t.Fatalf("Archive() failed: %v", err)
	}
	if crumb.State != types.StateArchived {
		t.Errorf("Expected state archived, got %q", crumb.State)
	}
	table.Set(id, crumb)

	// Final verification
	entity, _ = table.Get(id)
	final := entity.(*types.Crumb)
	if final.State != types.StateArchived {
		t.Errorf("Final persisted state mismatch: expected archived, got %q", final.State)
	}
}

// TestUC001_InvalidStateTransitions validates that invalid state methods return errors.
// Per prd-crumbs-interface R4.
func TestUC001_InvalidStateTransitions(t *testing.T) {
	crumb := &types.Crumb{
		Name:  "Test invalid transitions",
		State: types.StateDraft,
	}

	// Complete requires state=taken
	err := crumb.Complete()
	if err != types.ErrInvalidTransition {
		t.Errorf("Complete from draft expected ErrInvalidTransition, got %v", err)
	}

	// Fail requires state=taken
	err = crumb.Fail()
	if err != types.ErrInvalidTransition {
		t.Errorf("Fail from draft expected ErrInvalidTransition, got %v", err)
	}

	// SetState with invalid state
	err = crumb.SetState("invalid_state")
	if err != types.ErrInvalidState {
		t.Errorf("SetState(invalid) expected ErrInvalidState, got %v", err)
	}
}

// TestUC001_CrumbPropertyMethods validates crumb property operations.
// Per prd-crumbs-interface R5.
func TestUC001_CrumbPropertyMethods(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	cupboard.Attach(config)
	defer cupboard.Detach()

	propTable, _ := cupboard.GetTable(types.PropertiesTable)
	crumbTable, _ := cupboard.GetTable(types.CrumbsTable)

	// Define a text property
	prop := &types.Property{
		Name:        "description",
		Description: "Task description",
		ValueType:   types.ValueTypeText,
	}
	propID, err := propTable.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a crumb (should auto-initialize property)
	crumb := &types.Crumb{
		Name:  "Test properties",
		State: types.StateDraft,
	}
	crumbID, err := crumbTable.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	// Verify property was auto-initialized with default
	val, err := crumb.GetProperty(propID)
	if err != nil {
		t.Fatalf("GetProperty failed: %v", err)
	}
	if val != "" {
		t.Errorf("Text property default should be empty string, got %v", val)
	}

	// SetProperty
	err = crumb.SetProperty(propID, "This is a description")
	if err != nil {
		t.Fatalf("SetProperty failed: %v", err)
	}

	// Persist and verify
	_, err = crumbTable.Set(crumbID, crumb)
	if err != nil {
		t.Fatalf("Update crumb with property failed: %v", err)
	}

	val, err = crumb.GetProperty(propID)
	if err != nil {
		t.Fatalf("GetProperty after set failed: %v", err)
	}
	if val != "This is a description" {
		t.Errorf("Property value mismatch: expected %q, got %v", "This is a description", val)
	}

	// GetProperties (all)
	props := crumb.GetProperties()
	if len(props) != 1 {
		t.Errorf("Expected 1 property, got %d", len(props))
	}

	// ClearProperty
	err = crumb.ClearProperty(propID)
	if err != nil {
		t.Fatalf("ClearProperty failed: %v", err)
	}

	// After clear, property is removed from map (entity level)
	// Note: Full default-value semantics require Table.Set to reinitialize
	_, err = crumb.GetProperty(propID)
	if err != types.ErrPropertyNotFound {
		t.Errorf("GetProperty after clear expected ErrPropertyNotFound, got %v", err)
	}
}

// TestUC001_FetchWithMultipleFilters validates Fetch with state filtering.
// Per prd-crumbs-interface R7, R8.
func TestUC001_FetchWithMultipleFilters(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	cupboard.Attach(config)
	defer cupboard.Detach()

	table, _ := cupboard.GetTable(types.CrumbsTable)

	// Create crumbs in various states
	crumbs := []*types.Crumb{
		{Name: "Draft 1", State: types.StateDraft},
		{Name: "Draft 2", State: types.StateDraft},
		{Name: "Ready 1", State: types.StateReady},
		{Name: "Taken 1", State: types.StateTaken},
		{Name: "Archived 1", State: types.StateArchived},
	}

	for _, c := range crumbs {
		_, err := table.Set("", c)
		if err != nil {
			t.Fatalf("Create crumb %q failed: %v", c.Name, err)
		}
	}

	// Fetch all
	all, err := table.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch all failed: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("Fetch all expected 5 crumbs, got %d", len(all))
	}

	// Fetch draft only
	draft, err := table.Fetch(map[string]any{"State": types.StateDraft})
	if err != nil {
		t.Fatalf("Fetch draft failed: %v", err)
	}
	if len(draft) != 2 {
		t.Errorf("Fetch draft expected 2 crumbs, got %d", len(draft))
	}

	// Fetch ready only
	ready, err := table.Fetch(map[string]any{"State": types.StateReady})
	if err != nil {
		t.Fatalf("Fetch ready failed: %v", err)
	}
	if len(ready) != 1 {
		t.Errorf("Fetch ready expected 1 crumb, got %d", len(ready))
	}

	// Fetch archived only
	archived, err := table.Fetch(map[string]any{"State": types.StateArchived})
	if err != nil {
		t.Fatalf("Fetch archived failed: %v", err)
	}
	if len(archived) != 1 {
		t.Errorf("Fetch archived expected 1 crumb, got %d", len(archived))
	}

	// Verify archived crumb is not in draft results
	for _, e := range draft {
		c := e.(*types.Crumb)
		if c.State == types.StateArchived {
			t.Error("Archived crumb should not appear in draft filter results")
		}
	}
}

// TestUC001_TableErrorCases validates error handling for Table operations.
func TestUC001_TableErrorCases(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	cupboard.Attach(config)
	defer cupboard.Detach()

	table, _ := cupboard.GetTable(types.CrumbsTable)

	// Get with empty ID
	_, err := table.Get("")
	if err != types.ErrInvalidID {
		t.Errorf("Get with empty ID expected ErrInvalidID, got %v", err)
	}

	// Get non-existent ID
	_, err = table.Get("non-existent-id")
	if err != types.ErrNotFound {
		t.Errorf("Get non-existent expected ErrNotFound, got %v", err)
	}

	// Delete with empty ID
	err = table.Delete("")
	if err != types.ErrInvalidID {
		t.Errorf("Delete with empty ID expected ErrInvalidID, got %v", err)
	}

	// Delete non-existent ID
	err = table.Delete("non-existent-id")
	if err != types.ErrNotFound {
		t.Errorf("Delete non-existent expected ErrNotFound, got %v", err)
	}

	// Set with wrong type
	trail := &types.Trail{State: types.TrailStateActive}
	_, err = table.Set("", trail)
	if err != types.ErrInvalidData {
		t.Errorf("Set wrong type expected ErrInvalidData, got %v", err)
	}
}

// TestUC001_JSONLPersistence validates that data is persisted to JSONL files.
// Per prd-configuration-directories R4, R6.
func TestUC001_JSONLPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	err := cupboard.Attach(config)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	// Verify JSONL files are created
	jsonlFiles := []string{
		"crumbs.jsonl",
		"trails.jsonl",
		"properties.jsonl",
		"metadata.jsonl",
		"links.jsonl",
		"stashes.jsonl",
		"crumb_properties.jsonl",
	}

	for _, filename := range jsonlFiles {
		path := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("JSONL file %s was not created", filename)
		}
	}

	// Create a crumb
	table, _ := cupboard.GetTable(types.CrumbsTable)
	crumb := &types.Crumb{
		Name:  "Persistent crumb",
		State: types.StateDraft,
	}
	_, err = table.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	// Detach to ensure all data is flushed
	cupboard.Detach()

	// Verify crumbs.jsonl has content
	crumbsPath := filepath.Join(tmpDir, "crumbs.jsonl")
	info, err := os.Stat(crumbsPath)
	if err != nil {
		t.Fatalf("Stat crumbs.jsonl failed: %v", err)
	}
	if info.Size() == 0 {
		t.Error("crumbs.jsonl should have content after creating a crumb")
	}

	// Re-attach and verify data is loaded
	cupboard2 := NewBackend()
	err = cupboard2.Attach(config)
	if err != nil {
		t.Fatalf("Re-attach failed: %v", err)
	}
	defer cupboard2.Detach()

	table2, _ := cupboard2.GetTable(types.CrumbsTable)
	entities, err := table2.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch after re-attach failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Expected 1 crumb after re-attach, got %d", len(entities))
	}
	if len(entities) > 0 {
		loaded := entities[0].(*types.Crumb)
		if loaded.Name != "Persistent crumb" {
			t.Errorf("Loaded crumb name mismatch: expected %q, got %q", "Persistent crumb", loaded.Name)
		}
	}
}

// TestUC001_FullUseCaseFlow runs through the complete uc001 flow as described.
// This is the main tracer bullet test.
func TestUC001_FullUseCaseFlow(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create cupboard and attach
	cupboard := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := cupboard.Attach(config); err != nil {
		t.Fatalf("Step 1 (Attach): %v", err)
	}

	// Get tables
	crumbTable, _ := cupboard.GetTable(types.CrumbsTable)
	propTable, _ := cupboard.GetTable(types.PropertiesTable)

	// 2. Define a property (simulating built-in properties)
	descProp := &types.Property{
		Name:        "description",
		Description: "Task description",
		ValueType:   types.ValueTypeText,
	}
	descPropID, err := propTable.Set("", descProp)
	if err != nil {
		t.Fatalf("Step 2 (Define property): %v", err)
	}

	// 3. Add first crumb
	crumb1 := &types.Crumb{
		Name:  "Implement login feature",
		State: types.StateDraft,
	}
	id1, err := crumbTable.Set("", crumb1)
	if err != nil {
		t.Fatalf("Step 3 (Add first crumb): %v", err)
	}

	// 4. Verify property was initialized
	val, err := crumb1.GetProperty(descPropID)
	if err != nil {
		t.Fatalf("Step 4 (Verify property init): %v", err)
	}
	if val != "" {
		t.Errorf("Step 4: Expected empty string default, got %v", val)
	}

	// 5. Change crumb state (draft -> ready)
	if err := crumb1.SetState(types.StateReady); err != nil {
		t.Fatalf("Step 5 (Change state): %v", err)
	}
	if _, err := crumbTable.Set(id1, crumb1); err != nil {
		t.Fatalf("Step 5 (Persist state): %v", err)
	}

	// 6. Add second crumb
	crumb2 := &types.Crumb{
		Name:  "Fix authentication bug",
		State: types.StateDraft,
	}
	id2, err := crumbTable.Set("", crumb2)
	if err != nil {
		t.Fatalf("Step 6 (Add second crumb): %v", err)
	}

	// 7. Set property value on first crumb
	if err := crumb1.SetProperty(descPropID, "User authentication flow"); err != nil {
		t.Fatalf("Step 7 (Set property): %v", err)
	}
	if _, err := crumbTable.Set(id1, crumb1); err != nil {
		t.Fatalf("Step 7 (Persist property): %v", err)
	}

	// 8. Archive second crumb
	if err := crumb2.Archive(); err != nil {
		t.Fatalf("Step 8 (Archive): %v", err)
	}
	if _, err := crumbTable.Set(id2, crumb2); err != nil {
		t.Fatalf("Step 8 (Persist archive): %v", err)
	}

	// 9. Fetch with filter (only ready crumbs)
	ready, err := crumbTable.Fetch(map[string]any{"State": types.StateReady})
	if err != nil {
		t.Fatalf("Step 9 (Fetch ready): %v", err)
	}
	if len(ready) != 1 {
		t.Errorf("Step 9: Expected 1 ready crumb, got %d", len(ready))
	}
	if len(ready) > 0 {
		filtered := ready[0].(*types.Crumb)
		if filtered.CrumbID != id1 {
			t.Errorf("Step 9: Filtered crumb should be first crumb")
		}
	}

	// 10. Delete (purge) archived crumb
	if err := crumbTable.Delete(id2); err != nil {
		t.Fatalf("Step 10 (Delete): %v", err)
	}

	// Verify deleted
	if _, err := crumbTable.Get(id2); err != types.ErrNotFound {
		t.Errorf("Step 10: Deleted crumb should return ErrNotFound, got %v", err)
	}

	// 11. Detach cupboard
	if err := cupboard.Detach(); err != nil {
		t.Fatalf("Step 11 (Detach): %v", err)
	}

	// 12. Verify operations fail after detach
	_, err = cupboard.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("Step 12: Expected ErrCupboardDetached, got %v", err)
	}

	t.Log("uc001-crud-operations tracer bullet completed successfully")
}
