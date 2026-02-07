# Use Case: Table Benchmarks

## Summary

A developer runs Go benchmarks against the crumbs Table interface with varying data sizes and measures operation latency. The benchmarks establish baseline performance numbers that future releases must not regress.

## Actor and Trigger

The actor is a developer or CI pipeline validating performance. The trigger is any code change that could affect Table operation latency, such as modifying the SQLite backend, changing entity hydration, or altering JSONL persistence.

## Flow

This use case validates performance characteristics of the Table interface (prd-cupboard-core R3) with the SQLite backend (prd-sqlite-backend). The tracer bullet measures latency for all CRUD operations at multiple data scales. Benchmarks live in Go test files using `testing.B`, not a separate tool.

1. Create benchmark test files in the internal/sqlite package.

```go
// internal/sqlite/crumbs_bench_test.go
package sqlite_test

import (
    "testing"
    // imports
)

func BenchmarkCrumbsGet(b *testing.B) {
    // Setup: Create cupboard, attach, seed data
    // Run b.N iterations of Table.Get
    // Teardown: Detach
}
```

Benchmarks follow standard Go conventions. Each benchmark function receives a `*testing.B` and runs the target operation `b.N` times.

2. Implement Get benchmarks at different scales.

Table 1: Get benchmark functions

| Benchmark | Data Size | Operation | Reference |
|-----------|-----------|-----------|-----------|
| BenchmarkCrumbsGet10 | 10 crumbs | Table.Get by ID | prd-cupboard-core R3.2 |
| BenchmarkCrumbsGet100 | 100 crumbs | Table.Get by ID | prd-cupboard-core R3.2 |
| BenchmarkCrumbsGet1000 | 1,000 crumbs | Table.Get by ID | prd-cupboard-core R3.2 |
| BenchmarkCrumbsGet10000 | 10,000 crumbs | Table.Get by ID | prd-cupboard-core R3.2 |

Each benchmark seeds the specified number of crumbs before the timer starts (`b.ResetTimer()`). Get operations use random selection from the seeded IDs to measure average lookup time across the dataset.

3. Implement Set benchmarks (create and update).

Table 2: Set benchmark functions

| Benchmark | Data Size | Operation | Reference |
|-----------|-----------|-----------|-----------|
| BenchmarkCrumbsSetCreate10 | 10 existing | Table.Set with empty ID (create) | prd-cupboard-core R3.3 |
| BenchmarkCrumbsSetCreate1000 | 1,000 existing | Table.Set with empty ID (create) | prd-cupboard-core R3.3 |
| BenchmarkCrumbsSetUpdate10 | 10 crumbs | Table.Set with existing ID (update) | prd-cupboard-core R3.3 |
| BenchmarkCrumbsSetUpdate1000 | 1,000 crumbs | Table.Set with existing ID (update) | prd-cupboard-core R3.3 |

Set benchmarks measure both creation (empty ID triggers UUID v7 generation per prd-cupboard-core R8) and updates (existing ID triggers update path). The JSONL sync cost is included in the measurement since every write syncs to JSONL with the immediate strategy (prd-sqlite-backend R5.3, R16.2).

4. Implement Delete benchmarks.

Table 3: Delete benchmark functions

| Benchmark | Data Size | Operation | Reference |
|-----------|-----------|-----------|-----------|
| BenchmarkCrumbsDelete10 | 10 crumbs | Table.Delete by ID | prd-cupboard-core R3.4 |
| BenchmarkCrumbsDelete1000 | 1,000 crumbs | Table.Delete by ID | prd-cupboard-core R3.4 |

Delete benchmarks measure cascade deletion including property values, metadata, and links (prd-crumbs-interface R8.3). Each iteration creates a crumb, then deletes it, measuring the full delete path including JSONL persistence.

5. Implement Fetch benchmarks with filters.

Table 4: Fetch benchmark functions

| Benchmark | Data Size | Operation | Reference |
|-----------|-----------|-----------|-----------|
| BenchmarkCrumbsFetchAll100 | 100 crumbs | Table.Fetch with empty filter | prd-cupboard-core R3.5 |
| BenchmarkCrumbsFetchAll1000 | 1,000 crumbs | Table.Fetch with empty filter | prd-cupboard-core R3.5 |
| BenchmarkCrumbsFetchState100 | 100 crumbs | Table.Fetch with states filter | prd-crumbs-interface R9.2 |
| BenchmarkCrumbsFetchState1000 | 1,000 crumbs | Table.Fetch with states filter | prd-crumbs-interface R9.2 |
| BenchmarkCrumbsFetchProperties100 | 100 crumbs | Table.Fetch with properties filter | prd-crumbs-interface R9.2 |
| BenchmarkCrumbsFetchProperties1000 | 1,000 crumbs | Table.Fetch with properties filter | prd-crumbs-interface R9.2 |
| BenchmarkCrumbsFetchLimit100 | 1,000 crumbs | Table.Fetch with limit 10 | prd-crumbs-interface R9.2 |

Fetch benchmarks measure query performance with different filter types. The state filter exercises the `idx_crumbs_state` index (prd-sqlite-backend R3.3). Property filters exercise the `crumb_properties` table join.

6. Implement property operation benchmarks.

Table 5: Property benchmark functions

| Benchmark | Data Size | Operation | Reference |
|-----------|-----------|-----------|-----------|
| BenchmarkCrumbSetProperty10 | 10 crumbs | Crumb.SetProperty + Table.Set | prd-crumbs-interface R5.2 |
| BenchmarkCrumbSetProperty1000 | 1,000 crumbs | Crumb.SetProperty + Table.Set | prd-crumbs-interface R5.2 |
| BenchmarkCrumbGetProperty10 | 10 crumbs | Crumb.GetProperty | prd-crumbs-interface R5.3 |
| BenchmarkCrumbGetProperty1000 | 1,000 crumbs | Crumb.GetProperty | prd-crumbs-interface R5.3 |

Property benchmarks measure the overhead of property access. SetProperty includes Table.Set to measure the full persist path including JSONL write for `crumb_properties.jsonl` (prd-sqlite-backend R5.5).

7. Run benchmarks and capture baseline.

```bash
go test -bench=. -benchmem ./internal/sqlite/... > benchmark_baseline.txt
```

The `-benchmem` flag reports memory allocations per operation. Output format follows Go conventions:

```
BenchmarkCrumbsGet10-8         50000         23456 ns/op         1024 B/op         12 allocs/op
BenchmarkCrumbsGet1000-8       45000         25678 ns/op         1024 B/op         12 allocs/op
```

8. Compare against baseline on code changes.

```bash
go test -bench=. -benchmem ./internal/sqlite/... > benchmark_current.txt
benchstat benchmark_baseline.txt benchmark_current.txt
```

The `benchstat` tool (from `golang.org/x/perf/cmd/benchstat`) provides statistical comparison. Regressions exceeding 10% warrant investigation.

## Architecture Touchpoints

Table 6: Components and references

| Component | Operation | Reference |
|-----------|-----------|-----------|
| Table interface | Get, Set, Delete, Fetch | prd-cupboard-core R3 |
| SQLite backend | Entity hydration from rows | prd-sqlite-backend R14 |
| SQLite backend | Entity persistence to rows | prd-sqlite-backend R15 |
| SQLite backend | JSONL atomic write | prd-sqlite-backend R5.2 |
| SQLite backend | Immediate sync strategy | prd-sqlite-backend R16.2 |
| Crumb entity | SetProperty, GetProperty | prd-crumbs-interface R5 |
| Crumb table | Filter map queries | prd-crumbs-interface R9 |

This use case exercises:

- **Table CRUD operations**: Get, Set, Delete, Fetch at various scales (prd-cupboard-core R3)
- **Entity hydration**: SQLite row to Crumb struct conversion (prd-sqlite-backend R14)
- **Entity persistence**: Crumb struct to SQLite row and JSONL (prd-sqlite-backend R15, R5)
- **Property operations**: SetProperty and GetProperty entity methods (prd-crumbs-interface R5)
- **JSONL write cost**: Every Set includes JSONL persistence (prd-sqlite-backend R5.3)
- **Index performance**: State filters use `idx_crumbs_state` (prd-sqlite-backend R3.3)

## Success / Demo Criteria

Run benchmarks and verify observable outputs.

Table 7: Demo script

| Step | Command | Verify |
|------|---------|--------|
| 1 | `go test -bench=BenchmarkCrumbsGet -benchmem ./internal/sqlite/...` | Benchmarks run without error |
| 2 | `go test -bench=BenchmarkCrumbsSet -benchmem ./internal/sqlite/...` | Create and update benchmarks complete |
| 3 | `go test -bench=BenchmarkCrumbsFetch -benchmem ./internal/sqlite/...` | Fetch benchmarks complete for all filter types |
| 4 | `go test -bench=. -benchmem ./internal/sqlite/... > baseline.txt` | Full benchmark suite captured to file |
| 5 | Check `baseline.txt` | All benchmarks report ns/op, B/op, allocs/op |

Baseline establishment criteria:

- All benchmark functions execute without error
- Each benchmark reports consistent ns/op across multiple runs (variance under 10%)
- Memory allocations (B/op, allocs/op) are captured for regression tracking
- Baseline file is committed to the repository for future comparison

Performance targets (initial baselines, to be refined with real measurements):

Table 8: Initial performance targets

| Operation | Scale | Target | Notes |
|-----------|-------|--------|-------|
| Get | 1,000 crumbs | Under 1ms p99 | Single row lookup by primary key |
| Set (create) | 1,000 existing | Under 10ms p99 | Includes UUID generation and JSONL sync |
| Set (update) | 1,000 crumbs | Under 10ms p99 | Includes JSONL sync |
| Fetch (all) | 1,000 crumbs | Under 50ms p99 | Full table scan with hydration |
| Fetch (state filter) | 1,000 crumbs | Under 10ms p99 | Index-assisted query |
| Delete | 1,000 crumbs | Under 20ms p99 | Includes cascade and JSONL sync |

These targets will be refined based on actual baseline measurements. The goal is to establish numbers that future releases must not regress beyond.

## Out of Scope

- Concurrent access benchmarks (prd-sqlite-backend R8 describes single-writer model; concurrency is not the focus here)
- Network latency (SQLite backend is local; no network overhead)
- CLI command overhead (benchmarks measure Table interface, not CLI parsing)
- Non-immediate sync strategies (prd-sqlite-backend R16.3, R16.4 are deferred performance options)
- Trail and stash operations (those entity types are in later releases)

## Dependencies

- SQLite backend with JSONL persistence (prd-sqlite-backend R1–R5)
- Table interface with Get/Set/Delete/Fetch (prd-cupboard-core R3)
- Crumb entity with property methods (prd-crumbs-interface R1, R5)
- Built-in properties seeded (prd-sqlite-backend R9)
- Go testing infrastructure with `testing.B`

## Risks and Mitigations

Table 9: Risks

| Risk | Mitigation |
|------|------------|
| Benchmark variance makes comparisons unreliable | Run benchmarks multiple times; use `benchstat` for statistical analysis; run on consistent hardware |
| JSONL sync dominates write latency | This is expected and intentional (immediate sync strategy); document that non-immediate strategies trade durability for performance |
| Initial targets are arbitrary | Treat first run as baseline establishment; refine targets based on actual measurements and use case requirements |
| Benchmarks become stale as code evolves | Include benchmarks in CI; fail on significant regression; update targets when architecture changes |
