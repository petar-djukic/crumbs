// Table interface performance benchmarks.
// Validates test003-table-benchmarks.yaml test cases.
// Implements: docs/test-suites/test003-table-benchmarks.yaml;
//             docs/use-cases/rel02.1-uc002-table-benchmarks.md;
//             prd-cupboard-core R3 (Table interface);
//             prd-sqlite-backend.
package integration

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// BenchEnv provides an isolated benchmark environment with a pre-configured cupboard.
type BenchEnv struct {
	TempDir string
	Backend *sqlite.Backend
}

// NewBenchEnv creates a new benchmark environment with an attached cupboard.
func NewBenchEnv(b *testing.B) *BenchEnv {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "crumbs-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	backend := sqlite.NewBackend()
	cfg := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tempDir,
	}

	if err := backend.Attach(cfg); err != nil {
		os.RemoveAll(tempDir)
		b.Fatalf("failed to attach backend: %v", err)
	}

	return &BenchEnv{
		TempDir: tempDir,
		Backend: backend,
	}
}

// Cleanup releases resources and removes the temp directory.
func (e *BenchEnv) Cleanup() {
	if e.Backend != nil {
		e.Backend.Detach()
	}
	if e.TempDir != "" {
		os.RemoveAll(e.TempDir)
	}
}

// SeedCrumbs creates n crumbs and returns their IDs.
func (e *BenchEnv) SeedCrumbs(b *testing.B, n int) []string {
	b.Helper()

	table, err := e.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	ids := make([]string, n)
	states := []string{types.StateDraft, types.StatePending, types.StateReady}

	for i := 0; i < n; i++ {
		crumb := &types.Crumb{
			Name:  fmt.Sprintf("Benchmark crumb %d", i),
			State: states[i%len(states)],
		}
		id, err := table.Set("", crumb)
		if err != nil {
			b.Fatalf("failed to seed crumb %d: %v", i, err)
		}
		ids[i] = id
	}

	return ids
}

// SeedCrumbsWithProperties creates n crumbs with a test property set.
func (e *BenchEnv) SeedCrumbsWithProperties(b *testing.B, n int, propertyID string) []string {
	b.Helper()

	table, err := e.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	ids := make([]string, n)
	states := []string{types.StateDraft, types.StatePending, types.StateReady}

	for i := 0; i < n; i++ {
		crumb := &types.Crumb{
			Name:       fmt.Sprintf("Benchmark crumb %d", i),
			State:      states[i%len(states)],
			Properties: map[string]any{propertyID: fmt.Sprintf("value-%d", i)},
		}
		id, err := table.Set("", crumb)
		if err != nil {
			b.Fatalf("failed to seed crumb %d: %v", i, err)
		}
		ids[i] = id
	}

	return ids
}

// CreateProperty creates a test property and returns its ID.
func (e *BenchEnv) CreateProperty(b *testing.B, name string) string {
	b.Helper()

	table, err := e.Backend.GetTable(types.PropertiesTable)
	if err != nil {
		b.Fatalf("failed to get properties table: %v", err)
	}

	prop := &types.Property{
		Name:        name,
		Description: "Benchmark test property",
		ValueType:   types.ValueTypeText,
	}

	id, err := table.Set("", prop)
	if err != nil {
		b.Fatalf("failed to create property: %v", err)
	}

	return id
}

// --- Get benchmarks ---

func BenchmarkCrumbsGet10(b *testing.B) {
	benchmarkCrumbsGet(b, 10)
}

func BenchmarkCrumbsGet100(b *testing.B) {
	benchmarkCrumbsGet(b, 100)
}

func BenchmarkCrumbsGet1000(b *testing.B) {
	benchmarkCrumbsGet(b, 1000)
}

func BenchmarkCrumbsGet10000(b *testing.B) {
	benchmarkCrumbsGet(b, 10000)
}

func benchmarkCrumbsGet(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Seed crumbs
	ids := env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Random ID selection to measure average lookup time
		idx := rand.Intn(len(ids))
		_, err := table.Get(ids[idx])
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// --- Set (create) benchmarks ---

func BenchmarkCrumbsSetCreate10(b *testing.B) {
	benchmarkCrumbsSetCreate(b, 10)
}

func BenchmarkCrumbsSetCreate1000(b *testing.B) {
	benchmarkCrumbsSetCreate(b, 1000)
}

func benchmarkCrumbsSetCreate(b *testing.B, existingSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Seed existing crumbs
	_ = env.SeedCrumbs(b, existingSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		crumb := &types.Crumb{
			Name:  fmt.Sprintf("New crumb %d", i),
			State: types.StateDraft,
		}
		_, err := table.Set("", crumb) // Empty ID triggers create
		if err != nil {
			b.Fatalf("Set (create) failed: %v", err)
		}
	}
}

// --- Set (update) benchmarks ---

func BenchmarkCrumbsSetUpdate10(b *testing.B) {
	benchmarkCrumbsSetUpdate(b, 10)
}

func BenchmarkCrumbsSetUpdate1000(b *testing.B) {
	benchmarkCrumbsSetUpdate(b, 1000)
}

func benchmarkCrumbsSetUpdate(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Seed crumbs
	ids := env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := rand.Intn(len(ids))
		crumb := &types.Crumb{
			CrumbID: ids[idx],
			Name:    fmt.Sprintf("Updated crumb %d", i),
			State:   types.StatePending,
		}
		_, err := table.Set(ids[idx], crumb) // Existing ID triggers update
		if err != nil {
			b.Fatalf("Set (update) failed: %v", err)
		}
	}
}

// --- Delete benchmarks ---

func BenchmarkCrumbsDelete10(b *testing.B) {
	benchmarkCrumbsDelete(b, 10)
}

func BenchmarkCrumbsDelete1000(b *testing.B) {
	benchmarkCrumbsDelete(b, 1000)
}

func benchmarkCrumbsDelete(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	// Pre-seed baseline crumbs
	_ = env.SeedCrumbs(b, dataSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create a crumb to delete
		crumb := &types.Crumb{
			Name:  fmt.Sprintf("Delete target %d", i),
			State: types.StateDraft,
		}
		id, err := table.Set("", crumb)
		if err != nil {
			b.Fatalf("failed to create crumb for deletion: %v", err)
		}
		b.StartTimer()

		// Measure delete
		err = table.Delete(id)
		if err != nil {
			b.Fatalf("Delete failed: %v", err)
		}
	}
}

// --- Fetch benchmarks ---

func BenchmarkCrumbsFetchAll100(b *testing.B) {
	benchmarkCrumbsFetchAll(b, 100)
}

func BenchmarkCrumbsFetchAll1000(b *testing.B) {
	benchmarkCrumbsFetchAll(b, 1000)
}

func benchmarkCrumbsFetchAll(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Seed crumbs
	_ = env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Empty filter returns all crumbs
		results, err := table.Fetch(nil)
		if err != nil {
			b.Fatalf("Fetch failed: %v", err)
		}
		if len(results) != dataSize {
			b.Fatalf("expected %d results, got %d", dataSize, len(results))
		}
	}
}

func BenchmarkCrumbsFetchState100(b *testing.B) {
	benchmarkCrumbsFetchState(b, 100)
}

func BenchmarkCrumbsFetchState1000(b *testing.B) {
	benchmarkCrumbsFetchState(b, 1000)
}

func benchmarkCrumbsFetchState(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Seed crumbs (states cycle through draft, pending, ready)
	_ = env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Filter by state - exercises idx_crumbs_state index
		filter := map[string]any{"State": types.StateDraft}
		_, err := table.Fetch(filter)
		if err != nil {
			b.Fatalf("Fetch with state filter failed: %v", err)
		}
	}
}

func BenchmarkCrumbsFetchProperties100(b *testing.B) {
	benchmarkCrumbsFetchProperties(b, 100)
}

func BenchmarkCrumbsFetchProperties1000(b *testing.B) {
	benchmarkCrumbsFetchProperties(b, 1000)
}

func benchmarkCrumbsFetchProperties(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Create a property first
	propertyID := env.CreateProperty(b, "bench_property")

	// Seed crumbs with that property
	_ = env.SeedCrumbsWithProperties(b, dataSize, propertyID)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Filter by state (since property filtering via Fetch is done via state for now)
		// The crumb_properties table join is exercised during hydration
		filter := map[string]any{"State": types.StateDraft}
		_, err := table.Fetch(filter)
		if err != nil {
			b.Fatalf("Fetch with properties filter failed: %v", err)
		}
	}
}

func BenchmarkCrumbsFetchLimit100(b *testing.B) {
	// Note: The current Table interface doesn't support limit in Fetch filter.
	// This benchmark measures Fetch performance from 1000 crumbs (simulating pagination intent).
	// Once limit is added to the filter, this can be updated.
	benchmarkCrumbsFetchLimit(b, 1000)
}

func benchmarkCrumbsFetchLimit(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Seed crumbs
	_ = env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Fetch all and measure (limit would be applied in filter when supported)
		// For now we fetch all to establish baseline
		results, err := table.Fetch(nil)
		if err != nil {
			b.Fatalf("Fetch with limit failed: %v", err)
		}
		// Simulate taking first 10 (limit behavior)
		if len(results) > 10 {
			_ = results[:10]
		}
	}
}

// --- Property operation benchmarks ---

func BenchmarkCrumbSetProperty10(b *testing.B) {
	benchmarkCrumbSetProperty(b, 10)
}

func BenchmarkCrumbSetProperty1000(b *testing.B) {
	benchmarkCrumbSetProperty(b, 1000)
}

func benchmarkCrumbSetProperty(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Create a property
	propertyID := env.CreateProperty(b, "bench_set_property")

	// Seed crumbs
	ids := env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := rand.Intn(len(ids))

		// Get the crumb
		result, err := table.Get(ids[idx])
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
		crumb := result.(*types.Crumb)

		// Set property
		crumb.SetProperty(propertyID, fmt.Sprintf("value-%d", i))

		// Persist (includes JSONL write for crumb_properties)
		_, err = table.Set(crumb.CrumbID, crumb)
		if err != nil {
			b.Fatalf("Set after SetProperty failed: %v", err)
		}
	}
}

func BenchmarkCrumbGetProperty10(b *testing.B) {
	benchmarkCrumbGetProperty(b, 10)
}

func BenchmarkCrumbGetProperty1000(b *testing.B) {
	benchmarkCrumbGetProperty(b, 1000)
}

func benchmarkCrumbGetProperty(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// Create a property definition first
	propertyID := env.CreateProperty(b, "bench_get_property")

	// Create crumbs with properties already in the Properties map.
	// We prepare these in-memory since GetProperty is a read-only in-memory operation.
	crumbs := make([]*types.Crumb, dataSize)
	for i := 0; i < dataSize; i++ {
		crumbs[i] = &types.Crumb{
			CrumbID: fmt.Sprintf("bench-crumb-%d", i),
			Name:    fmt.Sprintf("Benchmark crumb %d", i),
			State:   types.StateDraft,
			Properties: map[string]any{
				propertyID: fmt.Sprintf("value-%d", i),
			},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := rand.Intn(len(crumbs))
		// GetProperty is a read-only operation on the in-memory Properties map
		_, err := crumbs[idx].GetProperty(propertyID)
		if err != nil {
			b.Fatalf("GetProperty failed: %v", err)
		}
	}
}
