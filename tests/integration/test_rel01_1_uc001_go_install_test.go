// Integration tests for go install and basic CLI operations.
// Validates that the cupboard binary builds, prints help, initializes storage,
// and round-trips crumbs through set and list.
// Implements: test-rel01.1-uc001-go-install;
//             rel01.1-uc001-go-install S1-S5.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S1: go build completes without errors (proxy for go install) ---

func TestGoInstall_Build(t *testing.T) {
	tests := []struct {
		name string
		check func(t *testing.T)
	}{
		{
			name: "S1: go build ./cmd/cupboard succeeds",
			check: func(t *testing.T) {
				bin := ensureBinary(t)
				assert.NotEmpty(t, bin, "binary path must not be empty")
			},
		},
		{
			name: "S1b: built binary exists on disk",
			check: func(t *testing.T) {
				bin := ensureBinary(t)
				info, err := os.Stat(bin)
				require.NoError(t, err, "binary must exist on disk")
				assert.True(t, info.Mode().IsRegular(), "binary must be a regular file")
			},
		},
		{
			name: "S1c: built binary is executable",
			check: func(t *testing.T) {
				bin := ensureBinary(t)
				info, err := os.Stat(bin)
				require.NoError(t, err)
				assert.NotZero(t, info.Mode()&0111, "binary must have execute permission")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// --- S2: cupboard --help prints usage ---

func TestGoInstall_Help(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantExitCode   int
		stdoutContains []string
	}{
		{
			name:         "S2a: help flag prints usage",
			args:         []string{"--help"},
			wantExitCode: 0,
			stdoutContains: []string{"Usage:"},
		},
		{
			name:         "S2b: help shows init command",
			args:         []string{"--help"},
			wantExitCode: 0,
			stdoutContains: []string{"init"},
		},
		{
			name:         "S2c: help shows set command",
			args:         []string{"--help"},
			wantExitCode: 0,
			stdoutContains: []string{"set"},
		},
		{
			name:         "S2d: help shows list command",
			args:         []string{"--help"},
			wantExitCode: 0,
			stdoutContains: []string{"list"},
		},
		{
			name:         "S2e: help shows get command",
			args:         []string{"--help"},
			wantExitCode: 0,
			stdoutContains: []string{"get"},
		},
		{
			name:         "S2f: help shows delete command",
			args:         []string{"--help"},
			wantExitCode: 0,
			stdoutContains: []string{"delete"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := ensureBinary(t)
			cmd := exec.Command(bin, tt.args...)
			var outBuf strings.Builder
			cmd.Stdout = &outBuf
			cmd.Stderr = &outBuf
			err := cmd.Run()

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("run cupboard: %v", err)
				}
			}

			assert.Equal(t, tt.wantExitCode, exitCode)
			output := outBuf.String()
			for _, substr := range tt.stdoutContains {
				assert.Contains(t, output, substr)
			}
		})
	}
}

// --- S3: cupboard init creates data directory with JSONL files ---

func TestGoInstall_Init(t *testing.T) {
	expectedJSONLFiles := []string{
		"crumbs.jsonl",
		"trails.jsonl",
		"properties.jsonl",
		"links.jsonl",
		"stashes.jsonl",
		"metadata.jsonl",
	}

	tests := []struct {
		name  string
		check func(t *testing.T, dataDir string)
	}{
		{
			name: "S3a: init creates data directory",
			check: func(t *testing.T, dataDir string) {
				info, err := os.Stat(dataDir)
				require.NoError(t, err, "data directory must exist")
				assert.True(t, info.IsDir(), "data path must be a directory")
			},
		},
		{
			name: "S3b: init creates JSONL files",
			check: func(t *testing.T, dataDir string) {
				for _, name := range expectedJSONLFiles {
					path := filepath.Join(dataDir, name)
					_, err := os.Stat(path)
					assert.NoError(t, err, "expected %s to exist", name)
				}
			},
		},
		{
			name: "S3c: init is idempotent",
			check: func(t *testing.T, dataDir string) {
				_, stderr, code := runCupboard(t, dataDir, "init")
				assert.Equal(t, 0, code, "second init must succeed: %s", stderr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)
			tt.check(t, dataDir)
		})
	}
}

// --- S4: set and list round-trip a crumb ---

func TestGoInstall_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, dataDir string)
	}{
		{
			name: "S4a: set creates a crumb and returns JSON with crumb_id",
			check: func(t *testing.T, dataDir string) {
				stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", `{"name":"Install test crumb","state":"draft"}`)
				require.Equal(t, 0, code, "set crumbs failed: %s", stderr)
				assert.Contains(t, stdout, "crumb_id")
				assert.Contains(t, stdout, "Install test crumb")
			},
		},
		{
			name: "S4b: list returns the created crumb",
			check: func(t *testing.T, dataDir string) {
				_, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", `{"name":"Roundtrip test","state":"draft"}`)
				require.Equal(t, 0, code, "set crumbs failed: %s", stderr)

				stdout, _, code := runCupboard(t, dataDir, "list", "crumbs")
				require.Equal(t, 0, code)
				assert.Contains(t, stdout, "Roundtrip test")
			},
		},
		{
			name: "S4c: list with empty table returns empty array",
			check: func(t *testing.T, dataDir string) {
				stdout, _, code := runCupboard(t, dataDir, "list", "crumbs")
				require.Equal(t, 0, code)
				arr := parseJSONArray(t, stdout)
				assert.Len(t, arr, 0)
			},
		},
		{
			name: "S4d: get retrieves created crumb by ID",
			check: func(t *testing.T, dataDir string) {
				stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", `{"name":"Get by ID test","state":"draft"}`)
				require.Equal(t, 0, code, "set crumbs failed: %s", stderr)

				id := extractJSONField(t, stdout, "crumb_id")
				stdout, stderr, code = runCupboard(t, dataDir, "get", "crumbs", id)
				require.Equal(t, 0, code, "get crumbs failed: %s", stderr)
				assert.Contains(t, stdout, "Get by ID test")
			},
		},
		{
			name: "S4e: crumb persists across sessions",
			check: func(t *testing.T, dataDir string) {
				_, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", `{"name":"Persistence test","state":"draft"}`)
				require.Equal(t, 0, code, "set crumbs failed: %s", stderr)

				// List in a separate invocation (simulates new session).
				stdout, _, code := runCupboard(t, dataDir, "list", "crumbs")
				require.Equal(t, 0, code)
				assert.Contains(t, stdout, "Persistence test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)
			tt.check(t, dataDir)
		})
	}
}

// --- S5: No manual compilation steps required ---

func TestGoInstall_Standalone(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "S5a: version command works from any directory",
			check: func(t *testing.T) {
				bin := ensureBinary(t)
				tmpDir := t.TempDir()
				cmd := exec.Command(bin, "version")
				cmd.Dir = tmpDir
				out, err := cmd.Output()
				require.NoError(t, err, "version command must succeed from arbitrary directory")
				assert.Contains(t, string(out), "cupboard")
			},
		},
		{
			name: "S5b: init works from any directory with --data-dir",
			check: func(t *testing.T) {
				bin := ensureBinary(t)
				workDir := t.TempDir()
				dataDir := filepath.Join(t.TempDir(), "standalone-data")
				cmd := exec.Command(bin, "--data-dir", dataDir, "init")
				cmd.Dir = workDir
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "init must succeed from arbitrary directory: %s", string(out))

				info, err := os.Stat(dataDir)
				require.NoError(t, err, "data directory must be created")
				assert.True(t, info.IsDir())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}
