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
	silenceAgent     bool
	maxIssues        int
	userPrompt       string
	generationBranch string
	tokenFile        string
	noContainer      bool
}

// registerCobblerFlags adds the shared flags to fs.
func registerCobblerFlags(fs *flag.FlagSet, cfg *cobblerConfig) {
	fs.BoolVar(&cfg.silenceAgent, flagSilenceAgent, true, "suppress Claude output")
	fs.IntVar(&cfg.maxIssues, flagMaxIssues, 10, "max issues to process")
	fs.StringVar(&cfg.userPrompt, flagUserPrompt, "", "user prompt text")
	fs.StringVar(&cfg.generationBranch, flagGenerationBranch, "", "generation branch to work on")
	fs.StringVar(&cfg.tokenFile, flagTokenFile, defaultTokenFile, "token file name in .secrets/")
	fs.BoolVar(&cfg.noContainer, flagNoContainer, false, "skip container runtime, use local claude binary")
}

// resolveCobblerBranch sets cfg.generationBranch from the first positional arg
// if the flag was not provided. Only the first positional arg is used because
// a single branch name is the only expected positional argument.
func resolveCobblerBranch(cfg *cobblerConfig, fs *flag.FlagSet) {
	if cfg.generationBranch == "" && fs.NArg() > 0 {
		cfg.generationBranch = fs.Arg(0)
	}
}

// runClaude executes Claude with the given prompt.
// Auto-detects runtime: podman → docker → direct claude binary.
// If dir is non-empty, the command runs in that directory (or it
// becomes the container's /workspace mount). tokenFile selects
// which credential file from .secrets/ to use (container mode only).
func runClaude(prompt, dir string, silence bool, tokenFile string, noContainer bool) error {
	if !noContainer {
		if rt := containerRuntime(); rt != "" {
			fmt.Fprintf(os.Stderr, "Running Claude (%s)...\n", rt)
			return runClaudeContainer(rt, prompt, dir, tokenFile, silence)
		}
	}

	fmt.Fprintln(os.Stderr, "Running Claude (direct)...")
	cmd := exec.Command(binClaude, claudeArgs...)
	cmd.Stdin = strings.NewReader(prompt)
	if dir != "" {
		cmd.Dir = dir
	}
	if !silence {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// worktreeBasePath returns the directory used for stitch worktrees.
func worktreeBasePath() string {
	repoRoot, _ := os.Getwd()
	return filepath.Join(os.TempDir(), filepath.Base(repoRoot)+"-worktrees")
}

// Reset removes the cobbler scratch directory.
func (Cobbler) Reset() error {
	return cobblerReset()
}

// cobblerReset removes the cobbler scratch directory.
func cobblerReset() error {
	fmt.Println("Resetting cobbler...")
	os.RemoveAll(cobblerDir)
	fmt.Println("Cobbler reset complete.")
	return nil
}

// beadsCommit syncs beads state and commits the .beads/ directory.
func beadsCommit(msg string) {
	_ = bdSync()
	_ = gitStageDir(beadsDir)
	_ = gitCommitAllowEmpty(msg)
}
