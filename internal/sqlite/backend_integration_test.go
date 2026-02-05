// Integration tests for SQLite backend CRUD operations with JSON persistence verification.
// Tests the full flow: Attach → GetTable → Set (create) → Get → Set (update) → Fetch → Delete
// for each entity type: Crumb, Trail, Property, Stash, Metadata, Link.
// Implements: prd-sqlite-backend (acceptance criteria);
//
//	prd-cupboard-core R2, R3, R4, R5;
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dukaforge/crumbs/pkg/types"
)

// TestIntegration_CrumbCRUD tests the full CRUD lifecycle for Crumb entities
// with JSON persistence verification at each step.
func TestIntegration_CrumbCRUD(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Attach
	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer backend.Detach()

	// Step 2: GetTable
	table, err := backend.GetTable(types.CrumbsTable)
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// Step 3: Set (create)
	crumb := &types.Crumb{
		Name:  "Integration test crumb",
		State: types.StateDraft,
	}
	id, err := table.Set("", crumb)
	if err != nil {
		t.Fatalf("Set (create) failed: %v", err)
	}
	if id == "" {
		t.Fatal("Set should return generated ID")
	}
	if crumb.CrumbID != id {
		t.Errorf("Crumb.CrumbID should be set to %q, got %q", id, crumb.CrumbID)
	}

	// Verify JSON file contains the crumb
	verifyJSONLContains(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)

	// Step 4: Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Crumb)
	if retrieved.Name != "Integration test crumb" {
		t.Errorf("Name mismatch: expected %q, got %q", "Integration test crumb", retrieved.Name)
	}
	if retrieved.State != types.StateDraft {
		t.Errorf("State mismatch: expected %q, got %q", types.StateDraft, retrieved.State)
	}

	// Step 5: Set (update)
	crumb.Name = "Updated crumb name"
	crumb.State = types.StateReady
	_, err = table.Set(id, crumb)
	if err != nil {
		t.Fatalf("Set (update) failed: %v", err)
	}

	// Verify JSON file reflects the update
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)
	if jsonRecord["name"] != "Updated crumb name" {
		t.Errorf("JSON name mismatch: expected %q, got %v", "Updated crumb name", jsonRecord["name"])
	}
	if jsonRecord["state"] != types.StateReady {
		t.Errorf("JSON state mismatch: expected %q, got %v", types.StateReady, jsonRecord["state"])
	}

	// Verify Get returns updated data
	entity, _ = table.Get(id)
	updated := entity.(*types.Crumb)
	if updated.Name != "Updated crumb name" {
		t.Errorf("Updated name not persisted: expected %q, got %q", "Updated crumb name", updated.Name)
	}

	// Step 6: Fetch
	entities, err := table.Fetch(map[string]any{"State": types.StateReady})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Fetch expected 1 crumb, got %d", len(entities))
	}

	// Step 7: Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted from SQLite
	_, err = table.Get(id)
	if err != types.ErrNotFound {
		t.Errorf("Get after delete expected ErrNotFound, got %v", err)
	}

	// Verify deleted from JSON
	verifyJSONLNotContains(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)
}

// TestIntegration_TrailCRUD tests the full CRUD lifecycle for Trail entities.
func TestIntegration_TrailCRUD(t *testing.T) {
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

	table, err := backend.GetTable(types.TrailsTable)
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// Create
	trail := &types.Trail{
		State: types.TrailStateActive,
	}
	id, err := table.Set("", trail)
	if err != nil {
		t.Fatalf("Set (create) failed: %v", err)
	}

	// Verify JSON
	verifyJSONLContains(t, filepath.Join(tmpDir, trailsJSONL), "trail_id", id)

	// Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Trail)
	if retrieved.State != types.TrailStateActive {
		t.Errorf("State mismatch: expected %q, got %q", types.TrailStateActive, retrieved.State)
	}

	// Update with ParentCrumbID
	parentID := "parent-crumb-123"
	trail.ParentCrumbID = &parentID
	_, err = table.Set(id, trail)
	if err != nil {
		t.Fatalf("Set (update) failed: %v", err)
	}

	// Verify JSON update
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, trailsJSONL), "trail_id", id)
	if jsonRecord["parent_crumb_id"] != parentID {
		t.Errorf("JSON parent_crumb_id mismatch: expected %q, got %v", parentID, jsonRecord["parent_crumb_id"])
	}

	// Complete the trail
	if err := trail.Complete(); err != nil {
		t.Fatalf("Trail.Complete failed: %v", err)
	}
	_, err = table.Set(id, trail)
	if err != nil {
		t.Fatalf("Set (complete) failed: %v", err)
	}

	// Verify completed state
	entity, _ = table.Get(id)
	completed := entity.(*types.Trail)
	if completed.State != types.TrailStateCompleted {
		t.Errorf("Completed state mismatch: expected %q, got %q", types.TrailStateCompleted, completed.State)
	}
	if completed.CompletedAt == nil {
		t.Error("CompletedAt should be set after Complete")
	}

	// Fetch
	entities, err := table.Fetch(map[string]any{"State": types.TrailStateCompleted})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Fetch expected 1 trail, got %d", len(entities))
	}

	// Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	verifyJSONLNotContains(t, filepath.Join(tmpDir, trailsJSONL), "trail_id", id)
}

// TestIntegration_PropertyCRUD tests the full CRUD lifecycle for Property entities.
func TestIntegration_PropertyCRUD(t *testing.T) {
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

	table, err := backend.GetTable(types.PropertiesTable)
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// Create
	prop := &types.Property{
		Name:        "test_property",
		Description: "A test property",
		ValueType:   types.ValueTypeText,
	}
	id, err := table.Set("", prop)
	if err != nil {
		t.Fatalf("Set (create) failed: %v", err)
	}

	// Verify JSON
	verifyJSONLContains(t, filepath.Join(tmpDir, propertiesJSONL), "property_id", id)

	// Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Property)
	if retrieved.Name != "test_property" {
		t.Errorf("Name mismatch: expected %q, got %q", "test_property", retrieved.Name)
	}
	if retrieved.ValueType != types.ValueTypeText {
		t.Errorf("ValueType mismatch: expected %q, got %q", types.ValueTypeText, retrieved.ValueType)
	}

	// Update
	prop.Description = "Updated description"
	_, err = table.Set(id, prop)
	if err != nil {
		t.Fatalf("Set (update) failed: %v", err)
	}

	// Verify JSON update
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, propertiesJSONL), "property_id", id)
	if jsonRecord["description"] != "Updated description" {
		t.Errorf("JSON description mismatch: expected %q, got %v", "Updated description", jsonRecord["description"])
	}

	// Fetch
	entities, err := table.Fetch(map[string]any{"ValueType": types.ValueTypeText})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Fetch expected 1 property, got %d", len(entities))
	}

	// Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	verifyJSONLNotContains(t, filepath.Join(tmpDir, propertiesJSONL), "property_id", id)
}

// TestIntegration_StashCRUD tests the full CRUD lifecycle for Stash entities.
func TestIntegration_StashCRUD(t *testing.T) {
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

	table, err := backend.GetTable(types.StashesTable)
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// Create counter stash
	stash := &types.Stash{
		Name:      "test_counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
		Version:   1,
	}
	id, err := table.Set("", stash)
	if err != nil {
		t.Fatalf("Set (create) failed: %v", err)
	}

	// Verify JSON
	verifyJSONLContains(t, filepath.Join(tmpDir, stashesJSONL), "stash_id", id)

	// Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.Name != "test_counter" {
		t.Errorf("Name mismatch: expected %q, got %q", "test_counter", retrieved.Name)
	}
	if retrieved.StashType != types.StashTypeCounter {
		t.Errorf("StashType mismatch: expected %q, got %q", types.StashTypeCounter, retrieved.StashType)
	}

	// Update with trail scope
	trailID := "trail-scope-123"
	stash.TrailID = &trailID
	stash.Version = 2
	_, err = table.Set(id, stash)
	if err != nil {
		t.Fatalf("Set (update) failed: %v", err)
	}

	// Verify JSON update
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, stashesJSONL), "stash_id", id)
	if jsonRecord["trail_id"] != trailID {
		t.Errorf("JSON trail_id mismatch: expected %q, got %v", trailID, jsonRecord["trail_id"])
	}

	// Test Increment
	newVal, err := stash.Increment(10)
	if err != nil {
		t.Fatalf("Stash.Increment failed: %v", err)
	}
	if newVal != 10 {
		t.Errorf("Increment result expected 10, got %d", newVal)
	}
	_, err = table.Set(id, stash)
	if err != nil {
		t.Fatalf("Set (after increment) failed: %v", err)
	}

	// Fetch
	entities, err := table.Fetch(map[string]any{"StashType": types.StashTypeCounter})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Fetch expected 1 stash, got %d", len(entities))
	}

	// Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	verifyJSONLNotContains(t, filepath.Join(tmpDir, stashesJSONL), "stash_id", id)
}

// TestIntegration_MetadataCRUD tests the full CRUD lifecycle for Metadata entities.
func TestIntegration_MetadataCRUD(t *testing.T) {
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

	table, err := backend.GetTable(types.MetadataTable)
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// Create
	meta := &types.Metadata{
		TableName: "comments",
		CrumbID:   "crumb-123",
		Content:   "This is a test comment",
	}
	id, err := table.Set("", meta)
	if err != nil {
		t.Fatalf("Set (create) failed: %v", err)
	}

	// Verify JSON
	verifyJSONLContains(t, filepath.Join(tmpDir, metadataJSONL), "metadata_id", id)

	// Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Metadata)
	if retrieved.TableName != "comments" {
		t.Errorf("TableName mismatch: expected %q, got %q", "comments", retrieved.TableName)
	}
	if retrieved.Content != "This is a test comment" {
		t.Errorf("Content mismatch: expected %q, got %q", "This is a test comment", retrieved.Content)
	}

	// Update
	meta.Content = "Updated comment content"
	propID := "property-456"
	meta.PropertyID = &propID
	_, err = table.Set(id, meta)
	if err != nil {
		t.Fatalf("Set (update) failed: %v", err)
	}

	// Verify JSON update
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, metadataJSONL), "metadata_id", id)
	if jsonRecord["content"] != "Updated comment content" {
		t.Errorf("JSON content mismatch: expected %q, got %v", "Updated comment content", jsonRecord["content"])
	}
	if jsonRecord["property_id"] != propID {
		t.Errorf("JSON property_id mismatch: expected %q, got %v", propID, jsonRecord["property_id"])
	}

	// Fetch
	entities, err := table.Fetch(map[string]any{"CrumbID": "crumb-123"})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Fetch expected 1 metadata, got %d", len(entities))
	}

	// Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	verifyJSONLNotContains(t, filepath.Join(tmpDir, metadataJSONL), "metadata_id", id)
}

// TestIntegration_LinkCRUD tests the full CRUD lifecycle for Link entities.
func TestIntegration_LinkCRUD(t *testing.T) {
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

	table, err := backend.GetTable(types.LinksTable)
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// Create belongs_to link
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   "crumb-123",
		ToID:     "trail-456",
	}
	id, err := table.Set("", link)
	if err != nil {
		t.Fatalf("Set (create) failed: %v", err)
	}

	// Verify JSON
	verifyJSONLContains(t, filepath.Join(tmpDir, linksJSONL), "link_id", id)

	// Get
	entity, err := table.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Link)
	if retrieved.LinkType != types.LinkTypeBelongsTo {
		t.Errorf("LinkType mismatch: expected %q, got %q", types.LinkTypeBelongsTo, retrieved.LinkType)
	}
	if retrieved.FromID != "crumb-123" {
		t.Errorf("FromID mismatch: expected %q, got %q", "crumb-123", retrieved.FromID)
	}
	if retrieved.ToID != "trail-456" {
		t.Errorf("ToID mismatch: expected %q, got %q", "trail-456", retrieved.ToID)
	}

	// Update (change link target)
	link.ToID = "trail-789"
	_, err = table.Set(id, link)
	if err != nil {
		t.Fatalf("Set (update) failed: %v", err)
	}

	// Verify JSON update
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, linksJSONL), "link_id", id)
	if jsonRecord["to_id"] != "trail-789" {
		t.Errorf("JSON to_id mismatch: expected %q, got %v", "trail-789", jsonRecord["to_id"])
	}

	// Create a child_of link
	childLink := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   "child-crumb",
		ToID:     "parent-crumb",
	}
	childID, err := table.Set("", childLink)
	if err != nil {
		t.Fatalf("Set (create child_of) failed: %v", err)
	}

	// Fetch by type
	entities, err := table.Fetch(map[string]any{"LinkType": types.LinkTypeBelongsTo})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Fetch expected 1 belongs_to link, got %d", len(entities))
	}

	// Fetch all
	allEntities, err := table.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch all failed: %v", err)
	}
	if len(allEntities) != 2 {
		t.Errorf("Fetch all expected 2 links, got %d", len(allEntities))
	}

	// Delete
	err = table.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	err = table.Delete(childID)
	if err != nil {
		t.Fatalf("Delete child_of failed: %v", err)
	}

	verifyJSONLNotContains(t, filepath.Join(tmpDir, linksJSONL), "link_id", id)
	verifyJSONLNotContains(t, filepath.Join(tmpDir, linksJSONL), "link_id", childID)
}

// TestIntegration_AttachDetachLifecycle tests the cupboard lifecycle.
func TestIntegration_AttachDetachLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	backend := NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	// Operations fail before Attach
	_, err := backend.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("GetTable before Attach expected ErrCupboardDetached, got %v", err)
	}

	// Attach
	if err := backend.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	// Operations succeed after Attach
	table, err := backend.GetTable(types.CrumbsTable)
	if err != nil {
		t.Fatalf("GetTable after Attach failed: %v", err)
	}

	// Create some data
	crumb := &types.Crumb{Name: "Lifecycle test", State: types.StateDraft}
	id, _ := table.Set("", crumb)

	// Double Attach returns ErrAlreadyAttached
	err = backend.Attach(config)
	if err != types.ErrAlreadyAttached {
		t.Errorf("Double Attach expected ErrAlreadyAttached, got %v", err)
	}

	// Detach
	if err := backend.Detach(); err != nil {
		t.Fatalf("Detach failed: %v", err)
	}

	// Operations fail after Detach
	_, err = backend.GetTable(types.CrumbsTable)
	if err != types.ErrCupboardDetached {
		t.Errorf("GetTable after Detach expected ErrCupboardDetached, got %v", err)
	}

	// Detach is idempotent
	err = backend.Detach()
	if err != nil {
		t.Errorf("Second Detach should be idempotent, got %v", err)
	}

	// Re-attach and verify data persists
	backend2 := NewBackend()
	if err := backend2.Attach(config); err != nil {
		t.Fatalf("Re-attach failed: %v", err)
	}
	defer backend2.Detach()

	table2, _ := backend2.GetTable(types.CrumbsTable)
	entity, err := table2.Get(id)
	if err != nil {
		t.Fatalf("Get after re-attach failed: %v", err)
	}
	loaded := entity.(*types.Crumb)
	if loaded.Name != "Lifecycle test" {
		t.Errorf("Data not persisted across restart: expected %q, got %q", "Lifecycle test", loaded.Name)
	}
}

// TestIntegration_ErrorCases tests error handling scenarios.
func TestIntegration_ErrorCases(t *testing.T) {
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

	t.Run("Get non-existent ID", func(t *testing.T) {
		table, _ := backend.GetTable(types.CrumbsTable)
		_, err := table.Get("non-existent-id")
		if err != types.ErrNotFound {
			t.Errorf("Get non-existent expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Delete non-existent ID", func(t *testing.T) {
		table, _ := backend.GetTable(types.CrumbsTable)
		err := table.Delete("non-existent-id")
		if err != types.ErrNotFound {
			t.Errorf("Delete non-existent expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Get with empty ID", func(t *testing.T) {
		table, _ := backend.GetTable(types.CrumbsTable)
		_, err := table.Get("")
		if err != types.ErrInvalidID {
			t.Errorf("Get with empty ID expected ErrInvalidID, got %v", err)
		}
	})

	t.Run("Delete with empty ID", func(t *testing.T) {
		table, _ := backend.GetTable(types.CrumbsTable)
		err := table.Delete("")
		if err != types.ErrInvalidID {
			t.Errorf("Delete with empty ID expected ErrInvalidID, got %v", err)
		}
	})

	t.Run("Set wrong entity type", func(t *testing.T) {
		table, _ := backend.GetTable(types.CrumbsTable)
		_, err := table.Set("", &types.Trail{State: types.TrailStateActive})
		if err != types.ErrInvalidData {
			t.Errorf("Set wrong type expected ErrInvalidData, got %v", err)
		}
	})

	t.Run("GetTable unknown table", func(t *testing.T) {
		_, err := backend.GetTable("unknown_table")
		if err != types.ErrTableNotFound {
			t.Errorf("GetTable unknown expected ErrTableNotFound, got %v", err)
		}
	})

	t.Run("Operations after Detach on table", func(t *testing.T) {
		// Get table reference before detach
		table, _ := backend.GetTable(types.CrumbsTable)

		// Detach
		backend.Detach()

		// Operations on cached table reference should fail
		_, err := table.Get("some-id")
		if err != types.ErrCupboardDetached {
			t.Errorf("Get after Detach expected ErrCupboardDetached, got %v", err)
		}

		_, err = table.Set("", &types.Crumb{Name: "Test", State: types.StateDraft})
		if err != types.ErrCupboardDetached {
			t.Errorf("Set after Detach expected ErrCupboardDetached, got %v", err)
		}

		err = table.Delete("some-id")
		if err != types.ErrCupboardDetached {
			t.Errorf("Delete after Detach expected ErrCupboardDetached, got %v", err)
		}

		_, err = table.Fetch(nil)
		if err != types.ErrCupboardDetached {
			t.Errorf("Fetch after Detach expected ErrCupboardDetached, got %v", err)
		}
	})
}

// TestIntegration_StashLockOperations tests lock stash acquire/release.
func TestIntegration_StashLockOperations(t *testing.T) {
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

	// Create lock stash
	stash := &types.Stash{
		Name:      "deploy_lock",
		StashType: types.StashTypeLock,
		Version:   1,
	}
	id, err := table.Set("", stash)
	if err != nil {
		t.Fatalf("Set (create lock) failed: %v", err)
	}

	// Acquire lock
	err = stash.Acquire("holder-1")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	_, err = table.Set(id, stash)
	if err != nil {
		t.Fatalf("Set (after acquire) failed: %v", err)
	}

	// Verify JSON has lock data
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, stashesJSONL), "stash_id", id)
	value := jsonRecord["value"].(map[string]any)
	if value["holder"] != "holder-1" {
		t.Errorf("JSON holder mismatch: expected %q, got %v", "holder-1", value["holder"])
	}

	// Acquire by same holder is reentrant
	err = stash.Acquire("holder-1")
	if err != nil {
		t.Errorf("Reentrant acquire should succeed, got %v", err)
	}

	// Acquire by different holder fails
	err = stash.Acquire("holder-2")
	if err != types.ErrLockHeld {
		t.Errorf("Acquire by different holder expected ErrLockHeld, got %v", err)
	}

	// Release by wrong holder fails
	err = stash.Release("holder-2")
	if err != types.ErrNotLockHolder {
		t.Errorf("Release by wrong holder expected ErrNotLockHolder, got %v", err)
	}

	// Release by correct holder succeeds
	err = stash.Release("holder-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	_, err = table.Set(id, stash)
	if err != nil {
		t.Fatalf("Set (after release) failed: %v", err)
	}

	// Verify lock is released (value is nil)
	if stash.Value != nil {
		t.Errorf("Lock value should be nil after release, got %v", stash.Value)
	}
}

// TestIntegration_DataPersistsAcrossRestart verifies JSON persistence across backend restart.
func TestIntegration_DataPersistsAcrossRestart(t *testing.T) {
	tmpDir := t.TempDir()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	// First session: create data
	backend1 := NewBackend()
	if err := backend1.Attach(config); err != nil {
		t.Fatalf("First attach failed: %v", err)
	}

	crumbTable, _ := backend1.GetTable(types.CrumbsTable)
	trailTable, _ := backend1.GetTable(types.TrailsTable)
	propTable, _ := backend1.GetTable(types.PropertiesTable)
	stashTable, _ := backend1.GetTable(types.StashesTable)
	metaTable, _ := backend1.GetTable(types.MetadataTable)
	linkTable, _ := backend1.GetTable(types.LinksTable)

	crumb := &types.Crumb{Name: "Persistent crumb", State: types.StateDraft}
	crumbID, _ := crumbTable.Set("", crumb)

	trail := &types.Trail{State: types.TrailStateActive}
	trailID, _ := trailTable.Set("", trail)

	prop := &types.Property{Name: "persist_prop", ValueType: types.ValueTypeText}
	propID, _ := propTable.Set("", prop)

	stash := &types.Stash{Name: "persist_stash", StashType: types.StashTypeContext, Value: map[string]any{"key": "value"}, Version: 1}
	stashID, _ := stashTable.Set("", stash)

	meta := &types.Metadata{TableName: "notes", CrumbID: crumbID, Content: "Persistent note"}
	metaID, _ := metaTable.Set("", meta)

	link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
	linkID, _ := linkTable.Set("", link)

	if err := backend1.Detach(); err != nil {
		t.Fatalf("First detach failed: %v", err)
	}

	// Second session: verify data persists
	backend2 := NewBackend()
	if err := backend2.Attach(config); err != nil {
		t.Fatalf("Second attach failed: %v", err)
	}
	defer backend2.Detach()

	// Verify crumb
	crumbTable2, _ := backend2.GetTable(types.CrumbsTable)
	entity, err := crumbTable2.Get(crumbID)
	if err != nil {
		t.Fatalf("Get crumb after restart failed: %v", err)
	}
	if entity.(*types.Crumb).Name != "Persistent crumb" {
		t.Errorf("Crumb not persisted correctly")
	}

	// Verify trail
	trailTable2, _ := backend2.GetTable(types.TrailsTable)
	entity, err = trailTable2.Get(trailID)
	if err != nil {
		t.Fatalf("Get trail after restart failed: %v", err)
	}
	if entity.(*types.Trail).State != types.TrailStateActive {
		t.Errorf("Trail not persisted correctly")
	}

	// Verify property
	propTable2, _ := backend2.GetTable(types.PropertiesTable)
	entity, err = propTable2.Get(propID)
	if err != nil {
		t.Fatalf("Get property after restart failed: %v", err)
	}
	if entity.(*types.Property).Name != "persist_prop" {
		t.Errorf("Property not persisted correctly")
	}

	// Verify stash
	stashTable2, _ := backend2.GetTable(types.StashesTable)
	entity, err = stashTable2.Get(stashID)
	if err != nil {
		t.Fatalf("Get stash after restart failed: %v", err)
	}
	loadedStash := entity.(*types.Stash)
	if loadedStash.Name != "persist_stash" {
		t.Errorf("Stash not persisted correctly")
	}

	// Verify metadata
	metaTable2, _ := backend2.GetTable(types.MetadataTable)
	entity, err = metaTable2.Get(metaID)
	if err != nil {
		t.Fatalf("Get metadata after restart failed: %v", err)
	}
	if entity.(*types.Metadata).Content != "Persistent note" {
		t.Errorf("Metadata not persisted correctly")
	}

	// Verify link
	linkTable2, _ := backend2.GetTable(types.LinksTable)
	entity, err = linkTable2.Get(linkID)
	if err != nil {
		t.Fatalf("Get link after restart failed: %v", err)
	}
	if entity.(*types.Link).LinkType != types.LinkTypeBelongsTo {
		t.Errorf("Link not persisted correctly")
	}
}

// TestIntegration_TimestampPersistence verifies timestamp fields are correctly persisted.
func TestIntegration_TimestampPersistence(t *testing.T) {
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

	// Create crumb with specific timestamp
	now := time.Now().Truncate(time.Second)
	crumb := &types.Crumb{
		Name:      "Timestamp test",
		State:     types.StateDraft,
		CreatedAt: now,
	}
	id, _ := table.Set("", crumb)

	// Get and verify CreatedAt is preserved
	entity, _ := table.Get(id)
	retrieved := entity.(*types.Crumb)
	if !retrieved.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt not preserved: expected %v, got %v", now, retrieved.CreatedAt)
	}
	if retrieved.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set automatically")
	}

	// Verify JSON has correct timestamp format (RFC3339)
	jsonRecord := readJSONLRecord(t, filepath.Join(tmpDir, crumbsJSONL), "crumb_id", id)
	createdAtStr := jsonRecord["created_at"].(string)
	parsedTime, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		t.Errorf("JSON created_at is not valid RFC3339: %v", err)
	}
	if !parsedTime.Equal(now) {
		t.Errorf("JSON created_at mismatch: expected %v, got %v", now, parsedTime)
	}
}

// Helper functions for JSON verification

// verifyJSONLContains checks that a JSONL file contains a record with the given ID.
func verifyJSONLContains(t *testing.T, path, idField, id string) {
	t.Helper()
	records := readAllJSONLRecords(t, path)
	for _, record := range records {
		if record[idField] == id {
			return
		}
	}
	t.Errorf("JSONL file %s does not contain record with %s=%s", path, idField, id)
}

// verifyJSONLNotContains checks that a JSONL file does not contain a record with the given ID.
func verifyJSONLNotContains(t *testing.T, path, idField, id string) {
	t.Helper()
	records := readAllJSONLRecords(t, path)
	for _, record := range records {
		if record[idField] == id {
			t.Errorf("JSONL file %s still contains record with %s=%s", path, idField, id)
			return
		}
	}
}

// readJSONLRecord reads a specific record from a JSONL file by ID.
func readJSONLRecord(t *testing.T, path, idField, id string) map[string]any {
	t.Helper()
	records := readAllJSONLRecords(t, path)
	for _, record := range records {
		if record[idField] == id {
			return record
		}
	}
	t.Fatalf("Record with %s=%s not found in %s", idField, id, path)
	return nil
}

// readAllJSONLRecords reads all records from a JSONL file.
func readAllJSONLRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read JSONL file %s: %v", path, err)
	}

	var records []map[string]any
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			continue // skip malformed lines
		}
		records = append(records, record)
	}
	return records
}

// splitLines splits byte data into lines (handles both \n and \r\n).
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	var currentLine []byte
	for _, b := range data {
		if b == '\n' {
			if len(currentLine) > 0 && currentLine[len(currentLine)-1] == '\r' {
				currentLine = currentLine[:len(currentLine)-1]
			}
			lines = append(lines, currentLine)
			currentLine = nil
		} else {
			currentLine = append(currentLine, b)
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}
	return lines
}
