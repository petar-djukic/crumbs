package main

import (
	"os"
	"os/exec"
)

// Binary names.
const (
	binGit    = "git"
	binBd     = "bd"
	binClaude = "claude"
	binGo     = "go"
	binLint   = "golangci-lint"
)

// Paths and prefixes.
const (
	beadsDir     = ".beads/"
	cobblerDir   = ".cobbler/"
	modulePath   = "github.com/mesh-intelligence/crumbs"
	genPrefix    = "generation-"
	versionFile  = "pkg/crumbs/version.go"
	cupboardMain = "cmd/cupboard/main.go"
)

// Flag names for cobbler targets.
const (
	flagSilenceAgent     = "silence-agent"
	flagMaxIssues        = "max-issues"
	flagUserPrompt       = "user-prompt"
	flagGenerationBranch = "generation-branch"
	flagCycles           = "cycles"
	flagTokenFile        = "token-file"
	flagNoContainer      = "no-container"
)

// claudeArgs are the CLI arguments for automated Claude execution.
var claudeArgs = []string{
	"--dangerously-skip-permissions",
	"-p",
	"--verbose",
	"--output-format", "stream-json",
}

// Git helpers.

func gitCheckout(branch string) error {
	return exec.Command(binGit, "checkout", branch).Run()
}

func gitCheckoutNew(branch string) error {
	return exec.Command(binGit, "checkout", "-b", branch).Run()
}

func gitCreateBranch(name string) error {
	return exec.Command(binGit, "branch", name).Run()
}

func gitDeleteBranch(name string) error {
	return exec.Command(binGit, "branch", "-d", name).Run()
}

func gitForceDeleteBranch(name string) error {
	return exec.Command(binGit, "branch", "-D", name).Run()
}

func gitBranchExists(name string) bool {
	return exec.Command(binGit, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

func gitListBranches(pattern string) []string {
	out, _ := exec.Command(binGit, "branch", "--list", pattern).Output()
	return parseBranchList(string(out))
}

func gitTag(name string) error {
	return exec.Command(binGit, "tag", name).Run()
}

func gitDeleteTag(name string) error {
	return exec.Command(binGit, "tag", "-d", name).Run()
}

// gitRenameTag creates newName at the same commit as oldName, then
// deletes oldName. Returns an error if the new tag cannot be created.
func gitRenameTag(oldName, newName string) error {
	// Create new tag pointing at the same commit as old tag.
	if err := exec.Command(binGit, "tag", newName, oldName).Run(); err != nil {
		return err
	}
	return gitDeleteTag(oldName)
}

func gitListTags(pattern string) []string {
	out, _ := exec.Command(binGit, "tag", "--list", pattern).Output()
	return parseBranchList(string(out))
}

func gitStageAll() error {
	return exec.Command(binGit, "add", "-A").Run()
}

func gitUnstageAll() error {
	return exec.Command(binGit, "reset", "HEAD").Run()
}

func gitStageDir(dir string) error {
	return exec.Command(binGit, "add", dir).Run()
}

func gitCommit(msg string) error {
	return exec.Command(binGit, "commit", "-m", msg).Run()
}

func gitCommitAllowEmpty(msg string) error {
	return exec.Command(binGit, "commit", "-m", msg, "--allow-empty").Run()
}

func gitRevParseHEAD() (string, error) {
	out, err := exec.Command(binGit, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return string(out[:len(out)-1]), nil
}

func gitResetSoft(ref string) error {
	return exec.Command(binGit, "reset", "--soft", ref).Run()
}

func gitMergeCmd(branch string) *exec.Cmd {
	return exec.Command(binGit, "merge", branch, "--no-edit")
}

func gitWorktreePrune() error {
	return exec.Command(binGit, "worktree", "prune").Run()
}

func gitWorktreeAdd(dir, branch string) *exec.Cmd {
	return exec.Command(binGit, "worktree", "add", dir, branch)
}

func gitWorktreeRemove(dir string) error {
	return exec.Command(binGit, "worktree", "remove", dir, "--force").Run()
}

func gitCurrentBranch() (string, error) {
	out, err := exec.Command(binGit, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return string(out[:len(out)-1]), nil // trim trailing newline
}

// Beads helpers.

func bdSync() error {
	return exec.Command(binBd, "sync").Run()
}

func bdAdminReset() error {
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		return nil // nothing to reset
	}
	// Stop the daemon before destroying the database; otherwise the
	// stale daemon blocks subsequent bd commands. The daemon stop
	// subcommand itself requires a database, so we must stop it while
	// .beads/ still exists.
	_ = exec.Command(binBd, "daemon", "stop", ".").Run()
	return exec.Command(binBd, "admin", "reset", "--force").Run()
}

func bdInit(prefix string) error {
	return exec.Command(binBd, "init", "--prefix", prefix, "--force").Run()
}

func bdClose(id string) error {
	return exec.Command(binBd, "close", id).Run()
}

func bdUpdateStatus(id, status string) error {
	return exec.Command(binBd, "update", id, "--status", status).Run()
}

func bdListJSON() ([]byte, error) {
	return exec.Command(binBd, "list", "--json").Output()
}

func bdListInProgressTasks() ([]byte, error) {
	return exec.Command(binBd, "list", "--json", "--status", "in_progress", "--type", "task").Output()
}

func bdNextReadyTask() ([]byte, error) {
	return exec.Command(binBd, "ready", "-n", "1", "--json", "--type", "task").Output()
}

func bdAddDep(childID, parentID string) error {
	return exec.Command(binBd, "dep", "add", childID, parentID).Run()
}

func bdCreateTask(title, description string) ([]byte, error) {
	return exec.Command(binBd, "create", "--type", "task", "--json", title, "--description", description).Output()
}

func bdListClosedTasks() ([]byte, error) {
	return exec.Command(binBd, "list", "--json", "--status", "closed", "--type", "task").Output()
}

func bdListReadyTasks() ([]byte, error) {
	return exec.Command(binBd, "list", "--json", "--status", "ready", "--type", "task").Output()
}

// Go helpers.

func goModInit() error {
	return exec.Command(binGo, "mod", "init", modulePath).Run()
}

func goModEditReplace(old, new string) error {
	return exec.Command(binGo, "mod", "edit", "-replace", old+"="+new).Run()
}

func goModTidy() error {
	return exec.Command(binGo, "mod", "tidy").Run()
}
