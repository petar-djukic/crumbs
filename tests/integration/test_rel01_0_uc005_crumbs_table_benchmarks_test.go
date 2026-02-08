// Crumbs Table performance benchmarks for core operations.
// Validates: docs/specs/test-suites/test-rel01.0-uc005-crumbs-table-benchmarks.yaml
// Implements: docs/specs/use-cases/rel01.0-uc005-crumbs-table-benchmarks.yaml;
//
//	prd001-cupboard-core R3 (Table interface);
//	prd002-sqlite-backend R14, R15 (entity hydration, JSONL persistence).
//
// This file contains benchmarks for core crumbs Table operations:
// Get, Set (create), Set (update), Delete, Fetch (all), Fetch (with state filter).
// Property operation benchmarks are out of scope for rel01.0; see test003 for those.
package integration

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// --- Get benchmarks (rel01.0-uc005 F3-F6, S5) ---

// BenchmarkCrumbsGet10_UC005 measures Table.Get latency with 10 seeded crumbs.
// Uses random ID selection to measure average lookup time.
func BenchmarkCrumbsGet10_UC005(b *testing.B) {
	benchmarkCrumbsGetUC005(b, 10)
}

// BenchmarkCrumbsGet100_UC005 measures Table.Get latency with 100 seeded crumbs.
func BenchmarkCrumbsGet100_UC005(b *testing.B) {
	benchmarkCrumbsGetUC005(b, 100)
}

// BenchmarkCrumbsGet1000_UC005 measures Table.Get latency with 1000 seeded crumbs.
func BenchmarkCrumbsGet1000_UC005(b *testing.B) {
	benchmarkCrumbsGetUC005(b, 1000)
}

// BenchmarkCrumbsGet10000_UC005 measures Table.Get latency with 10000 seeded crumbs.
func BenchmarkCrumbsGet10000_UC005(b *testing.B) {
	benchmarkCrumbsGetUC005(b, 10000)
}

func benchmarkCrumbsGetUC005(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	ids := env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		idx := rand.Intn(len(ids))
		_, err := table.Get(ids[idx])
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// --- Set (create) benchmarks (rel01.0-uc005 F7-F8, S6) ---

// BenchmarkCrumbsSetCreate10_UC005 measures Table.Set latency for creating new crumbs
// with 10 existing crumbs. Empty ID triggers UUID v7 generation.
func BenchmarkCrumbsSetCreate10_UC005(b *testing.B) {
	benchmarkCrumbsSetCreateUC005(b, 10)
}

// BenchmarkCrumbsSetCreate1000_UC005 measures Table.Set latency for creating new crumbs
// with 1000 existing crumbs.
func BenchmarkCrumbsSetCreate1000_UC005(b *testing.B) {
	benchmarkCrumbsSetCreateUC005(b, 1000)
}

func benchmarkCrumbsSetCreateUC005(b *testing.B, existingSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	_ = env.SeedCrumbs(b, existingSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		crumb := &types.Crumb{
			Name:  fmt.Sprintf("New crumb %d", i),
			State: types.StateDraft,
		}
		_, err := table.Set("", crumb)
		if err != nil {
			b.Fatalf("Set (create) failed: %v", err)
		}
	}
}

// --- Set (update) benchmarks (rel01.0-uc005 F9-F10, S7) ---

// BenchmarkCrumbsSetUpdate10_UC005 measures Table.Set latency for updating existing crumbs
// with 10 seeded crumbs. Uses random ID selection and updates Name field.
func BenchmarkCrumbsSetUpdate10_UC005(b *testing.B) {
	benchmarkCrumbsSetUpdateUC005(b, 10)
}

// BenchmarkCrumbsSetUpdate1000_UC005 measures Table.Set latency for updating existing crumbs
// with 1000 seeded crumbs.
func BenchmarkCrumbsSetUpdate1000_UC005(b *testing.B) {
	benchmarkCrumbsSetUpdateUC005(b, 1000)
}

func benchmarkCrumbsSetUpdateUC005(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	ids := env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		idx := rand.Intn(len(ids))
		crumb := &types.Crumb{
			CrumbID: ids[idx],
			Name:    fmt.Sprintf("Updated crumb %d", i),
			State:   types.StatePending,
		}
		_, err := table.Set(ids[idx], crumb)
		if err != nil {
			b.Fatalf("Set (update) failed: %v", err)
		}
	}
}

// --- Delete benchmarks (rel01.0-uc005 F11-F12, S8) ---

// BenchmarkCrumbsDelete10_UC005 measures Table.Delete latency with 10 seeded crumbs.
// Each iteration creates then deletes a crumb to measure the full delete path.
// StopTimer/StartTimer excludes create cost from measurement.
func BenchmarkCrumbsDelete10_UC005(b *testing.B) {
	benchmarkCrumbsDeleteUC005(b, 10)
}

// BenchmarkCrumbsDelete1000_UC005 measures Table.Delete latency with 1000 seeded crumbs.
func BenchmarkCrumbsDelete1000_UC005(b *testing.B) {
	benchmarkCrumbsDeleteUC005(b, 1000)
}

func benchmarkCrumbsDeleteUC005(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	_ = env.SeedCrumbs(b, dataSize)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		crumb := &types.Crumb{
			Name:  fmt.Sprintf("Delete target %d", i),
			State: types.StateDraft,
		}
		id, err := table.Set("", crumb)
		if err != nil {
			b.Fatalf("failed to create crumb for deletion: %v", err)
		}
		b.StartTimer()

		err = table.Delete(id)
		if err != nil {
			b.Fatalf("Delete failed: %v", err)
		}
	}
}

// --- Fetch (all) benchmarks (rel01.0-uc005 F13-F14, S9) ---

// BenchmarkCrumbsFetchAll100_UC005 measures Table.Fetch latency with empty filter
// returning all 100 seeded crumbs. Exercises full table scan with entity hydration.
func BenchmarkCrumbsFetchAll100_UC005(b *testing.B) {
	benchmarkCrumbsFetchAllUC005(b, 100)
}

// BenchmarkCrumbsFetchAll1000_UC005 measures Table.Fetch latency with empty filter
// returning all 1000 seeded crumbs.
func BenchmarkCrumbsFetchAll1000_UC005(b *testing.B) {
	benchmarkCrumbsFetchAllUC005(b, 1000)
}

func benchmarkCrumbsFetchAllUC005(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	_ = env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, err := table.Fetch(nil)
		if err != nil {
			b.Fatalf("Fetch failed: %v", err)
		}
		if len(results) != dataSize {
			b.Fatalf("expected %d results, got %d", dataSize, len(results))
		}
	}
}

// --- Fetch (with state filter) benchmarks (rel01.0-uc005 F15-F16, S10) ---

// BenchmarkCrumbsFetchState100_UC005 measures Table.Fetch latency with state filter
// on 100 seeded crumbs. Exercises idx_crumbs_state index per prd002-sqlite-backend R3.3.
func BenchmarkCrumbsFetchState100_UC005(b *testing.B) {
	benchmarkCrumbsFetchStateUC005(b, 100)
}

// BenchmarkCrumbsFetchState1000_UC005 measures Table.Fetch latency with state filter
// on 1000 seeded crumbs.
func BenchmarkCrumbsFetchState1000_UC005(b *testing.B) {
	benchmarkCrumbsFetchStateUC005(b, 1000)
}

func benchmarkCrumbsFetchStateUC005(b *testing.B, dataSize int) {
	env := NewBenchEnv(b)
	defer env.Cleanup()

	// SeedCrumbs cycles states: draft, pending, ready
	// So ~1/3 of crumbs will be draft
	_ = env.SeedCrumbs(b, dataSize)

	table, err := env.Backend.GetTable(types.CrumbsTable)
	if err != nil {
		b.Fatalf("failed to get crumbs table: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		filter := map[string]any{"states": []string{types.StateDraft}}
		_, err := table.Fetch(filter)
		if err != nil {
			b.Fatalf("Fetch with state filter failed: %v", err)
		}
	}
}
