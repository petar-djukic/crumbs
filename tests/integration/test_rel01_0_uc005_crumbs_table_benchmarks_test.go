// Benchmark tests for crumbs Table interface operations (Get, Set, Delete, Fetch)
// at various data scales. Each benchmark function creates a fresh cupboard with a
// temp directory, seeds data, and measures the target operation.
// Implements: test-rel01.0-uc005-crumbs-table-benchmarks;
//             prd001-cupboard-core R3; prd002-sqlite-backend R12-R15;
//             prd003-crumbs-interface R3, R6-R10.
package integration

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// benchBackend creates a fresh SQLite backend attached to a temp directory for
// benchmarks. The caller must call cleanup when done.
func benchBackend(b *testing.B) (*sqlite.Backend, func()) {
	b.Helper()
	dataDir := b.TempDir()
	backend := sqlite.NewBackend()
	cfg := types.Config{Backend: "sqlite", DataDir: dataDir}
	if err := backend.Attach(cfg); err != nil {
		b.Fatalf("Attach failed: %v", err)
	}
	return backend, func() { backend.Detach() }
}

// seedCrumbs creates n crumbs in the table and returns their IDs.
func seedCrumbs(b *testing.B, tbl types.Table, n int) []string {
	b.Helper()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id, err := tbl.Set("", &types.Crumb{Name: fmt.Sprintf("crumb-%d", i)})
		if err != nil {
			b.Fatalf("seeding crumb %d: %v", i, err)
		}
		ids[i] = id
	}
	return ids
}

// seedCrumbsWithStates creates n crumbs cycling through the given states and
// returns their IDs.
func seedCrumbsWithStates(b *testing.B, tbl types.Table, n int, states []string) []string {
	b.Helper()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		state := states[i%len(states)]
		crumb := &types.Crumb{Name: fmt.Sprintf("crumb-%d", i)}
		id, err := tbl.Set("", crumb)
		if err != nil {
			b.Fatalf("seeding crumb %d: %v", i, err)
		}
		// Update to target state (Set creates in draft).
		if state != types.StateDraft {
			got, err := tbl.Get(id)
			if err != nil {
				b.Fatalf("getting crumb %d for state update: %v", i, err)
			}
			c := got.(*types.Crumb)
			c.State = state
			if _, err := tbl.Set(id, c); err != nil {
				b.Fatalf("updating crumb %d state: %v", i, err)
			}
		}
		ids[i] = id
	}
	return ids
}

// --- Get benchmarks (S5: 4 scales) ---

func BenchmarkCrumbsGet10_UC005(b *testing.B) {
	benchmarkCrumbsGet(b, 10)
}

func BenchmarkCrumbsGet100_UC005(b *testing.B) {
	benchmarkCrumbsGet(b, 100)
}

func BenchmarkCrumbsGet1000_UC005(b *testing.B) {
	benchmarkCrumbsGet(b, 1000)
}

func BenchmarkCrumbsGet10000_UC005(b *testing.B) {
	benchmarkCrumbsGet(b, 10000)
}

func benchmarkCrumbsGet(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableCrumbs)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	ids := seedCrumbs(b, tbl, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[rand.Intn(len(ids))]
		if _, err := tbl.Get(id); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// --- Set create benchmarks (S6: 2 scales) ---

func BenchmarkCrumbsSetCreate10_UC005(b *testing.B) {
	benchmarkCrumbsSetCreate(b, 10)
}

func BenchmarkCrumbsSetCreate1000_UC005(b *testing.B) {
	benchmarkCrumbsSetCreate(b, 1000)
}

func benchmarkCrumbsSetCreate(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableCrumbs)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	seedCrumbs(b, tbl, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tbl.Set("", &types.Crumb{Name: fmt.Sprintf("new-%d", i)}); err != nil {
			b.Fatalf("Set create: %v", err)
		}
	}
}

// --- Set update benchmarks (S7: 2 scales) ---

func BenchmarkCrumbsSetUpdate10_UC005(b *testing.B) {
	benchmarkCrumbsSetUpdate(b, 10)
}

func BenchmarkCrumbsSetUpdate1000_UC005(b *testing.B) {
	benchmarkCrumbsSetUpdate(b, 1000)
}

func benchmarkCrumbsSetUpdate(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableCrumbs)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	ids := seedCrumbs(b, tbl, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[rand.Intn(len(ids))]
		got, err := tbl.Get(id)
		if err != nil {
			b.Fatalf("Get for update: %v", err)
		}
		c := got.(*types.Crumb)
		c.Name = fmt.Sprintf("updated-%d", i)
		if _, err := tbl.Set(id, c); err != nil {
			b.Fatalf("Set update: %v", err)
		}
	}
}

// --- Delete benchmarks (S8: 2 scales) ---

func BenchmarkCrumbsDelete10_UC005(b *testing.B) {
	benchmarkCrumbsDelete(b, 10)
}

func BenchmarkCrumbsDelete1000_UC005(b *testing.B) {
	benchmarkCrumbsDelete(b, 1000)
}

func benchmarkCrumbsDelete(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableCrumbs)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	seedCrumbs(b, tbl, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a crumb, then delete it. Use StopTimer/StartTimer to exclude
		// the create cost from measurement.
		b.StopTimer()
		id, err := tbl.Set("", &types.Crumb{Name: fmt.Sprintf("del-%d", i)})
		if err != nil {
			b.Fatalf("Set for delete: %v", err)
		}
		b.StartTimer()

		if err := tbl.Delete(id); err != nil {
			b.Fatalf("Delete: %v", err)
		}
	}
}

// --- Fetch all benchmarks (S9: 2 scales) ---

func BenchmarkCrumbsFetchAll100_UC005(b *testing.B) {
	benchmarkCrumbsFetchAll(b, 100)
}

func BenchmarkCrumbsFetchAll1000_UC005(b *testing.B) {
	benchmarkCrumbsFetchAll(b, 1000)
}

func benchmarkCrumbsFetchAll(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableCrumbs)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	seedCrumbs(b, tbl, scale)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := tbl.Fetch(nil)
		if err != nil {
			b.Fatalf("Fetch all: %v", err)
		}
		if len(results) < scale {
			b.Fatalf("expected at least %d results, got %d", scale, len(results))
		}
	}
}

// --- Fetch with state filter benchmarks (S10: 2 scales) ---

func BenchmarkCrumbsFetchState100_UC005(b *testing.B) {
	benchmarkCrumbsFetchState(b, 100)
}

func BenchmarkCrumbsFetchState1000_UC005(b *testing.B) {
	benchmarkCrumbsFetchState(b, 1000)
}

func benchmarkCrumbsFetchState(b *testing.B, scale int) {
	backend, cleanup := benchBackend(b)
	defer cleanup()

	tbl, err := backend.GetTable(types.TableCrumbs)
	if err != nil {
		b.Fatalf("GetTable: %v", err)
	}

	states := []string{types.StateDraft, types.StatePending, types.StateReady}
	seedCrumbsWithStates(b, tbl, scale, states)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := tbl.Fetch(types.Filter{"states": []string{types.StateDraft}})
		if err != nil {
			b.Fatalf("Fetch state: %v", err)
		}
		if len(results) == 0 {
			b.Fatal("expected at least one result for state filter")
		}
	}
}
