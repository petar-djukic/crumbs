package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Test groups test targets (all, unit, integration).
type Test mg.Namespace

// All runs all tests (unit and integration).
func (Test) All() error {
	return sh.RunV(binGo, "test", "-v", "./...")
}

// Unit runs only unit tests, excluding the tests/ directory.
func (Test) Unit() error {
	pkgs, err := sh.Output(binGo, "list", "./...")
	if err != nil {
		return err
	}
	var unitPkgs []string
	for pkg := range strings.SplitSeq(pkgs, "\n") {
		if pkg != "" && !strings.Contains(pkg, "/tests/") && !strings.HasSuffix(pkg, "/tests") {
			unitPkgs = append(unitPkgs, pkg)
		}
	}
	if len(unitPkgs) == 0 {
		fmt.Println("No unit test packages found.")
		return nil
	}
	args := append([]string{"test", "-v"}, unitPkgs...)
	return sh.RunV(binGo, args...)
}

// Integration builds first, then runs only integration tests.
func (Test) Integration() error {
	if _, err := os.Stat("tests"); os.IsNotExist(err) {
		fmt.Println("No integration test directory found (tests/).")
		return nil
	}
	mg.Deps(Build)
	return sh.RunV(binGo, "test", "-v", "./tests/...")
}

// Cobbler runs the cobbler regression suite: measure creates 3 issues,
// stitch resolves them, then verifies all are closed and no branches
// or worktrees leak. Resets beads before and after.
//
// Logs each step with timestamps and elapsed time. Reports LOC created
// and issue counts in a final summary.
//
// Requires: bd and claude CLIs on PATH.
func (Test) Cobbler() error {
	suiteStart := time.Now()
	logStep := func(name string) time.Time {
		now := time.Now()
		fmt.Printf("\n[%s] --- %s ---\n", now.Format(time.RFC3339), name)
		return now
	}
	logDone := func(name string, start time.Time) {
		fmt.Printf("[%s] %s completed in %s\n", time.Now().Format(time.RFC3339), name, time.Since(start).Round(time.Second))
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Cobbler regression test")
	fmt.Println("========================================")

	// Snapshot LOC before any work.
	statsBefore, err := collectStats()
	if err != nil {
		return fmt.Errorf("collecting baseline stats: %w", err)
	}
	logf("test:cobbler: baseline LOC: prod=%d test=%d", statsBefore.GoProdLOC, statsBefore.GoTestLOC)

	// Step: reset beads.
	t := logStep("setup: reset beads")
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}
	logDone("setup", t)

	// Step: measure (create 3 issues).
	t = logStep("measure: create 3 issues")
	mCfg := measureConfig{cobblerConfig: cobblerConfig{
		silenceAgent: true,
		maxIssues:    3,
		noContainer:  true,
	}}
	if err := measure(mCfg); err != nil {
		return fmt.Errorf("measure: %w", err)
	}
	logDone("measure", t)

	// Step: verify issue count.
	t = logStep("verify issue count")
	issueCount, err := countIssues(bdListJSON)
	if err != nil {
		return fmt.Errorf("counting issues: %w", err)
	}
	logf("test:cobbler: issues created by measure: %d", issueCount)
	if issueCount != 3 {
		return fmt.Errorf("expected 3 issues, got %d", issueCount)
	}
	logDone("verify issue count", t)

	// Step: stitch (resolve all issues).
	t = logStep("stitch: resolve all issues")
	sCfg := stitchConfig{cobblerConfig: cobblerConfig{
		silenceAgent: true,
		noContainer:  true,
	}}
	if err := stitch(sCfg); err != nil {
		return fmt.Errorf("stitch: %w", err)
	}
	logDone("stitch", t)

	// Step: verify all closed.
	t = logStep("verify all closed")
	closedCount, err := countIssues(bdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed issues: %w", err)
	}
	logf("test:cobbler: closed issues: %d", closedCount)
	if closedCount != 3 {
		return fmt.Errorf("expected 3 closed issues, got %d", closedCount)
	}
	logDone("verify all closed", t)

	// Step: verify no stale branches.
	t = logStep("verify no stale branches")
	branch, _ := gitCurrentBranch()
	taskBranches := gitListBranches(taskBranchPattern(branch))
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	logf("test:cobbler: no stale task branches")
	logDone("verify no stale branches", t)

	// Snapshot LOC after stitch.
	statsAfter, err := collectStats()
	if err != nil {
		return fmt.Errorf("collecting final stats: %w", err)
	}

	// Step: cleanup.
	t = logStep("cleanup: reset beads")
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}
	logDone("cleanup", t)

	// Summary.
	totalDuration := time.Since(suiteStart).Round(time.Second)
	prodDelta := statsAfter.GoProdLOC - statsBefore.GoProdLOC
	testDelta := statsAfter.GoTestLOC - statsBefore.GoTestLOC

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Cobbler regression test PASSED")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Total time:       %s\n", totalDuration)
	fmt.Printf("  Issues created:   %d\n", issueCount)
	fmt.Printf("  Issues resolved:  %d\n", closedCount)
	fmt.Printf("  LOC prod:         %d -> %d (%+d)\n", statsBefore.GoProdLOC, statsAfter.GoProdLOC, prodDelta)
	fmt.Printf("  LOC test:         %d -> %d (%+d)\n", statsBefore.GoTestLOC, statsAfter.GoTestLOC, testDelta)
	fmt.Println()
	return nil
}

// countIssues calls a bd list function and returns the number of issues
// in the JSON array response.
func countIssues(listFn func() ([]byte, error)) (int, error) {
	out, err := listFn()
	if err != nil {
		return 0, err
	}
	var issues []json.RawMessage
	if err := json.Unmarshal(out, &issues); err != nil {
		return 0, fmt.Errorf("parsing JSON: %w", err)
	}
	return len(issues), nil
}
