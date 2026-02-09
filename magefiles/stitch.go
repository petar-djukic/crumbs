package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed prompts/stitch.tmpl
var stitchPromptTmpl string

// stitchConfig holds options for the Stitch target.
type stitchConfig struct {
	silence bool
	branch  string
}

func parseStitchFlags() stitchConfig {
	var cfg stitchConfig
	fs := flag.NewFlagSet("cobbler:stitch", flag.ContinueOnError)
	fs.BoolVar(&cfg.silence, "silence", false, "suppress Claude output")
	fs.StringVar(&cfg.branch, "branch", "", "generation branch to work on")
	parseTargetFlags(fs)
	if cfg.branch == "" && fs.NArg() > 0 {
		cfg.branch = fs.Arg(0)
	}
	return cfg
}

// Stitch picks ready tasks from beads and invokes Claude to execute them.
//
// Flags:
//
//	--silence       suppress Claude output
//	--branch NAME   generation branch to work on
func (Cobbler) Stitch() error {
	return stitch(parseStitchFlags())
}

func stitch(cfg stitchConfig) error {
	branch, err := resolveBranch(cfg.branch)
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

	projectName := filepath.Base(repoRoot)
	worktreeBase := filepath.Join(os.TempDir(), projectName+"-worktrees")

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

		if err := doOneTask(task, baseBranch, repoRoot, cfg.silence); err != nil {
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

func gitCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// recoverStaleTasks cleans up task branches and orphaned in_progress issues
// from a previous interrupted run.
func recoverStaleTasks(baseBranch, worktreeBase string) error {
	recovered := false

	// 1. Find stale task branches under <base>/task/*.
	out, _ := exec.Command("git", "branch", "--list", baseBranch+"/task/*").Output()
	branches := parseBranchList(string(out))

	for _, branch := range branches {
		recovered = true
		fmt.Printf("Recovering stale branch: %s\n", branch)

		issueID := branch[strings.LastIndex(branch, "/")+1:]
		worktreeDir := filepath.Join(worktreeBase, issueID)

		if _, err := os.Stat(worktreeDir); err == nil {
			fmt.Printf("  Removing worktree: %s\n", worktreeDir)
			_ = exec.Command("git", "worktree", "remove", worktreeDir, "--force").Run()
		}

		fmt.Printf("  Deleting branch: %s\n", branch)
		_ = exec.Command("git", "branch", "-D", branch).Run()

		if issueID != "" {
			fmt.Printf("  Resetting issue to ready: %s\n", issueID)
			_ = exec.Command("bd", "update", issueID, "--status", "ready").Run()
		}
	}

	// 2. Reset orphaned in_progress issues with no task branch.
	inProgressJSON, _ := exec.Command("bd", "list", "--json", "--status", "in_progress", "--type", "task").Output()
	if len(inProgressJSON) > 0 && string(inProgressJSON) != "[]" {
		var issues []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(inProgressJSON, &issues); err == nil {
			for _, issue := range issues {
				ref := fmt.Sprintf("refs/heads/%s/task/%s", baseBranch, issue.ID)
				if exec.Command("git", "show-ref", "--verify", "--quiet", ref).Run() != nil {
					recovered = true
					fmt.Printf("Resetting orphaned in_progress issue: %s\n", issue.ID)
					_ = exec.Command("bd", "update", issue.ID, "--status", "ready").Run()
				}
			}
		}
	}

	_ = exec.Command("git", "worktree", "prune").Run()

	if recovered {
		_ = exec.Command("bd", "sync").Run()
		_ = exec.Command("git", "add", ".beads/").Run()
		_ = exec.Command("git", "commit", "-m", "Recover stale tasks from interrupted run", "--allow-empty").Run()
		fmt.Println("Recovery complete.")
		fmt.Println()
	}

	return nil
}

func parseBranchList(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches
}

func pickTask(baseBranch, worktreeBase string) (stitchTask, error) {
	out, err := exec.Command("bd", "ready", "-n", "1", "--json", "--type", "task").Output()
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

func doOneTask(task stitchTask, baseBranch, repoRoot string, silence bool) error {
	// Claim.
	fmt.Println("Task claimed.")
	_ = exec.Command("bd", "update", task.id, "--status", "in_progress").Run()

	// Create worktree.
	if err := createWorktree(task); err != nil {
		return fmt.Errorf("creating worktree: %w", err)
	}

	// Build and run prompt.
	prompt := buildStitchPrompt(task)
	if err := runClaudeInWorktree(prompt, task.worktreeDir, repoRoot, silence); err != nil {
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

	// Create branch from current HEAD if it doesn't exist.
	ref := "refs/heads/" + task.branchName
	if exec.Command("git", "show-ref", "--verify", "--quiet", ref).Run() != nil {
		if err := exec.Command("git", "branch", task.branchName).Run(); err != nil {
			return fmt.Errorf("creating branch %s: %w", task.branchName, err)
		}
	}

	cmd := exec.Command("git", "worktree", "add", task.worktreeDir, task.branchName)
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

func runClaudeInWorktree(prompt, worktreeDir, repoRoot string, silence bool) error {
	fmt.Println("Running Claude in worktree...")

	args := []string{"--dangerously-skip-permissions", "-p", "--verbose", "--output-format", "stream-json"}
	cmd := exec.Command("claude", args...)
	cmd.Dir = worktreeDir
	cmd.Stdin = strings.NewReader(prompt)

	if silence {
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
		jq := exec.Command("jq")
		jq.Stdout = os.Stdout
		jq.Stderr = os.Stderr
		var pipeErr error
		jq.Stdin, pipeErr = cmd.StdoutPipe()
		if pipeErr != nil {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			if err := jq.Start(); err != nil {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			} else {
				defer func() { _ = jq.Wait() }()
			}
		}
	}

	return cmd.Run()
}

func mergeBranch(branchName, baseBranch, repoRoot string) error {
	fmt.Println()
	fmt.Printf("Merging %s into %s...\n", branchName, baseBranch)

	cmd := exec.Command("git", "checkout", baseBranch)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("checking out %s: %w", baseBranch, err)
	}

	cmd = exec.Command("git", "merge", branchName, "--no-edit")
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
	_ = exec.Command("git", "worktree", "remove", task.worktreeDir, "--force").Run()
	_ = exec.Command("git", "branch", "-d", task.branchName).Run()
	fmt.Println("Worktree removed.")
}

func closeStitchTask(task stitchTask) {
	fmt.Println()
	fmt.Printf("Closing task: %s\n", task.id)
	_ = exec.Command("bd", "close", task.id).Run()
	_ = exec.Command("bd", "sync").Run()

	fmt.Println("Committing beads changes...")
	_ = exec.Command("git", "add", ".beads/").Run()
	_ = exec.Command("git", "commit", "-m", fmt.Sprintf("Close %s", task.id), "--allow-empty").Run()

	fmt.Println("Done.")
}
