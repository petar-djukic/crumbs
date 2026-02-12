// Integration tests for the generic table CLI commands (get, set, list, delete).
// Exercises the cupboard binary via os/exec against all table types.
// Implements: test-rel01.1-uc004-generic-table-cli;
//             prd009-cupboard-cli R3; rel01.1-uc004-generic-table-cli S1-S12.
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	cupboardBin  string
	buildOnce    sync.Once
	buildErr     error
	buildTmpDir  string
)

// ensureBinary builds the cupboard binary once and returns the path to it.
func ensureBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		buildTmpDir, buildErr = os.MkdirTemp("", "cupboard-cli-test-*")
		if buildErr != nil {
			return
		}
		binPath := filepath.Join(buildTmpDir, "cupboard")
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cupboard")
		cmd.Dir = cliProjectRoot()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		buildErr = cmd.Run()
		if buildErr == nil {
			cupboardBin = binPath
		}
	})
	require.NoError(t, buildErr, "build cupboard binary")
	return cupboardBin
}

// cliProjectRoot returns the absolute path to the project root by walking up
// from the working directory until go.mod is found.
func cliProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found")
		}
		dir = parent
	}
}

// runCupboard executes the cupboard binary with the given arguments and a
// --data-dir flag pointing to the provided data directory.
func runCupboard(t *testing.T, dataDir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := ensureBinary(t)
	fullArgs := append([]string{"--data-dir", dataDir}, args...)
	cmd := exec.Command(bin, fullArgs...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run cupboard: %v", err)
		}
	}
	return stdout, stderr, exitCode
}

// initCupboard initializes a fresh cupboard in a temp directory.
func initCupboard(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	_, stderr, code := runCupboard(t, dataDir, "init")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	return dataDir
}

// createCrumb creates a crumb via set and returns the crumb_id. The backend
// forces state to "draft" on creation, so if a different state is requested we
// issue a second set to update it.
func createCrumb(t *testing.T, dataDir, name, state string) string {
	t.Helper()
	payload := fmt.Sprintf(`{"name":"%s","state":"%s"}`, name, state)
	stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", payload)
	require.Equal(t, 0, code, "create crumb failed: %s", stderr)
	id := extractJSONField(t, stdout, "crumb_id")

	if state != "" && state != "draft" {
		updatePayload := fmt.Sprintf(`{"crumb_id":"%s","name":"%s","state":"%s"}`, id, name, state)
		_, stderr, code = runCupboard(t, dataDir, "set", "crumbs", id, updatePayload)
		require.Equal(t, 0, code, "update crumb state failed: %s", stderr)
	}
	return id
}

// createTrail creates a trail via set and returns the trail_id.
func createTrail(t *testing.T, dataDir, state string) string {
	t.Helper()
	payload := fmt.Sprintf(`{"state":"%s"}`, state)
	stdout, stderr, code := runCupboard(t, dataDir, "set", "trails", "", payload)
	require.Equal(t, 0, code, "create trail failed: %s", stderr)
	return extractJSONField(t, stdout, "trail_id")
}

// createLink creates a link via set and returns the link_id.
func createLink(t *testing.T, dataDir, linkType, fromID, toID string) string {
	t.Helper()
	payload := fmt.Sprintf(`{"link_type":"%s","from_id":"%s","to_id":"%s"}`, linkType, fromID, toID)
	stdout, stderr, code := runCupboard(t, dataDir, "set", "links", "", payload)
	require.Equal(t, 0, code, "create link failed: %s", stderr)
	return extractJSONField(t, stdout, "link_id")
}

// extractJSONField unmarshals JSON output and returns the string value of a field.
func extractJSONField(t *testing.T, jsonStr, field string) string {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &m), "unmarshal JSON: %s", jsonStr)
	val, ok := m[field]
	require.True(t, ok, "field %q not found in JSON: %s", field, jsonStr)
	return fmt.Sprintf("%v", val)
}

// parseJSONArray unmarshals a JSON array string into a slice of maps.
func parseJSONArray(t *testing.T, jsonStr string) []map[string]any {
	t.Helper()
	var arr []map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &arr), "unmarshal JSON array: %s", jsonStr)
	return arr
}

// --- Get command tests (S1, S2, S3) ---

func TestGenericTableCLI_GetCommand(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, dataDir string) []string
		args           []string
		wantExitCode   int
		stdoutContains string
		stderrContains string
	}{
		{
			name: "S1: get crumb by ID returns JSON object",
			setup: func(t *testing.T, dataDir string) []string {
				id := createCrumb(t, dataDir, "Test crumb", "draft")
				return []string{id}
			},
			args:           []string{"get", "crumbs"},
			wantExitCode:   0,
			stdoutContains: "Test crumb",
		},
		{
			name:           "S2: get nonexistent crumb returns exit 1",
			args:           []string{"get", "crumbs", "nonexistent-id-12345"},
			wantExitCode:   1,
			stderrContains: `entity "nonexistent-id-12345" not found in table "crumbs"`,
		},
		{
			name:           "S3: get with unknown table returns exit 1 with valid names",
			args:           []string{"get", "invalid-table", "abc123"},
			wantExitCode:   1,
			stderrContains: `unknown table "invalid-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)
			args := make([]string, len(tt.args))
			copy(args, tt.args)

			if tt.setup != nil {
				ids := tt.setup(t, dataDir)
				if len(ids) > 0 {
					args = append(args, ids[0])
				}
			}

			stdout, stderr, code := runCupboard(t, dataDir, args...)
			assert.Equal(t, tt.wantExitCode, code)

			if tt.stdoutContains != "" {
				assert.Contains(t, stdout, tt.stdoutContains)
			}
			if tt.stderrContains != "" {
				assert.Contains(t, stderr, tt.stderrContains)
			}
		})
	}
}

// --- Set command tests (S4, S5, S6) ---

func TestGenericTableCLI_SetCommand(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, dataDir string) []string
		args           []string
		wantExitCode   int
		stdoutContains string
		stderrContains string
		checkState     func(t *testing.T, dataDir, stdout string)
	}{
		{
			name:           "S4: set crumb with empty ID creates new entity",
			args:           []string{"set", "crumbs", "", `{"name":"New task","state":"draft"}`},
			wantExitCode:   0,
			stdoutContains: "crumb_id",
			checkState: func(t *testing.T, dataDir, stdout string) {
				id := extractJSONField(t, stdout, "crumb_id")
				assert.NotEmpty(t, id)
				assert.Contains(t, stdout, "New task")
			},
		},
		{
			name: "S5: set crumb with existing ID updates entity",
			setup: func(t *testing.T, dataDir string) []string {
				id := createCrumb(t, dataDir, "Original", "draft")
				return []string{id}
			},
			args:         []string{"set", "crumbs"},
			wantExitCode: 0,
			checkState: func(t *testing.T, dataDir, stdout string) {
				assert.Contains(t, stdout, "Updated")
			},
		},
		{
			name:           "S4b: set trail creates new trail",
			args:           []string{"set", "trails", "", `{"state":"active"}`},
			wantExitCode:   0,
			stdoutContains: "trail_id",
		},
		{
			name:           "S6: set with invalid JSON returns exit 1",
			args:           []string{"set", "crumbs", "", `{not valid json}`},
			wantExitCode:   1,
			stderrContains: "parse JSON",
		},
		{
			name:           "S3b: set with unknown table returns exit 1",
			args:           []string{"set", "unknown-table", "", `{"field":"value"}`},
			wantExitCode:   1,
			stderrContains: `unknown table "unknown-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)
			args := make([]string, len(tt.args))
			copy(args, tt.args)

			if tt.setup != nil {
				ids := tt.setup(t, dataDir)
				if len(ids) > 0 {
					id := ids[0]
					args = append(args, id, fmt.Sprintf(`{"crumb_id":"%s","name":"Updated","state":"ready"}`, id))
				}
			}

			stdout, stderr, code := runCupboard(t, dataDir, args...)
			assert.Equal(t, tt.wantExitCode, code)

			if tt.stdoutContains != "" {
				assert.Contains(t, stdout, tt.stdoutContains)
			}
			if tt.stderrContains != "" {
				assert.Contains(t, stderr, tt.stderrContains)
			}
			if tt.checkState != nil {
				tt.checkState(t, dataDir, stdout)
			}
		})
	}
}

// --- List command tests (S7, S8, S9) ---

func TestGenericTableCLI_ListCommand(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, dataDir string)
		args           []string
		wantExitCode   int
		stderrContains string
		checkOutput    func(t *testing.T, stdout string)
	}{
		{
			name: "S7: list crumbs returns JSON array",
			setup: func(t *testing.T, dataDir string) {
				createCrumb(t, dataDir, "Task A", "draft")
				createCrumb(t, dataDir, "Task B", "ready")
			},
			args:         []string{"list", "crumbs"},
			wantExitCode: 0,
			checkOutput: func(t *testing.T, stdout string) {
				arr := parseJSONArray(t, stdout)
				assert.Len(t, arr, 2)
			},
		},
		{
			name:         "S7b: list empty table returns empty JSON array",
			args:         []string{"list", "crumbs"},
			wantExitCode: 0,
			checkOutput: func(t *testing.T, stdout string) {
				arr := parseJSONArray(t, stdout)
				assert.Len(t, arr, 0)
			},
		},
		{
			name: "S8: list with State=draft filter returns matching entities",
			setup: func(t *testing.T, dataDir string) {
				createCrumb(t, dataDir, "Draft task", "draft")
				createCrumb(t, dataDir, "Ready task", "ready")
			},
			args:         []string{"list", "crumbs", "State=draft"},
			wantExitCode: 0,
			checkOutput: func(t *testing.T, stdout string) {
				arr := parseJSONArray(t, stdout)
				assert.Len(t, arr, 1)
				assert.Contains(t, stdout, "Draft task")
			},
		},
		{
			name:           "S9: list with invalid filter returns exit 1",
			args:           []string{"list", "crumbs", "malformed-filter"},
			wantExitCode:   1,
			stderrContains: `invalid filter "malformed-filter" (expected key=value)`,
		},
		{
			name:           "S3c: list with unknown table returns exit 1",
			args:           []string{"list", "fake-table"},
			wantExitCode:   1,
			stderrContains: `unknown table "fake-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)

			if tt.setup != nil {
				tt.setup(t, dataDir)
			}

			stdout, stderr, code := runCupboard(t, dataDir, tt.args...)
			assert.Equal(t, tt.wantExitCode, code)

			if tt.stderrContains != "" {
				assert.Contains(t, stderr, tt.stderrContains)
			}
			if tt.checkOutput != nil {
				tt.checkOutput(t, stdout)
			}
		})
	}
}

// --- Delete command tests (S10, S11) ---

func TestGenericTableCLI_DeleteCommand(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, dataDir string) string
		args           []string
		wantExitCode   int
		stdoutContains string
		stderrContains string
		checkState     func(t *testing.T, dataDir string)
	}{
		{
			name: "S10: delete crumb by ID succeeds",
			setup: func(t *testing.T, dataDir string) string {
				return createCrumb(t, dataDir, "To delete", "draft")
			},
			args:           []string{"delete", "crumbs"},
			wantExitCode:   0,
			stdoutContains: "Deleted crumbs/",
			checkState: func(t *testing.T, dataDir string) {
				stdout, _, code := runCupboard(t, dataDir, "list", "crumbs")
				require.Equal(t, 0, code)
				arr := parseJSONArray(t, stdout)
				assert.Len(t, arr, 0)
			},
		},
		{
			name:           "S11: delete nonexistent crumb returns exit 1",
			args:           []string{"delete", "crumbs", "nonexistent-id-xyz"},
			wantExitCode:   1,
			stderrContains: `entity "nonexistent-id-xyz" not found in table "crumbs"`,
		},
		{
			name:           "S3d: delete with unknown table returns exit 1",
			args:           []string{"delete", "bad-table", "some-id"},
			wantExitCode:   1,
			stderrContains: `unknown table "bad-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)
			args := make([]string, len(tt.args))
			copy(args, tt.args)

			if tt.setup != nil {
				id := tt.setup(t, dataDir)
				args = append(args, id)
			}

			stdout, stderr, code := runCupboard(t, dataDir, args...)
			assert.Equal(t, tt.wantExitCode, code)

			if tt.stdoutContains != "" {
				assert.Contains(t, stdout, tt.stdoutContains)
			}
			if tt.stderrContains != "" {
				assert.Contains(t, stderr, tt.stderrContains)
			}
			if tt.checkState != nil {
				tt.checkState(t, dataDir)
			}
		})
	}
}

// --- Cross-table operation tests (S12) ---

func TestGenericTableCLI_CrossTableOperations(t *testing.T) {
	t.Run("S12a: create and get trail", func(t *testing.T) {
		dataDir := initCupboard(t)

		trailID := createTrail(t, dataDir, "active")
		assert.NotEmpty(t, trailID)

		stdout, stderr, code := runCupboard(t, dataDir, "get", "trails", trailID)
		assert.Equal(t, 0, code, "get trail failed: %s", stderr)
		assert.Contains(t, stdout, trailID)

		stdout, _, code = runCupboard(t, dataDir, "list", "trails")
		assert.Equal(t, 0, code)
		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 1)

		stdout, stderr, code = runCupboard(t, dataDir, "delete", "trails", trailID)
		assert.Equal(t, 0, code, "delete trail failed: %s", stderr)
		assert.Contains(t, stdout, "Deleted trails/")
	})

	t.Run("S12b: create and get link", func(t *testing.T) {
		dataDir := initCupboard(t)

		crumbID := createCrumb(t, dataDir, "Test crumb", "draft")
		trailID := createTrail(t, dataDir, "active")
		linkID := createLink(t, dataDir, "belongs_to", crumbID, trailID)
		assert.NotEmpty(t, linkID)

		stdout, stderr, code := runCupboard(t, dataDir, "get", "links", linkID)
		assert.Equal(t, 0, code, "get link failed: %s", stderr)
		assert.Contains(t, stdout, "belongs_to")

		stdout, _, code = runCupboard(t, dataDir, "list", "links")
		assert.Equal(t, 0, code)
		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 1)
	})

	t.Run("S12c: filter across multiple crumbs", func(t *testing.T) {
		dataDir := initCupboard(t)

		createCrumb(t, dataDir, "Draft 1", "draft")
		createCrumb(t, dataDir, "Draft 2", "draft")
		createCrumb(t, dataDir, "Ready 1", "ready")
		createCrumb(t, dataDir, "Taken 1", "taken")

		stdout, _, code := runCupboard(t, dataDir, "list", "crumbs", "State=draft")
		assert.Equal(t, 0, code)
		draftArr := parseJSONArray(t, stdout)
		assert.Len(t, draftArr, 2)

		stdout, _, code = runCupboard(t, dataDir, "list", "crumbs")
		assert.Equal(t, 0, code)
		allArr := parseJSONArray(t, stdout)
		assert.Len(t, allArr, 4)

		stdout, _, code = runCupboard(t, dataDir, "list", "crumbs", "State=ready")
		assert.Equal(t, 0, code)
		readyArr := parseJSONArray(t, stdout)
		assert.Len(t, readyArr, 1)
	})
}
