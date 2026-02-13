// Unit tests for built-in property seeding on backend attach.
// Validates: prd002-sqlite-backend R9 (built-in properties seeding);
//            prd004-properties-interface R9 (built-in properties and categories);
//            test-rel02.0-uc001-property-enforcement (test cases 1-3).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// setupTestDB creates a temporary directory with empty JSONL files, opens an
// in-memory SQLite database, and initializes the schema. It returns the db, the
// temp dir path, and a cleanup function.
func setupTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	dataDir := t.TempDir()

	for _, name := range jsonlFiles {
		f, err := os.Create(filepath.Join(dataDir, name))
		require.NoError(t, err)
		f.Close()
	}

	dbPath := filepath.Join(dataDir, "cupboard.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	require.NoError(t, err)

	for _, ddl := range schemaDDL {
		_, err := db.Exec(ddl)
		require.NoError(t, err)
	}
	for _, ddl := range indexDDL {
		_, err := db.Exec(ddl)
		require.NoError(t, err)
	}

	return db, dataDir
}

func TestSeedBuiltInProperties(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, db *sql.DB)
		check func(t *testing.T, db *sql.DB, dataDir string)
	}{
		{
			name:  "seeds five built-in properties on empty database",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var count int
				err := db.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 5, count)
			},
		},
		{
			name:  "seeds priority property with categorical type",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var valueType string
				err := db.QueryRow(
					"SELECT value_type FROM properties WHERE name = ?",
					types.PropertyPriority,
				).Scan(&valueType)
				require.NoError(t, err)
				assert.Equal(t, types.ValueTypeCategorical, valueType)
			},
		},
		{
			name:  "seeds type property with categorical type",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var valueType string
				err := db.QueryRow(
					"SELECT value_type FROM properties WHERE name = ?",
					types.PropertyType,
				).Scan(&valueType)
				require.NoError(t, err)
				assert.Equal(t, types.ValueTypeCategorical, valueType)
			},
		},
		{
			name:  "seeds description property with text type",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var valueType string
				err := db.QueryRow(
					"SELECT value_type FROM properties WHERE name = ?",
					types.PropertyDescription,
				).Scan(&valueType)
				require.NoError(t, err)
				assert.Equal(t, types.ValueTypeText, valueType)
			},
		},
		{
			name:  "seeds owner property with text type",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var valueType string
				err := db.QueryRow(
					"SELECT value_type FROM properties WHERE name = ?",
					types.PropertyOwner,
				).Scan(&valueType)
				require.NoError(t, err)
				assert.Equal(t, types.ValueTypeText, valueType)
			},
		},
		{
			name:  "seeds labels property with list type",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var valueType string
				err := db.QueryRow(
					"SELECT value_type FROM properties WHERE name = ?",
					types.PropertyLabels,
				).Scan(&valueType)
				require.NoError(t, err)
				assert.Equal(t, types.ValueTypeList, valueType)
			},
		},
		{
			name:  "seeds five priority categories with correct ordinals",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var propID string
				err := db.QueryRow(
					"SELECT property_id FROM properties WHERE name = ?",
					types.PropertyPriority,
				).Scan(&propID)
				require.NoError(t, err)

				rows, err := db.Query(
					"SELECT name, ordinal FROM categories WHERE property_id = ? ORDER BY ordinal",
					propID,
				)
				require.NoError(t, err)
				defer rows.Close()

				expected := []struct {
					name    string
					ordinal int
				}{
					{"highest", 0},
					{"high", 1},
					{"medium", 2},
					{"low", 3},
					{"lowest", 4},
				}

				var i int
				for rows.Next() {
					var name string
					var ordinal int
					require.NoError(t, rows.Scan(&name, &ordinal))
					require.Less(t, i, len(expected), "more categories than expected")
					assert.Equal(t, expected[i].name, name)
					assert.Equal(t, expected[i].ordinal, ordinal)
					i++
				}
				assert.Equal(t, len(expected), i, "expected 5 priority categories")
			},
		},
		{
			name:  "seeds four type categories with correct ordinals",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				var propID string
				err := db.QueryRow(
					"SELECT property_id FROM properties WHERE name = ?",
					types.PropertyType,
				).Scan(&propID)
				require.NoError(t, err)

				rows, err := db.Query(
					"SELECT name, ordinal FROM categories WHERE property_id = ? ORDER BY ordinal",
					propID,
				)
				require.NoError(t, err)
				defer rows.Close()

				expected := []struct {
					name    string
					ordinal int
				}{
					{"task", 0},
					{"epic", 1},
					{"bug", 2},
					{"chore", 3},
				}

				var i int
				for rows.Next() {
					var name string
					var ordinal int
					require.NoError(t, rows.Scan(&name, &ordinal))
					require.Less(t, i, len(expected), "more categories than expected")
					assert.Equal(t, expected[i].name, name)
					assert.Equal(t, expected[i].ordinal, ordinal)
					i++
				}
				assert.Equal(t, len(expected), i, "expected 4 type categories")
			},
		},
		{
			name:  "seeds no categories for non-categorical properties",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				for _, propName := range []string{
					types.PropertyDescription,
					types.PropertyOwner,
					types.PropertyLabels,
				} {
					var propID string
					err := db.QueryRow(
						"SELECT property_id FROM properties WHERE name = ?",
						propName,
					).Scan(&propID)
					require.NoError(t, err)

					var count int
					err = db.QueryRow(
						"SELECT COUNT(*) FROM categories WHERE property_id = ?",
						propID,
					).Scan(&count)
					require.NoError(t, err)
					assert.Equal(t, 0, count, "expected no categories for %s", propName)
				}
			},
		},
		{
			name:  "all seeded properties have valid UUID v7 IDs",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				rows, err := db.Query("SELECT property_id FROM properties")
				require.NoError(t, err)
				defer rows.Close()

				for rows.Next() {
					var id string
					require.NoError(t, rows.Scan(&id))
					assert.NotEmpty(t, id)
					assert.Len(t, id, 36, "UUID should be 36 characters")
				}
			},
		},
		{
			name:  "all seeded categories have valid UUID v7 IDs",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				rows, err := db.Query("SELECT category_id FROM categories")
				require.NoError(t, err)
				defer rows.Close()

				for rows.Next() {
					var id string
					require.NoError(t, rows.Scan(&id))
					assert.NotEmpty(t, id)
					assert.Len(t, id, 36, "UUID should be 36 characters")
				}
			},
		},
		{
			name:  "persists seeded properties to JSONL",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				records, err := readJSONL(filepath.Join(dataDir, "properties.jsonl"))
				require.NoError(t, err)
				assert.Len(t, records, 5)

				names := make(map[string]bool)
				for _, rec := range records {
					var obj map[string]any
					require.NoError(t, json.Unmarshal(rec, &obj))
					name, ok := obj["name"].(string)
					require.True(t, ok)
					names[name] = true
				}
				assert.True(t, names[types.PropertyPriority])
				assert.True(t, names[types.PropertyType])
				assert.True(t, names[types.PropertyDescription])
				assert.True(t, names[types.PropertyOwner])
				assert.True(t, names[types.PropertyLabels])
			},
		},
		{
			name:  "persists seeded categories to JSONL",
			setup: nil,
			check: func(t *testing.T, db *sql.DB, dataDir string) {
				records, err := readJSONL(filepath.Join(dataDir, "categories.jsonl"))
				require.NoError(t, err)
				// 5 priority categories + 4 type categories = 9
				assert.Len(t, records, 9)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, dataDir := setupTestDB(t)
			if tt.setup != nil {
				tt.setup(t, db)
			}
			err := seedBuiltInProperties(db, dataDir)
			require.NoError(t, err)
			tt.check(t, db, dataDir)
		})
	}
}

func TestSeedIdempotency(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, db *sql.DB)
	}{
		{
			name: "second seed does not duplicate properties",
			check: func(t *testing.T, db *sql.DB) {
				var count int
				err := db.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 5, count)
			},
		},
		{
			name: "second seed does not duplicate categories",
			check: func(t *testing.T, db *sql.DB) {
				var count int
				err := db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count)
				require.NoError(t, err)
				// 5 priority + 4 type = 9
				assert.Equal(t, 9, count)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, dataDir := setupTestDB(t)

			err := seedBuiltInProperties(db, dataDir)
			require.NoError(t, err)

			err = seedBuiltInProperties(db, dataDir)
			require.NoError(t, err)

			tt.check(t, db)
		})
	}
}

func TestSeedWithExistingProperties(t *testing.T) {
	t.Run("does not seed when properties already exist", func(t *testing.T) {
		db, dataDir := setupTestDB(t)

		// Insert a single property before seeding.
		_, err := db.Exec(
			"INSERT INTO properties (property_id, name, description, value_type, created_at) VALUES (?, ?, ?, ?, ?)",
			"00000000-0000-0000-0000-000000000001", "custom", "A custom property", "text", "2025-01-15T10:30:00Z",
		)
		require.NoError(t, err)

		err = seedBuiltInProperties(db, dataDir)
		require.NoError(t, err)

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "seeding should not add properties when some already exist")
	})
}

func TestSeedRetrievableViaFetch(t *testing.T) {
	t.Run("seeded properties are retrievable via Backend.GetTable and Fetch", func(t *testing.T) {
		b := NewBackend()
		dataDir := t.TempDir()
		config := types.Config{
			Backend: "sqlite",
			DataDir: dataDir,
		}

		err := b.Attach(config)
		require.NoError(t, err)
		defer b.Detach()

		table, err := b.GetTable(types.TableProperties)
		require.NoError(t, err)

		results, err := table.Fetch(nil)
		require.NoError(t, err)
		assert.Len(t, results, 5)

		names := make(map[string]string)
		for _, r := range results {
			prop, ok := r.(*types.Property)
			require.True(t, ok)
			names[prop.Name] = prop.ValueType
		}

		assert.Equal(t, types.ValueTypeCategorical, names[types.PropertyPriority])
		assert.Equal(t, types.ValueTypeCategorical, names[types.PropertyType])
		assert.Equal(t, types.ValueTypeText, names[types.PropertyDescription])
		assert.Equal(t, types.ValueTypeText, names[types.PropertyOwner])
		assert.Equal(t, types.ValueTypeList, names[types.PropertyLabels])
	})
}

func TestSeedSurvivesReattach(t *testing.T) {
	t.Run("seeded properties survive detach and re-attach", func(t *testing.T) {
		dataDir := t.TempDir()
		config := types.Config{
			Backend: "sqlite",
			DataDir: dataDir,
		}

		// First attach seeds the properties.
		b := NewBackend()
		err := b.Attach(config)
		require.NoError(t, err)
		err = b.Detach()
		require.NoError(t, err)

		// Second attach loads from JSONL.
		b2 := NewBackend()
		err = b2.Attach(config)
		require.NoError(t, err)
		defer b2.Detach()

		table, err := b2.GetTable(types.TableProperties)
		require.NoError(t, err)

		results, err := table.Fetch(nil)
		require.NoError(t, err)
		assert.Len(t, results, 5, "five built-in properties should survive re-attach")

		// Verify categories also survived.
		for _, r := range results {
			prop := r.(*types.Property)
			if prop.ValueType != types.ValueTypeCategorical {
				continue
			}
			switch prop.Name {
			case types.PropertyPriority:
				cats := countCategories(t, b2, prop.PropertyID)
				assert.Equal(t, 5, cats, "priority should have 5 categories after re-attach")
			case types.PropertyType:
				cats := countCategories(t, b2, prop.PropertyID)
				assert.Equal(t, 4, cats, "type should have 4 categories after re-attach")
			}
		}
	})

	t.Run("re-attach does not duplicate properties or categories", func(t *testing.T) {
		dataDir := t.TempDir()
		config := types.Config{
			Backend: "sqlite",
			DataDir: dataDir,
		}

		// Attach three times.
		for range 3 {
			b := NewBackend()
			err := b.Attach(config)
			require.NoError(t, err)
			err = b.Detach()
			require.NoError(t, err)
		}

		b := NewBackend()
		err := b.Attach(config)
		require.NoError(t, err)
		defer b.Detach()

		table, err := b.GetTable(types.TableProperties)
		require.NoError(t, err)

		results, err := table.Fetch(nil)
		require.NoError(t, err)
		assert.Len(t, results, 5, "should still have exactly 5 properties")
	})
}

// countCategories returns the number of categories for a property by querying
// the categories SQLite table directly.
func countCategories(t *testing.T, b *Backend, propertyID string) int {
	t.Helper()
	var count int
	err := b.db.QueryRow(
		"SELECT COUNT(*) FROM categories WHERE property_id = ?",
		propertyID,
	).Scan(&count)
	require.NoError(t, err)
	return count
}
