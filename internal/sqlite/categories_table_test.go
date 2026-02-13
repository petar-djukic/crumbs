// Unit tests for categories table operations.
// Validates: prd004-properties-interface R7, R8 (DefineCategory, GetCategories);
//            test-rel02.0-uc001-property-enforcement (test cases S12-S15).
package sqlite

import (
	"testing"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefineCategory(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, b *Backend)
	}{
		{
			name: "define category creates category with populated ID",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "severity",
					ValueType:   types.ValueTypeCategorical,
					Description: "Issue severity",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define a category.
				cat, err := prop.DefineCategory(b, "high", 1)
				require.NoError(t, err)
				assert.NotEmpty(t, cat.CategoryID, "CategoryID should be populated")
				assert.Equal(t, prop.PropertyID, cat.PropertyID)
				assert.Equal(t, "high", cat.Name)
				assert.Equal(t, 1, cat.Ordinal)
			},
		},
		{
			name: "define category persists to backend",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "status",
					ValueType:   types.ValueTypeCategorical,
					Description: "Status",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define a category.
				cat, err := prop.DefineCategory(b, "open", 0)
				require.NoError(t, err)

				// Retrieve and verify.
				entity, err := catsTable.Get(cat.CategoryID)
				require.NoError(t, err)
				got := entity.(*types.Category)
				assert.Equal(t, "open", got.Name)
				assert.Equal(t, 0, got.Ordinal)
			},
		},
		{
			name: "define category returns ErrInvalidValueType for non-categorical property",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a text property.
				prop := &types.Property{
					Name:        "text_field",
					ValueType:   types.ValueTypeText,
					Description: "Text field",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Attempt to define a category.
				_, err = prop.DefineCategory(b, "value", 0)
				assert.ErrorIs(t, err, types.ErrInvalidValueType)
			},
		},
		{
			name: "define category returns ErrInvalidName for empty name",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "empty_name_test",
					ValueType:   types.ValueTypeCategorical,
					Description: "Empty name test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Attempt to define a category with empty name.
				_, err = prop.DefineCategory(b, "", 0)
				assert.ErrorIs(t, err, types.ErrInvalidName)
			},
		},
		{
			name: "define category returns ErrDuplicateName for duplicate name within property",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "dup_name_test",
					ValueType:   types.ValueTypeCategorical,
					Description: "Duplicate name test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define first category.
				_, err = prop.DefineCategory(b, "task", 0)
				require.NoError(t, err)

				// Attempt to define duplicate.
				_, err = prop.DefineCategory(b, "task", 1)
				assert.ErrorIs(t, err, types.ErrDuplicateName)
			},
		},
		{
			name: "define category allows same name on different properties",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create two categorical properties.
				prop1 := &types.Property{
					Name:        "prop_a",
					ValueType:   types.ValueTypeCategorical,
					Description: "Prop A",
				}
				_, err = propsTable.Set("", prop1)
				require.NoError(t, err)

				prop2 := &types.Property{
					Name:        "prop_b",
					ValueType:   types.ValueTypeCategorical,
					Description: "Prop B",
				}
				_, err = propsTable.Set("", prop2)
				require.NoError(t, err)

				// Define same category name on both properties.
				_, err = prop1.DefineCategory(b, "shared_name", 1)
				require.NoError(t, err)

				_, err = prop2.DefineCategory(b, "shared_name", 1)
				require.NoError(t, err)
			},
		},
		{
			name: "define category allows negative ordinals",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "neg_ord",
					ValueType:   types.ValueTypeCategorical,
					Description: "Negative ordinal test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define category with negative ordinal.
				cat, err := prop.DefineCategory(b, "top_priority", -10)
				require.NoError(t, err)
				assert.Equal(t, -10, cat.Ordinal)
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

func TestGetCategories(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, b *Backend)
	}{
		{
			name: "get categories returns categories ordered by ordinal ascending",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "severity",
					ValueType:   types.ValueTypeCategorical,
					Description: "Issue severity",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define categories in non-ordinal order.
				_, err = prop.DefineCategory(b, "low", 3)
				require.NoError(t, err)
				_, err = prop.DefineCategory(b, "high", 1)
				require.NoError(t, err)
				_, err = prop.DefineCategory(b, "medium", 2)
				require.NoError(t, err)

				// Retrieve and verify order.
				cats, err := prop.GetCategories(b)
				require.NoError(t, err)
				require.Len(t, cats, 3)
				assert.Equal(t, "high", cats[0].Name)
				assert.Equal(t, "medium", cats[1].Name)
				assert.Equal(t, "low", cats[2].Name)
			},
		},
		{
			name: "get categories orders by name for same ordinal",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "status",
					ValueType:   types.ValueTypeCategorical,
					Description: "Status",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define categories with same ordinal.
				_, err = prop.DefineCategory(b, "zebra", 1)
				require.NoError(t, err)
				_, err = prop.DefineCategory(b, "alpha", 1)
				require.NoError(t, err)
				_, err = prop.DefineCategory(b, "beta", 1)
				require.NoError(t, err)

				// Retrieve and verify alphabetical order.
				cats, err := prop.GetCategories(b)
				require.NoError(t, err)
				require.Len(t, cats, 3)
				assert.Equal(t, "alpha", cats[0].Name)
				assert.Equal(t, "beta", cats[1].Name)
				assert.Equal(t, "zebra", cats[2].Name)
			},
		},
		{
			name: "get categories returns empty slice for property with no categories",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "empty_cat",
					ValueType:   types.ValueTypeCategorical,
					Description: "Empty",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Retrieve categories.
				cats, err := prop.GetCategories(b)
				require.NoError(t, err)
				assert.NotNil(t, cats)
				assert.Empty(t, cats)
			},
		},
		{
			name: "get categories returns ErrInvalidValueType for text property",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a text property.
				prop := &types.Property{
					Name:        "text_prop_test",
					ValueType:   types.ValueTypeText,
					Description: "Text property test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Attempt to get categories.
				_, err = prop.GetCategories(b)
				assert.ErrorIs(t, err, types.ErrInvalidValueType)
			},
		},
		{
			name: "get categories returns ErrInvalidValueType for list property",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a list property.
				prop := &types.Property{
					Name:        "list_prop_test",
					ValueType:   types.ValueTypeList,
					Description: "List property test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Attempt to get categories.
				_, err = prop.GetCategories(b)
				assert.ErrorIs(t, err, types.ErrInvalidValueType)
			},
		},
		{
			name: "negative ordinals sort before positive ordinals",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "ord_sort",
					ValueType:   types.ValueTypeCategorical,
					Description: "Ordinal sort test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define categories with mixed ordinals.
				_, err = prop.DefineCategory(b, "positive", 5)
				require.NoError(t, err)
				_, err = prop.DefineCategory(b, "negative", -5)
				require.NoError(t, err)
				_, err = prop.DefineCategory(b, "zero", 0)
				require.NoError(t, err)

				// Retrieve and verify order.
				cats, err := prop.GetCategories(b)
				require.NoError(t, err)
				require.Len(t, cats, 3)
				assert.Equal(t, "negative", cats[0].Name)
				assert.Equal(t, "zero", cats[1].Name)
				assert.Equal(t, "positive", cats[2].Name)
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

func TestCategoriesTableCRUD(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, b *Backend)
	}{
		{
			name: "get category returns ErrInvalidID for empty id",
			check: func(t *testing.T, b *Backend) {
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)

				_, err = catsTable.Get("")
				assert.ErrorIs(t, err, types.ErrInvalidID)
			},
		},
		{
			name: "get category returns ErrNotFound for nonexistent id",
			check: func(t *testing.T, b *Backend) {
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)

				_, err = catsTable.Get("nonexistent-id")
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},
		{
			name: "set category returns ErrInvalidName for empty name",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "test_prop",
					ValueType:   types.ValueTypeCategorical,
					Description: "Test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Attempt to set category with empty name.
				cat := &types.Category{
					PropertyID: prop.PropertyID,
					Name:       "",
					Ordinal:    0,
				}
				_, err = catsTable.Set("", cat)
				assert.ErrorIs(t, err, types.ErrInvalidName)
			},
		},
		{
			name: "delete category removes category",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create a categorical property.
				prop := &types.Property{
					Name:        "del_test",
					ValueType:   types.ValueTypeCategorical,
					Description: "Delete test",
				}
				_, err = propsTable.Set("", prop)
				require.NoError(t, err)

				// Define a category.
				cat, err := prop.DefineCategory(b, "to_delete", 0)
				require.NoError(t, err)

				// Delete it.
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)
				err = catsTable.Delete(cat.CategoryID)
				require.NoError(t, err)

				// Verify it's gone.
				_, err = catsTable.Get(cat.CategoryID)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},
		{
			name: "delete category returns ErrNotFound for nonexistent id",
			check: func(t *testing.T, b *Backend) {
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)

				err = catsTable.Delete("nonexistent-id")
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},
		{
			name: "fetch categories with property_id filter",
			check: func(t *testing.T, b *Backend) {
				propsTable, err := b.GetTable(types.TableProperties)
				require.NoError(t, err)

				// Create two categorical properties.
				prop1 := &types.Property{
					Name:        "prop1",
					ValueType:   types.ValueTypeCategorical,
					Description: "Prop 1",
				}
				_, err = propsTable.Set("", prop1)
				require.NoError(t, err)

				prop2 := &types.Property{
					Name:        "prop2",
					ValueType:   types.ValueTypeCategorical,
					Description: "Prop 2",
				}
				_, err = propsTable.Set("", prop2)
				require.NoError(t, err)

				// Define categories on both.
				_, err = prop1.DefineCategory(b, "cat1", 0)
				require.NoError(t, err)
				_, err = prop2.DefineCategory(b, "cat2", 0)
				require.NoError(t, err)

				// Fetch categories for prop1 only.
				catsTable, err := b.GetTable(types.TableCategories)
				require.NoError(t, err)
				filter := types.Filter{"property_id": prop1.PropertyID}
				entities, err := catsTable.Fetch(filter)
				require.NoError(t, err)
				require.Len(t, entities, 1)
				cat := entities[0].(*types.Category)
				assert.Equal(t, "cat1", cat.Name)
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
