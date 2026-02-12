package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// currentGeneration holds the active generation name. When set, logf
// includes it right after the timestamp so every log line within a
// generation is tagged automatically.
var currentGeneration string

// setGeneration sets the active generation name for log tagging.
func setGeneration(name string) { currentGeneration = name }

// clearGeneration removes the generation tag from subsequent log lines.
func clearGeneration() { currentGeneration = "" }

// logf prints a timestamped log line to stderr. When currentGeneration
// is set, the generation name appears right after the timestamp.
func logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format(time.RFC3339)
	if currentGeneration != "" {
		fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n", ts, currentGeneration, msg)
	} else {
		fmt.Fprintf(os.Stderr, "[%s] %s\n", ts, msg)
	}
}

// cobblerConfig holds options shared by measure and stitch targets.
type cobblerConfig struct {
	silenceAgent     bool
	maxIssues        int
	userPrompt       string
	generationBranch string
	tokenFile        string
	noContainer      bool
}

// logConfig prints the resolved configuration for debugging.
func (c *cobblerConfig) logConfig(target string) {
	logf("%s config: silenceAgent=%v maxIssues=%d noContainer=%v tokenFile=%s generationBranch=%q",
		target, c.silenceAgent, c.maxIssues, c.noContainer, c.tokenFile, c.generationBranch)
	if c.userPrompt != "" {
		logf("%s config: userPrompt=%q", target, c.userPrompt)
	}
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
	logf("runClaude: promptLen=%d dir=%q silence=%v noContainer=%v", len(prompt), dir, silence, noContainer)

	if !noContainer {
		if rt := containerRuntime(); rt != "" {
			logf("runClaude: Running Claude (%s)", rt)
			start := time.Now()
			err := runClaudeContainer(rt, prompt, dir, tokenFile, silence)
			logf("runClaude: container finished in %s (err=%v)", time.Since(start).Round(time.Second), err)
			return err
		}
		logf("runClaude: no container runtime available, falling back to direct")
	}

	logf("runClaude: Running Claude (direct)")
	logf("runClaude: exec %s %v", binClaude, claudeArgs)
	cmd := exec.Command(binClaude, claudeArgs...)
	cmd.Stdin = strings.NewReader(prompt)
	if dir != "" {
		cmd.Dir = dir
	}
	if !silence {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	start := time.Now()
	err := cmd.Run()
	logf("runClaude: direct finished in %s (err=%v)", time.Since(start).Round(time.Second), err)
	return err
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
	logf("cobblerReset: removing %s", cobblerDir)
	os.RemoveAll(cobblerDir)
	logf("cobblerReset: done")
	return nil
}

// beadsCommit syncs beads state and commits the .beads/ directory.
func beadsCommit(msg string) {
	logf("beadsCommit: %s", msg)
	if err := bdSync(); err != nil {
		logf("beadsCommit: bdSync warning: %v", err)
	}
	if err := gitStageDir(beadsDir); err != nil {
		logf("beadsCommit: gitStageDir warning: %v", err)
	}
	if err := gitCommitAllowEmpty(msg); err != nil {
		logf("beadsCommit: gitCommit warning: %v", err)
	}
	logf("beadsCommit: done")
}
