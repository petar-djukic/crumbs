package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// runConfig holds options for the generator:run target.
type runConfig struct {
	cobblerConfig
	cycles int
}

func parseRunFlags() runConfig {
	var cfg runConfig
	cfg.cycles = 1
	fs := flag.NewFlagSet("generator:run", flag.ContinueOnError)
	registerCobblerFlags(fs, &cfg.cobblerConfig)
	fs.IntVar(&cfg.cycles, flagCycles, 1, "number of measure+stitch cycles")
	parseTargetFlags(fs)
	return cfg
}

func parseResumeFlags() runConfig {
	var cfg runConfig
	cfg.cycles = 1
	fs := flag.NewFlagSet("generator:resume", flag.ContinueOnError)
	registerCobblerFlags(fs, &cfg.cobblerConfig)
	fs.IntVar(&cfg.cycles, flagCycles, 1, "number of measure+stitch cycles")
	parseTargetFlags(fs)
	resolveCobblerBranch(&cfg.cobblerConfig, fs)
	return cfg
}

// Run executes N cycles of Measure + Stitch within the current generation.
//
// Flags:
//
//	--silence-agent        suppress Claude output (default true)
//	--cycles N             number of measure+stitch cycles (default 1)
//	--max-issues N         issues per measure cycle (default 10)
//	--user-prompt TEXT     user prompt text
//	--generation-branch    generation branch to work on
func (Generator) Run() error {
	cfg := parseRunFlags()

	currentBranch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	cfg.generationBranch = currentBranch
	return runCycles(cfg, "run")
}

// Resume recovers from an interrupted generator:run and continues.
//
// Resolves the generation branch (positional arg or auto-detect),
// commits any uncommitted work on the current branch, switches to
// the generation branch, cleans up stale worktrees/branches/issues,
// removes cobbler scratch files, and runs measure+stitch cycles.
//
// Usage:
//
//	mage generator:resume                              # auto-detect
//	mage generator:resume generation-2026-02-10-15-04  # explicit
//
// Flags:
//
//	--silence-agent        suppress Claude output (default true)
//	--cycles N             number of measure+stitch cycles (default 1)
//	--max-issues N         issues per measure cycle (default 10)
//	--user-prompt TEXT     user prompt text
//	--no-container         skip container runtime, use local claude binary
func (Generator) Resume() error {
	cfg := parseResumeFlags()

	// Resolve generation branch from positional arg or auto-detect.
	branch := cfg.generationBranch
	if branch == "" {
		resolved, err := resolveBranch("")
		if err != nil {
			return fmt.Errorf("resolving generation branch: %w", err)
		}
		branch = resolved
	}

	if !strings.HasPrefix(branch, genPrefix) {
		return fmt.Errorf("not a generation branch: %s\nUsage: mage generator:resume [generation-name]", branch)
	}
	if !gitBranchExists(branch) {
		return fmt.Errorf("branch does not exist: %s", branch)
	}

	logf("resume: target branch=%s", branch)

	// Commit uncommitted work on the current branch before switching.
	if err := gitStageAll(); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}
	if err := gitCommit(fmt.Sprintf("WIP: save state before resuming on %s", branch)); err != nil {
		_ = gitUnstageAll()
	}

	// Switch to the generation branch.
	if err := ensureOnBranch(branch); err != nil {
		return fmt.Errorf("switching to %s: %w", branch, err)
	}

	// Pre-flight cleanup.
	logf("resume: pre-flight cleanup")
	wtBase := worktreeBasePath()

	logf("resume: pruning worktrees")
	_ = gitWorktreePrune()

	if _, err := os.Stat(wtBase); err == nil {
		logf("resume: removing worktree directory %s", wtBase)
		os.RemoveAll(wtBase)
	}

	logf("resume: recovering stale tasks")
	if err := recoverStaleTasks(branch, wtBase); err != nil {
		logf("resume: recoverStaleTasks warning: %v", err)
	}

	logf("resume: resetting cobbler scratch")
	cobblerReset()

	cfg.generationBranch = branch
	return runCycles(cfg, "resume")
}

// runCycles runs N measure+stitch cycles with the given config.
// The label parameter identifies the caller for log messages.
func runCycles(cfg runConfig, label string) error {
	gen := cfg.generationBranch
	mCfg := measureConfig{cobblerConfig: cfg.cobblerConfig}
	sCfg := stitchConfig{cobblerConfig: cfg.cobblerConfig}

	logf("generator %s [%s]: starting %d cycle(s), %d issues per cycle", label, gen, cfg.cycles, cfg.maxIssues)

	for cycle := 1; cycle <= cfg.cycles; cycle++ {
		logf("generator %s [%s]: cycle %d/%d — measure", label, gen, cycle, cfg.cycles)
		if err := measure(mCfg); err != nil {
			return fmt.Errorf("cycle %d measure: %w", cycle, err)
		}

		logf("generator %s [%s]: cycle %d/%d — stitch", label, gen, cycle, cfg.cycles)
		if err := stitch(sCfg); err != nil {
			return fmt.Errorf("cycle %d stitch: %w", cycle, err)
		}
	}

	logf("generator %s [%s]: complete, ran %d cycle(s)", label, gen, cfg.cycles)
	return nil
}

// Start begins a new generation trail.
//
// Tags current main state, creates a generation branch, deletes Go files,
// reinitializes the Go module, and commits the clean state.
// Must be run from main.
func (Generator) Start() error {
	if err := ensureOnBranch("main"); err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}

	genName := genPrefix + time.Now().Format("2006-01-02-15-04-05")
	startTag := genName + "-start"

	logf("generator:start [%s]: beginning", genName)

	// Tag current main state before the generation begins.
	logf("generator:start [%s]: tagging current state as %s", genName, startTag)
	if err := gitTag(startTag); err != nil {
		return fmt.Errorf("tagging main: %w", err)
	}

	// Create and switch to generation branch.
	logf("generator:start [%s]: creating branch", genName)
	if err := gitCheckoutNew(genName); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}

	// Reset beads database and reinitialize with generation prefix.
	if err := beadsReset(); err != nil {
		return fmt.Errorf("resetting beads: %w", err)
	}
	if err := beadsInit(genName); err != nil {
		return fmt.Errorf("initializing beads: %w", err)
	}

	// Reset Go sources and reinitialize module.
	logf("generator:start [%s]: resetting Go sources", genName)
	if err := resetGoSources(genName); err != nil {
		return fmt.Errorf("resetting Go sources: %w", err)
	}

	// Commit the clean state.
	logf("generator:start [%s]: committing clean state", genName)
	_ = gitStageAll()
	msg := fmt.Sprintf("Start generation: %s\n\nDelete Go files, reinitialize module.\nTagged previous state as %s.", genName, genName)
	if err := gitCommit(msg); err != nil {
		return fmt.Errorf("committing clean state: %w", err)
	}

	logf("generator:start [%s]: done, run mage generator:run to begin building", genName)
	return nil
}

// Stop completes a generation trail and merges it into main.
//
// Pass the generation name as a positional argument, or omit it to
// auto-detect. Without an argument, Stop checks the current branch
// first: if it is a generation branch, that branch is stopped. If on
// main with no generation branches, it exits with an error.
//
//	mage generator:stop                              # auto-detect
//	mage generator:stop generation-2026-02-10-15-04  # explicit
//
// Tags the generation branch, switches to main, deletes Go code from main,
// merges the generation branch, tags the merge, and deletes the generation branch.
func (Generator) Stop() error {
	fs := flag.NewFlagSet("generator:stop", flag.ContinueOnError)
	parseTargetFlags(fs)

	var branch string
	if fs.NArg() > 0 {
		// Explicit generation name provided.
		branch = fs.Arg(0)
		if !gitBranchExists(branch) {
			return fmt.Errorf("branch does not exist: %s", branch)
		}
	} else {
		// No argument: check current branch first, then fall back to resolveBranch.
		current, err := gitCurrentBranch()
		if err != nil {
			return fmt.Errorf("getting current branch: %w", err)
		}
		if strings.HasPrefix(current, genPrefix) {
			branch = current
			logf("generator:stop: stopping current branch %s", branch)
		} else {
			resolved, err := resolveBranch("")
			if err != nil {
				return err
			}
			branch = resolved
		}
	}

	if !strings.HasPrefix(branch, genPrefix) {
		return fmt.Errorf("not a generation branch: %s\nUsage: mage generator:stop [generation-name]", branch)
	}

	finishedTag := branch + "-finished"

	logf("generator:stop [%s]: beginning", branch)

	// Switch to the generation branch and tag its final state.
	if err := ensureOnBranch(branch); err != nil {
		return fmt.Errorf("switching to generation branch: %w", err)
	}
	logf("generator:stop [%s]: tagging as %s", branch, finishedTag)
	if err := gitTag(finishedTag); err != nil {
		return fmt.Errorf("tagging generation: %w", err)
	}

	// Switch to main.
	logf("generator:stop [%s]: switching to main", branch)
	if err := gitCheckout("main"); err != nil {
		return fmt.Errorf("checking out main: %w", err)
	}

	if err := mergeGenerationIntoMain(branch); err != nil {
		return err
	}

	logf("generator:stop [%s]: done, work is on main", branch)
	return nil
}

// mergeGenerationIntoMain resets Go sources, commits the clean state,
// merges the generation branch, tags the result, and deletes the branch.
func mergeGenerationIntoMain(branch string) error {
	logf("generator:stop [%s]: resetting Go sources on main", branch)
	_ = resetGoSources(branch)

	_ = gitStageAll()
	prepareMsg := fmt.Sprintf("Prepare main for generation merge: delete Go code\n\nDocumentation preserved for merge. Code will be replaced by %s.", branch)
	if err := gitCommitAllowEmpty(prepareMsg); err != nil {
		return fmt.Errorf("committing prepare step: %w", err)
	}

	logf("generator:stop [%s]: merging into main", branch)
	cmd := gitMergeCmd(branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merging %s: %w", branch, err)
	}

	mainTag := branch + "-merged"
	logf("generator:stop [%s]: tagging main as %s", branch, mainTag)
	if err := gitTag(mainTag); err != nil {
		return fmt.Errorf("tagging merge: %w", err)
	}

	logf("generator:stop [%s]: deleting branch", branch)
	_ = gitDeleteBranch(branch)
	return nil
}

// listGenerationBranches returns all generation-* branch names.
func listGenerationBranches() []string {
	return gitListBranches(genPrefix + "*")
}

// tagSuffixes lists the lifecycle tag suffixes in order.
var tagSuffixes = []string{"-start", "-finished", "-merged", "-incomplete"}

// generationName strips the lifecycle suffix from a tag to recover
// the generation name. Returns the tag unchanged if no suffix matches.
func generationName(tag string) string {
	for _, suffix := range tagSuffixes {
		if cut, ok := strings.CutSuffix(tag, suffix); ok {
			return cut
		}
	}
	return tag
}

// cleanupUnmergedTags renames tags for generations that were never
// merged. Each unmerged -start or -finished tag becomes a single
// -incomplete tag so the generation remains discoverable in
// generator:list --all without cluttering the default view.
func cleanupUnmergedTags() {
	tags := gitListTags(genPrefix + "*")
	if len(tags) == 0 {
		return
	}

	// Build set of merged generation names.
	merged := make(map[string]bool)
	for _, t := range tags {
		if name, ok := strings.CutSuffix(t, "-merged"); ok {
			merged[name] = true
		}
	}

	// For unmerged generations, replace all tags with a single -incomplete tag.
	marked := make(map[string]bool)
	for _, t := range tags {
		name := generationName(t)
		if merged[name] {
			continue
		}
		if !marked[name] {
			marked[name] = true
			incTag := name + "-incomplete"
			if t != incTag {
				logf("generator:reset: marking incomplete: %s -> %s", t, incTag)
				_ = gitRenameTag(t, incTag)
			}
		} else {
			logf("generator:reset: removing tag %s", t)
			_ = gitDeleteTag(t)
		}
	}
}

// resolveBranch determines which branch to work on.
// If explicit is non-empty, it verifies the branch exists.
// Otherwise: 0 generation branches -> current branch, 1 -> that branch,
// 2+ -> error (caller must specify with --generation-branch).
func resolveBranch(explicit string) (string, error) {
	if explicit != "" {
		if !gitBranchExists(explicit) {
			return "", fmt.Errorf("branch does not exist: %s", explicit)
		}
		return explicit, nil
	}

	branches := listGenerationBranches()
	switch len(branches) {
	case 0:
		return gitCurrentBranch()
	case 1:
		return branches[0], nil
	default:
		sort.Strings(branches)
		return "", fmt.Errorf("multiple generation branches exist (%s); specify one with --generation-branch", strings.Join(branches, ", "))
	}
}

// ensureOnBranch switches to the given branch if not already on it.
func ensureOnBranch(branch string) error {
	current, err := gitCurrentBranch()
	if err != nil {
		return err
	}
	if current == branch {
		return nil
	}
	logf("ensureOnBranch: switching from %s to %s", current, branch)
	return gitCheckout(branch)
}

// List shows active branches and past generations discoverable
// through tags. By default only active branches and merged
// generations are shown. Use --all to include incomplete generations.
//
// Flags:
//
//	--all    show all generations including incomplete
func (Generator) List() error {
	var showAll bool
	fs := flag.NewFlagSet("generator:list", flag.ContinueOnError)
	fs.BoolVar(&showAll, "all", false, "show all generations including incomplete")
	parseTargetFlags(fs)

	branches := listGenerationBranches()
	tags := gitListTags(genPrefix + "*")
	current, _ := gitCurrentBranch()

	// Build a set of generation names from branches and tags.
	nameSet := make(map[string]bool)
	branchSet := make(map[string]bool)
	for _, b := range branches {
		nameSet[b] = true
		branchSet[b] = true
	}

	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
		nameSet[generationName(t)] = true
	}

	if len(nameSet) == 0 {
		fmt.Println("No generations found.")
		return nil
	}

	// Sort names (timestamp-based, so lexicographic order is chronological).
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	shown := 0
	for _, name := range names {
		isActive := branchSet[name]
		isMerged := tagSet[name+"-merged"]
		isIncomplete := tagSet[name+"-incomplete"]

		// Default view: active branches and merged generations only.
		if !showAll && !isActive && !isMerged {
			continue
		}

		marker := " "
		if name == current {
			marker = "*"
		}

		// Lifecycle tags present for this generation.
		var lifecycle []string
		for _, suffix := range tagSuffixes {
			if tagSet[name+suffix] {
				lifecycle = append(lifecycle, suffix[1:]) // strip leading "-"
			}
		}

		if isActive {
			if len(lifecycle) > 0 {
				fmt.Printf("%s %s  (active, tags: %s)\n", marker, name, strings.Join(lifecycle, ", "))
			} else {
				fmt.Printf("%s %s  (active)\n", marker, name)
			}
		} else if isIncomplete {
			fmt.Printf("%s %s  (incomplete)\n", marker, name)
		} else {
			fmt.Printf("%s %s  (tags: %s)\n", marker, name, strings.Join(lifecycle, ", "))
		}
		shown++
	}

	if shown == 0 {
		fmt.Println("No generations found. Use --all to show incomplete generations.")
	}
	return nil
}

// Switch commits current work and checks out another generation branch.
//
// Usage:
//
//	mage generator:switch generation-2026-02-10-15-04
//
// The target must be a generation branch or "main". Any uncommitted
// changes on the current branch are staged and committed before
// switching.
func (Generator) Switch() error {
	fs := flag.NewFlagSet("generator:switch", flag.ContinueOnError)
	parseTargetFlags(fs)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: mage generator:switch <branch>\nAvailable branches: %s, main", strings.Join(listGenerationBranches(), ", "))
	}
	target := fs.Arg(0)

	if target != "main" && !strings.HasPrefix(target, genPrefix) {
		return fmt.Errorf("not a generation branch or main: %s", target)
	}
	if !gitBranchExists(target) {
		return fmt.Errorf("branch does not exist: %s", target)
	}

	current, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	if current == target {
		logf("generator:switch: already on %s", target)
		return nil
	}

	// Commit any uncommitted work on the current branch.
	if err := gitStageAll(); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}
	if err := gitCommit(fmt.Sprintf("WIP: save state before switching to %s", target)); err != nil {
		// Commit fails when there is nothing to commit; that is fine.
		// But if staged changes remain, checkout will fail, so unstage.
		_ = gitUnstageAll()
	}

	logf("generator:switch: %s -> %s", current, target)
	if err := gitCheckout(target); err != nil {
		return fmt.Errorf("switching to %s: %w", target, err)
	}

	logf("generator:switch: now on %s", target)
	return nil
}

// Reset destroys generation branches, worktrees, and Go source directories.
// Generation tags are preserved so past generations remain discoverable.
// Does not touch cobbler or beads; use top-level reset for a full wipe.
func (Generator) Reset() error {
	logf("generator:reset: beginning")

	// Must be on main before deleting other branches.
	if err := ensureOnBranch("main"); err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}

	// Remove task branches and worktrees for each generation branch.
	wtBase := worktreeBasePath()
	genBranches := listGenerationBranches()
	if len(genBranches) > 0 {
		logf("generator:reset: removing task branches and worktrees")
		for _, gb := range genBranches {
			recoverStaleBranches(gb, wtBase)
		}
	}

	_ = gitWorktreePrune()

	if _, err := os.Stat(wtBase); err == nil {
		logf("generator:reset: removing worktree directory %s", wtBase)
		os.RemoveAll(wtBase)
	}

	if len(genBranches) > 0 {
		logf("generator:reset: removing %d generation branch(es)", len(genBranches))
		for _, gb := range genBranches {
			logf("generator:reset: deleting branch %s", gb)
			_ = gitForceDeleteBranch(gb)
		}
	}

	// Remove tags for generations that were never merged. Completed
	// generations (with a -merged tag) are preserved so past work
	// remains discoverable via generator:list.
	cleanupUnmergedTags()

	logf("generator:reset: removing Go source directories")
	for _, dir := range goSourceDirs {
		logf("generator:reset: removing %s", dir)
		os.RemoveAll(dir)
	}
	os.RemoveAll("bin/")

	logf("generator:reset: seeding Go sources and reinitializing go.mod")
	if err := seedVersionFile("main"); err != nil {
		return fmt.Errorf("seeding version file: %w", err)
	}
	if err := seedCupboardMain(); err != nil {
		return fmt.Errorf("seeding cupboard main: %w", err)
	}
	if err := reinitGoModule(); err != nil {
		return fmt.Errorf("reinitializing go module: %w", err)
	}

	logf("generator:reset: committing clean state")
	_ = gitStageAll()
	// Commit may fail when the tree is already in the seeded state
	// (e.g. running reset twice). That is not an error.
	_ = gitCommit("Generator reset: return to clean state")

	logf("generator:reset: done, only main branch remains")
	return nil
}

// goSourceDirs lists the directories that contain Go source files.
var goSourceDirs = []string{"cmd/", "pkg/", "internal/", "tests/"}

// resetGoSources deletes Go files, removes empty source dirs,
// clears build artifacts, reinitializes the Go module, and seeds
// the source tree with a version file. The version parameter is
// written into the Version constant (typically the generation name).
func resetGoSources(version string) error {
	deleteGoFiles(".")
	for _, dir := range goSourceDirs {
		removeEmptyDirs(dir)
	}
	os.RemoveAll("bin/")
	if err := seedVersionFile(version); err != nil {
		return fmt.Errorf("seeding version file: %w", err)
	}
	if err := seedCupboardMain(); err != nil {
		return fmt.Errorf("seeding cupboard main: %w", err)
	}
	return reinitGoModule()
}

// seedVersionFile creates pkg/crumbs/version.go with a Version constant
// set to the given value. This ensures the Go source tree has at least
// one package after a reset.
func seedVersionFile(version string) error {
	dir := filepath.Dir(versionFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf("package crumbs\n\nconst Version = %q\n", version)
	return os.WriteFile(versionFile, []byte(content), 0o644)
}

// seedCupboardMain creates cmd/cupboard/main.go with a minimal main
// function that prints the version. This ensures the build target has
// an entry point after a reset.
func seedCupboardMain() error {
	dir := filepath.Dir(cupboardMain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`package main

import (
	"fmt"

	"%s/pkg/crumbs"
)

func main() {
	fmt.Println("cupboard", crumbs.Version)
}
`, modulePath)
	return os.WriteFile(cupboardMain, []byte(content), 0o644)
}

// reinitGoModule removes go.sum and go.mod, then creates a fresh module
// with a local replace directive and resolves mage dependencies.
func reinitGoModule() error {
	os.Remove("go.sum")
	os.Remove("go.mod")
	if err := goModInit(); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}
	if err := goModEditReplace(modulePath, "./"); err != nil {
		return fmt.Errorf("go mod edit -replace: %w", err)
	}
	if err := goModTidy(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	return nil
}

// deleteGoFiles removes all .go files except those in .git/.
func deleteGoFiles(root string) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (path == ".git" || path == "magefiles") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			os.Remove(path)
		}
		return nil
	})
}

// removeEmptyDirs removes empty directories under the given root.
func removeEmptyDirs(root string) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	}
	// Walk bottom-up by collecting dirs then removing in reverse.
	var dirs []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	// Remove in reverse order (deepest first).
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err == nil && len(entries) == 0 {
			os.Remove(dirs[i])
		}
	}
}
