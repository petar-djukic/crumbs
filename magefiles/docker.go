package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
)

// Container image constants.
const (
	dockerImageName  = "crumbs"
	dockerImageTag   = "latest"
	dockerfileDir    = "magefiles"
	secretsDir       = ".secrets"
	defaultTokenFile = "claude.json"
	containerCredDst = "/home/crumbs/.claude/.credentials.json"
)

// containerRuntime returns "podman" or "docker" if a working runtime
// is available, or "" if neither is usable. It checks both that the
// binary exists on PATH and that it can connect to its daemon/machine.
func containerRuntime() string {
	for _, name := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(name); err != nil {
			continue
		}
		if exec.Command(name, "info").Run() != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %s found on PATH but not usable (is the daemon/machine running?)\n", name)
			continue
		}
		return name
	}
	return ""
}

// imageRef returns the full image reference (name:tag).
func imageRef() string {
	return dockerImageName + ":" + dockerImageTag
}

// buildImage builds the container image from magefiles/Dockerfile.claude.
// The build context is the repo root.
func buildImage(rt string) error {
	fmt.Fprintln(os.Stderr, "Building container image...")
	cmd := exec.Command(rt, "build",
		"-t", imageRef(),
		"-f", filepath.Join(dockerfileDir, "Dockerfile.claude"),
		".")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// removeImage removes the container image. Errors are ignored because
// the image may not exist.
func removeImage(rt string) {
	fmt.Fprintln(os.Stderr, "Removing container image...")
	_ = exec.Command(rt, "rmi", imageRef()).Run()
}

// runClaudeContainer executes claude inside a container and returns token usage.
//
// dir is mounted as /workspace (repo root or worktree). The credential
// file from .secrets/ is bind-mounted read-only into the location
// Claude Code expects on Linux.
func runClaudeContainer(rt, prompt, dir, tokenFile string, silence bool) (claudeResult, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return claudeResult{}, fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Resolve absolute path for the credential file.
	repoRoot, _ := os.Getwd()
	credFile := filepath.Join(repoRoot, secretsDir, tokenFile)
	if _, err := os.Stat(credFile); err != nil {
		return claudeResult{}, fmt.Errorf("token file not found: %s", credFile)
	}

	args := []string{
		"run", "--rm", "-i",
		"-v", dir + ":/workspace",
		"-v", credFile + ":" + containerCredDst + ":ro",
		"-w", "/workspace",
		imageRef(),
		binClaude,
	}
	args = append(args, claudeArgs...)

	cmd := exec.Command(rt, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdoutBuf bytes.Buffer
	if silence {
		cmd.Stdout = &stdoutBuf
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	result := parseClaudeTokens(stdoutBuf.Bytes())
	return result, err
}

// Docker builds the container image, then runs claude with "Hello World".
func (Test) Docker() error {
	rt := containerRuntime()
	if rt == "" {
		return fmt.Errorf("no container runtime found (tried podman, docker)")
	}

	mg.Deps(Build)

	fmt.Fprintln(os.Stderr, "Testing container image with Hello World prompt...")
	_, err := runClaudeContainer(rt, "Say hello world and nothing else.", "", defaultTokenFile, false)
	return err
}
