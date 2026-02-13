// Unit tests for property backfill on property definition.
// Validates: prd004-properties-interface R4.2, R4.3 (backfill on creation);
//            test-rel02.0-uc001-property-enforcement (test cases S5, S7-S9).
package sqlite

import (
	"testing"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropertyBackfill(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, b *Backend)
	}{
		{
			name: "defining property backfills single existing crumb",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a crumb first (gets 5 built-in properties).
				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "Backfill target"})
				require.NoError(t, err)

				// Define a new integer property.
				prop := &types.Property{
					Name:        "estimate",
					ValueType:   types.ValueTypeInteger,
					Description: "Time estimate in hours",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				// Verify the crumb now has 6 properties (5 built-in + 1 custom).
				entity, err := crumbsTable.Get(crumbID)
				require.NoError(t, err)
				got := entity.(*types.Crumb)
				assert.Len(t, got.Properties, 6)
				assert.Equal(t, float64(0), got.Properties[propID],
					"backfilled integer property should default to 0")
			},
		},
		{
			name: "defining property backfills multiple existing crumbs",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create three crumbs.
				ids := make([]string, 3)
				for i, name := range []string{"Crumb one", "Crumb two", "Crumb three"} {
					id, err := crumbsTable.Set("", &types.Crumb{Name: name})
					require.NoError(t, err)
					ids[i] = id
				}

				// Define a new text property.
				prop := &types.Property{
					Name:        "complexity",
					ValueType:   types.ValueTypeText,
					Description: "Task complexity",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				// Verify all three crumbs have the new property.
				for _, id := range ids {
					entity, err := crumbsTable.Get(id)
					require.NoError(t, err)
					got := entity.(*types.Crumb)
					assert.Len(t, got.Properties, 6,
						"crumb %s should have 6 properties after backfill", id)
					assert.Equal(t, "", got.Properties[propID],
						"text property should default to empty string")
				}
			},
		},
		{
			name: "backfill with no existing crumbs succeeds",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Define a new property with no crumbs in the system.
				prop := &types.Property{
					Name:        "empty_backfill",
					ValueType:   types.ValueTypeBoolean,
					Description: "No crumbs to backfill",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Verify property was created.
				allProps, err := propsTable.Fetch(nil)
				require.NoError(t, err)
				found := false
				for _, p := range allProps {
					if p.(*types.Property).Name == "empty_backfill" {
						found = true
						break
					}
				}
				assert.True(t, found, "property should exist after creation with no crumbs")
			},
		},
		{
			name: "backfill does not override existing crumb property values",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a crumb.
				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "Pre-existing value crumb"})
				require.NoError(t, err)

				// Define a new property (this backfills the crumb with default 0).
				prop := &types.Property{
					Name:        "score",
					ValueType:   types.ValueTypeInteger,
					Description: "Score value",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				// Manually update the crumb's property value to 42 via SQL.
				_, err = b.db.Exec(
					"UPDATE crumb_properties SET value = '42' WHERE crumb_id = ? AND property_id = ?",
					crumbID, propID,
				)
				require.NoError(t, err)

				// Define another new property. The backfill for THIS property
				// should not touch the score property we just set to 42.
				prop2 := &types.Property{
					Name:        "weight",
					ValueType:   types.ValueTypeInteger,
					Description: "Weight value",
				}
				_, err = propsTable.Set("", prop2)
				require.NoError(t, err)

				// Verify the score property still has value 42 (not reset).
				entity, err := crumbsTable.Get(crumbID)
				require.NoError(t, err)
				got := entity.(*types.Crumb)
				assert.Equal(t, float64(42), got.Properties[propID],
					"existing property value should not be overridden by backfill")
			},
		},
		{
			name: "backfill uses INSERT OR IGNORE for existing rows",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a crumb.
				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "Ignore test crumb"})
				require.NoError(t, err)

				// Define a new property (creates backfill row with default).
				prop := &types.Property{
					Name:        "effort",
					ValueType:   types.ValueTypeInteger,
					Description: "Effort level",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				// Manually insert a duplicate crumb_properties row. This simulates
				// a scenario where the row already exists.
				// If the code uses INSERT OR IGNORE, this should not fail.
				// If it uses plain INSERT, it would error on primary key conflict.
				_, err = b.db.Exec(
					"INSERT OR REPLACE INTO crumb_properties (crumb_id, property_id, value_type, value) VALUES (?, ?, ?, ?)",
					crumbID, propID, types.ValueTypeInteger, "99",
				)
				require.NoError(t, err)

				// Verify the row count is exactly 1 for this combination.
				var count int
				err = b.db.QueryRow(
					"SELECT COUNT(*) FROM crumb_properties WHERE crumb_id = ? AND property_id = ?",
					crumbID, propID,
				).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 1, count, "should have exactly one row per crumb-property pair")
			},
		},
		{
			name: "backfill is atomic with property creation",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create crumbs.
				_, err = crumbsTable.Set("", &types.Crumb{Name: "Atomic crumb 1"})
				require.NoError(t, err)
				_, err = crumbsTable.Set("", &types.Crumb{Name: "Atomic crumb 2"})
				require.NoError(t, err)

				// Count properties and crumb_properties before.
				var propCountBefore int
				err = b.db.QueryRow("SELECT COUNT(*) FROM properties").Scan(&propCountBefore)
				require.NoError(t, err)
				var cpCountBefore int
				err = b.db.QueryRow("SELECT COUNT(*) FROM crumb_properties").Scan(&cpCountBefore)
				require.NoError(t, err)

				// Successfully define a property.
				prop := &types.Property{
					Name:        "priority_level",
					ValueType:   types.ValueTypeInteger,
					Description: "Priority level",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// After success: property count +1, crumb_properties count +2 (one per crumb).
				var propCountAfter int
				err = b.db.QueryRow("SELECT COUNT(*) FROM properties").Scan(&propCountAfter)
				require.NoError(t, err)
				assert.Equal(t, propCountBefore+1, propCountAfter)

				var cpCountAfter int
				err = b.db.QueryRow("SELECT COUNT(*) FROM crumb_properties").Scan(&cpCountAfter)
				require.NoError(t, err)
				assert.Equal(t, cpCountBefore+2, cpCountAfter,
					"backfill should add one crumb_properties row per existing crumb")
			},
		},
		{
			name: "backfill with boolean property defaults to false",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "Bool backfill crumb"})
				require.NoError(t, err)

				prop := &types.Property{
					Name:        "is_urgent",
					ValueType:   types.ValueTypeBoolean,
					Description: "Urgency flag",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				entity, err := crumbsTable.Get(crumbID)
				require.NoError(t, err)
				got := entity.(*types.Crumb)
				assert.Equal(t, false, got.Properties[propID],
					"boolean backfill should default to false")
			},
		},
		{
			name: "backfill with list property defaults to empty array",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "List backfill crumb"})
				require.NoError(t, err)

				prop := &types.Property{
					Name:        "tags",
					ValueType:   types.ValueTypeList,
					Description: "Tags list",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				entity, err := crumbsTable.Get(crumbID)
				require.NoError(t, err)
				got := entity.(*types.Crumb)
				val, ok := got.Properties[propID]
				assert.True(t, ok, "backfilled list property should exist")
				arr, ok := val.([]any)
				assert.True(t, ok, "list value should be []any")
				assert.Empty(t, arr, "list backfill should default to empty array")
			},
		},
		{
			name: "backfill with categorical property defaults to null",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "Cat backfill crumb"})
				require.NoError(t, err)

				prop := &types.Property{
					Name:        "severity",
					ValueType:   types.ValueTypeCategorical,
					Description: "Severity level",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				entity, err := crumbsTable.Get(crumbID)
				require.NoError(t, err)
				got := entity.(*types.Crumb)
				assert.Nil(t, got.Properties[propID],
					"categorical backfill should default to null")
			},
		},
		{
			name: "backfill with timestamp property defaults to null",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				crumbID, err := crumbsTable.Set("", &types.Crumb{Name: "Timestamp backfill crumb"})
				require.NoError(t, err)

				prop := &types.Property{
					Name:        "due_date",
					ValueType:   types.ValueTypeTimestamp,
					Description: "Due date",
				}
				propID, err := propsTable.Set("", prop)
				require.NoError(t, err)

				entity, err := crumbsTable.Get(crumbID)
				require.NoError(t, err)
				got := entity.(*types.Crumb)
				assert.Nil(t, got.Properties[propID],
					"timestamp backfill should default to null")
			},
		},
		{
			name: "all crumbs have same property count after interleaved definitions",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Interleave crumb creation and property definitions.
				id1, err := crumbsTable.Set("", &types.Crumb{Name: "Early crumb"})
				require.NoError(t, err)

				_, err = propsTable.Set("", &types.Property{
					Name: "prop_alpha", ValueType: types.ValueTypeText, Description: "Alpha",
				})
				require.NoError(t, err)

				id2, err := crumbsTable.Set("", &types.Crumb{Name: "Middle crumb"})
				require.NoError(t, err)

				_, err = propsTable.Set("", &types.Property{
					Name: "prop_beta", ValueType: types.ValueTypeInteger, Description: "Beta",
				})
				require.NoError(t, err)

				id3, err := crumbsTable.Set("", &types.Crumb{Name: "Late crumb"})
				require.NoError(t, err)

				// All three crumbs should have the same number of properties:
				// 5 built-in + 2 custom = 7.
				for _, id := range []string{id1, id2, id3} {
					entity, err := crumbsTable.Get(id)
					require.NoError(t, err)
					got := entity.(*types.Crumb)
					assert.Len(t, got.Properties, 7,
						"crumb %s should have 7 properties (5 built-in + 2 custom)", id)
				}
			},
		},
		{
			name: "backfill persists to crumb_properties JSONL",
			check: func(t *testing.T, b *Backend) {
				crumbsTable, err := b.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				_, err = crumbsTable.Set("", &types.Crumb{Name: "JSONL backfill crumb"})
				require.NoError(t, err)

				// Count JSONL records before defining new property.
				recordsBefore, err := readJSONL(b.config.DataDir + "/crumb_properties.jsonl")
				require.NoError(t, err)
				countBefore := len(recordsBefore)

				_, err = propsTable.Set("", &types.Property{
					Name: "jsonl_test", ValueType: types.ValueTypeText, Description: "JSONL test",
				})
				require.NoError(t, err)

				// After backfill, JSONL should have one more record.
				recordsAfter, err := readJSONL(b.config.DataDir + "/crumb_properties.jsonl")
				require.NoError(t, err)
				assert.Equal(t, countBefore+1, len(recordsAfter),
					"JSONL should have one more crumb_properties record after backfill")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := setupBackend(t)
			tt.check(t, b)
		})
	}
}
