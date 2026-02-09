package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cobblerConfig holds options shared by measure and stitch targets.
type cobblerConfig struct {
	silenceAgent bool
	maxIssues    int
	promptArg    string
	branch       string
}

// registerCobblerFlags adds the shared flags to fs.
func registerCobblerFlags(fs *flag.FlagSet, cfg *cobblerConfig) {
	fs.BoolVar(&cfg.silenceAgent, "silence-agent", true, "suppress Claude output")
	fs.IntVar(&cfg.maxIssues, "max-issues", 10, "max issues to process")
	fs.StringVar(&cfg.promptArg, "prompt", "", "user prompt text")
	fs.StringVar(&cfg.branch, "branch", "", "generation branch to work on")
}

// resolveCobblerBranch sets cfg.branch from the first positional arg if unset.
func resolveCobblerBranch(cfg *cobblerConfig, fs *flag.FlagSet) {
	if cfg.branch == "" && fs.NArg() > 0 {
		cfg.branch = fs.Arg(0)
	}
}

// runClaude executes Claude with the given prompt.
// If dir is non-empty, the command runs in that directory.
func runClaude(prompt, dir string, silence bool) error {
	fmt.Println("Running Claude...")

	cmd := exec.Command(binClaude, claudeArgs...)
	cmd.Stdin = strings.NewReader(prompt)
	if dir != "" {
		cmd.Dir = dir
	}

	if silence {
		return cmd.Run()
	}

	jq := exec.Command(binJq)
	jq.Stdout = os.Stdout
	jq.Stderr = os.Stderr

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	jq.Stdin = pipe

	if err := jq.Start(); err != nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	defer func() { _ = jq.Wait() }()

	return cmd.Run()
}

// worktreeBasePath returns the directory used for stitch worktrees.
func worktreeBasePath() string {
	repoRoot, _ := os.Getwd()
	return filepath.Join(os.TempDir(), filepath.Base(repoRoot)+"-worktrees")
}

// Cleanup resets the project to a clean state.
//
// Switches to main, removes all generation worktrees, task branches,
// generation branches and tags, resets beads, deletes Go source directories
// (cmd/, pkg/, internal/, tests/, bin/), and reinitializes go.mod.
func (Cobbler) Cleanup() error {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Cobbler cleanup: resetting to clean state")
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

	// Clean orphaned worktree references.
	_ = gitWorktreePrune()

	// Remove worktree temp directory.
	if _, err := os.Stat(wtBase); err == nil {
		fmt.Printf("Removing worktree directory: %s\n", wtBase)
		os.RemoveAll(wtBase)
	}

	// Delete generation branches.
	if len(genBranches) > 0 {
		fmt.Println("Removing generation branches...")
		for _, gb := range genBranches {
			fmt.Printf("  Deleting branch: %s\n", gb)
			_ = gitForceDeleteBranch(gb)
		}
	}

	// Delete generation tags.
	fmt.Println("Removing generation tags...")
	removeGenerationTags()

	// Reset beads.
	fmt.Println("Resetting beads...")
	if err := bdAdminReset(); err != nil {
		return fmt.Errorf("resetting beads: %w", err)
	}

	// Remove Go source directories.
	fmt.Println("Removing Go source directories...")
	for _, dir := range goSourceDirs {
		fmt.Printf("  Removing %s\n", dir)
		os.RemoveAll(dir)
	}
	os.RemoveAll("bin/")

	// Reinitialize go.mod.
	fmt.Println("Reinitializing go.mod...")
	if err := reinitGoModule(); err != nil {
		return fmt.Errorf("reinitializing go module: %w", err)
	}

	fmt.Println()
	fmt.Println("Cleanup complete. Only main branch remains.")
	fmt.Println()
	return nil
}

// removeGenerationTags deletes all tags with the generation prefix.
func removeGenerationTags() {
	for _, tag := range gitListTags(genPrefix + "*") {
		fmt.Printf("  Deleting tag: %s\n", tag)
		_ = gitDeleteTag(tag)
	}
}

// beadsCommit syncs beads state and commits the .beads/ directory.
func beadsCommit(msg string) {
	_ = bdSync()
	_ = gitStageDir(beadsDir)
	_ = gitCommitAllowEmpty(msg)
}
