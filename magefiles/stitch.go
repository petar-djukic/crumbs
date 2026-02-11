package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed prompts/stitch.tmpl
var stitchPromptTmpl string

// stitchConfig holds options for the Stitch target.
type stitchConfig struct {
	cobblerConfig
}

func parseStitchFlags() stitchConfig {
	var cfg stitchConfig
	fs := flag.NewFlagSet("cobbler:stitch", flag.ContinueOnError)
	registerCobblerFlags(fs, &cfg.cobblerConfig)
	parseTargetFlags(fs)
	resolveCobblerBranch(&cfg.cobblerConfig, fs)
	return cfg
}

// Stitch picks ready tasks from beads and invokes Claude to execute them.
//
// Flags:
//
//	--silence-agent          suppress Claude output (default true)
//	--max-issues N           max issues to process (default 10)
//	--user-prompt TEXT       user prompt text
//	--generation-branch NAME generation branch to work on
func (Cobbler) Stitch() error {
	return stitch(parseStitchFlags())
}

func stitch(cfg stitchConfig) error {
	branch, err := resolveBranch(cfg.generationBranch)
	if err != nil {
		return err
	}
	if err := ensureOnBranch(branch); err != nil {
		return fmt.Errorf("switching to branch: %w", err)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	worktreeBase := worktreeBasePath()

	baseBranch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	fmt.Printf("Base branch: %s\n", baseBranch)

	if err := recoverStaleTasks(baseBranch, worktreeBase); err != nil {
		return fmt.Errorf("recovery: %w", err)
	}

	totalTasks := 0
	for {
		task, err := pickTask(baseBranch, worktreeBase)
		if err != nil {
			break // No tasks available.
		}

		if err := doOneTask(task, baseBranch, repoRoot, cfg.silenceAgent, cfg.tokenFile); err != nil {
			return fmt.Errorf("executing task %s: %w", task.id, err)
		}

		totalTasks++
		fmt.Println()
		fmt.Println("----------------------------------------")
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Done. Completed %d task(s).\n", totalTasks)
	fmt.Println("========================================")
	return nil
}

type stitchTask struct {
	id          string
	title       string
	description string
	issueType   string
	branchName  string
	worktreeDir string
}

// recoverStaleTasks cleans up task branches and orphaned in_progress issues
// from a previous interrupted run.
func recoverStaleTasks(baseBranch, worktreeBase string) error {
	staleBranches := recoverStaleBranches(baseBranch, worktreeBase)
	orphanedIssues := resetOrphanedIssues(baseBranch)

	_ = gitWorktreePrune()

	if staleBranches || orphanedIssues {
		beadsCommit("Recover stale tasks from interrupted run")
		fmt.Println("Recovery complete.")
		fmt.Println()
	}

	return nil
}

// recoverStaleBranches removes leftover task branches and worktrees,
// resetting their issues to ready. Returns true if any were recovered.
func recoverStaleBranches(baseBranch, worktreeBase string) bool {
	branches := gitListBranches(baseBranch + "/task/*")
	if len(branches) == 0 {
		return false
	}

	for _, branch := range branches {
		fmt.Printf("Recovering stale branch: %s\n", branch)

		issueID := branch[strings.LastIndex(branch, "/")+1:]
		worktreeDir := filepath.Join(worktreeBase, issueID)

		if _, err := os.Stat(worktreeDir); err == nil {
			fmt.Printf("  Removing worktree: %s\n", worktreeDir)
			_ = gitWorktreeRemove(worktreeDir)
		}

		fmt.Printf("  Deleting branch: %s\n", branch)
		_ = gitForceDeleteBranch(branch)

		if issueID != "" {
			fmt.Printf("  Resetting issue to ready: %s\n", issueID)
			_ = bdUpdateStatus(issueID, "ready")
		}
	}
	return true
}

// resetOrphanedIssues finds in_progress issues with no corresponding task
// branch and resets them to ready. Returns true if any were reset.
func resetOrphanedIssues(baseBranch string) bool {
	out, _ := bdListInProgressTasks()
	if len(out) == 0 || string(out) == "[]" {
		return false
	}

	var issues []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &issues); err != nil {
		return false
	}

	recovered := false
	for _, issue := range issues {
		taskBranch := baseBranch + "/task/" + issue.ID
		if !gitBranchExists(taskBranch) {
			recovered = true
			fmt.Printf("Resetting orphaned in_progress issue: %s\n", issue.ID)
			_ = bdUpdateStatus(issue.ID, "ready")
		}
	}
	return recovered
}

func parseBranchList(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "*+ ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches
}

func pickTask(baseBranch, worktreeBase string) (stitchTask, error) {
	out, err := bdNextReadyTask()
	if err != nil || len(out) == 0 || string(out) == "[]" {
		return stitchTask{}, fmt.Errorf("no tasks available")
	}

	var issues []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal(out, &issues); err != nil || len(issues) == 0 {
		return stitchTask{}, fmt.Errorf("failed to parse issue")
	}

	issue := issues[0]
	task := stitchTask{
		id:          issue.ID,
		title:       issue.Title,
		description: issue.Description,
		issueType:   issue.Type,
		branchName:  baseBranch + "/task/" + issue.ID,
		worktreeDir: filepath.Join(worktreeBase, issue.ID),
	}

	if task.issueType == "" {
		task.issueType = "task"
	}

	fmt.Printf("Picking up task: %s - %s\n", task.id, task.title)
	return task, nil
}

func doOneTask(task stitchTask, baseBranch, repoRoot string, silence bool, tokenFile string) error {
	// Claim.
	fmt.Println("Task claimed.")
	_ = bdUpdateStatus(task.id, "in_progress")

	// Create worktree.
	if err := createWorktree(task); err != nil {
		return fmt.Errorf("creating worktree: %w", err)
	}

	// Build and run prompt.
	prompt := buildStitchPrompt(task)
	if err := runClaude(prompt, task.worktreeDir, silence, tokenFile); err != nil {
		return fmt.Errorf("running Claude: %w", err)
	}

	// Merge branch back.
	if err := mergeBranch(task.branchName, baseBranch, repoRoot); err != nil {
		return fmt.Errorf("merging branch: %w", err)
	}

	// Cleanup worktree.
	cleanupWorktree(task)

	// Close task.
	closeStitchTask(task)

	return nil
}

func createWorktree(task stitchTask) error {
	fmt.Printf("Creating worktree at %s...\n", task.worktreeDir)

	_ = os.MkdirAll(filepath.Dir(task.worktreeDir), 0o755)

	if !gitBranchExists(task.branchName) {
		if err := gitCreateBranch(task.branchName); err != nil {
			return fmt.Errorf("creating branch %s: %w", task.branchName, err)
		}
	}

	cmd := gitWorktreeAdd(task.worktreeDir, task.branchName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("adding worktree: %w", err)
	}

	fmt.Printf("Worktree created on branch %s\n\n", task.branchName)
	return nil
}

type stitchPromptData struct {
	Title       string
	ID          string
	IssueType   string
	Description string
}

func buildStitchPrompt(task stitchTask) string {
	tmpl := template.Must(template.New("stitch").Parse(stitchPromptTmpl))
	data := stitchPromptData{
		Title:       task.title,
		ID:          task.id,
		IssueType:   task.issueType,
		Description: task.description,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("stitch prompt template: %v", err))
	}
	return buf.String()
}

func mergeBranch(branchName, baseBranch, repoRoot string) error {
	fmt.Println()
	fmt.Printf("Merging %s into %s...\n", branchName, baseBranch)

	if err := gitCheckout(baseBranch); err != nil {
		return fmt.Errorf("checking out %s: %w", baseBranch, err)
	}

	cmd := gitMergeCmd(branchName)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merging %s: %w", branchName, err)
	}

	fmt.Println("Branch merged.")
	return nil
}

func cleanupWorktree(task stitchTask) {
	fmt.Println("Cleaning up worktree...")
	_ = gitWorktreeRemove(task.worktreeDir)
	_ = gitDeleteBranch(task.branchName)
	fmt.Println("Worktree removed.")
}

func closeStitchTask(task stitchTask) {
	fmt.Println()
	fmt.Printf("Closing task: %s\n", task.id)
	_ = bdClose(task.id)
	beadsCommit(fmt.Sprintf("Close %s", task.id))

	fmt.Println("Done.")
}
