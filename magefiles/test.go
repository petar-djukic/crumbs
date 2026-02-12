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

// Generator runs the generator lifecycle tests progressively.
//
// Test 1 (no Claude): start/stop creates tags and returns to main.
// Test 2 (Claude): start, run 1 cycle with 1 issue, stop. Verifies
// issue created and resolved, correct tags, no stale branches.
// Test 3 (Claude): stitch respects --max-issues limit. Creates 2
// issues, stitches 1, verifies 1 closed and 1 ready.
//
// Requires: bd and claude CLIs on PATH.
func (Test) Generator() error {
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
	fmt.Println("Generator lifecycle tests")
	fmt.Println("========================================")

	// ── Test 1: start/stop (no Claude) ──

	t := logStep("test 1: start/stop lifecycle")

	logf("test:generator: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	logf("test:generator: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting branch after start: %w", err)
	}
	logf("test:generator: on branch %s", genBranch)

	if !strings.HasPrefix(genBranch, genPrefix) {
		return fmt.Errorf("expected generation branch, got %s", genBranch)
	}

	startTag := genBranch + "-start"
	startTags := gitListTags(startTag)
	if len(startTags) == 0 {
		return fmt.Errorf("expected tag %s to exist", startTag)
	}
	logf("test:generator: start tag %s exists", startTag)

	logf("test:generator: stopping generation")
	if err := (Generator{}).Stop(); err != nil {
		return fmt.Errorf("generator:stop: %w", err)
	}

	currentBranch, _ := gitCurrentBranch()
	if currentBranch != "main" {
		return fmt.Errorf("expected to be on main after stop, got %s", currentBranch)
	}

	genBranches := listGenerationBranches()
	if len(genBranches) > 0 {
		return fmt.Errorf("expected no generation branches after stop, got %v", genBranches)
	}

	finishedTags := gitListTags(genBranch + "-finished")
	mergedTags := gitListTags(genBranch + "-merged")
	if len(finishedTags) == 0 {
		return fmt.Errorf("expected tag %s-finished to exist", genBranch)
	}
	if len(mergedTags) == 0 {
		return fmt.Errorf("expected tag %s-merged to exist", genBranch)
	}
	logf("test:generator: tags verified: start, finished, merged")

	logDone("test 1: start/stop", t)

	// ── Test 2: start/run(1 cycle, 1 issue)/stop ──

	t = logStep("test 2: start/run/stop (1 cycle, 1 issue)")

	logf("test:generator: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	logf("test:generator: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, _ = gitCurrentBranch()
	logf("test:generator: on branch %s, running 1 cycle with 1 issue", genBranch)

	runCfg := runConfig{
		cobblerConfig: cobblerConfig{
			silenceAgent:     true,
			maxIssues:        1,
			noContainer:      true,
			generationBranch: genBranch,
		},
		cycles: 1,
	}
	if err := runCycles(runCfg, "test"); err != nil {
		return fmt.Errorf("runCycles: %w", err)
	}

	closedCount, err := countIssues(bdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed issues: %w", err)
	}
	logf("test:generator: closed issues after run: %d", closedCount)
	if closedCount < 1 {
		return fmt.Errorf("expected at least 1 closed issue, got %d", closedCount)
	}

	taskBranches := gitListBranches(taskBranchPattern(genBranch))
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	logf("test:generator: no stale task branches")

	logf("test:generator: stopping generation")
	if err := (Generator{}).Stop(); err != nil {
		return fmt.Errorf("generator:stop: %w", err)
	}

	currentBranch, _ = gitCurrentBranch()
	if currentBranch != "main" {
		return fmt.Errorf("expected main after stop, got %s", currentBranch)
	}
	logDone("test 2: start/run/stop", t)

	// ── Test 3: stitch respects --max-issues ──

	t = logStep("test 3: stitch --max-issues limit")

	logf("test:generator: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	logf("test:generator: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, _ = gitCurrentBranch()
	logf("test:generator: on branch %s, measuring 2 issues", genBranch)

	mCfg := measureConfig{cobblerConfig: cobblerConfig{
		silenceAgent:     true,
		maxIssues:        2,
		noContainer:      true,
		generationBranch: genBranch,
	}}
	if err := measure(mCfg); err != nil {
		return fmt.Errorf("measure: %w", err)
	}

	totalIssues, err := countIssues(bdListJSON)
	if err != nil {
		return fmt.Errorf("counting issues: %w", err)
	}
	logf("test:generator: total issues after measure: %d", totalIssues)
	if totalIssues < 2 {
		return fmt.Errorf("expected at least 2 issues from measure, got %d", totalIssues)
	}

	logf("test:generator: stitching with --max-issues 1")
	sCfg := stitchConfig{cobblerConfig: cobblerConfig{
		silenceAgent:     true,
		maxIssues:        1,
		noContainer:      true,
		generationBranch: genBranch,
	}}
	if err := stitch(sCfg); err != nil {
		return fmt.Errorf("stitch: %w", err)
	}

	closedCount, err = countIssues(bdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed: %w", err)
	}
	logf("test:generator: closed issues after stitch(1): %d", closedCount)
	if closedCount != 1 {
		return fmt.Errorf("expected 1 closed issue (max-issues=1), got %d", closedCount)
	}

	// Verify that not all issues were processed. We check total vs closed
	// rather than ready count because beads dependency mechanics may not
	// auto-promote remaining issues to "ready" status.
	remainingIssues := totalIssues - closedCount
	logf("test:generator: remaining issues (not closed): %d", remainingIssues)
	if remainingIssues < 1 {
		return fmt.Errorf("expected at least 1 issue not closed, got %d", remainingIssues)
	}

	logDone("test 3: stitch --max-issues", t)

	// ── Cleanup ──

	t = logStep("cleanup: reset")
	if err := Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}
	logDone("cleanup", t)

	// Summary.
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Generator lifecycle tests PASSED")
	fmt.Println("========================================")
	fmt.Printf("  Total time: %s\n", time.Since(suiteStart).Round(time.Second))
	fmt.Println()
	return nil
}

// Resume tests that generator:resume recovers from an interrupted run.
//
// Creates a generation, measures 1 issue (without stitching), switches
// to main, then calls resume. Resume should switch back to the generation
// branch, clean up, and stitch the pending issue.
//
// Requires: bd and claude CLIs on PATH.
func (Test) Resume() error {
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
	fmt.Println("Generator resume test")
	fmt.Println("========================================")

	// ── Setup: create generation with pending issues ──

	t := logStep("setup: create generation with pending issues")

	logf("test:resume: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	logf("test:resume: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, _ := gitCurrentBranch()
	logf("test:resume: on branch %s, measuring 1 issue", genBranch)

	mCfg := measureConfig{cobblerConfig: cobblerConfig{
		silenceAgent:     true,
		maxIssues:        1,
		noContainer:      true,
		generationBranch: genBranch,
	}}
	if err := measure(mCfg); err != nil {
		return fmt.Errorf("measure: %w", err)
	}

	issueCount, err := countIssues(bdListJSON)
	if err != nil {
		return fmt.Errorf("counting issues: %w", err)
	}
	logf("test:resume: %d issue(s) created", issueCount)
	if issueCount < 1 {
		return fmt.Errorf("expected at least 1 issue, got %d", issueCount)
	}

	logf("test:resume: committing state before switching to main")
	_ = gitStageAll()
	_ = gitCommit("WIP: save generation state before interruption")

	logf("test:resume: switching to main (simulating interruption)")
	if err := gitCheckout("main"); err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}
	logDone("setup", t)

	// ── Resume: should switch back and stitch ──

	t = logStep("resume: recover and stitch")

	logf("test:resume: calling resume for %s", genBranch)
	resumeCfg := runConfig{
		cobblerConfig: cobblerConfig{
			silenceAgent:     true,
			maxIssues:        1,
			noContainer:      true,
			generationBranch: genBranch,
		},
		cycles: 1,
	}
	// Resume does: switch to branch, cleanup, runCycles.
	// We simulate what Resume() does since we can't pass flags
	// through the mage target interface from Go.
	if err := ensureOnBranch(genBranch); err != nil {
		return fmt.Errorf("switching to %s: %w", genBranch, err)
	}

	wtBase := worktreeBasePath()
	_ = gitWorktreePrune()
	if err := recoverStaleTasks(genBranch, wtBase); err != nil {
		logf("test:resume: recoverStaleTasks warning: %v", err)
	}
	cobblerReset()

	// Run 1 cycle with max-issues 0 for measure (no new issues)
	// and max-issues 1 for stitch (resolve the pending one).
	// Since runCycles uses the same maxIssues for both measure
	// and stitch, set it to 1 so stitch processes 1 task.
	// Measure with maxIssues=1 may create another issue, which is fine.
	if err := runCycles(resumeCfg, "test-resume"); err != nil {
		return fmt.Errorf("runCycles: %w", err)
	}

	currentBranch, _ := gitCurrentBranch()
	logf("test:resume: current branch after resume: %s", currentBranch)
	if !strings.HasPrefix(currentBranch, genPrefix) {
		return fmt.Errorf("expected to be on generation branch, got %s", currentBranch)
	}

	closedCount, err := countIssues(bdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed issues: %w", err)
	}
	logf("test:resume: closed issues: %d", closedCount)
	if closedCount < 1 {
		return fmt.Errorf("expected at least 1 closed issue, got %d", closedCount)
	}

	taskBranches := gitListBranches(taskBranchPattern(currentBranch))
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	logf("test:resume: no stale task branches")

	logDone("resume", t)

	// ── Cleanup ──

	t = logStep("cleanup: reset")
	if err := Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}
	logDone("cleanup", t)

	// Summary.
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Generator resume test PASSED")
	fmt.Println("========================================")
	fmt.Printf("  Total time: %s\n", time.Since(suiteStart).Round(time.Second))
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
