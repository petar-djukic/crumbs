package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
// Requires: bd and claude CLIs on PATH.
func (Test) Cobbler() error {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Cobbler regression test")
	fmt.Println("========================================")
	fmt.Println()

	// Reset beads to start clean.
	fmt.Println("--- setup: reset beads ---")
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	// Measure: create 3 issues.
	fmt.Println()
	fmt.Println("--- step 1: measure (create 3 issues) ---")
	mCfg := measureConfig{cobblerConfig: cobblerConfig{
		silenceAgent: true,
		maxIssues:    3,
		noContainer:  true,
	}}
	if err := measure(mCfg); err != nil {
		return fmt.Errorf("measure: %w", err)
	}

	// Verify 3 issues exist.
	fmt.Println()
	fmt.Println("--- step 2: verify issue count ---")
	issueCount, err := countIssues(bdListJSON)
	if err != nil {
		return fmt.Errorf("counting issues: %w", err)
	}
	fmt.Printf("Issues after measure: %d\n", issueCount)
	if issueCount != 3 {
		return fmt.Errorf("expected 3 issues, got %d", issueCount)
	}

	// Stitch: resolve all issues.
	fmt.Println()
	fmt.Println("--- step 3: stitch (resolve all issues) ---")
	sCfg := stitchConfig{cobblerConfig: cobblerConfig{
		silenceAgent: true,
		noContainer:  true,
	}}
	if err := stitch(sCfg); err != nil {
		return fmt.Errorf("stitch: %w", err)
	}

	// Verify all 3 closed.
	fmt.Println()
	fmt.Println("--- step 4: verify all closed ---")
	closedCount, err := countIssues(bdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed issues: %w", err)
	}
	fmt.Printf("Closed issues: %d\n", closedCount)
	if closedCount != 3 {
		return fmt.Errorf("expected 3 closed issues, got %d", closedCount)
	}

	// Verify no task branches remain.
	fmt.Println()
	fmt.Println("--- step 5: verify no stale branches ---")
	branch, _ := gitCurrentBranch()
	taskBranches := gitListBranches(taskBranchPattern(branch))
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	fmt.Println("No stale task branches.")

	// Cleanup: reset beads.
	fmt.Println()
	fmt.Println("--- cleanup: reset beads ---")
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Cobbler regression test PASSED")
	fmt.Println("========================================")
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
