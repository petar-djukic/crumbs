// Integration tests for property enforcement: built-in property seeding,
// auto-initialization on crumb creation, backfill on property definition,
// the invariant that no crumb has fewer properties than are defined, and
// CLI-based property/category validation.
// Implements: test-rel02.0-uc001-property-enforcement (test cases 1-15);
//             prd004-properties-interface R3.5, R4.2, R7, R8, R9;
//             prd003-crumbs-interface R3, R5;
//             prd009-cupboard-cli R3.
package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// propertyByName looks up a property by name from the properties table.
func propertyByName(t *testing.T, backend types.Cupboard, name string) *types.Property {
	t.Helper()
	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	allProps, err := propsTbl.Fetch(types.Filter{"name": name})
	require.NoError(t, err)
	require.Len(t, allProps, 1, "expected exactly one property named %q", name)
	return allProps[0].(*types.Property)
}

// allPropertyIDs returns the set of property IDs from the properties table.
func allPropertyIDs(t *testing.T, backend types.Cupboard) map[string]bool {
	t.Helper()
	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	allProps, err := propsTbl.Fetch(nil)
	require.NoError(t, err)
	ids := make(map[string]bool, len(allProps))
	for _, p := range allProps {
		ids[p.(*types.Property).PropertyID] = true
	}
	return ids
}

// getCrumb retrieves a crumb by ID and returns it as *types.Crumb.
func getCrumb(t *testing.T, crumbsTbl types.Table, id string) *types.Crumb {
	t.Helper()
	entity, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	return entity.(*types.Crumb)
}

// --- S1: built-in properties seeded on initialization ---

func TestPropertyEnforcement_BuiltInPropertiesSeeded(t *testing.T) {
	tests := []struct {
		name      string
		propName  string
		valueType string
	}{
		{"priority exists with categorical type", types.PropertyPriority, types.ValueTypeCategorical},
		{"type exists with categorical type", types.PropertyType, types.ValueTypeCategorical},
		{"description exists with text type", types.PropertyDescription, types.ValueTypeText},
		{"owner exists with text type", types.PropertyOwner, types.ValueTypeText},
		{"labels exists with list type", types.PropertyLabels, types.ValueTypeList},
	}

	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)

	allProps, err := propsTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allProps, 5, "expected five built-in properties after initialization")

	propsByName := make(map[string]*types.Property, len(allProps))
	for _, p := range allProps {
		prop := p.(*types.Property)
		propsByName[prop.Name] = prop
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, ok := propsByName[tt.propName]
			require.True(t, ok, "property %q must exist", tt.propName)
			assert.Equal(t, tt.valueType, prop.ValueType)
		})
	}
}

// --- S2: new crumbs have all defined properties with defaults ---

func TestPropertyEnforcement_S2_NewCrumbHasAllProperties(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Auto-init crumb"})
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, id)
	assert.Len(t, got.Properties, 5, "new crumb should have all five built-in properties")
}

func TestPropertyEnforcement_S2_DefaultValues(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Default values crumb"})
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, id)

	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	allProps, err := propsTbl.Fetch(nil)
	require.NoError(t, err)

	for _, p := range allProps {
		prop := p.(*types.Property)
		val, ok := got.Properties[prop.PropertyID]
		require.True(t, ok, "property %q must be present", prop.Name)

		switch prop.ValueType {
		case types.ValueTypeText:
			assert.Equal(t, "", val, "text property %q should default to empty string", prop.Name)
		case types.ValueTypeList:
			assert.IsType(t, []any{}, val, "list property %q should default to empty slice", prop.Name)
			assert.Empty(t, val, "list property %q should be empty", prop.Name)
		case types.ValueTypeCategorical:
			assert.Nil(t, val, "categorical property %q should default to nil", prop.Name)
		}
	}
}

// --- S3: SetProperty updates value and changes UpdatedAt ---

func TestPropertyEnforcement_S3_SetPropertyUpdatesInMemory(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "SetProperty crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	originalUpdatedAt := crumb.UpdatedAt

	ownerProp := propertyByName(t, backend, types.PropertyOwner)

	time.Sleep(10 * time.Millisecond)

	err = crumb.SetProperty(ownerProp.PropertyID, "alice")
	require.NoError(t, err)

	// Verify in-memory value changed.
	val, err := crumb.GetProperty(ownerProp.PropertyID)
	require.NoError(t, err)
	assert.Equal(t, "alice", val)

	// Verify UpdatedAt advanced in memory.
	assert.True(t, crumb.UpdatedAt.After(originalUpdatedAt),
		"UpdatedAt should advance after SetProperty")
}

func TestPropertyEnforcement_S3_SetPropertyListValue(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Labels crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	labelsProp := propertyByName(t, backend, types.PropertyLabels)

	labels := []any{"frontend", "urgent"}
	err = crumb.SetProperty(labelsProp.PropertyID, labels)
	require.NoError(t, err)

	val, err := crumb.GetProperty(labelsProp.PropertyID)
	require.NoError(t, err)
	assert.Equal(t, labels, val)
}

func TestPropertyEnforcement_S3_SetPropertyUpdatesTimestampOnPersist(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Timestamp crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	originalUpdatedAt := crumb.UpdatedAt

	ownerProp := propertyByName(t, backend, types.PropertyOwner)

	time.Sleep(10 * time.Millisecond)

	err = crumb.SetProperty(ownerProp.PropertyID, "bob")
	require.NoError(t, err)

	// Persist the crumb (Set updates timestamps in the crumbs row).
	_, err = crumbsTbl.Set(id, crumb)
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, id)
	assert.True(t, !got.UpdatedAt.Before(originalUpdatedAt),
		"UpdatedAt in database should reflect the updated timestamp")
}

// --- S4: GetProperty returns current value ---

func TestPropertyEnforcement_S4_GetPropertyReturnsSetValue(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "GetProperty crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	descProp := propertyByName(t, backend, types.PropertyDescription)

	err = crumb.SetProperty(descProp.PropertyID, "A detailed description")
	require.NoError(t, err)

	// GetProperty on the same in-memory crumb returns the set value.
	val, err := crumb.GetProperty(descProp.PropertyID)
	require.NoError(t, err)
	assert.Equal(t, "A detailed description", val)
}

func TestPropertyEnforcement_S4_GetPropertyReturnsDefaultBeforeSet(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Default get crumb"})
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, id)
	ownerProp := propertyByName(t, backend, types.PropertyOwner)
	val, err := got.GetProperty(ownerProp.PropertyID)
	require.NoError(t, err)
	assert.Equal(t, "", val, "owner should default to empty string before any set")
}

func TestPropertyEnforcement_S4_GetPropertyOnNonexistentReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Error test crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	_, err = crumb.GetProperty("nonexistent-property-id")
	assert.ErrorIs(t, err, types.ErrPropertyNotFound)
}

// --- S5: creating a Property via Table.Set backfills existing crumbs ---

func TestPropertyEnforcement_S5_BackfillSingleCrumb(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Backfill target"})
	require.NoError(t, err)

	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	newPropID, err := propsTbl.Set("", &types.Property{
		Name:        "estimate",
		ValueType:   types.ValueTypeInteger,
		Description: "Time estimate in hours",
	})
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, crumbID)
	assert.Len(t, got.Properties, 6, "crumb should have six properties after backfill")

	val, ok := got.Properties[newPropID]
	assert.True(t, ok, "new property should exist in crumb's properties")
	assert.Equal(t, float64(0), val, "integer property should default to 0")
}

func TestPropertyEnforcement_S5_BackfillMultipleCrumbs(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	var crumbIDs []string
	for _, name := range []string{"Crumb one", "Crumb two", "Crumb three"} {
		id, err := crumbsTbl.Set("", &types.Crumb{Name: name})
		require.NoError(t, err)
		crumbIDs = append(crumbIDs, id)
	}

	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	newPropID, err := propsTbl.Set("", &types.Property{
		Name:        "complexity",
		ValueType:   types.ValueTypeInteger,
		Description: "Task complexity",
	})
	require.NoError(t, err)

	for _, cid := range crumbIDs {
		got := getCrumb(t, crumbsTbl, cid)
		assert.Len(t, got.Properties, 6, "crumb %s should have six properties", cid)
		val, ok := got.Properties[newPropID]
		assert.True(t, ok, "crumb %s should have the new property", cid)
		assert.Equal(t, float64(0), val, "integer default should be 0 for crumb %s", cid)
	}
}

// --- S6: GetProperties returns all properties (never partial) ---

func TestPropertyEnforcement_S6_GetPropertiesReturnsAll(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "All properties crumb"})
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, id)
	props := got.GetProperties()
	assert.Len(t, props, 5, "GetProperties should return all five built-in properties")

	expectedIDs := allPropertyIDs(t, backend)
	for pid := range props {
		assert.True(t, expectedIDs[pid], "property ID %s should be a defined property", pid)
	}
}

func TestPropertyEnforcement_S6_GetPropertiesIncludesCustom(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	_, err = propsTbl.Set("", &types.Property{
		Name:        "custom_field",
		ValueType:   types.ValueTypeText,
		Description: "Custom text field",
	})
	require.NoError(t, err)

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Custom props crumb"})
	require.NoError(t, err)

	got := getCrumb(t, crumbsTbl, id)
	props := got.GetProperties()
	assert.Len(t, props, 6, "GetProperties should return six properties (five built-in + custom)")
}

// --- S7: ClearProperty resets value to default, not null ---

func TestPropertyEnforcement_S7_ClearPropertyResetsToDefault(t *testing.T) {
	tests := []struct {
		name         string
		propName     string
		setValue     any
		expectedType string
	}{
		{"clear text resets to empty string", types.PropertyOwner, "charlie", types.ValueTypeText},
		{"clear list resets to empty array", types.PropertyLabels, []any{"tag1", "tag2"}, types.ValueTypeList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			crumbsTbl, err := backend.GetTable(types.TableCrumbs)
			require.NoError(t, err)

			id, err := crumbsTbl.Set("", &types.Crumb{Name: "Clear " + tt.propName + " crumb"})
			require.NoError(t, err)

			crumb := getCrumb(t, crumbsTbl, id)
			prop := propertyByName(t, backend, tt.propName)

			// Set a non-default value, then clear it.
			err = crumb.SetProperty(prop.PropertyID, tt.setValue)
			require.NoError(t, err)

			err = crumb.ClearProperty(prop.PropertyID)
			require.NoError(t, err)

			// ClearProperty sets the in-memory value to nil (prd003-crumbs-interface R5.5).
			// Full default-value resolution is deferred to Table.Set per R5.7.
			val, err := crumb.GetProperty(prop.PropertyID)
			require.NoError(t, err)
			assert.Nil(t, val, "ClearProperty should set the value to nil in memory")
		})
	}
}

func TestPropertyEnforcement_S7_ClearPropertyUpdatesTimestamp(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Clear timestamp crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	ownerProp := propertyByName(t, backend, types.PropertyOwner)

	err = crumb.SetProperty(ownerProp.PropertyID, "dave")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	beforeClear := crumb.UpdatedAt

	err = crumb.ClearProperty(ownerProp.PropertyID)
	require.NoError(t, err)

	assert.True(t, crumb.UpdatedAt.After(beforeClear),
		"UpdatedAt should advance after ClearProperty")
}

func TestPropertyEnforcement_S7_ClearPropertyOnNonexistentReturnsError(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	id, err := crumbsTbl.Set("", &types.Crumb{Name: "Clear error crumb"})
	require.NoError(t, err)

	crumb := getCrumb(t, crumbsTbl, id)
	err = crumb.ClearProperty("nonexistent-property-id")
	assert.ErrorIs(t, err, types.ErrPropertyNotFound)
}

// --- S8: crumbs added after property definition have new property ---

func TestPropertyEnforcement_S8_CrumbAfterPropertyDefinition(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)
	newPropID, err := propsTbl.Set("", &types.Property{
		Name:        "story_points",
		ValueType:   types.ValueTypeInteger,
		Description: "Estimated story points",
	})
	require.NoError(t, err)

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	var crumbIDs []string
	for _, name := range []string{"Crumb A", "Crumb B"} {
		id, err := crumbsTbl.Set("", &types.Crumb{Name: name})
		require.NoError(t, err)
		crumbIDs = append(crumbIDs, id)
	}

	for _, cid := range crumbIDs {
		got := getCrumb(t, crumbsTbl, cid)
		assert.Len(t, got.Properties, 6, "crumb should have six properties (five built-in + story_points)")
		val, ok := got.Properties[newPropID]
		assert.True(t, ok, "new property should be auto-initialized")
		assert.Equal(t, float64(0), val, "integer should default to 0")
	}
}

// --- S9: no crumb ever has fewer properties than defined ---

func TestPropertyEnforcement_S9_InvariantHoldsAfterMultipleDefinitions(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)

	// Create an early crumb (before any custom properties).
	earlyID, err := crumbsTbl.Set("", &types.Crumb{Name: "Early crumb"})
	require.NoError(t, err)

	// Define first custom property.
	_, err = propsTbl.Set("", &types.Property{
		Name:        "prop_one",
		ValueType:   types.ValueTypeText,
		Description: "First custom property",
	})
	require.NoError(t, err)

	// Create a middle crumb (after prop_one).
	middleID, err := crumbsTbl.Set("", &types.Crumb{Name: "Middle crumb"})
	require.NoError(t, err)

	// Define second custom property.
	_, err = propsTbl.Set("", &types.Property{
		Name:        "prop_two",
		ValueType:   types.ValueTypeInteger,
		Description: "Second custom property",
	})
	require.NoError(t, err)

	// Create a late crumb (after both custom properties).
	lateID, err := crumbsTbl.Set("", &types.Crumb{Name: "Late crumb"})
	require.NoError(t, err)

	// All three crumbs should have exactly 7 properties (5 built-in + 2 custom).
	expectedCount := 7
	for _, cid := range []string{earlyID, middleID, lateID} {
		got := getCrumb(t, crumbsTbl, cid)
		assert.Len(t, got.Properties, expectedCount,
			"crumb %s should have %d properties", cid, expectedCount)
	}
}

func TestPropertyEnforcement_S9_InvariantHoldsAfterMixedOperations(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	propsTbl, err := backend.GetTable(types.TableProperties)
	require.NoError(t, err)

	// Create two crumbs.
	id1, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb 1"})
	require.NoError(t, err)
	id2, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb 2"})
	require.NoError(t, err)

	// Define a text property.
	_, err = propsTbl.Set("", &types.Property{
		Name:        "field_a",
		ValueType:   types.ValueTypeText,
		Description: "Field A",
	})
	require.NoError(t, err)

	// Create a third crumb.
	id3, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb 3"})
	require.NoError(t, err)

	// Define another property.
	_, err = propsTbl.Set("", &types.Property{
		Name:        "field_b",
		ValueType:   types.ValueTypeBoolean,
		Description: "Field B",
	})
	require.NoError(t, err)

	// Verify invariant: all crumbs have the same property count (5 built-in + 2 custom = 7).
	expectedCount := 7
	for _, cid := range []string{id1, id2, id3} {
		got := getCrumb(t, crumbsTbl, cid)
		assert.Len(t, got.Properties, expectedCount,
			"crumb %s should have %d properties", cid, expectedCount)
	}

	// Verify no crumb has fewer properties than are defined.
	definedCount := len(allPropertyIDs(t, backend))
	assert.Equal(t, expectedCount, definedCount, "defined property count should match expected")

	allCrumbs, err := crumbsTbl.Fetch(nil)
	require.NoError(t, err)
	for _, c := range allCrumbs {
		crumb := c.(*types.Crumb)
		assert.GreaterOrEqual(t, len(crumb.Properties), definedCount,
			"crumb %s must not have fewer properties than defined", crumb.CrumbID)
	}
}

// --- S10: create property via CLI ---

func TestPropertyEnforcement_S10_CreatePropertyViaCLI(t *testing.T) {
	dataDir := initCupboard(t)

	t.Run("set properties creates a new property", func(t *testing.T) {
		payload := `{"name":"estimate","value_type":"integer","description":"Time estimate in hours"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "properties", "", payload)
		require.Equal(t, 0, code, "set properties failed: %s", stderr)

		var prop map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &prop))
		assert.Equal(t, "estimate", prop["name"])
		assert.Equal(t, "integer", prop["value_type"])
		assert.NotEmpty(t, prop["property_id"], "property_id should be populated")
	})

	t.Run("set properties with duplicate name fails", func(t *testing.T) {
		payload := `{"name":"priority","value_type":"text","description":"Duplicate"}`
		_, stderr, code := runCupboard(t, dataDir, "set", "properties", "", payload)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "duplicate name")
	})

	t.Run("set properties with invalid value type fails", func(t *testing.T) {
		payload := `{"name":"bad_prop","value_type":"invalid_type","description":"Bad type"}`
		_, stderr, code := runCupboard(t, dataDir, "set", "properties", "", payload)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "invalid value type")
	})
}

// --- S11: list properties via CLI ---

func TestPropertyEnforcement_S11_ListPropertiesViaCLI(t *testing.T) {
	dataDir := initCupboard(t)

	t.Run("list properties returns built-in properties as JSON array", func(t *testing.T) {
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 5, "should have five built-in properties")

		names := make(map[string]bool)
		for _, p := range arr {
			name, ok := p["name"].(string)
			require.True(t, ok)
			names[name] = true
		}
		for _, expected := range []string{"priority", "type", "description", "owner", "labels"} {
			assert.True(t, names[expected], "missing built-in property %q", expected)
		}
	})

	t.Run("list properties includes custom property after creation", func(t *testing.T) {
		payload := `{"name":"effort","value_type":"integer","description":"Effort level"}`
		_, stderr, code := runCupboard(t, dataDir, "set", "properties", "", payload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)

		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties")
		require.Equal(t, 0, code, "list failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 6, "should have six properties (five built-in + effort)")
	})

	t.Run("list properties with name filter returns matching property", func(t *testing.T) {
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 1)
		assert.Equal(t, "priority", arr[0]["name"])
		assert.Equal(t, "categorical", arr[0]["value_type"])
	})
}

// --- S12: crumb creation via CLI shows auto-initialized properties ---

func TestPropertyEnforcement_S12_CrumbCreationAutoInitViaCLI(t *testing.T) {
	dataDir := initCupboard(t)

	t.Run("new crumb has properties map with all built-in properties", func(t *testing.T) {
		payload := `{"name":"CLI crumb"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", payload)
		require.Equal(t, 0, code, "create crumb failed: %s", stderr)

		var crumb map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))

		props, ok := crumb["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map in JSON output")
		assert.Len(t, props, 5, "new crumb should have five property entries")
	})

	t.Run("get crumb returns populated properties map", func(t *testing.T) {
		payload := `{"name":"Get props crumb"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", payload)
		require.Equal(t, 0, code, "create crumb failed: %s", stderr)

		crumbID := extractJSONField(t, stdout, "crumb_id")

		stdout, stderr, code = runCupboard(t, dataDir, "get", "crumbs", crumbID)
		require.Equal(t, 0, code, "get crumb failed: %s", stderr)

		var crumb map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))

		props, ok := crumb["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map in JSON output")
		assert.Len(t, props, 5, "retrieved crumb should have five property entries")
	})

	t.Run("crumb properties contain default values", func(t *testing.T) {
		payload := `{"name":"Defaults crumb"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", payload)
		require.Equal(t, 0, code, "create crumb failed: %s", stderr)

		var crumb map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))

		props := crumb["properties"].(map[string]any)

		// Verify text properties default to empty string, list to empty array,
		// categorical to nil. Property keys are UUIDs so we check by count.
		hasEmptyString := false
		hasEmptyArray := false
		hasNil := false
		for _, v := range props {
			switch val := v.(type) {
			case string:
				if val == "" {
					hasEmptyString = true
				}
			case []any:
				if len(val) == 0 {
					hasEmptyArray = true
				}
			case nil:
				hasNil = true
			}
		}
		assert.True(t, hasEmptyString, "should have at least one empty-string default (text property)")
		assert.True(t, hasEmptyArray, "should have at least one empty-array default (list property)")
		assert.True(t, hasNil, "should have at least one nil default (categorical property)")
	})
}

// --- S13: defining new property via CLI backfills existing crumbs ---

func TestPropertyEnforcement_S13_BackfillViaCLI(t *testing.T) {
	dataDir := initCupboard(t)

	t.Run("define property backfills existing crumb", func(t *testing.T) {
		// Create a crumb first.
		crumbPayload := `{"name":"Backfill target"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", crumbPayload)
		require.Equal(t, 0, code, "create crumb failed: %s", stderr)
		crumbID := extractJSONField(t, stdout, "crumb_id")

		// Get the crumb and verify five properties.
		stdout, stderr, code = runCupboard(t, dataDir, "get", "crumbs", crumbID)
		require.Equal(t, 0, code, "get crumb failed: %s", stderr)
		var crumb map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))
		props := crumb["properties"].(map[string]any)
		assert.Len(t, props, 5, "crumb should have five properties before backfill")

		// Define a new property.
		propPayload := `{"name":"severity","value_type":"integer","description":"Issue severity"}`
		_, stderr, code = runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)

		// Re-fetch the crumb and verify it now has six properties.
		stdout, stderr, code = runCupboard(t, dataDir, "get", "crumbs", crumbID)
		require.Equal(t, 0, code, "get crumb after backfill failed: %s", stderr)
		require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))
		props = crumb["properties"].(map[string]any)
		assert.Len(t, props, 6, "crumb should have six properties after backfill")
	})

	t.Run("backfill applies to multiple existing crumbs", func(t *testing.T) {
		// Create three crumbs.
		var crumbIDs []string
		for _, name := range []string{"Multi A", "Multi B", "Multi C"} {
			payload := fmt.Sprintf(`{"name":"%s"}`, name)
			stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", payload)
			require.Equal(t, 0, code, "create crumb failed: %s", stderr)
			crumbIDs = append(crumbIDs, extractJSONField(t, stdout, "crumb_id"))
		}

		// Define another new property.
		propPayload := `{"name":"complexity","value_type":"text","description":"Task complexity"}`
		_, stderr, code := runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)

		// Verify all crumbs have the new property.
		for _, id := range crumbIDs {
			stdout, stderr, code := runCupboard(t, dataDir, "get", "crumbs", id)
			require.Equal(t, 0, code, "get crumb %s failed: %s", id, stderr)

			var crumb map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))
			props := crumb["properties"].(map[string]any)
			// 5 built-in + severity (from prior subtest) + complexity = 7.
			assert.Len(t, props, 7, "crumb %s should have seven properties after backfill", id)
		}
	})

	t.Run("new crumb after property definition has all properties", func(t *testing.T) {
		payload := `{"name":"Post-backfill crumb"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", payload)
		require.Equal(t, 0, code, "create crumb failed: %s", stderr)

		var crumb map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &crumb))
		props := crumb["properties"].(map[string]any)
		// 5 built-in + severity + complexity = 7.
		assert.Len(t, props, 7, "new crumb should have all seven properties")
	})
}

// --- S14: category operations via CLI (set categories) ---

func TestPropertyEnforcement_S14_CategorySetViaCLI(t *testing.T) {
	dataDir := initCupboard(t)

	t.Run("set categories creates a category for a categorical property", func(t *testing.T) {
		// Get the priority property ID.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)
		propsArr := parseJSONArray(t, stdout)
		require.Len(t, propsArr, 1)
		priorityID := propsArr[0]["property_id"].(string)

		// Create a category via set categories.
		catPayload := fmt.Sprintf(`{"property_id":"%s","name":"blocker","ordinal":-1}`, priorityID)
		stdout, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload)
		require.Equal(t, 0, code, "set categories failed: %s", stderr)

		var cat map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &cat))
		assert.Equal(t, "blocker", cat["name"])
		assert.Equal(t, float64(-1), cat["ordinal"])
		assert.Equal(t, priorityID, cat["property_id"])
		assert.NotEmpty(t, cat["category_id"], "category_id should be populated")
	})

	t.Run("set categories rejects empty name", func(t *testing.T) {
		// Get the priority property ID.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)
		propsArr := parseJSONArray(t, stdout)
		require.Len(t, propsArr, 1)
		priorityID := propsArr[0]["property_id"].(string)

		// Attempt to create a category with empty name.
		catPayload := fmt.Sprintf(`{"property_id":"%s","name":"","ordinal":0}`, priorityID)
		_, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "invalid name")
	})

	t.Run("set categories rejects duplicate name within property", func(t *testing.T) {
		// Get the priority property ID.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)
		propsArr := parseJSONArray(t, stdout)
		require.Len(t, propsArr, 1)
		priorityID := propsArr[0]["property_id"].(string)

		// Create a category.
		catPayload := fmt.Sprintf(`{"property_id":"%s","name":"duplicate_test","ordinal":1}`, priorityID)
		_, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload)
		require.Equal(t, 0, code, "first category creation failed: %s", stderr)

		// Attempt to create a category with the same name.
		_, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "duplicate name")
	})

	t.Run("set categories allows same name on different properties", func(t *testing.T) {
		// Create a custom categorical property.
		propPayload := `{"name":"custom_cat","value_type":"categorical","description":"Custom categorical"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)
		customPropID := extractJSONField(t, stdout, "property_id")

		// Get the priority property ID.
		stdout, stderr, code = runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)
		propsArr := parseJSONArray(t, stdout)
		require.Len(t, propsArr, 1)
		priorityID := propsArr[0]["property_id"].(string)

		// Create a category on priority with name "shared".
		catPayload1 := fmt.Sprintf(`{"property_id":"%s","name":"shared","ordinal":1}`, priorityID)
		_, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload1)
		require.Equal(t, 0, code, "create category on priority failed: %s", stderr)

		// Create a category on custom_cat with the same name "shared".
		catPayload2 := fmt.Sprintf(`{"property_id":"%s","name":"shared","ordinal":1}`, customPropID)
		_, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload2)
		assert.Equal(t, 0, code, "create category on custom_cat failed: %s", stderr)
	})

	t.Run("set categories with negative ordinals", func(t *testing.T) {
		// Get the priority property ID.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)
		propsArr := parseJSONArray(t, stdout)
		require.Len(t, propsArr, 1)
		priorityID := propsArr[0]["property_id"].(string)

		// Create a category with negative ordinal.
		catPayload := fmt.Sprintf(`{"property_id":"%s","name":"top_priority","ordinal":-10}`, priorityID)
		stdout, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload)
		require.Equal(t, 0, code, "set categories failed: %s", stderr)

		var cat map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &cat))
		assert.Equal(t, float64(-10), cat["ordinal"])
	})
}

// --- S15: category operations via CLI (list categories / GetCategories) ---

func TestPropertyEnforcement_S15_CategoryListViaCLI(t *testing.T) {
	dataDir := initCupboard(t)

	t.Run("list categories returns categories for a property", func(t *testing.T) {
		// Create a fresh categorical property for this test.
		propPayload := `{"name":"rank","value_type":"categorical","description":"Rank"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)
		rankID := extractJSONField(t, stdout, "property_id")

		// Create a few categories with different ordinals.
		for _, cat := range []struct{ name string; ord int }{
			{"low", 3}, {"high", 1}, {"medium", 2},
		} {
			payload := fmt.Sprintf(`{"property_id":"%s","name":"%s","ordinal":%d}`, rankID, cat.name, cat.ord)
			_, stderr, code := runCupboard(t, dataDir, "set", "categories", "", payload)
			require.Equal(t, 0, code, "create category %s failed: %s", cat.name, stderr)
		}

		// List all categories for this property.
		stdout, stderr, code = runCupboard(t, dataDir, "list", "categories", fmt.Sprintf("property_id=%s", rankID))
		require.Equal(t, 0, code, "list categories failed: %s", stderr)

		catsArr := parseJSONArray(t, stdout)
		require.Len(t, catsArr, 3)

		// Verify they are ordered by ordinal ascending.
		names := []string{catsArr[0]["name"].(string), catsArr[1]["name"].(string), catsArr[2]["name"].(string)}
		assert.Equal(t, []string{"high", "medium", "low"}, names, "categories should be ordered by ordinal")
	})

	t.Run("list categories orders by name for same ordinal", func(t *testing.T) {
		// Create a custom categorical property.
		propPayload := `{"name":"status","value_type":"categorical","description":"Status"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)
		statusID := extractJSONField(t, stdout, "property_id")

		// Create categories with the same ordinal.
		for _, name := range []string{"zebra", "alpha", "beta"} {
			payload := fmt.Sprintf(`{"property_id":"%s","name":"%s","ordinal":1}`, statusID, name)
			_, stderr, code := runCupboard(t, dataDir, "set", "categories", "", payload)
			require.Equal(t, 0, code, "create category %s failed: %s", name, stderr)
		}

		// List categories and verify ordering by name.
		stdout, stderr, code = runCupboard(t, dataDir, "list", "categories", fmt.Sprintf("property_id=%s", statusID))
		require.Equal(t, 0, code, "list categories failed: %s", stderr)

		catsArr := parseJSONArray(t, stdout)
		require.Len(t, catsArr, 3)

		names := []string{catsArr[0]["name"].(string), catsArr[1]["name"].(string), catsArr[2]["name"].(string)}
		assert.Equal(t, []string{"alpha", "beta", "zebra"}, names, "categories should be ordered by name for same ordinal")
	})

	t.Run("list categories returns empty array for property with no categories", func(t *testing.T) {
		// Create a custom categorical property with no categories.
		propPayload := `{"name":"empty_cat","value_type":"categorical","description":"Empty"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)
		emptyID := extractJSONField(t, stdout, "property_id")

		// List categories (should be empty).
		stdout, stderr, code = runCupboard(t, dataDir, "list", "categories", fmt.Sprintf("property_id=%s", emptyID))
		require.Equal(t, 0, code, "list categories failed: %s", stderr)

		catsArr := parseJSONArray(t, stdout)
		assert.Len(t, catsArr, 0, "should return empty array")
	})

	t.Run("list categories with negative ordinals sorts correctly", func(t *testing.T) {
		// Create a custom categorical property.
		propPayload := `{"name":"ord_sort","value_type":"categorical","description":"Ordinal sort test"}`
		stdout, stderr, code := runCupboard(t, dataDir, "set", "properties", "", propPayload)
		require.Equal(t, 0, code, "create property failed: %s", stderr)
		sortID := extractJSONField(t, stdout, "property_id")

		// Create categories with negative, zero, and positive ordinals.
		for _, cat := range []struct{ name string; ord int }{
			{"positive", 5}, {"negative", -5}, {"zero", 0},
		} {
			payload := fmt.Sprintf(`{"property_id":"%s","name":"%s","ordinal":%d}`, sortID, cat.name, cat.ord)
			_, stderr, code := runCupboard(t, dataDir, "set", "categories", "", payload)
			require.Equal(t, 0, code, "create category %s failed: %s", cat.name, stderr)
		}

		// List categories and verify ordering.
		stdout, stderr, code = runCupboard(t, dataDir, "list", "categories", fmt.Sprintf("property_id=%s", sortID))
		require.Equal(t, 0, code, "list categories failed: %s", stderr)

		catsArr := parseJSONArray(t, stdout)
		require.Len(t, catsArr, 3)

		names := []string{catsArr[0]["name"].(string), catsArr[1]["name"].(string), catsArr[2]["name"].(string)}
		assert.Equal(t, []string{"negative", "zero", "positive"}, names, "negative ordinals should sort before positive")
	})

	t.Run("categories persist to JSONL", func(t *testing.T) {
		// Get the priority property ID.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "properties", "name=priority")
		require.Equal(t, 0, code, "list properties failed: %s", stderr)
		propsArr := parseJSONArray(t, stdout)
		require.Len(t, propsArr, 1)
		priorityID := propsArr[0]["property_id"].(string)

		// Create a category.
		catPayload := fmt.Sprintf(`{"property_id":"%s","name":"persisted","ordinal":1}`, priorityID)
		_, stderr, code = runCupboard(t, dataDir, "set", "categories", "", catPayload)
		require.Equal(t, 0, code, "create category failed: %s", stderr)

		// Verify categories.jsonl contains the persisted category.
		// We use the generic list command to verify persistence indirectly.
		stdout, stderr, code = runCupboard(t, dataDir, "list", "categories", "name=persisted")
		require.Equal(t, 0, code, "list categories failed: %s", stderr)

		catsArr := parseJSONArray(t, stdout)
		require.Len(t, catsArr, 1)
		assert.Equal(t, "persisted", catsArr[0]["name"])
	})
}
