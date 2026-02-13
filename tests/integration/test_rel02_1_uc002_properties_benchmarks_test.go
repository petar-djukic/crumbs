// Benchmark tests for properties and categories Table interface operations
// at various data scales. Each benchmark function creates a fresh cupboard with a
// temp directory, seeds data, and measures the target operation.
// Implements: test-rel02.1-uc002-table-benchmarks (property and category operations);
//             prd001-cupboard-core R3; prd002-sqlite-backend R12-R15;
//             prd004-properties-interface R1-R10.
package integration

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// seedProperties creates n properties in the table and returns their IDs.
func seedProperties(b *testing.B, tbl types.Table, n int) []string {
	b.Helper()
	ids := make([]string, n)
	valueTypes := []string{
		types.ValueTypeCategorical,
		types.ValueTypeText,
		types.ValueTypeInteger,
		types.ValueTypeBoolean,
		types.ValueTypeTimestamp,
		types.ValueTypeList,
	}
	for i := 0; i < n; i++ {
		valueType := valueTypes[i%len(valueTypes)]
		prop := &types.Property{
			Name:      fmt.Sprintf("property-%d", i),
			ValueType: valueType,
		}
		id, err := tbl.Set("", prop)
		if err != nil {
			b.Fatalf("seeding property %d: %v", i, err)
		}
		ids[i] = id
	}
	return ids
}

// seedCategoricalProperty creates a categorical property and returns its ID.
func seedCategoricalProperty(b *testing.B, tbl types.Table, name string) string {
	b.Helper()
	prop := &types.Property{
		Name:      name,
		ValueType: types.ValueTypeCategorical,
	}
	id, err := tbl.Set("", prop)
	if err != nil {
		b.Fatalf("seeding categorical property %s: %v", name, err)
	}
	return id
}

// seedCategories creates n categories for the given property and returns their IDs.
func seedCategories(b *testing.B, tbl types.Table, propertyID string, n int) []string {
	b.Helper()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		cat := &types.Category{
			PropertyID: propertyID,
			Name:       fmt.Sprintf("category-%d", i),
			Ordinal:    i,
		}
		id, err := tbl.Set("", cat)
		if err != nil {
			b.Fatalf("seeding category %d: %v", i, err)
		}
		ids[i] = id
	}
	return ids
}

// --- Properties Get benchmarks (3 scales: 10, 100, 1000) ---

func BenchmarkPropertiesGet10_UC002(b *testing.B) {
	benchmarkPropertiesGet(b, 10)
}

func BenchmarkPropertiesGet100_UC002(b *testing.B) {
	benchmarkPropertiesGet(b, 100)
}

func BenchmarkPropertiesGet1000_UC002(b *testing.B) {
	benchmarkPropertiesGet(b, 1000)
}

func benchmarkPropertiesGet(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableProperties)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	ids := seedProperties(b, tbl, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[rand.Intn(len(ids))]
		if _, err := tbl.Get(id); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// --- Properties Set create benchmarks (2 scales: 10, 100) ---

func BenchmarkPropertiesSetCreate10_UC002(b *testing.B) {
	benchmarkPropertiesSetCreate(b, 10)
}

func BenchmarkPropertiesSetCreate100_UC002(b *testing.B) {
	benchmarkPropertiesSetCreate(b, 100)
}

func benchmarkPropertiesSetCreate(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableProperties)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	// Seed properties to establish baseline dataset.
	seedProperties(b, tbl, scale)

	// Benchmark measures creation including JSONL sync cost.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prop := &types.Property{
			Name:      fmt.Sprintf("new-property-%d", i),
			ValueType: types.ValueTypeText,
		}
		if _, err := tbl.Set("", prop); err != nil {
			b.Fatalf("Set create: %v", err)
		}
	}
}

// --- Properties Fetch benchmarks ---

func BenchmarkPropertiesFetchAll_UC002(b *testing.B) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableProperties)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	// Seed 100 properties for fetch all.
	seedProperties(b, tbl, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := tbl.Fetch(nil)
		if err != nil {
			b.Fatalf("Fetch all: %v", err)
		}
		// Verify results include built-in properties plus seeded ones.
		if len(results) < 100 {
			b.Fatalf("expected at least 100 results, got %d", len(results))
		}
	}
}

func BenchmarkPropertiesFetchByValueType_UC002(b *testing.B) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableProperties)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	// Seed 100 properties cycling through value types.
	seedProperties(b, tbl, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := tbl.Fetch(types.Filter{"value_type": types.ValueTypeCategorical})
		if err != nil {
			b.Fatalf("Fetch by value_type: %v", err)
		}
		if len(results) == 0 {
			b.Fatal("expected at least one result for value_type filter")
		}
	}
}

// --- Categories Get benchmarks (2 scales: 10, 100) ---

func BenchmarkCategoriesGet10_UC002(b *testing.B) {
	benchmarkCategoriesGet(b, 10)
}

func BenchmarkCategoriesGet100_UC002(b *testing.B) {
	benchmarkCategoriesGet(b, 100)
}

func benchmarkCategoriesGet(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	propTbl, err := backend.GetTable(types.TableProperties)
	if err != nil {
		b.Fatalf("GetTable properties: %v", err)
	}

	catTbl, err := backend.GetTable(types.TableCategories)
	if err != nil {
		b.Fatalf("GetTable categories: %v", err)
	}

	// Seed one categorical property.
	propID := seedCategoricalProperty(b, propTbl, "benchmark-property")

	// Seed categories for that property.
	ids := seedCategories(b, catTbl, propID, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[rand.Intn(len(ids))]
		if _, err := catTbl.Get(id); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// --- Categories Fetch benchmarks ---

func BenchmarkCategoriesFetchByProperty_UC002(b *testing.B) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	propTbl, err := backend.GetTable(types.TableProperties)
	if err != nil {
		b.Fatalf("GetTable properties: %v", err)
	}

	catTbl, err := backend.GetTable(types.TableCategories)
	if err != nil {
		b.Fatalf("GetTable categories: %v", err)
	}

	// Seed one categorical property.
	propID := seedCategoricalProperty(b, propTbl, "benchmark-property")

	// Seed 100 categories for that property.
	seedCategories(b, catTbl, propID, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := catTbl.Fetch(types.Filter{"property_id": propID})
		if err != nil {
			b.Fatalf("Fetch by property_id: %v", err)
		}
		if len(results) < 100 {
			b.Fatalf("expected at least 100 results, got %d", len(results))
		}
	}
}
