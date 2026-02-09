package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// constructConfig holds options for the generation:construct target.
type constructConfig struct {
	silence   bool
	cycles    int
	maxIssues int
}

func parseConstructFlags() constructConfig {
	cfg := constructConfig{cycles: 1, maxIssues: 5}
	fs := flag.NewFlagSet("generation:construct", flag.ContinueOnError)
	fs.BoolVar(&cfg.silence, "silence", false, "suppress Claude output")
	fs.IntVar(&cfg.cycles, "cycles", 1, "number of measure+stitch cycles")
	fs.IntVar(&cfg.maxIssues, "max-issues", 5, "issues per measure cycle")
	parseTargetFlags(fs)
	return cfg
}

// Construct executes N cycles of Measure + Stitch within the current generation.
//
// Flags:
//
//	--silence    suppress Claude output
//	--cycles N   number of measure+stitch cycles (default 1)
//	--limit N    issues per measure cycle (default 5)
func (Generation) Construct() error {
	cfg := parseConstructFlags()

	currentBranch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Generation construct: %d cycle(s), %d issues per cycle\n", cfg.cycles, cfg.maxIssues)
	fmt.Println("========================================")
	fmt.Println()

	shared := cobblerConfig{
		silenceAgent: cfg.silence,
		maxIssues:    cfg.maxIssues,
		branch:       currentBranch,
	}
	mCfg := measureConfig{cobblerConfig: shared}
	sCfg := stitchConfig{cobblerConfig: shared}

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
	fmt.Printf("Generation construct complete. Ran %d cycle(s).\n", cfg.cycles)
	fmt.Println("========================================")
	return nil
}

// Start begins a new generation session.
//
// Tags current main state, creates a generation branch, deletes Go files,
// reinitializes the Go module, and commits the clean state.
// Must be run from main with no existing generation branches.
func (Generation) Start() error {
	if err := ensureOnBranch("main"); err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}

	// Check no existing generation branch.
	if branches := listGenerationBranches(); len(branches) > 0 {
		return fmt.Errorf("a generation branch already exists: %s. Finish it first or delete it", branches[0])
	}

	genName := genPrefix + time.Now().Format("2006-01-02-15-04")

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Starting generation: %s\n", genName)
	fmt.Println("========================================")
	fmt.Println()

	// Tag current main.
	fmt.Printf("Tagging current state as %s...\n", genName)
	if err := gitTag(genName); err != nil {
		return fmt.Errorf("tagging main: %w", err)
	}

	// Create and switch to generation branch.
	fmt.Printf("Creating branch %s...\n", genName)
	if err := gitCheckoutNew(genName); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}

	// Reset beads database and reinitialize with branch prefix.
	fmt.Println("Resetting beads database...")
	if err := bdAdminReset(); err != nil {
		return fmt.Errorf("resetting beads: %w", err)
	}
	fmt.Printf("Reinitializing beads with prefix %s...\n", genName)
	if err := bdInit(genName); err != nil {
		return fmt.Errorf("reinitializing beads: %w", err)
	}

	// Reset Go sources and reinitialize module.
	fmt.Println("Resetting Go sources...")
	if err := resetGoSources(); err != nil {
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
	fmt.Printf("Generation started on branch %s.\n", genName)
	fmt.Println("Run mage generation:construct to begin building.")
	fmt.Println()
	return nil
}

// Finish completes the current generation session and merges into main.
//
// Tags the generation branch, switches to main, deletes Go code from main,
// merges the generation branch, tags the merge, and deletes the generation branch.
// Must be run from a generation-* branch.
func (Generation) Finish() error {
	branch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	if !strings.HasPrefix(branch, genPrefix) {
		return fmt.Errorf("must be on a generation branch (currently on %s)", branch)
	}

	closedTag := branch + "-closed"

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Finishing generation: %s\n", branch)
	fmt.Println("========================================")
	fmt.Println()

	// Tag the final state of the generation branch.
	fmt.Printf("Tagging generation as %s...\n", closedTag)
	if err := gitTag(closedTag); err != nil {
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
	fmt.Println("Generation finished. Work is on main.")
	fmt.Println()
	return nil
}

// mergeGenerationIntoMain resets Go sources, commits the clean state,
// merges the generation branch, tags the result, and deletes the branch.
func mergeGenerationIntoMain(branch string) error {
	fmt.Println("Resetting Go sources on main...")
	_ = resetGoSources()

	_ = gitStageAll()
	prepareMsg := fmt.Sprintf("Prepare main for generation merge: delete Go code\n\nDocumentation preserved for merge. Code will be replaced by %s.", branch)
	if err := gitCommit(prepareMsg); err != nil {
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

// resolveBranch determines which branch to work on.
// If explicit is non-empty, it verifies the branch exists.
// Otherwise: 0 generation branches -> current branch, 1 -> that branch, 2+ -> error.
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
		return "", fmt.Errorf("multiple generation branches; specify with --branch:\n  %s",
			strings.Join(branches, "\n  "))
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

// List shows all generation branches.
func (Generation) List() error {
	branches := listGenerationBranches()
	if len(branches) == 0 {
		fmt.Println("No generation branches.")
		return nil
	}
	current, _ := gitCurrentBranch()
	for _, b := range branches {
		if b == current {
			fmt.Printf("* %s\n", b)
		} else {
			fmt.Printf("  %s\n", b)
		}
	}
	return nil
}

// goSourceDirs lists the directories that contain Go source files.
var goSourceDirs = []string{"cmd/", "pkg/", "internal/", "tests/"}

// resetGoSources deletes Go files, removes empty source dirs,
// clears build artifacts, and reinitializes the Go module.
func resetGoSources() error {
	deleteGoFiles(".")
	for _, dir := range goSourceDirs {
		removeEmptyDirs(dir)
	}
	os.RemoveAll("bin/")
	return reinitGoModule()
}

// reinitGoModule removes go.sum and go.mod, then runs go mod init.
func reinitGoModule() error {
	os.Remove("go.sum")
	os.Remove("go.mod")
	return goModInit()
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
