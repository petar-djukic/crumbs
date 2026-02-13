// Integration tests for regeneration compatibility: JSONL files with unknown
// fields load without error, crumb data survives simulated regeneration cycles,
// new properties work with existing data, and JSONL files remain valid after
// write-back with a new struct version.
// Implements: test-rel02.0-uc002-regeneration-compatibility;
//             prd002-sqlite-backend R4, R4.2, R7.2; prd001-cupboard-core R4, R5.
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRawJSONL writes raw JSON lines to a JSONL file. Each entry in lines is
// a map that gets marshaled to one line.
func writeRawJSONL(t *testing.T, path string, lines []map[string]any) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	for _, m := range lines {
		data, err := json.Marshal(m)
		require.NoError(t, err)
		_, err = f.Write(data)
		require.NoError(t, err)
		_, err = f.WriteString("\n")
		require.NoError(t, err)
	}
}

// appendRawJSONL appends raw JSON lines to an existing JSONL file.
func appendRawJSONL(t *testing.T, path string, lines []map[string]any) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	for _, m := range lines {
		data, err := json.Marshal(m)
		require.NoError(t, err)
		_, err = f.Write(data)
		require.NoError(t, err)
		_, err = f.WriteString("\n")
		require.NoError(t, err)
	}
}

// createEmptyJSONLFiles creates all standard JSONL files as empty in the
// given directory so that Attach does not fail on missing files.
func createEmptyJSONLFiles(t *testing.T, dataDir string) {
	t.Helper()
	for _, name := range []string{
		"crumbs.jsonl", "trails.jsonl", "links.jsonl",
		"properties.jsonl", "categories.jsonl", "crumb_properties.jsonl",
		"metadata.jsonl", "stashes.jsonl", "stash_history.jsonl",
	} {
		f, err := os.Create(filepath.Join(dataDir, name))
		require.NoError(t, err)
		f.Close()
	}
}

// TestRegenerationCompatibility groups all regeneration compatibility tests
// that validate S1-S8 from the use case specification.
func TestRegenerationCompatibility(t *testing.T) {
	t.Run("S1_S2_unknown_fields_load_without_error", testUnknownFieldsLoadWithoutError)
	t.Run("S3_S4_data_survives_regeneration_cycle", testDataSurvivesRegenerationCycle)
	t.Run("S5_new_property_on_existing_data", testNewPropertyOnExistingData)
	t.Run("S6_removed_field_still_loads", testRemovedFieldStillLoads)
	t.Run("S7_concurrent_access_two_instances", testConcurrentAccessTwoInstances)
	t.Run("S8_writeback_produces_valid_JSONL", testWritebackProducesValidJSONL)
	t.Run("malformed_lines_skipped", testMalformedLinesSkipped)
	t.Run("missing_jsonl_graceful", testMissingJSONLGraceful)
}

// testUnknownFieldsLoadWithoutError validates S1 and S2: JSONL files with
// unknown fields load without error; the loaded crumbs contain correct values.
func testUnknownFieldsLoadWithoutError(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Format(time.RFC3339)

	// Write crumbs.jsonl with extra fields that a future generation might add.
	writeRawJSONL(t, filepath.Join(dataDir, "crumbs.jsonl"), []map[string]any{
		{
			"crumb_id":     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"name":         "Crumb with unknown fields",
			"state":        "draft",
			"created_at":   now,
			"updated_at":   now,
			"future_field": "this field does not exist in the current struct",
			"priority_v2":  42,
			"tags":         []string{"alpha", "beta"},
		},
		{
			"crumb_id":        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			"name":            "Another future crumb",
			"state":           "ready",
			"created_at":      now,
			"updated_at":      now,
			"nested_metadata": map[string]any{"version": 2, "source": "gen-n+1"},
		},
	})

	// Create remaining empty JSONL files.
	for _, name := range []string{
		"trails.jsonl", "links.jsonl", "properties.jsonl",
		"categories.jsonl", "crumb_properties.jsonl",
		"metadata.jsonl", "stashes.jsonl", "stash_history.jsonl",
	} {
		f, err := os.Create(filepath.Join(dataDir, name))
		require.NoError(t, err)
		f.Close()
	}

	backend := sqlite.NewBackend()
	err := backend.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "Attach must succeed with unknown fields in JSONL")
	defer backend.Detach()

	tbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Verify both crumbs loaded correctly.
	crumbs, err := tbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, crumbs, 2, "both crumbs with unknown fields must load")

	// Verify individual crumb data is correct.
	got1, err := tbl.Get("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	require.NoError(t, err)
	c1 := got1.(*types.Crumb)
	assert.Equal(t, "Crumb with unknown fields", c1.Name)
	assert.Equal(t, types.StateDraft, c1.State)

	got2, err := tbl.Get("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	require.NoError(t, err)
	c2 := got2.(*types.Crumb)
	assert.Equal(t, "Another future crumb", c2.Name)
	assert.Equal(t, types.StateReady, c2.State)
}

// testDataSurvivesRegenerationCycle validates S3 and S4: data created by
// generation N is written to JSONL, and a fresh backend (simulating N+1)
// reads it back correctly.
func testDataSurvivesRegenerationCycle(t *testing.T) {
	dataDir := t.TempDir()

	// Generation N: create crumbs.
	backendN := sqlite.NewBackend()
	err := backendN.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)

	tblN, err := backendN.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id1, err := tblN.Set("", &types.Crumb{Name: "Compat crumb alpha"})
	require.NoError(t, err)
	id2, err := tblN.Set("", &types.Crumb{Name: "Compat crumb beta"})
	require.NoError(t, err)
	id3, err := tblN.Set("", &types.Crumb{Name: "Compat crumb gamma"})
	require.NoError(t, err)

	// Update one crumb's state.
	_, err = tblN.Set(id2, &types.Crumb{
		CrumbID: id2, Name: "Compat crumb beta", State: types.StateReady,
	})
	require.NoError(t, err)

	// Verify JSONL persisted.
	jsonlPath := filepath.Join(dataDir, "crumbs.jsonl")
	lines := readJSONLLines(t, jsonlPath)
	assert.Len(t, lines, 3, "JSONL must contain all three crumbs")

	// Detach generation N (simulates end of generation N).
	require.NoError(t, backendN.Detach())

	// Generation N+1: fresh backend attaches to same data directory.
	backendN1 := sqlite.NewBackend()
	err = backendN1.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "Generation N+1 must attach to generation N data")
	defer backendN1.Detach()

	tblN1, err := backendN1.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Verify all crumbs readable.
	allCrumbs, err := tblN1.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allCrumbs, 3, "generation N+1 must read all generation N crumbs")

	// Verify individual crumb data.
	got1, err := tblN1.Get(id1)
	require.NoError(t, err)
	assert.Equal(t, "Compat crumb alpha", got1.(*types.Crumb).Name)
	assert.Equal(t, types.StateDraft, got1.(*types.Crumb).State)

	got2, err := tblN1.Get(id2)
	require.NoError(t, err)
	assert.Equal(t, "Compat crumb beta", got2.(*types.Crumb).Name)
	assert.Equal(t, types.StateReady, got2.(*types.Crumb).State)

	got3, err := tblN1.Get(id3)
	require.NoError(t, err)
	assert.Equal(t, "Compat crumb gamma", got3.(*types.Crumb).Name)
}

// testNewPropertyOnExistingData validates S5: adding a new property to
// existing JSONL data works. A fresh backend attaches to data that already
// has crumbs, and creating a new property backfills crumb_properties.
func testNewPropertyOnExistingData(t *testing.T) {
	dataDir := t.TempDir()

	// Generation N: create crumbs.
	backendN := sqlite.NewBackend()
	err := backendN.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)

	crumbsTbl, err := backendN.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id1, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb for property test"})
	require.NoError(t, err)
	id2, err := crumbsTbl.Set("", &types.Crumb{Name: "Another crumb for property test"})
	require.NoError(t, err)

	require.NoError(t, backendN.Detach())

	// Generation N+1: attach and add a new custom property.
	backendN1 := sqlite.NewBackend()
	err = backendN1.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)
	defer backendN1.Detach()

	propsTbl, err := backendN1.GetTable(types.TableProperties)
	require.NoError(t, err)

	// Create a new text property; this should backfill crumb_properties.
	newPropID, err := propsTbl.Set("", &types.Property{
		Name:        "regen_test_prop",
		ValueType:   types.ValueTypeText,
		Description: "Test property for regeneration",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, newPropID)

	// Verify the property persisted to properties.jsonl.
	propsPath := filepath.Join(dataDir, "properties.jsonl")
	propLines := readJSONLLines(t, propsPath)
	found := false
	for _, line := range propLines {
		if name, ok := line["name"].(string); ok && name == "regen_test_prop" {
			found = true
			break
		}
	}
	assert.True(t, found, "regen_test_prop must be in properties.jsonl")

	// Verify crumb_properties.jsonl was backfilled for both crumbs.
	cpPath := filepath.Join(dataDir, "crumb_properties.jsonl")
	cpLines := readJSONLLines(t, cpPath)
	backfilledCount := 0
	for _, line := range cpLines {
		propID, _ := line["property_id"].(string)
		crumbID, _ := line["crumb_id"].(string)
		if propID == newPropID && (crumbID == id1 || crumbID == id2) {
			backfilledCount++
		}
	}
	assert.Equal(t, 2, backfilledCount, "new property must backfill to both existing crumbs")
}

// testRemovedFieldStillLoads validates S6: JSONL written by a generation
// that had more fields can be loaded by a generation that has fewer fields.
// The loader extracts only mapped columns and ignores the rest.
func testRemovedFieldStillLoads(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Format(time.RFC3339)

	// Simulate JSONL from an older generation that had extra columns
	// (e.g., "description" directly on the crumb, or "priority" as a field).
	writeRawJSONL(t, filepath.Join(dataDir, "crumbs.jsonl"), []map[string]any{
		{
			"crumb_id":    "cccccccc-cccc-cccc-cccc-cccccccccccc",
			"name":        "Old gen crumb with extra fields",
			"state":       "draft",
			"created_at":  now,
			"updated_at":  now,
			"description": "This field was removed in the new generation",
			"priority":    "high",
			"assignee":    "alice",
		},
	})

	// Write properties.jsonl with an extra field on the property record.
	writeRawJSONL(t, filepath.Join(dataDir, "properties.jsonl"), []map[string]any{
		{
			"property_id":  "dddddddd-dddd-dddd-dddd-dddddddddddd",
			"name":         "old_property",
			"description":  "An old property definition",
			"value_type":   "text",
			"created_at":   now,
			"deprecated":   true,
			"removed_field": "should be ignored",
		},
	})

	// Create remaining empty JSONL files.
	for _, name := range []string{
		"trails.jsonl", "links.jsonl", "categories.jsonl",
		"crumb_properties.jsonl", "metadata.jsonl",
		"stashes.jsonl", "stash_history.jsonl",
	} {
		f, err := os.Create(filepath.Join(dataDir, name))
		require.NoError(t, err)
		f.Close()
	}

	backend := sqlite.NewBackend()
	err := backend.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "Attach must succeed with old-gen JSONL containing removed fields")
	defer backend.Detach()

	// Verify crumb loaded correctly despite extra fields.
	tbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	got, err := tbl.Get("cccccccc-cccc-cccc-cccc-cccccccccccc")
	require.NoError(t, err)
	crumb := got.(*types.Crumb)
	assert.Equal(t, "Old gen crumb with extra fields", crumb.Name)
	assert.Equal(t, types.StateDraft, crumb.State)

	// Verify property loaded despite extra fields.
	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)

	gotProp, err := propsTbl.Get("dddddddd-dddd-dddd-dddd-dddddddddddd")
	require.NoError(t, err)
	prop := gotProp.(*types.Property)
	assert.Equal(t, "old_property", prop.Name)
	assert.Equal(t, types.ValueTypeText, prop.ValueType)
}

// testConcurrentAccessTwoInstances validates S7: two cupboard instances can
// operate on the same data directory sequentially without corruption. True
// concurrent write access to the same SQLite file is not supported, but
// sequential attach-operate-detach cycles must preserve data integrity.
func testConcurrentAccessTwoInstances(t *testing.T) {
	dataDir := t.TempDir()

	// Instance A: create crumbs.
	backendA := sqlite.NewBackend()
	err := backendA.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)

	tblA, err := backendA.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	idA, err := tblA.Set("", &types.Crumb{Name: "Instance A crumb"})
	require.NoError(t, err)

	require.NoError(t, backendA.Detach())

	// Instance B: attach, read A's data, add more.
	backendB := sqlite.NewBackend()
	err = backendB.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)

	tblB, err := backendB.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// B reads A's crumb.
	gotA, err := tblB.Get(idA)
	require.NoError(t, err)
	assert.Equal(t, "Instance A crumb", gotA.(*types.Crumb).Name)

	// B creates a new crumb.
	idB, err := tblB.Set("", &types.Crumb{Name: "Instance B crumb"})
	require.NoError(t, err)

	// B modifies A's crumb.
	_, err = tblB.Set(idA, &types.Crumb{
		CrumbID: idA, Name: "Instance A crumb modified by B", State: types.StateReady,
	})
	require.NoError(t, err)

	require.NoError(t, backendB.Detach())

	// Instance A (re-attached): verify all data including B's changes.
	backendA2 := sqlite.NewBackend()
	err = backendA2.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)
	defer backendA2.Detach()

	tblA2, err := backendA2.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	allCrumbs, err := tblA2.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allCrumbs, 2, "both instances' crumbs must be present")

	gotAMod, err := tblA2.Get(idA)
	require.NoError(t, err)
	assert.Equal(t, "Instance A crumb modified by B", gotAMod.(*types.Crumb).Name)
	assert.Equal(t, types.StateReady, gotAMod.(*types.Crumb).State)

	gotB, err := tblA2.Get(idB)
	require.NoError(t, err)
	assert.Equal(t, "Instance B crumb", gotB.(*types.Crumb).Name)
}

// testWritebackProducesValidJSONL validates S8: after write-back with a
// new struct version, JSONL files remain valid. Data written by generation
// N+1 can be loaded by a fresh backend.
func testWritebackProducesValidJSONL(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Format(time.RFC3339)

	// Pre-seed crumbs.jsonl with records that have unknown future fields.
	writeRawJSONL(t, filepath.Join(dataDir, "crumbs.jsonl"), []map[string]any{
		{
			"crumb_id":     "11111111-1111-1111-1111-111111111111",
			"name":         "Pre-existing crumb",
			"state":        "draft",
			"created_at":   now,
			"updated_at":   now,
			"future_field": "should survive load but get stripped on write-back",
		},
	})

	// Create remaining empty JSONL files.
	for _, name := range []string{
		"trails.jsonl", "links.jsonl", "properties.jsonl",
		"categories.jsonl", "crumb_properties.jsonl",
		"metadata.jsonl", "stashes.jsonl", "stash_history.jsonl",
	} {
		f, err := os.Create(filepath.Join(dataDir, name))
		require.NoError(t, err)
		f.Close()
	}

	// Generation N+1 loads and modifies the data.
	backend := sqlite.NewBackend()
	err := backend.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)

	tbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	// Modify the pre-existing crumb (triggers JSONL write-back).
	_, err = tbl.Set("11111111-1111-1111-1111-111111111111", &types.Crumb{
		CrumbID: "11111111-1111-1111-1111-111111111111",
		Name:    "Pre-existing crumb updated",
		State:   types.StatePending,
	})
	require.NoError(t, err)

	// Add a new crumb (triggers JSONL write-back).
	newID, err := tbl.Set("", &types.Crumb{Name: "New generation crumb"})
	require.NoError(t, err)

	require.NoError(t, backend.Detach())

	// Verify the written JSONL is valid by loading it with a fresh backend.
	backend2 := sqlite.NewBackend()
	err = backend2.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "fresh backend must load JSONL written by previous generation")
	defer backend2.Detach()

	tbl2, err := backend2.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	allCrumbs, err := tbl2.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allCrumbs, 2, "both crumbs must survive write-back and reload")

	got1, err := tbl2.Get("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, "Pre-existing crumb updated", got1.(*types.Crumb).Name)
	assert.Equal(t, types.StatePending, got1.(*types.Crumb).State)

	got2, err := tbl2.Get(newID)
	require.NoError(t, err)
	assert.Equal(t, "New generation crumb", got2.(*types.Crumb).Name)

	// Verify the JSONL lines are well-formed JSON.
	jsonlPath := filepath.Join(dataDir, "crumbs.jsonl")
	lines := readJSONLLines(t, jsonlPath)
	assert.Len(t, lines, 2, "JSONL must contain exactly two records after write-back")
	for i, line := range lines {
		_, hasID := line["crumb_id"]
		_, hasName := line["name"]
		_, hasState := line["state"]
		assert.True(t, hasID, "line %d must have crumb_id", i)
		assert.True(t, hasName, "line %d must have name", i)
		assert.True(t, hasState, "line %d must have state", i)
	}
}

// testMalformedLinesSkipped validates that malformed JSON lines in JSONL are
// skipped without causing load failure (prd002-sqlite-backend R4.2).
func testMalformedLinesSkipped(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Format(time.RFC3339)

	// Write crumbs.jsonl with a valid record, a malformed line, and another
	// valid record.
	crumbsPath := filepath.Join(dataDir, "crumbs.jsonl")
	f, err := os.Create(crumbsPath)
	require.NoError(t, err)

	validLine1, _ := json.Marshal(map[string]any{
		"crumb_id": "22222222-2222-2222-2222-222222222222",
		"name":     "Valid before corrupt",
		"state":    "draft",
		"created_at": now,
		"updated_at": now,
	})
	validLine2, _ := json.Marshal(map[string]any{
		"crumb_id": "33333333-3333-3333-3333-333333333333",
		"name":     "Valid after corrupt",
		"state":    "ready",
		"created_at": now,
		"updated_at": now,
	})

	fmt.Fprintf(f, "%s\n", validLine1)
	fmt.Fprintf(f, "not valid json at all\n")
	fmt.Fprintf(f, "%s\n", validLine2)
	f.Close()

	// Create remaining empty JSONL files.
	for _, name := range []string{
		"trails.jsonl", "links.jsonl", "properties.jsonl",
		"categories.jsonl", "crumb_properties.jsonl",
		"metadata.jsonl", "stashes.jsonl", "stash_history.jsonl",
	} {
		f, err := os.Create(filepath.Join(dataDir, name))
		require.NoError(t, err)
		f.Close()
	}

	backend := sqlite.NewBackend()
	err = backend.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "Attach must succeed despite malformed JSONL line")
	defer backend.Detach()

	tbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumbs, err := tbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, crumbs, 2, "malformed line must be skipped; both valid crumbs loaded")

	got1, err := tbl.Get("22222222-2222-2222-2222-222222222222")
	require.NoError(t, err)
	assert.Equal(t, "Valid before corrupt", got1.(*types.Crumb).Name)

	got2, err := tbl.Get("33333333-3333-3333-3333-333333333333")
	require.NoError(t, err)
	assert.Equal(t, "Valid after corrupt", got2.(*types.Crumb).Name)
}

// testMissingJSONLGraceful validates that a fresh data directory with empty
// JSONL files results in an empty table, not an error.
func testMissingJSONLGraceful(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	tbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumbs, err := tbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, crumbs, 0, "empty data directory must produce empty table")
}

// TestRegenerationCompatibilityPropertyPersistence validates S8 (property
// aspect): properties and their crumb_properties values created by generation
// N are readable by generation N+1.
func TestRegenerationCompatibilityPropertyPersistence(t *testing.T) {
	dataDir := t.TempDir()

	// Generation N: create crumbs and a custom property.
	backendN := sqlite.NewBackend()
	err := backendN.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err)

	crumbsTbl, err := backendN.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	propsTbl, err := backendN.GetTable(types.TableProperties)
	require.NoError(t, err)

	// Create two crumbs.
	crumbID1, err := crumbsTbl.Set("", &types.Crumb{Name: "Prop test crumb 1"})
	require.NoError(t, err)
	_, err = crumbsTbl.Set("", &types.Crumb{Name: "Prop test crumb 2"})
	require.NoError(t, err)

	// Create a custom property (backfills to both crumbs).
	customPropID, err := propsTbl.Set("", &types.Property{
		Name:        "custom_regen",
		ValueType:   types.ValueTypeText,
		Description: "Custom property for regeneration testing",
	})
	require.NoError(t, err)

	// Verify built-in properties were seeded.
	builtInProps, err := propsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(builtInProps), 6, "built-in properties plus custom must exist")

	// Verify crumb_properties.jsonl has entries.
	cpLines := readJSONLLines(t, filepath.Join(dataDir, "crumb_properties.jsonl"))
	assert.Greater(t, len(cpLines), 0, "crumb_properties.jsonl must have backfill entries")

	require.NoError(t, backendN.Detach())

	// Generation N+1: attach to same data and verify properties survive.
	backendN1 := sqlite.NewBackend()
	err = backendN1.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "generation N+1 must attach to generation N property data")
	defer backendN1.Detach()

	propsTblN1, err := backendN1.GetTable(types.TableProperties)
	require.NoError(t, err)

	// Verify custom property exists.
	gotProp, err := propsTblN1.Get(customPropID)
	require.NoError(t, err)
	prop := gotProp.(*types.Property)
	assert.Equal(t, "custom_regen", prop.Name)
	assert.Equal(t, types.ValueTypeText, prop.ValueType)

	// Verify built-in properties survived.
	allProps, err := propsTblN1.Fetch(nil)
	require.NoError(t, err)
	propNames := make(map[string]bool)
	for _, p := range allProps {
		propNames[p.(*types.Property).Name] = true
	}
	assert.True(t, propNames[types.PropertyPriority], "built-in priority must survive")
	assert.True(t, propNames[types.PropertyType], "built-in type must survive")
	assert.True(t, propNames[types.PropertyDescription], "built-in description must survive")
	assert.True(t, propNames[types.PropertyOwner], "built-in owner must survive")
	assert.True(t, propNames[types.PropertyLabels], "built-in labels must survive")
	assert.True(t, propNames["custom_regen"], "custom property must survive")

	// Verify crumb_properties were loaded (backfill entries from gen N).
	cpLinesN1 := readJSONLLines(t, filepath.Join(dataDir, "crumb_properties.jsonl"))
	backfillFound := false
	for _, line := range cpLinesN1 {
		propID, _ := line["property_id"].(string)
		crumbID, _ := line["crumb_id"].(string)
		if propID == customPropID && crumbID == crumbID1 {
			backfillFound = true
			break
		}
	}
	assert.True(t, backfillFound, "crumb_properties backfill must survive regeneration")
}

// TestRegenerationCompatibilityMultiTableUnknownFields validates that unknown
// fields in non-crumb JSONL files (trails, links, properties, etc.) are also
// ignored, enabling forward compatibility across all table types.
func TestRegenerationCompatibilityMultiTableUnknownFields(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Format(time.RFC3339)

	createEmptyJSONLFiles(t, dataDir)

	// Write trails.jsonl with unknown fields.
	writeRawJSONL(t, filepath.Join(dataDir, "trails.jsonl"), []map[string]any{
		{
			"trail_id":     "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
			"state":        "draft",
			"created_at":   now,
			"completed_at": nil,
			"future_trail_field": "ignored by current loader",
		},
	})

	// Write links.jsonl with unknown fields.
	writeRawJSONL(t, filepath.Join(dataDir, "links.jsonl"), []map[string]any{
		{
			"link_id":    "ffffffff-ffff-ffff-ffff-ffffffffffff",
			"link_type":  "belongs_to",
			"from_id":    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"to_id":      "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
			"created_at": now,
			"weight":     0.5,
			"metadata":   map[string]any{"source": "gen-n+1"},
		},
	})

	backend := sqlite.NewBackend()
	err := backend.Attach(types.Config{Backend: "sqlite", DataDir: dataDir})
	require.NoError(t, err, "Attach must succeed with unknown fields in trails and links JSONL")
	defer backend.Detach()

	// Verify trail loaded.
	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)
	trails, err := trailsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, trails, 1, "trail with unknown fields must load")

	// Verify link loaded.
	linksTbl, err := backend.GetTable(types.TableLinks)
	require.NoError(t, err)
	links, err := linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, links, 1, "link with unknown fields must load")
}
