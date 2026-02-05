// Package integration provides CLI integration tests for cupboard.
// Implements: crumbs-ag8.1 (convert validation script to Go tests).
package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	// cupboardBin is the path to the built cupboard binary.
	cupboardBin string
	// buildErr captures any build error.
	buildErr error
)

// BuildError wraps a build error with output.
type BuildError struct {
	Err    error
	Output string
}

func (e *BuildError) Error() string {
	return e.Err.Error() + ": " + e.Output
}

// FindProjectRoot finds the project root by walking up and looking for go.mod.
func FindProjectRoot() (string, error) {
	// Start from the current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// SetCupboardBin sets the path to the cupboard binary (called from TestMain).
func SetCupboardBin(path string) {
	cupboardBin = path
}

// SetBuildErr sets the build error (called from TestMain).
func SetBuildErr(err error) {
	buildErr = err
}

// TestEnv provides an isolated test environment with its own config and data directory.
type TestEnv struct {
	t       *testing.T
	TempDir string
	Config  string
	DataDir string
}

// NewTestEnv creates a new isolated test environment.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	if buildErr != nil {
		t.Fatalf("failed to build cupboard: %v", buildErr)
	}
	if cupboardBin == "" {
		t.Fatal("cupboard binary not built (cupboardBin is empty)")
	}

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, ".crumbs-data")
	configFile := filepath.Join(tempDir, ".crumbs.yaml")

	// Write config file
	configContent := "backend: sqlite\ndatadir: " + dataDir + "\n"
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return &TestEnv{
		t:       t,
		TempDir: tempDir,
		Config:  configFile,
		DataDir: dataDir,
	}
}

// CmdResult holds the result of a cupboard command execution.
type CmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCupboard executes the cupboard CLI with the given arguments.
// Returns stdout, stderr, and exit code.
func (e *TestEnv) RunCupboard(args ...string) CmdResult {
	e.t.Helper()

	allArgs := append([]string{"--config", e.Config}, args...)
	cmd := exec.Command(cupboardBin, allArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			e.t.Fatalf("failed to run cupboard: %v", err)
		}
	}

	return CmdResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// MustRunCupboard executes the cupboard CLI and fails the test if it returns non-zero.
func (e *TestEnv) MustRunCupboard(args ...string) CmdResult {
	e.t.Helper()
	result := e.RunCupboard(args...)
	if result.ExitCode != 0 {
		e.t.Fatalf("cupboard %v failed with exit code %d:\nstdout: %s\nstderr: %s",
			args, result.ExitCode, result.Stdout, result.Stderr)
	}
	return result
}

// ParseJSON parses JSON output into the target type.
func ParseJSON[T any](t *testing.T, jsonStr string) T {
	t.Helper()
	var result T
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to parse JSON %q: %v", jsonStr, err)
	}
	return result
}

// Crumb represents a crumb entity for JSON parsing.
type Crumb struct {
	CrumbID    string         `json:"CrumbID"`
	Name       string         `json:"Name"`
	State      string         `json:"State"`
	CreatedAt  string         `json:"CreatedAt"`
	UpdatedAt  string         `json:"UpdatedAt"`
	Properties map[string]any `json:"Properties"`
}

// Trail represents a trail entity for JSON parsing.
type Trail struct {
	TrailID       string  `json:"TrailID"`
	ParentCrumbID *string `json:"ParentCrumbID"`
	State         string  `json:"State"`
	CreatedAt     string  `json:"CreatedAt"`
	CompletedAt   *string `json:"CompletedAt"`
}

// Link represents a link entity for JSON parsing.
type Link struct {
	LinkID    string `json:"LinkID"`
	LinkType  string `json:"LinkType"`
	FromID    string `json:"FromID"`
	ToID      string `json:"ToID"`
	CreatedAt string `json:"CreatedAt"`
}

// JSONFile represents the structure of persisted JSON files.
type JSONFile struct {
	Crumbs []struct {
		CrumbID string `json:"crumb_id"`
		Name    string `json:"name"`
		State   string `json:"state"`
	} `json:"crumbs"`
}

// ReadJSONFile reads and parses a JSON file from the data directory.
func ReadJSONFile[T any](t *testing.T, path string) T {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return ParseJSON[T](t, string(data))
}
