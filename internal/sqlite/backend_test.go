// Tests for SQLite backend implementation.
// Implements: prd-sqlite-backend acceptance criteria (unit tests).
package sqlite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dukaforge/crumbs/pkg/types"
)

func TestBackend_Attach(t *testing.T) {
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

	// Verify database file created
	dbPath := filepath.Join(tmpDir, "cupboard.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("cupboard.db not created")
	}

	// Verify double attach fails
	err = b.Attach(config)
	if err != types.ErrAlreadyAttached {
		t.Errorf("expected ErrAlreadyAttached, got %v", err)
	}

	// Clean up
	b.Detach()
}

func TestBackend_Detach(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	b.Attach(config)

	err := b.Detach()
	if err != nil {
		t.Fatalf("Detach failed: %v", err)
	}

	// Verify idempotent
	err = b.Detach()
	if err != nil {
		t.Errorf("second Detach should not error, got %v", err)
	}

	// Verify operations fail after detach
	_, err = b.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("expected ErrCupboardDetached, got %v", err)
	}
}

func TestBackend_GetTable(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tables := []string{
		types.CrumbsTable,
		types.TrailsTable,
		types.PropertiesTable,
		types.MetadataTable,
		types.LinksTable,
		types.StashesTable,
	}

	for _, name := range tables {
		tbl, err := b.GetTable(name)
		if err != nil {
			t.Errorf("GetTable(%q) failed: %v", name, err)
		}
		if tbl == nil {
			t.Errorf("GetTable(%q) returned nil", name)
		}
	}

	// Unknown table
	_, err := b.GetTable("unknown")
	if err != types.ErrTableNotFound {
		t.Errorf("expected ErrTableNotFound for unknown table, got %v", err)
	}
}

func TestCrumbTable_CRUD(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Create
	crumb := &types.Crumb{
		Name:  "Test Crumb",
		State: types.StatePending,
	}

	id, err := tbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if id == "" {
		t.Error("Set should return generated ID")
	}

	// Read
	result, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	gotCrumb, ok := result.(*types.Crumb)
	if !ok {
		t.Fatalf("expected *types.Crumb, got %T", result)
	}
	if gotCrumb.Name != "Test Crumb" {
		t.Errorf("expected Name='Test Crumb', got %q", gotCrumb.Name)
	}
	if gotCrumb.State != types.StatePending {
		t.Errorf("expected State='pending', got %q", gotCrumb.State)
	}

	// Update
	crumb.Name = "Updated Crumb"
	crumb.CrumbID = id
	_, err = tbl.Set(id, crumb)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	result, _ = tbl.Get(id)
	gotCrumb = result.(*types.Crumb)
	if gotCrumb.Name != "Updated Crumb" {
		t.Errorf("expected updated Name, got %q", gotCrumb.Name)
	}

	// Delete
	err = tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = tbl.Get(id)
	if err != types.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCrumbTable_Fetch(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Insert test data
	crumbs := []*types.Crumb{
		{Name: "Crumb A", State: types.StatePending},
		{Name: "Crumb B", State: types.StatePending},
		{Name: "Crumb C", State: types.StateReady},
	}

	for _, c := range crumbs {
		tbl.Set("", c)
	}

	// Fetch all
	results, err := tbl.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch all failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 crumbs, got %d", len(results))
	}

	// Fetch with filter
	results, err = tbl.Fetch(map[string]any{"State": types.StatePending})
	if err != nil {
		t.Fatalf("Fetch with filter failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 pending crumbs, got %d", len(results))
	}
}

func TestTrailTable_CRUD(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.TrailsTable)

	// Create
	trail := &types.Trail{
		State: types.TrailStateActive,
	}

	id, err := tbl.Set("", trail)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read
	result, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	gotTrail, ok := result.(*types.Trail)
	if !ok {
		t.Fatalf("expected *types.Trail, got %T", result)
	}
	if gotTrail.State != types.TrailStateActive {
		t.Errorf("expected State='active', got %q", gotTrail.State)
	}

	// Update with parent crumb
	parentID := "parent-123"
	trail.ParentCrumbID = &parentID
	trail.TrailID = id
	tbl.Set(id, trail)

	result, _ = tbl.Get(id)
	gotTrail = result.(*types.Trail)
	if gotTrail.ParentCrumbID == nil || *gotTrail.ParentCrumbID != "parent-123" {
		t.Errorf("expected ParentCrumbID='parent-123', got %v", gotTrail.ParentCrumbID)
	}

	// Delete
	err = tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestPropertyTable_CRUD(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.PropertiesTable)

	// Create
	prop := &types.Property{
		Name:        "priority",
		Description: "Task priority",
		ValueType:   types.ValueTypeCategorical,
	}

	id, err := tbl.Set("", prop)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read
	result, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	gotProp, ok := result.(*types.Property)
	if !ok {
		t.Fatalf("expected *types.Property, got %T", result)
	}
	if gotProp.Name != "priority" {
		t.Errorf("expected Name='priority', got %q", gotProp.Name)
	}

	// Delete
	err = tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestLinkTable_CRUD(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.LinksTable)

	// Create
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   "crumb-123",
		ToID:     "trail-456",
	}

	id, err := tbl.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read
	result, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	gotLink, ok := result.(*types.Link)
	if !ok {
		t.Fatalf("expected *types.Link, got %T", result)
	}
	if gotLink.LinkType != types.LinkTypeBelongsTo {
		t.Errorf("expected LinkType='belongs_to', got %q", gotLink.LinkType)
	}
	if gotLink.FromID != "crumb-123" {
		t.Errorf("expected FromID='crumb-123', got %q", gotLink.FromID)
	}

	// Fetch by type
	results, err := tbl.Fetch(map[string]any{"LinkType": types.LinkTypeBelongsTo})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 link, got %d", len(results))
	}

	// Delete
	err = tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestStashTable_CRUD(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.StashesTable)

	// Create counter stash
	stash := &types.Stash{
		Name:      "task_counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
		Version:   1,
	}

	id, err := tbl.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read
	result, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	gotStash, ok := result.(*types.Stash)
	if !ok {
		t.Fatalf("expected *types.Stash, got %T", result)
	}
	if gotStash.Name != "task_counter" {
		t.Errorf("expected Name='task_counter', got %q", gotStash.Name)
	}
	if gotStash.StashType != types.StashTypeCounter {
		t.Errorf("expected StashType='counter', got %q", gotStash.StashType)
	}

	// Update stash with trail scope
	trailID := "trail-789"
	stash.TrailID = &trailID
	stash.StashID = id
	stash.Version = 2
	tbl.Set(id, stash)

	result, _ = tbl.Get(id)
	gotStash = result.(*types.Stash)
	if gotStash.TrailID == nil || *gotStash.TrailID != "trail-789" {
		t.Errorf("expected TrailID='trail-789', got %v", gotStash.TrailID)
	}

	// Delete
	err = tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestMetadataTable_CRUD(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.MetadataTable)

	// Create
	meta := &types.Metadata{
		TableName: "comments",
		CrumbID:   "crumb-123",
		Content:   "This is a comment",
	}

	id, err := tbl.Set("", meta)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read
	result, err := tbl.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	gotMeta, ok := result.(*types.Metadata)
	if !ok {
		t.Fatalf("expected *types.Metadata, got %T", result)
	}
	if gotMeta.TableName != "comments" {
		t.Errorf("expected TableName='comments', got %q", gotMeta.TableName)
	}
	if gotMeta.Content != "This is a comment" {
		t.Errorf("expected Content='This is a comment', got %q", gotMeta.Content)
	}

	// Fetch by crumb
	results, err := tbl.Fetch(map[string]any{"CrumbID": "crumb-123"})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 metadata entry, got %d", len(results))
	}

	// Delete
	err = tbl.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestTable_ErrNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Get non-existent
	_, err := tbl.Get("non-existent-id")
	if err != types.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Delete non-existent
	err = tbl.Delete("non-existent-id")
	if err != types.ErrNotFound {
		t.Errorf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestTable_InvalidData(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Try to set wrong type
	_, err := tbl.Set("", &types.Trail{})
	if err != types.ErrInvalidData {
		t.Errorf("expected ErrInvalidData for wrong type, got %v", err)
	}
}

func TestTable_TimestampPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	tbl, _ := b.GetTable(types.CrumbsTable)

	// Create with specific timestamp
	now := time.Now().Truncate(time.Second)
	crumb := &types.Crumb{
		Name:      "Test",
		State:     types.StatePending,
		CreatedAt: now,
	}

	id, _ := tbl.Set("", crumb)

	result, _ := tbl.Get(id)
	gotCrumb := result.(*types.Crumb)

	// CreatedAt should be preserved
	if !gotCrumb.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt not preserved: expected %v, got %v", now, gotCrumb.CreatedAt)
	}

	// UpdatedAt should be set automatically
	if gotCrumb.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set automatically")
	}
}

// Tests for property auto-initialization on crumb creation.
// Implements: prd-crumbs-interface R3.7; prd-properties-interface R3.5

func TestCrumbTable_PropertyAutoInit_TextProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define a text property
	prop := &types.Property{
		Name:        "description",
		Description: "Task description",
		ValueType:   types.ValueTypeText,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err = crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	// Verify the crumb has the property initialized with default value
	if crumb.Properties == nil {
		t.Fatal("Properties map should be initialized")
	}

	val, ok := crumb.Properties[propID]
	if !ok {
		t.Fatalf("Property %s should be present in crumb.Properties", propID)
	}

	if val != "" {
		t.Errorf("Text property default should be empty string, got %v", val)
	}
}

func TestCrumbTable_PropertyAutoInit_IntegerProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define an integer property
	prop := &types.Property{
		Name:        "priority_level",
		Description: "Numeric priority",
		ValueType:   types.ValueTypeInteger,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err = crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	val, ok := crumb.Properties[propID]
	if !ok {
		t.Fatalf("Property %s should be present in crumb.Properties", propID)
	}

	// Integer default should be 0
	intVal, ok := val.(int64)
	if !ok {
		t.Fatalf("Integer property should be int64, got %T", val)
	}
	if intVal != 0 {
		t.Errorf("Integer property default should be 0, got %d", intVal)
	}
}

func TestCrumbTable_PropertyAutoInit_BooleanProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define a boolean property
	prop := &types.Property{
		Name:        "is_urgent",
		Description: "Urgency flag",
		ValueType:   types.ValueTypeBoolean,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err = crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	val, ok := crumb.Properties[propID]
	if !ok {
		t.Fatalf("Property %s should be present in crumb.Properties", propID)
	}

	// Boolean default should be false
	boolVal, ok := val.(bool)
	if !ok {
		t.Fatalf("Boolean property should be bool, got %T", val)
	}
	if boolVal != false {
		t.Errorf("Boolean property default should be false, got %v", boolVal)
	}
}

func TestCrumbTable_PropertyAutoInit_ListProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define a list property
	prop := &types.Property{
		Name:        "labels",
		Description: "Task labels",
		ValueType:   types.ValueTypeList,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err = crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	val, ok := crumb.Properties[propID]
	if !ok {
		t.Fatalf("Property %s should be present in crumb.Properties", propID)
	}

	// List default should be empty slice
	listVal, ok := val.([]string)
	if !ok {
		t.Fatalf("List property should be []string, got %T", val)
	}
	if len(listVal) != 0 {
		t.Errorf("List property default should be empty, got %v", listVal)
	}
}

func TestCrumbTable_PropertyAutoInit_TimestampProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define a timestamp property
	prop := &types.Property{
		Name:        "due_date",
		Description: "Due date",
		ValueType:   types.ValueTypeTimestamp,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err = crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	val, ok := crumb.Properties[propID]
	if !ok {
		t.Fatalf("Property %s should be present in crumb.Properties", propID)
	}

	// Timestamp default should be nil (null)
	if val != nil {
		t.Errorf("Timestamp property default should be nil, got %v", val)
	}
}

func TestCrumbTable_PropertyAutoInit_CategoricalProperty_NoCategories(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define a categorical property (without categories)
	prop := &types.Property{
		Name:        "status",
		Description: "Status enum",
		ValueType:   types.ValueTypeCategorical,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err = crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	val, ok := crumb.Properties[propID]
	if !ok {
		t.Fatalf("Property %s should be present in crumb.Properties", propID)
	}

	// Categorical without categories defaults to empty string
	if val != "" {
		t.Errorf("Categorical property without categories should default to empty string, got %v", val)
	}
}

func TestCrumbTable_PropertyAutoInit_MultipleProperties(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define multiple properties
	textProp := &types.Property{Name: "desc", ValueType: types.ValueTypeText}
	intProp := &types.Property{Name: "count", ValueType: types.ValueTypeInteger}
	boolProp := &types.Property{Name: "done", ValueType: types.ValueTypeBoolean}

	textID, _ := propTbl.Set("", textProp)
	intID, _ := propTbl.Set("", intProp)
	boolID, _ := propTbl.Set("", boolProp)

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err := crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	// Verify all properties are initialized
	if len(crumb.Properties) != 3 {
		t.Errorf("Expected 3 properties, got %d", len(crumb.Properties))
	}

	if crumb.Properties[textID] != "" {
		t.Errorf("Text property should be empty string")
	}
	if crumb.Properties[intID] != int64(0) {
		t.Errorf("Integer property should be 0")
	}
	if crumb.Properties[boolID] != false {
		t.Errorf("Boolean property should be false")
	}
}

func TestCrumbTable_PropertyAutoInit_UpdateDoesNotReinitialize(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)
	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Define a text property
	prop := &types.Property{
		Name:      "description",
		ValueType: types.ValueTypeText,
	}
	propID, _ := propTbl.Set("", prop)

	// Create a new crumb
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	crumbID, _ := crumbTbl.Set("", crumb)

	// Modify the property value
	crumb.Properties[propID] = "custom value"

	// Update the crumb (with existing ID)
	_, err := crumbTbl.Set(crumbID, crumb)
	if err != nil {
		t.Fatalf("Update crumb failed: %v", err)
	}

	// The property value should remain "custom value", not be reset to default
	if crumb.Properties[propID] != "custom value" {
		t.Errorf("Property should retain custom value after update, got %v", crumb.Properties[propID])
	}
}

func TestCrumbTable_PropertyAutoInit_NoPropertiesDefined(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.CrumbsTable)

	// Create a new crumb without any properties defined
	crumb := &types.Crumb{
		Name:  "Test Task",
		State: types.StateDraft,
	}
	_, err := crumbTbl.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}

	// Properties map should be initialized but empty
	if crumb.Properties == nil {
		t.Error("Properties map should be initialized even with no properties defined")
	}
	if len(crumb.Properties) != 0 {
		t.Errorf("Properties map should be empty when no properties defined, got %d", len(crumb.Properties))
	}
}

// Tests for property backfill on property definition.
// Implements: prd-properties-interface R4.2-R4.5

func TestPropertyTable_Backfill_ExistingCrumbs(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.CrumbsTable)
	propTbl, _ := b.GetTable(types.PropertiesTable)

	// Create several crumbs BEFORE defining the property
	crumb1 := &types.Crumb{Name: "Task 1", State: types.StateDraft}
	crumb2 := &types.Crumb{Name: "Task 2", State: types.StateReady}
	crumb3 := &types.Crumb{Name: "Task 3", State: types.StateTaken}

	id1, err := crumbTbl.Set("", crumb1)
	if err != nil {
		t.Fatalf("Create crumb1 failed: %v", err)
	}
	id2, err := crumbTbl.Set("", crumb2)
	if err != nil {
		t.Fatalf("Create crumb2 failed: %v", err)
	}
	id3, err := crumbTbl.Set("", crumb3)
	if err != nil {
		t.Fatalf("Create crumb3 failed: %v", err)
	}

	// Now define a new property - this should backfill all existing crumbs
	prop := &types.Property{
		Name:        "estimate",
		Description: "Story point estimate",
		ValueType:   types.ValueTypeInteger,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Verify backfill by checking the crumb_properties table via direct query
	// (since we don't reload the in-memory crumb objects)
	var count int
	err = b.db.QueryRow("SELECT COUNT(*) FROM crumb_properties WHERE property_id = ?", propID).Scan(&count)
	if err != nil {
		t.Fatalf("Query crumb_properties failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 crumb_properties entries for backfill, got %d", count)
	}

	// Verify each crumb has the correct default value
	var value string
	for _, crumbID := range []string{id1, id2, id3} {
		err = b.db.QueryRow(
			"SELECT value FROM crumb_properties WHERE crumb_id = ? AND property_id = ?",
			crumbID, propID,
		).Scan(&value)
		if err != nil {
			t.Fatalf("Query crumb_property for %s failed: %v", crumbID, err)
		}
		// Integer default is 0, stored as JSON
		if value != "0" {
			t.Errorf("Expected integer default '0' for crumb %s, got %q", crumbID, value)
		}
	}
}

func TestPropertyTable_Backfill_TextProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.CrumbsTable)
	propTbl, _ := b.GetTable(types.PropertiesTable)

	// Create a crumb first
	crumb := &types.Crumb{Name: "Task", State: types.StateDraft}
	crumbID, _ := crumbTbl.Set("", crumb)

	// Define a text property
	prop := &types.Property{
		Name:      "description",
		ValueType: types.ValueTypeText,
	}
	propID, _ := propTbl.Set("", prop)

	// Verify backfill with text default (empty string)
	var value string
	err := b.db.QueryRow(
		"SELECT value FROM crumb_properties WHERE crumb_id = ? AND property_id = ?",
		crumbID, propID,
	).Scan(&value)
	if err != nil {
		t.Fatalf("Query crumb_property failed: %v", err)
	}
	if value != `""` {
		t.Errorf("Expected text default '\"\"' (empty string), got %q", value)
	}
}

func TestPropertyTable_Backfill_BooleanProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.CrumbsTable)
	propTbl, _ := b.GetTable(types.PropertiesTable)

	// Create a crumb first
	crumb := &types.Crumb{Name: "Task", State: types.StateDraft}
	crumbID, _ := crumbTbl.Set("", crumb)

	// Define a boolean property
	prop := &types.Property{
		Name:      "is_urgent",
		ValueType: types.ValueTypeBoolean,
	}
	propID, _ := propTbl.Set("", prop)

	// Verify backfill with boolean default (false)
	var value string
	err := b.db.QueryRow(
		"SELECT value FROM crumb_properties WHERE crumb_id = ? AND property_id = ?",
		crumbID, propID,
	).Scan(&value)
	if err != nil {
		t.Fatalf("Query crumb_property failed: %v", err)
	}
	if value != "false" {
		t.Errorf("Expected boolean default 'false', got %q", value)
	}
}

func TestPropertyTable_Backfill_ListProperty(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.CrumbsTable)
	propTbl, _ := b.GetTable(types.PropertiesTable)

	// Create a crumb first
	crumb := &types.Crumb{Name: "Task", State: types.StateDraft}
	crumbID, _ := crumbTbl.Set("", crumb)

	// Define a list property
	prop := &types.Property{
		Name:      "labels",
		ValueType: types.ValueTypeList,
	}
	propID, _ := propTbl.Set("", prop)

	// Verify backfill with list default (empty array)
	var value string
	err := b.db.QueryRow(
		"SELECT value FROM crumb_properties WHERE crumb_id = ? AND property_id = ?",
		crumbID, propID,
	).Scan(&value)
	if err != nil {
		t.Fatalf("Query crumb_property failed: %v", err)
	}
	if value != "[]" {
		t.Errorf("Expected list default '[]' (empty array), got %q", value)
	}
}

func TestPropertyTable_Backfill_NoCrumbs(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	propTbl, _ := b.GetTable(types.PropertiesTable)

	// Define a property when there are no crumbs
	prop := &types.Property{
		Name:      "estimate",
		ValueType: types.ValueTypeInteger,
	}
	propID, err := propTbl.Set("", prop)
	if err != nil {
		t.Fatalf("Create property failed: %v", err)
	}

	// Should succeed (no-op for backfill)
	if propID == "" {
		t.Error("Property should be created even with no crumbs")
	}

	// Verify no crumb_properties entries (since no crumbs exist)
	var count int
	b.db.QueryRow("SELECT COUNT(*) FROM crumb_properties").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 crumb_properties entries, got %d", count)
	}
}

func TestPropertyTable_Backfill_UpdateDoesNotRebackfill(t *testing.T) {
	tmpDir := t.TempDir()

	b := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	b.Attach(config)
	defer b.Detach()

	crumbTbl, _ := b.GetTable(types.CrumbsTable)
	propTbl, _ := b.GetTable(types.PropertiesTable)

	// Create a crumb
	crumb := &types.Crumb{Name: "Task", State: types.StateDraft}
	crumbID, _ := crumbTbl.Set("", crumb)

	// Define a property (triggers backfill)
	prop := &types.Property{
		Name:      "estimate",
		ValueType: types.ValueTypeInteger,
	}
	propID, _ := propTbl.Set("", prop)

	// Manually update the crumb_property to a non-default value
	_, err := b.db.Exec(
		"UPDATE crumb_properties SET value = ? WHERE crumb_id = ? AND property_id = ?",
		"42", crumbID, propID,
	)
	if err != nil {
		t.Fatalf("Update crumb_property failed: %v", err)
	}

	// Update the property (using the existing ID)
	prop.Description = "Updated description"
	_, err = propTbl.Set(propID, prop)
	if err != nil {
		t.Fatalf("Update property failed: %v", err)
	}

	// Verify the custom value was NOT overwritten (update should not re-backfill)
	var value string
	b.db.QueryRow(
		"SELECT value FROM crumb_properties WHERE crumb_id = ? AND property_id = ?",
		crumbID, propID,
	).Scan(&value)
	if value != "42" {
		t.Errorf("Property update should not re-backfill; expected '42', got %q", value)
	}
}
