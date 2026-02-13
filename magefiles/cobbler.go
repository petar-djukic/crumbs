package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

// claudeResult holds token usage from a Claude invocation.
type claudeResult struct {
	InputTokens  int
	OutputTokens int
}

// locSnapshot holds a point-in-time LOC count.
type locSnapshot struct {
	Production int `json:"production"`
	Test       int `json:"test"`
}

// captureLOC returns the current Go LOC counts. Errors are swallowed
// because stats collection is best-effort.
func captureLOC() locSnapshot {
	rec, err := collectStats()
	if err != nil {
		logf("captureLOC: collectStats error: %v", err)
		return locSnapshot{}
	}
	return locSnapshot{Production: rec.GoProdLOC, Test: rec.GoTestLOC}
}

// invocationRecord is the JSON blob recorded as a beads comment after
// every Claude invocation. Multiple records may exist per issue.
type invocationRecord struct {
	Caller    string       `json:"caller"`
	StartedAt string      `json:"started_at"`
	DurationS int         `json:"duration_s"`
	Tokens    claudeTokens `json:"tokens"`
	LOCBefore locSnapshot  `json:"loc_before"`
	LOCAfter  locSnapshot  `json:"loc_after"`
	Diff      diffRecord   `json:"diff"`
}

type claudeTokens struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

type diffRecord struct {
	Files      int `json:"files"`
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

// recordInvocation serializes an invocationRecord to JSON and adds it
// as a beads comment on the given issue.
func recordInvocation(issueID string, rec invocationRecord) {
	data, err := json.Marshal(rec)
	if err != nil {
		logf("recordInvocation: marshal error: %v", err)
		return
	}
	if err := bdCommentAdd(issueID, string(data)); err != nil {
		logf("recordInvocation: bd comment error for %s: %v", issueID, err)
	}
}

// parseClaudeTokens extracts token usage from Claude's stream-json
// output. The final JSON line has "type":"result" with a "usage" object
// containing "input_tokens" and "output_tokens".
func parseClaudeTokens(output []byte) claudeResult {
	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		var msg struct {
			Type  string `json:"type"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(lines[i], &msg); err != nil {
			continue
		}
		if msg.Type == "result" {
			return claudeResult{
				InputTokens:  msg.Usage.InputTokens,
				OutputTokens: msg.Usage.OutputTokens,
			}
		}
	}
	return claudeResult{}
}

// runClaude executes Claude with the given prompt and returns token usage.
// Auto-detects runtime: podman → docker → direct claude binary.
// If dir is non-empty, the command runs in that directory (or it
// becomes the container's /workspace mount). tokenFile selects
// which credential file from .secrets/ to use (container mode only).
func runClaude(prompt, dir string, silence bool, tokenFile string, noContainer bool) (claudeResult, error) {
	logf("runClaude: promptLen=%d dir=%q silence=%v noContainer=%v", len(prompt), dir, silence, noContainer)

	if !noContainer {
		if rt := containerRuntime(); rt != "" {
			logf("runClaude: Running Claude (%s)", rt)
			start := time.Now()
			result, err := runClaudeContainer(rt, prompt, dir, tokenFile, silence)
			logf("runClaude: container finished in %s (err=%v)", time.Since(start).Round(time.Second), err)
			return result, err
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

	var stdoutBuf bytes.Buffer
	if silence {
		cmd.Stdout = &stdoutBuf
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = os.Stderr
	}

	start := time.Now()
	err := cmd.Run()
	result := parseClaudeTokens(stdoutBuf.Bytes())
	logf("runClaude: direct finished in %s tokens(in=%d out=%d) (err=%v)",
		time.Since(start).Round(time.Second), result.InputTokens, result.OutputTokens, err)
	return result, err
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
