// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mesh-intelligence/mage-claude-orchestrator/pkg/orchestrator"
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

// testOrch creates an orchestrator configured for testing with
// silence enabled and the given maxIssues limit.
func testOrch(maxIssues int) *orchestrator.Orchestrator {
	cfg := baseCfg
	cfg.SilenceAgent = boolPtr(true)
	cfg.MaxIssues = maxIssues
	return orchestrator.New(cfg)
}

// testOrchWithBranch creates a test orchestrator targeting a specific branch.
func testOrchWithBranch(maxIssues, cycles int, branch string) *orchestrator.Orchestrator {
	cfg := baseCfg
	cfg.SilenceAgent = boolPtr(true)
	cfg.MaxIssues = maxIssues
	cfg.Cycles = cycles
	cfg.GenerationBranch = branch
	return orchestrator.New(cfg)
}

// --- Test verification helpers (git) ---

func tGitCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func tGitRevParseHEAD() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func tGitListBranches(pattern string) []string {
	out, err := exec.Command("git", "branch", "--list", pattern).Output()
	if err != nil {
		return nil
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

func tGitListTags(pattern string) []string {
	out, err := exec.Command("git", "tag", "--list", pattern).Output()
	if err != nil {
		return nil
	}
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		tag := strings.TrimSpace(line)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func tGitCountCommits(from, to string) (int, error) {
	out, err := exec.Command("git", "rev-list", "--count", from+".."+to).Output()
	if err != nil {
		return 0, err
	}
	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count); err != nil {
		return 0, err
	}
	return count, nil
}

func tGitWorktreeCount() int {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	// Subtract 1 for the main worktree.
	if count > 0 {
		count--
	}
	return count
}

func tGitStageAll() error {
	return exec.Command("git", "add", "-A").Run()
}

func tGitCommit(msg string) error {
	return exec.Command("git", "commit", "--allow-empty", "-m", msg).Run()
}

func tGitCheckout(branch string) error {
	return exec.Command("git", "checkout", branch).Run()
}

// --- Test verification helpers (beads) ---

func tBdListJSON() ([]byte, error) {
	return exec.Command("bd", "list", "--json").Output()
}

func tBdListClosedTasks() ([]byte, error) {
	return exec.Command("bd", "list", "--status", "closed", "--json").Output()
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

// Cobbler runs the cobbler regression suite: measure creates 3 issues,
// stitch resolves them, then verifies all are closed and no branches
// or worktrees leak. Resets beads before and after.
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

	// Step: reset beads.
	t := logStep("setup: reset beads")
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}
	logDone("setup", t)

	// Step: measure (create 3 issues).
	t = logStep("measure: create 3 issues")
	if err := testOrch(3).Measure(); err != nil {
		return fmt.Errorf("measure: %w", err)
	}
	logDone("measure", t)

	// Step: verify issue count.
	t = logStep("verify issue count")
	issueCount, err := countIssues(tBdListJSON)
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
	if err := testOrch(baseCfg.MaxIssues).Stitch(); err != nil {
		return fmt.Errorf("stitch: %w", err)
	}
	logDone("stitch", t)

	// Step: verify all closed.
	t = logStep("verify all closed")
	closedCount, err := countIssues(tBdListClosedTasks)
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
	branch, _ := tGitCurrentBranch()
	taskBranches := tGitListBranches("task/" + branch + "-*")
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	logf("test:cobbler: no stale task branches")
	logDone("verify no stale branches", t)

	// Step: cleanup.
	t = logStep("cleanup: reset beads")
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}
	logDone("cleanup", t)

	// Summary.
	totalDuration := time.Since(suiteStart).Round(time.Second)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Cobbler regression test PASSED")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Total time:       %s\n", totalDuration)
	fmt.Printf("  Issues created:   %d\n", issueCount)
	fmt.Printf("  Issues resolved:  %d\n", closedCount)
	fmt.Println()
	return nil
}

// Generator runs the generator lifecycle tests progressively.
//
// Test 1 (no Claude): start/stop creates tags and returns to main.
// Test 2 (Claude): start, run 1 cycle with 1 issue, stop. Verifies
// issue created and resolved, correct tags, no stale branches.
// Test 3 (Claude): stitch respects max-issues limit. Creates 2
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

	beforeResetSHA, err := tGitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD before reset: %w", err)
	}

	logf("test:generator: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	// Verify Reset() squashed into at most 1 commit.
	resetCommits, err := tGitCountCommits(beforeResetSHA, "HEAD")
	if err != nil {
		return fmt.Errorf("counting reset commits: %w", err)
	}
	logf("test:generator: reset produced %d commit(s)", resetCommits)
	if resetCommits > 1 {
		return fmt.Errorf("expected reset to produce at most 1 commit, got %d", resetCommits)
	}

	logf("test:generator: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, err := tGitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting branch after start: %w", err)
	}
	logf("test:generator: on branch %s", genBranch)

	if !strings.HasPrefix(genBranch, baseCfg.GenPrefix) {
		return fmt.Errorf("expected generation branch, got %s", genBranch)
	}

	startTag := genBranch + "-start"
	startTags := tGitListTags(startTag)
	if len(startTags) == 0 {
		return fmt.Errorf("expected tag %s to exist", startTag)
	}
	logf("test:generator: start tag %s exists", startTag)

	// Verify Start() squashed into exactly 1 commit ahead of start tag.
	startCommits, err := tGitCountCommits(startTag, "HEAD")
	if err != nil {
		return fmt.Errorf("counting start commits: %w", err)
	}
	logf("test:generator: generation branch is %d commit(s) ahead of %s", startCommits, startTag)
	if startCommits != 1 {
		return fmt.Errorf("expected generation branch to be 1 commit ahead of %s, got %d", startTag, startCommits)
	}

	// Verify no stale worktrees.
	if wt := tGitWorktreeCount(); wt > 0 {
		return fmt.Errorf("expected 0 linked worktrees after start, got %d", wt)
	}

	logf("test:generator: stopping generation")
	if err := (Generator{}).Stop(); err != nil {
		return fmt.Errorf("generator:stop: %w", err)
	}

	currentBranch, _ := tGitCurrentBranch()
	if currentBranch != "main" {
		return fmt.Errorf("expected to be on main after stop, got %s", currentBranch)
	}

	genBranches := tGitListBranches(baseCfg.GenPrefix + "*")
	if len(genBranches) > 0 {
		return fmt.Errorf("expected no generation branches after stop, got %v", genBranches)
	}

	finishedTags := tGitListTags(genBranch + "-finished")
	mergedTags := tGitListTags(genBranch + "-merged")
	if len(finishedTags) == 0 {
		return fmt.Errorf("expected tag %s-finished to exist", genBranch)
	}
	if len(mergedTags) == 0 {
		return fmt.Errorf("expected tag %s-merged to exist", genBranch)
	}
	logf("test:generator: tags verified: start, finished, merged")

	// Verify no stale worktrees after stop.
	if wt := tGitWorktreeCount(); wt > 0 {
		return fmt.Errorf("expected 0 linked worktrees after stop, got %d", wt)
	}

	logDone("test 1: start/stop", t)

	// ── Test 2: start/run(1 cycle, 1 issue)/stop ──

	t = logStep("test 2: start/run/stop (1 cycle, 1 issue)")

	beforeResetSHA, err = tGitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD before reset: %w", err)
	}

	logf("test:generator: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	resetCommits, err = tGitCountCommits(beforeResetSHA, "HEAD")
	if err != nil {
		return fmt.Errorf("counting reset commits: %w", err)
	}
	logf("test:generator: reset produced %d commit(s)", resetCommits)
	if resetCommits > 1 {
		return fmt.Errorf("expected reset to produce at most 1 commit, got %d", resetCommits)
	}

	logf("test:generator: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, _ = tGitCurrentBranch()

	startTag = genBranch + "-start"
	startCommits, err = tGitCountCommits(startTag, "HEAD")
	if err != nil {
		return fmt.Errorf("counting start commits: %w", err)
	}
	logf("test:generator: generation branch is %d commit(s) ahead of %s", startCommits, startTag)
	if startCommits != 1 {
		return fmt.Errorf("expected generation branch to be 1 commit ahead of %s, got %d", startTag, startCommits)
	}

	logf("test:generator: on branch %s, running 1 cycle with 1 issue", genBranch)

	if err := testOrchWithBranch(1, 1, genBranch).RunCycles("test"); err != nil {
		return fmt.Errorf("runCycles: %w", err)
	}

	closedCount, err := countIssues(tBdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed issues: %w", err)
	}
	logf("test:generator: closed issues after run: %d", closedCount)
	if closedCount < 1 {
		return fmt.Errorf("expected at least 1 closed issue, got %d", closedCount)
	}

	taskBranches := tGitListBranches("task/" + genBranch + "-*")
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	logf("test:generator: no stale task branches")

	logf("test:generator: stopping generation")
	if err := (Generator{}).Stop(); err != nil {
		return fmt.Errorf("generator:stop: %w", err)
	}

	currentBranch, _ = tGitCurrentBranch()
	if currentBranch != "main" {
		return fmt.Errorf("expected main after stop, got %s", currentBranch)
	}
	logDone("test 2: start/run/stop", t)

	// ── Test 3: stitch respects max-issues ──

	t = logStep("test 3: stitch max-issues limit")

	beforeResetSHA, err = tGitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD before reset: %w", err)
	}

	logf("test:generator: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	resetCommits, err = tGitCountCommits(beforeResetSHA, "HEAD")
	if err != nil {
		return fmt.Errorf("counting reset commits: %w", err)
	}
	logf("test:generator: reset produced %d commit(s)", resetCommits)
	if resetCommits > 1 {
		return fmt.Errorf("expected reset to produce at most 1 commit, got %d", resetCommits)
	}

	logf("test:generator: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, _ = tGitCurrentBranch()

	startTag = genBranch + "-start"
	startCommits, err = tGitCountCommits(startTag, "HEAD")
	if err != nil {
		return fmt.Errorf("counting start commits: %w", err)
	}
	logf("test:generator: generation branch is %d commit(s) ahead of %s", startCommits, startTag)
	if startCommits != 1 {
		return fmt.Errorf("expected generation branch to be 1 commit ahead of %s, got %d", startTag, startCommits)
	}

	logf("test:generator: on branch %s, measuring 2 issues", genBranch)

	if err := testOrchWithBranch(2, 0, genBranch).Measure(); err != nil {
		return fmt.Errorf("measure: %w", err)
	}

	totalIssues, err := countIssues(tBdListJSON)
	if err != nil {
		return fmt.Errorf("counting issues: %w", err)
	}
	logf("test:generator: total issues after measure: %d", totalIssues)
	if totalIssues < 2 {
		return fmt.Errorf("expected at least 2 issues from measure, got %d", totalIssues)
	}

	logf("test:generator: stitching with max-issues 1")
	if err := testOrchWithBranch(1, 0, genBranch).Stitch(); err != nil {
		return fmt.Errorf("stitch: %w", err)
	}

	closedCount, err = countIssues(tBdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed: %w", err)
	}
	logf("test:generator: closed issues after stitch(1): %d", closedCount)
	if closedCount != 1 {
		return fmt.Errorf("expected 1 closed issue (max-issues=1), got %d", closedCount)
	}

	// Verify that not all issues were processed.
	remainingIssues := totalIssues - closedCount
	logf("test:generator: remaining issues (not closed): %d", remainingIssues)
	if remainingIssues < 1 {
		return fmt.Errorf("expected at least 1 issue not closed, got %d", remainingIssues)
	}

	logDone("test 3: stitch max-issues", t)

	// ── Cleanup ──

	t = logStep("cleanup: reset")

	beforeResetSHA, err = tGitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD before cleanup reset: %w", err)
	}

	if err := Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}

	resetCommits, err = tGitCountCommits(beforeResetSHA, "HEAD")
	if err != nil {
		return fmt.Errorf("counting cleanup reset commits: %w", err)
	}
	logf("test:generator: cleanup reset produced %d commit(s)", resetCommits)
	if resetCommits > 1 {
		return fmt.Errorf("expected cleanup reset to produce at most 1 commit, got %d", resetCommits)
	}

	if wt := tGitWorktreeCount(); wt > 0 {
		return fmt.Errorf("expected 0 linked worktrees after cleanup, got %d", wt)
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

	beforeResetSHA, err := tGitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD before reset: %w", err)
	}

	logf("test:resume: resetting to clean state")
	if err := Reset(); err != nil {
		return fmt.Errorf("setup reset: %w", err)
	}

	resetCommits, err := tGitCountCommits(beforeResetSHA, "HEAD")
	if err != nil {
		return fmt.Errorf("counting reset commits: %w", err)
	}
	logf("test:resume: reset produced %d commit(s)", resetCommits)
	if resetCommits > 1 {
		return fmt.Errorf("expected reset to produce at most 1 commit, got %d", resetCommits)
	}

	logf("test:resume: starting generation")
	if err := (Generator{}).Start(); err != nil {
		return fmt.Errorf("generator:start: %w", err)
	}

	genBranch, _ := tGitCurrentBranch()

	startTag := genBranch + "-start"
	startCommits, err := tGitCountCommits(startTag, "HEAD")
	if err != nil {
		return fmt.Errorf("counting start commits: %w", err)
	}
	logf("test:resume: generation branch is %d commit(s) ahead of %s", startCommits, startTag)
	if startCommits != 1 {
		return fmt.Errorf("expected generation branch to be 1 commit ahead of %s, got %d", startTag, startCommits)
	}

	logf("test:resume: on branch %s, measuring 1 issue", genBranch)

	if err := testOrchWithBranch(1, 0, genBranch).Measure(); err != nil {
		return fmt.Errorf("measure: %w", err)
	}

	issueCount, err := countIssues(tBdListJSON)
	if err != nil {
		return fmt.Errorf("counting issues: %w", err)
	}
	logf("test:resume: %d issue(s) created", issueCount)
	if issueCount < 1 {
		return fmt.Errorf("expected at least 1 issue, got %d", issueCount)
	}

	logf("test:resume: committing state before switching to main")
	_ = tGitStageAll()
	_ = tGitCommit("WIP: save generation state before interruption")

	logf("test:resume: switching to main (simulating interruption)")
	if err := tGitCheckout("main"); err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}
	logDone("setup", t)

	// ── Resume: should switch back and stitch ──

	t = logStep("resume: recover and stitch")

	logf("test:resume: calling resume for %s", genBranch)
	if err := testOrchWithBranch(1, 1, genBranch).GeneratorResume(); err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	currentBranch, _ := tGitCurrentBranch()
	logf("test:resume: current branch after resume: %s", currentBranch)
	if !strings.HasPrefix(currentBranch, baseCfg.GenPrefix) {
		return fmt.Errorf("expected to be on generation branch, got %s", currentBranch)
	}

	closedCount, err := countIssues(tBdListClosedTasks)
	if err != nil {
		return fmt.Errorf("counting closed issues: %w", err)
	}
	logf("test:resume: closed issues: %d", closedCount)
	if closedCount < 1 {
		return fmt.Errorf("expected at least 1 closed issue, got %d", closedCount)
	}

	taskBranches := tGitListBranches("task/" + currentBranch + "-*")
	if len(taskBranches) > 0 {
		return fmt.Errorf("stale task branches remain: %v", taskBranches)
	}
	logf("test:resume: no stale task branches")

	if wt := tGitWorktreeCount(); wt > 0 {
		return fmt.Errorf("expected 0 linked worktrees after resume, got %d", wt)
	}

	logDone("resume", t)

	// ── Cleanup ──

	t = logStep("cleanup: reset")

	beforeResetSHA, err = tGitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD before cleanup reset: %w", err)
	}

	if err := Reset(); err != nil {
		return fmt.Errorf("cleanup reset: %w", err)
	}

	resetCommits, err = tGitCountCommits(beforeResetSHA, "HEAD")
	if err != nil {
		return fmt.Errorf("counting cleanup reset commits: %w", err)
	}
	logf("test:resume: cleanup reset produced %d commit(s)", resetCommits)
	if resetCommits > 1 {
		return fmt.Errorf("expected cleanup reset to produce at most 1 commit, got %d", resetCommits)
	}

	if wt := tGitWorktreeCount(); wt > 0 {
		return fmt.Errorf("expected 0 linked worktrees after cleanup, got %d", wt)
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
