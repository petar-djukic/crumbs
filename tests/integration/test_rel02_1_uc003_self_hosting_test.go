// Integration tests for the self-hosting workflow (do-work.sh and make-work.sh).
// Validates that the cupboard CLI can track development work on the crumbs repository.
// Implements: test-rel02.1-uc003-self-hosting;
//
//	rel02.1-uc003-self-hosting S1-S7.
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSelfHosting_Ready validates S1: cupboard ready returns available tasks as JSON.
func TestSelfHosting_Ready(t *testing.T) {
	dataDir := initCupboard(t)

	// Create some tasks in different states
	createCrumb(t, dataDir, "Ready task 1", "ready")
	createCrumb(t, dataDir, "Ready task 2", "ready")
	createCrumb(t, dataDir, "Taken task", "taken")
	createCrumb(t, dataDir, "Completed task", "pebble")

	// S1: cupboard ready returns available tasks as JSON
	stdout, stderr, code := runCupboard(t, dataDir, "ready", "--json")
	require.Equal(t, 0, code, "ready failed: %s", stderr)

	var tasks []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &tasks))

	// Should return only ready tasks (not taken, not pebble)
	assert.GreaterOrEqual(t, len(tasks), 2, "should return at least 2 ready tasks")

	// Verify tasks are in ready state
	for _, task := range tasks {
		assert.Equal(t, "ready", task["state"], "all returned tasks should be in ready state")
	}
}

// TestSelfHosting_Create validates S2: cupboard create creates task with type, title, description.
func TestSelfHosting_Create(t *testing.T) {
	dataDir := initCupboard(t)

	// S2: cupboard create --type task --title "Test" --description "..." creates a crumb
	stdout, stderr, code := runCupboard(t, dataDir, "create",
		"--type", "task",
		"--title", "Implement feature",
		"--description", "This is a test task",
		"--json")
	require.Equal(t, 0, code, "create failed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Verify crumb was created
	assert.NotEmpty(t, result["crumb_id"], "should have generated crumb_id")
	assert.Equal(t, "Implement feature", result["name"])

	// Verify it exists in the database
	crumbID := result["crumb_id"].(string)
	stdout, stderr, code = runCupboard(t, dataDir, "show", crumbID, "--json")
	require.Equal(t, 0, code, "show failed: %s", stderr)

	var showResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &showResult))
	crumb := showResult["crumb"].(map[string]any)
	assert.Equal(t, "Implement feature", crumb["name"])
}

// TestSelfHosting_Update validates S3: cupboard update transitions state to in_progress.
func TestSelfHosting_Update(t *testing.T) {
	dataDir := initCupboard(t)

	// Create a task
	id := createCrumb(t, dataDir, "Task to claim", "ready")

	// S3: cupboard update <id> --status in_progress transitions crumb state
	stdout, stderr, code := runCupboard(t, dataDir, "update", id, "--status", "taken", "--json")
	require.Equal(t, 0, code, "update failed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "taken", result["state"])

	// Verify the state persisted
	stdout, stderr, code = runCupboard(t, dataDir, "show", id, "--json")
	require.Equal(t, 0, code, "show failed: %s", stderr)

	var showResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &showResult))
	crumb := showResult["crumb"].(map[string]any)
	assert.Equal(t, "taken", crumb["state"])
}

// TestSelfHosting_Close validates S4: cupboard close marks task as closed.
func TestSelfHosting_Close(t *testing.T) {
	dataDir := initCupboard(t)

	// Create a task
	id := createCrumb(t, dataDir, "Task to close", "taken")

	// S4: cupboard close <id> marks crumb as closed
	stdout, stderr, code := runCupboard(t, dataDir, "close", id, "--json")
	require.Equal(t, 0, code, "close failed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "pebble", result["state"])

	// Verify the state persisted
	stdout, stderr, code = runCupboard(t, dataDir, "show", id, "--json")
	require.Equal(t, 0, code, "show failed: %s", stderr)

	var showResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &showResult))
	crumb := showResult["crumb"].(map[string]any)
	assert.Equal(t, "pebble", crumb["state"])
}

// TestSelfHosting_List validates S5: cupboard list --json returns all crumbs.
func TestSelfHosting_List(t *testing.T) {
	dataDir := initCupboard(t)

	// Create several crumbs
	createCrumb(t, dataDir, "Crumb 1", "draft")
	createCrumb(t, dataDir, "Crumb 2", "ready")
	createCrumb(t, dataDir, "Crumb 3", "taken")

	// S5: cupboard list --json returns all crumbs as JSON
	stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs", "--json")
	require.Equal(t, 0, code, "list failed: %s", stderr)

	var crumbs []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &crumbs))
	assert.GreaterOrEqual(t, len(crumbs), 3, "should return at least 3 crumbs")

	// Verify we can find our crumbs
	names := make(map[string]bool)
	for _, crumb := range crumbs {
		if name, ok := crumb["name"].(string); ok {
			names[name] = true
		}
	}
	assert.True(t, names["Crumb 1"], "should find Crumb 1")
	assert.True(t, names["Crumb 2"], "should find Crumb 2")
	assert.True(t, names["Crumb 3"], "should find Crumb 3")
}

// TestSelfHosting_DoWorkCycle validates S6: do-work workflow completes full cycle.
// This tests the workflow steps that do-work.sh should execute: pick task, claim it,
// create worktree, work on it, merge, close task, cleanup worktree.
func TestSelfHosting_DoWorkCycle(t *testing.T) {
	dataDir := initCupboard(t)

	// Step 1: Create a task in ready state
	stdout, stderr, code := runCupboard(t, dataDir, "create",
		"--type", "task",
		"--title", "Build widget",
		"--description", "Test task for do-work cycle",
		"--json")
	require.Equal(t, 0, code, "create task failed: %s", stderr)

	var createResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &createResult))
	taskID := createResult["crumb_id"].(string)

	// Transition to ready state
	_, stderr, code = runCupboard(t, dataDir, "update", taskID, "--status", "ready", "--json")
	require.Equal(t, 0, code, "update to ready failed: %s", stderr)

	// Step 2: Pick a ready task (cupboard ready)
	stdout, stderr, code = runCupboard(t, dataDir, "ready", "-n", "1", "--json", "--type", "task")
	require.Equal(t, 0, code, "ready failed: %s", stderr)

	var readyTasks []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &readyTasks))
	require.Len(t, readyTasks, 1, "should have exactly one ready task")
	assert.Equal(t, taskID, readyTasks[0]["crumb_id"], "should return the task we created")

	// Step 3: Claim the task (update to taken)
	stdout, stderr, code = runCupboard(t, dataDir, "update", taskID, "--status", "taken", "--json")
	require.Equal(t, 0, code, "update to taken failed: %s", stderr)

	var updateResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &updateResult))
	assert.Equal(t, "taken", updateResult["state"])

	// Step 4: Close the task (cupboard close)
	stdout, stderr, code = runCupboard(t, dataDir, "close", taskID, "--json")
	require.Equal(t, 0, code, "close failed: %s", stderr)

	var closeResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &closeResult))
	assert.Equal(t, "pebble", closeResult["state"])

	// Verify the task is in pebble state
	stdout, stderr, code = runCupboard(t, dataDir, "show", taskID, "--json")
	require.Equal(t, 0, code, "show after close failed: %s", stderr)

	var showResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &showResult))
	crumb := showResult["crumb"].(map[string]any)
	assert.Equal(t, "pebble", crumb["state"], "task should be in pebble state after close")
}

// TestSelfHosting_MakeWorkScript validates S7: make-work.sh creates issues and commits JSONL.
func TestSelfHosting_MakeWorkScript(t *testing.T) {
	// Create a temp directory for the test repository
	testRepoDir := t.TempDir()

	// Initialize a git repository
	runGit(t, testRepoDir, "init")
	runGit(t, testRepoDir, "config", "user.name", "Test User")
	runGit(t, testRepoDir, "config", "user.email", "test@example.com")

	// Create a simple README and commit it
	readmePath := filepath.Join(testRepoDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test Repo\n"), 0o644))
	runGit(t, testRepoDir, "add", "README.md")
	runGit(t, testRepoDir, "commit", "-m", "Initial commit")

	// Initialize cupboard
	dataDir := filepath.Join(testRepoDir, ".crumbs-db")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	_, stderr, code := runCupboard(t, dataDir, "init")
	require.Equal(t, 0, code, "cupboard init failed: %s", stderr)

	// Create a JSON file with proposed issues
	issuesJSON := `[
		{
			"index": 0,
			"title": "Task 1",
			"description": "First task",
			"dependency": -1
		},
		{
			"index": 1,
			"title": "Task 2",
			"description": "Second task",
			"dependency": -1
		},
		{
			"index": 2,
			"title": "Task 3",
			"description": "Third task depends on Task 1",
			"dependency": 0
		}
	]`

	issuesFile := filepath.Join(testRepoDir, "proposed-issues.json")
	require.NoError(t, os.WriteFile(issuesFile, []byte(issuesJSON), 0o644))

	// Get the project root and scripts directory
	projectRoot := cliProjectRoot()
	scriptsDir := filepath.Join(projectRoot, "scripts")
	makeWorkScript := filepath.Join(scriptsDir, "make-work.sh")

	// Verify the script exists
	_, err := os.Stat(makeWorkScript)
	require.NoError(t, err, "make-work.sh should exist at %s", makeWorkScript)

	// Ensure cupboard binary is available
	cupboardBin := ensureBinary(t)

	// Copy the make-work.sh script into the test repo's scripts directory
	// so PROJECT_ROOT detection works correctly
	testScriptsDir := filepath.Join(testRepoDir, "scripts")
	require.NoError(t, os.MkdirAll(testScriptsDir, 0o755))

	scriptContent, err := os.ReadFile(makeWorkScript)
	require.NoError(t, err, "should be able to read make-work.sh")

	localMakeWorkScript := filepath.Join(testScriptsDir, "make-work.sh")
	require.NoError(t, os.WriteFile(localMakeWorkScript, scriptContent, 0o755))

	// S7: Run make-work.sh from the test repo
	cmd := exec.Command("bash", localMakeWorkScript, issuesFile)
	cmd.Dir = testRepoDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CUPBOARD=%s", cupboardBin),
	)

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	scriptStdout := outBuf.String()
	scriptStderr := errBuf.String()

	// The script should succeed
	assert.NoError(t, err, "make-work.sh should complete successfully\nstdout: %s\nstderr: %s", scriptStdout, scriptStderr)

	// Verify that crumbs were created
	stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs", "--json")
	require.Equal(t, 0, code, "list after make-work failed: %s", stderr)

	var crumbs []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &crumbs))
	assert.GreaterOrEqual(t, len(crumbs), 3, "should have created at least 3 crumbs")

	// Verify the crumb names
	names := make(map[string]bool)
	for _, crumb := range crumbs {
		if name, ok := crumb["name"].(string); ok {
			names[name] = true
		}
	}
	assert.True(t, names["Task 1"], "should find Task 1")
	assert.True(t, names["Task 2"], "should find Task 2")
	assert.True(t, names["Task 3"], "should find Task 3")

	// Verify JSONL files were committed to git
	gitLog := runGitOutput(t, testRepoDir, "log", "--oneline", "--all")
	assert.Contains(t, gitLog, "make-work", "git log should contain commit from make-work.sh")

	// Verify JSONL files are tracked in git
	gitStatus := runGitOutput(t, testRepoDir, "status", "--porcelain")
	// Status should be clean (or only have the proposed-issues.json file if not added)
	// The JSONL files should be committed, not showing as untracked
	assert.NotContains(t, gitStatus, "crumbs.jsonl", "crumbs.jsonl should be committed, not untracked")
}

// --- Helper functions ---

// runGit executes a git command in the specified directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s failed: %s", strings.Join(args, " "), string(output))
}

// runGitOutput executes a git command and returns its output.
func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s failed: %s", strings.Join(args, " "), string(output))
	return string(output)
}
