package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Generate runs N cycles of Measure + Stitch.
func Generate() error {
	silence := os.Getenv("GENERATE_SILENCE") == "true"

	cycles := 1
	if v := os.Getenv("GENERATE_CYCLES"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &cycles); err != nil || cycles < 1 {
			cycles = 1
		}
	}

	measureLimit := 5
	if v := os.Getenv("GENERATE_MEASURE_LIMIT"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &measureLimit); err != nil || measureLimit < 1 {
			measureLimit = 5
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Generate: %d cycle(s), %d issues per cycle\n", cycles, measureLimit)
	fmt.Println("========================================")
	fmt.Println()

	// Propagate settings to Measure and Stitch via env vars.
	os.Setenv("MEASURE_LIMIT", fmt.Sprintf("%d", measureLimit))
	if silence {
		os.Setenv("MEASURE_SILENCE", "true")
		os.Setenv("STITCH_SILENCE", "true")
	}

	for cycle := 1; cycle <= cycles; cycle++ {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Printf("Cycle %d of %d\n", cycle, cycles)
		fmt.Println("========================================")
		fmt.Println()

		fmt.Println("--- measure ---")
		if err := Measure(); err != nil {
			return fmt.Errorf("cycle %d measure: %w", cycle, err)
		}

		fmt.Println()
		fmt.Println("--- stitch ---")
		if err := Stitch(); err != nil {
			return fmt.Errorf("cycle %d stitch: %w", cycle, err)
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Generate complete. Ran %d cycle(s).\n", cycles)
	fmt.Println("========================================")
	return nil
}

// OpenGeneration starts a new generation session.
//
// Tags current main state, creates a generation branch, deletes Go files,
// reinitializes the Go module, and commits the clean state.
// Must be run from main with no existing generation branches.
func OpenGeneration() error {
	branch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	if branch != "main" {
		return fmt.Errorf("must be on main (currently on %s)", branch)
	}

	// Check no existing generation branch.
	out, _ := exec.Command("git", "branch", "--list", "generation-*").Output()
	if branches := parseBranchList(string(out)); len(branches) > 0 {
		return fmt.Errorf("a generation branch already exists: %s. Close it first or delete it", branches[0])
	}

	genName := "generation-" + time.Now().Format("2006-01-02-15-04")

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Opening generation: %s\n", genName)
	fmt.Println("========================================")
	fmt.Println()

	// Tag current main.
	fmt.Printf("Tagging current state as %s...\n", genName)
	if err := exec.Command("git", "tag", genName).Run(); err != nil {
		return fmt.Errorf("tagging main: %w", err)
	}

	// Create and switch to generation branch.
	fmt.Printf("Creating branch %s...\n", genName)
	if err := exec.Command("git", "checkout", "-b", genName).Run(); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}

	// Delete Go source files.
	fmt.Println("Deleting Go source files...")
	deleteGoFiles(".")

	// Remove empty directories in Go source dirs.
	for _, dir := range []string{"cmd/", "pkg/", "internal/", "tests/"} {
		removeEmptyDirs(dir)
	}

	// Remove build artifacts and dependency lock.
	os.RemoveAll("bin/")
	os.Remove("go.sum")

	// Reinitialize Go module.
	fmt.Println("Reinitializing Go module...")
	os.Remove("go.mod")
	if err := exec.Command("go", "mod", "init", "github.com/mesh-intelligence/crumbs").Run(); err != nil {
		return fmt.Errorf("reinitializing module: %w", err)
	}

	// Commit the clean state.
	fmt.Println("Committing clean state...")
	exec.Command("git", "add", "-A").Run()
	commitMsg := fmt.Sprintf("Open generation: %s\n\nDelete Go files, reinitialize module.\nTagged previous state as %s.", genName, genName)
	if err := exec.Command("git", "commit", "-m", commitMsg).Run(); err != nil {
		return fmt.Errorf("committing clean state: %w", err)
	}

	fmt.Println()
	fmt.Printf("Generation opened on branch %s.\n", genName)
	fmt.Println("Run mage stitch to start building.")
	fmt.Println()
	return nil
}

// CloseGeneration finishes the current generation session and merges into main.
//
// Tags the generation branch, switches to main, deletes Go code from main,
// merges the generation branch, tags the merge, and deletes the generation branch.
// Must be run from a generation-* branch.
func CloseGeneration() error {
	branch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	if !strings.HasPrefix(branch, "generation-") {
		return fmt.Errorf("must be on a generation branch (currently on %s)", branch)
	}

	closedTag := branch + "-closed"

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Closing generation: %s\n", branch)
	fmt.Println("========================================")
	fmt.Println()

	// Tag the final state of the generation branch.
	fmt.Printf("Tagging generation as %s...\n", closedTag)
	if err := exec.Command("git", "tag", closedTag).Run(); err != nil {
		return fmt.Errorf("tagging generation: %w", err)
	}

	// Switch to main.
	fmt.Println("Switching to main...")
	if err := exec.Command("git", "checkout", "main").Run(); err != nil {
		return fmt.Errorf("checking out main: %w", err)
	}

	// Delete Go code from main to prepare for clean merge.
	fmt.Println("Deleting Go code from main...")
	deleteGoFiles(".")

	for _, dir := range []string{"cmd/", "pkg/", "internal/", "tests/"} {
		removeEmptyDirs(dir)
	}

	os.RemoveAll("bin/")
	os.Remove("go.sum")

	// Reinitialize Go module.
	os.Remove("go.mod")
	exec.Command("go", "mod", "init", "github.com/mesh-intelligence/crumbs").Run()

	exec.Command("git", "add", "-A").Run()
	prepareMsg := fmt.Sprintf("Prepare main for generation merge: delete Go code\n\nDocumentation preserved for merge. Code will be replaced by %s.", branch)
	if err := exec.Command("git", "commit", "-m", prepareMsg).Run(); err != nil {
		return fmt.Errorf("committing prepare step: %w", err)
	}

	// Merge the generation branch.
	fmt.Printf("Merging %s into main...\n", branch)
	cmd := exec.Command("git", "merge", branch, "--no-edit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merging %s: %w", branch, err)
	}

	// Tag main after merge.
	mainTag := branch + "-merged"
	fmt.Printf("Tagging main as %s...\n", mainTag)
	if err := exec.Command("git", "tag", mainTag).Run(); err != nil {
		return fmt.Errorf("tagging merge: %w", err)
	}

	// Delete the generation branch.
	fmt.Printf("Deleting branch %s...\n", branch)
	exec.Command("git", "branch", "-d", branch).Run()

	fmt.Println()
	fmt.Println("Generation closed. Work is on main.")
	fmt.Println()
	return nil
}

// deleteGoFiles removes all .go files except those in .git/.
func deleteGoFiles(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
