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

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Generator run: %d cycle(s), %d issues per cycle\n", cfg.cycles, cfg.maxIssues)
	fmt.Println("========================================")
	fmt.Println()

	cfg.generationBranch = currentBranch
	mCfg := measureConfig{cobblerConfig: cfg.cobblerConfig}
	sCfg := stitchConfig{cobblerConfig: cfg.cobblerConfig}

	for cycle := 1; cycle <= cfg.cycles; cycle++ {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Printf("Cycle %d of %d\n", cycle, cfg.cycles)
		fmt.Println("========================================")
		fmt.Println()

		fmt.Println("--- measure ---")
		if err := measure(mCfg); err != nil {
			return fmt.Errorf("cycle %d measure: %w", cycle, err)
		}

		fmt.Println()
		fmt.Println("--- stitch ---")
		if err := stitch(sCfg); err != nil {
			return fmt.Errorf("cycle %d stitch: %w", cycle, err)
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Generator run complete. Ran %d cycle(s).\n", cfg.cycles)
	fmt.Println("========================================")
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

	genName := genPrefix + time.Now().Format("2006-01-02-15-04")
	startTag := genName + "-start"

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Starting generator: %s\n", genName)
	fmt.Println("========================================")
	fmt.Println()

	// Tag current main state before the generation begins.
	fmt.Printf("Tagging current state as %s...\n", startTag)
	if err := gitTag(startTag); err != nil {
		return fmt.Errorf("tagging main: %w", err)
	}

	// Create and switch to generation branch.
	fmt.Printf("Creating branch %s...\n", genName)
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
	fmt.Println("Resetting Go sources...")
	if err := resetGoSources(genName); err != nil {
		return fmt.Errorf("resetting Go sources: %w", err)
	}

	// Commit the clean state.
	fmt.Println("Committing clean state...")
	_ = gitStageAll()
	msg := fmt.Sprintf("Start generation: %s\n\nDelete Go files, reinitialize module.\nTagged previous state as %s.", genName, genName)
	if err := gitCommit(msg); err != nil {
		return fmt.Errorf("committing clean state: %w", err)
	}

	fmt.Println()
	fmt.Printf("Generator started on branch %s.\n", genName)
	fmt.Println("Run mage generator:run to begin building.")
	fmt.Println()
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
			fmt.Printf("Warning: stopping current branch %s\n", branch)
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

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Stopping generator: %s\n", branch)
	fmt.Println("========================================")
	fmt.Println()

	// Switch to the generation branch and tag its final state.
	if err := ensureOnBranch(branch); err != nil {
		return fmt.Errorf("switching to generation branch: %w", err)
	}
	fmt.Printf("Tagging generation as %s...\n", finishedTag)
	if err := gitTag(finishedTag); err != nil {
		return fmt.Errorf("tagging generation: %w", err)
	}

	// Switch to main.
	fmt.Println("Switching to main...")
	if err := gitCheckout("main"); err != nil {
		return fmt.Errorf("checking out main: %w", err)
	}

	if err := mergeGenerationIntoMain(branch); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Generator stopped. Work is on main.")
	fmt.Println()
	return nil
}

// mergeGenerationIntoMain resets Go sources, commits the clean state,
// merges the generation branch, tags the result, and deletes the branch.
func mergeGenerationIntoMain(branch string) error {
	fmt.Println("Resetting Go sources on main...")
	_ = resetGoSources(branch)

	_ = gitStageAll()
	prepareMsg := fmt.Sprintf("Prepare main for generation merge: delete Go code\n\nDocumentation preserved for merge. Code will be replaced by %s.", branch)
	if err := gitCommitAllowEmpty(prepareMsg); err != nil {
		return fmt.Errorf("committing prepare step: %w", err)
	}

	fmt.Printf("Merging %s into main...\n", branch)
	cmd := gitMergeCmd(branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merging %s: %w", branch, err)
	}

	mainTag := branch + "-merged"
	fmt.Printf("Tagging main as %s...\n", mainTag)
	if err := gitTag(mainTag); err != nil {
		return fmt.Errorf("tagging merge: %w", err)
	}

	fmt.Printf("Deleting branch %s...\n", branch)
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
				fmt.Printf("  Marking incomplete: %s -> %s\n", t, incTag)
				_ = gitRenameTag(t, incTag)
			}
		} else {
			fmt.Printf("  Removing tag: %s\n", t)
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
	fmt.Printf("Switching to branch %s...\n", branch)
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
		fmt.Printf("Already on %s\n", target)
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

	fmt.Printf("Switching from %s to %s...\n", current, target)
	if err := gitCheckout(target); err != nil {
		return fmt.Errorf("switching to %s: %w", target, err)
	}

	fmt.Printf("Now on %s\n", target)
	return nil
}

// Reset destroys all branches, worktrees, beads, and Go
// source directories.
// Generation tags are preserved so past generations remain discoverable.
func (Generator) Reset() error {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Generator reset: returning to clean state")
	fmt.Println("========================================")
	fmt.Println()

	// Must be on main before deleting other branches.
	if err := ensureOnBranch("main"); err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}

	// Remove task branches and worktrees for each generation branch.
	wtBase := worktreeBasePath()
	genBranches := listGenerationBranches()
	if len(genBranches) > 0 {
		fmt.Println("Removing task branches and worktrees...")
		for _, gb := range genBranches {
			recoverStaleBranches(gb, wtBase)
		}
	}

	_ = gitWorktreePrune()

	if _, err := os.Stat(wtBase); err == nil {
		fmt.Printf("Removing worktree directory: %s\n", wtBase)
		os.RemoveAll(wtBase)
	}

	if len(genBranches) > 0 {
		fmt.Println("Removing generation branches...")
		for _, gb := range genBranches {
			fmt.Printf("  Deleting branch: %s\n", gb)
			_ = gitForceDeleteBranch(gb)
		}
	}

	// Remove tags for generations that were never merged. Completed
	// generations (with a -merged tag) are preserved so past work
	// remains discoverable via generator:list.
	cleanupUnmergedTags()

	if err := beadsReset(); err != nil {
		return fmt.Errorf("resetting beads: %w", err)
	}

	fmt.Println("Removing Go source directories...")
	for _, dir := range goSourceDirs {
		fmt.Printf("  Removing %s\n", dir)
		os.RemoveAll(dir)
	}
	os.RemoveAll("bin/")

	fmt.Println("Seeding Go sources and reinitializing go.mod...")
	if err := seedVersionFile("main"); err != nil {
		return fmt.Errorf("seeding version file: %w", err)
	}
	if err := seedCupboardMain(); err != nil {
		return fmt.Errorf("seeding cupboard main: %w", err)
	}
	if err := reinitGoModule(); err != nil {
		return fmt.Errorf("reinitializing go module: %w", err)
	}

	fmt.Println("Committing clean state...")
	_ = gitStageAll()
	if err := gitCommit("Generator reset: return to clean state"); err != nil {
		return fmt.Errorf("committing cleanup: %w", err)
	}

	fmt.Println()
	fmt.Println("Reset complete. Only main branch remains.")
	fmt.Println()
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
