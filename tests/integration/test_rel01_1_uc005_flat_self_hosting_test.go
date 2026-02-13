// Integration tests for the flat self-hosting issue-tracking workflow.
// Exercises the cupboard CLI as an issue tracker using only generic table
// commands (get, set, list, delete). No properties, trails, or issue-tracking
// commands are used.
// Implements: test-rel01.1-uc005-flat-self-hosting;
//
//	rel01.1-uc005-flat-self-hosting S1-S10.
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S1: Initialize cupboard ---

func TestFlatSelfHosting_Init(t *testing.T) {
	dataDir := t.TempDir()
	stdout, stderr, code := runCupboard(t, dataDir, "init")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	assert.Contains(t, stdout, "initialized")

	// Data directory should exist (created by init).
	_, err := os.Stat(dataDir)
	require.NoError(t, err, "data directory should exist")

	// JSONL file for crumbs should exist.
	jsonlPath := filepath.Join(dataDir, "crumbs.jsonl")
	_, err = os.Stat(jsonlPath)
	require.NoError(t, err, "crumbs.jsonl should exist after init")
}

// --- S2: Create crumb with set ---

func TestFlatSelfHosting_CreateCrumb(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		wantExitCode   int
		stdoutContains string
		stderrContains string
		checkOutput    func(t *testing.T, stdout string)
	}{
		{
			name:         "create crumb with name and state returns JSON with crumb_id",
			payload:      `{"name":"Implement feature","state":"draft"}`,
			wantExitCode: 0,
			checkOutput: func(t *testing.T, stdout string) {
				var m map[string]any
				require.NoError(t, json.Unmarshal([]byte(stdout), &m))
				assert.NotEmpty(t, m["crumb_id"], "crumb_id should be generated")
				assert.Equal(t, "Implement feature", m["name"])
				// State is forced to draft on creation.
				assert.Equal(t, "draft", m["state"])
			},
		},
		{
			name:         "create crumb returns generated UUID verifiable via list",
			payload:      `{"name":"Write tests","state":"draft"}`,
			wantExitCode: 0,
			checkOutput: func(t *testing.T, stdout string) {
				id := extractJSONField(t, stdout, "crumb_id")
				assert.NotEmpty(t, id)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := initCupboard(t)
			stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", tt.payload)
			assert.Equal(t, tt.wantExitCode, code)

			if tt.stdoutContains != "" {
				assert.Contains(t, stdout, tt.stdoutContains)
			}
			if tt.stderrContains != "" {
				assert.Contains(t, stderr, tt.stderrContains)
			}
			if tt.checkOutput != nil {
				tt.checkOutput(t, stdout)
			}
		})
	}
}

// --- S3: List all crumbs ---

func TestFlatSelfHosting_ListAll(t *testing.T) {
	t.Run("list returns all crumbs as JSON array", func(t *testing.T) {
		dataDir := initCupboard(t)
		createCrumb(t, dataDir, "Task A", "draft")
		createCrumb(t, dataDir, "Task B", "draft")

		stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs")
		require.Equal(t, 0, code, "list failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 2)
	})

	t.Run("list empty cupboard returns empty array", func(t *testing.T) {
		dataDir := initCupboard(t)

		stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs")
		require.Equal(t, 0, code, "list failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 0)
	})
}

// --- S4: List with State=draft filter ---

func TestFlatSelfHosting_ListWithFilter(t *testing.T) {
	t.Run("list with State=draft returns only draft crumbs", func(t *testing.T) {
		dataDir := initCupboard(t)
		createCrumb(t, dataDir, "Draft task", "draft")
		createCrumb(t, dataDir, "Taken task", "taken")

		stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs", "State=draft")
		require.Equal(t, 0, code, "list with filter failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 1)
		assert.Contains(t, stdout, "Draft task")
	})

	t.Run("list with State=taken returns only taken crumbs", func(t *testing.T) {
		dataDir := initCupboard(t)
		createCrumb(t, dataDir, "Draft task", "draft")
		createCrumb(t, dataDir, "Taken task", "taken")

		stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs", "State=taken")
		require.Equal(t, 0, code, "list with filter failed: %s", stderr)

		arr := parseJSONArray(t, stdout)
		assert.Len(t, arr, 1)
		assert.Contains(t, stdout, "Taken task")
	})
}

// --- S5: Update state to taken ---

func TestFlatSelfHosting_UpdateStateToTaken(t *testing.T) {
	dataDir := initCupboard(t)
	id := createCrumb(t, dataDir, "Claimable", "draft")

	payload := fmt.Sprintf(`{"crumb_id":"%s","name":"Claimable","state":"taken"}`, id)
	_, stderr, code := runCupboard(t, dataDir, "set", "crumbs", id, payload)
	require.Equal(t, 0, code, "update to taken failed: %s", stderr)

	// Verify the state changed.
	stdout, _, code := runCupboard(t, dataDir, "get", "crumbs", id)
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, `"taken"`)
}

// --- S6: Update state to pebble (close) ---

func TestFlatSelfHosting_UpdateStateToPebble(t *testing.T) {
	dataDir := initCupboard(t)
	id := createCrumb(t, dataDir, "Closeable", "taken")

	payload := fmt.Sprintf(`{"crumb_id":"%s","name":"Closeable","state":"pebble"}`, id)
	_, stderr, code := runCupboard(t, dataDir, "set", "crumbs", id, payload)
	require.Equal(t, 0, code, "update to pebble failed: %s", stderr)

	stdout, _, code := runCupboard(t, dataDir, "get", "crumbs", id)
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, `"pebble"`)
}

// --- S7: Get crumb details ---

func TestFlatSelfHosting_GetCrumb(t *testing.T) {
	t.Run("get crumb by ID returns JSON", func(t *testing.T) {
		dataDir := initCupboard(t)
		id := createCrumb(t, dataDir, "Visible task", "draft")

		stdout, stderr, code := runCupboard(t, dataDir, "get", "crumbs", id)
		require.Equal(t, 0, code, "get crumb failed: %s", stderr)
		assert.Contains(t, stdout, "Visible task")
		assert.Contains(t, stdout, id)
	})

	t.Run("get nonexistent ID returns exit 1", func(t *testing.T) {
		dataDir := initCupboard(t)

		_, _, code := runCupboard(t, dataDir, "get", "crumbs", "nonexistent-id")
		assert.Equal(t, 1, code)
	})

	t.Run("get with invalid table returns exit 1", func(t *testing.T) {
		dataDir := initCupboard(t)

		_, stderr, code := runCupboard(t, dataDir, "get", "invalid-table", "abc")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, `unknown table "invalid-table"`)
	})
}

// --- S7 (delete variant): Delete crumb ---

func TestFlatSelfHosting_DeleteCrumb(t *testing.T) {
	t.Run("delete crumb by ID succeeds", func(t *testing.T) {
		dataDir := initCupboard(t)
		id := createCrumb(t, dataDir, "To delete", "draft")

		stdout, stderr, code := runCupboard(t, dataDir, "delete", "crumbs", id)
		require.Equal(t, 0, code, "delete failed: %s", stderr)
		assert.Contains(t, stdout, "Deleted")

		// Verify it's gone.
		_, _, code = runCupboard(t, dataDir, "get", "crumbs", id)
		assert.Equal(t, 1, code)
	})

	t.Run("delete nonexistent ID returns exit 1", func(t *testing.T) {
		dataDir := initCupboard(t)

		_, _, code := runCupboard(t, dataDir, "delete", "crumbs", "nonexistent-id")
		assert.Equal(t, 1, code)
	})
}

// --- S8: Do-work cycle ---

func TestFlatSelfHosting_DoWorkCycle(t *testing.T) {
	dataDir := initCupboard(t)

	// Step 1: Create a task in draft state.
	id := createCrumb(t, dataDir, "Build widget", "draft")

	// Step 2: List draft tasks — should find our task.
	stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs", "State=draft")
	require.Equal(t, 0, code, "list draft failed: %s", stderr)
	draftArr := parseJSONArray(t, stdout)
	require.Len(t, draftArr, 1, "should have exactly one draft crumb")
	assert.Contains(t, stdout, "Build widget")

	// Step 3: Claim the task — set state to taken.
	claimPayload := fmt.Sprintf(`{"crumb_id":"%s","name":"Build widget","state":"taken"}`, id)
	_, stderr, code = runCupboard(t, dataDir, "set", "crumbs", id, claimPayload)
	require.Equal(t, 0, code, "claim (taken) failed: %s", stderr)

	// Verify draft list is now empty.
	stdout, _, code = runCupboard(t, dataDir, "list", "crumbs", "State=draft")
	require.Equal(t, 0, code)
	draftArr = parseJSONArray(t, stdout)
	assert.Len(t, draftArr, 0, "draft list should be empty after claiming")

	// Step 4: Close the task — set state to pebble.
	closePayload := fmt.Sprintf(`{"crumb_id":"%s","name":"Build widget","state":"pebble"}`, id)
	_, stderr, code = runCupboard(t, dataDir, "set", "crumbs", id, closePayload)
	require.Equal(t, 0, code, "close (pebble) failed: %s", stderr)

	// Verify the crumb is in pebble state.
	stdout, _, code = runCupboard(t, dataDir, "get", "crumbs", id)
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, `"pebble"`)
}

// --- S9: Make-work cycle ---

func TestFlatSelfHosting_MakeWorkCycle(t *testing.T) {
	dataDir := initCupboard(t)

	// Step 1: Create one existing task.
	createCrumb(t, dataDir, "Existing task", "draft")

	// Step 2: List existing — should have one.
	stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs")
	require.Equal(t, 0, code, "list failed: %s", stderr)
	arr := parseJSONArray(t, stdout)
	require.Len(t, arr, 1)

	// Step 3: Create multiple new crumbs.
	createCrumb(t, dataDir, "New task 1", "draft")
	createCrumb(t, dataDir, "New task 2", "draft")
	createCrumb(t, dataDir, "New task 3", "draft")

	// Step 4: Verify total count.
	stdout, _, code = runCupboard(t, dataDir, "list", "crumbs")
	require.Equal(t, 0, code)
	arr = parseJSONArray(t, stdout)
	assert.Len(t, arr, 4, "should have 4 crumbs after make-work cycle")
}

// --- S8+S6: Full workflow draft → taken → pebble ---

func TestFlatSelfHosting_FullWorkflow(t *testing.T) {
	dataDir := initCupboard(t)

	// Create task.
	stdout, stderr, code := runCupboard(t, dataDir, "set", "crumbs", "", `{"name":"Full workflow task","state":"draft"}`)
	require.Equal(t, 0, code, "create failed: %s", stderr)
	id := extractJSONField(t, stdout, "crumb_id")

	// Verify draft state via list filter.
	stdout, _, code = runCupboard(t, dataDir, "list", "crumbs", "State=draft")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "Full workflow task")

	// Transition to taken.
	payload := fmt.Sprintf(`{"crumb_id":"%s","name":"Full workflow task","state":"taken"}`, id)
	_, stderr, code = runCupboard(t, dataDir, "set", "crumbs", id, payload)
	require.Equal(t, 0, code, "taken transition failed: %s", stderr)

	// Verify taken state via list filter.
	stdout, _, code = runCupboard(t, dataDir, "list", "crumbs", "State=taken")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "Full workflow task")

	// Transition to pebble.
	payload = fmt.Sprintf(`{"crumb_id":"%s","name":"Full workflow task","state":"pebble"}`, id)
	_, stderr, code = runCupboard(t, dataDir, "set", "crumbs", id, payload)
	require.Equal(t, 0, code, "pebble transition failed: %s", stderr)

	// Verify pebble state via list filter.
	stdout, _, code = runCupboard(t, dataDir, "list", "crumbs", "State=pebble")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "Full workflow task")
}

// --- S10: JSONL persistence survives database deletion ---

func TestFlatSelfHosting_JSONLPersistence(t *testing.T) {
	t.Run("JSONL file contains persisted crumbs", func(t *testing.T) {
		dataDir := initCupboard(t)
		createCrumb(t, dataDir, "Crumb 1", "draft")
		createCrumb(t, dataDir, "Crumb 2", "draft")
		createCrumb(t, dataDir, "Crumb 3", "draft")

		// Verify the JSONL file exists and has content.
		jsonlPath := filepath.Join(dataDir, "crumbs.jsonl")
		data, err := os.ReadFile(jsonlPath)
		require.NoError(t, err, "crumbs.jsonl should be readable")
		assert.NotEmpty(t, data, "crumbs.jsonl should have content")
	})

	t.Run("data survives database deletion", func(t *testing.T) {
		dataDir := initCupboard(t)
		createCrumb(t, dataDir, "Persistent crumb", "draft")

		// Verify the crumb exists.
		stdout, _, code := runCupboard(t, dataDir, "list", "crumbs")
		require.Equal(t, 0, code)
		arr := parseJSONArray(t, stdout)
		require.Len(t, arr, 1)

		// Delete the SQLite database file.
		dbPath := filepath.Join(dataDir, "cupboard.db")
		err := os.Remove(dbPath)
		require.NoError(t, err, "should be able to remove cupboard.db")

		// List crumbs again — backend reinitializes from JSONL.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs")
		require.Equal(t, 0, code, "list after db delete failed: %s", stderr)
		arr = parseJSONArray(t, stdout)
		assert.Len(t, arr, 1, "crumb should survive database deletion")
		assert.Contains(t, stdout, "Persistent crumb")
	})

	t.Run("multiple sessions maintain data", func(t *testing.T) {
		dataDir := initCupboard(t)
		createCrumb(t, dataDir, "Session 1 task", "draft")

		// Simulate a new session by running a fresh list command.
		// Each CLI invocation is a fresh attach/detach cycle.
		stdout, stderr, code := runCupboard(t, dataDir, "list", "crumbs")
		require.Equal(t, 0, code, "list in new session failed: %s", stderr)
		assert.Contains(t, stdout, "Session 1 task")
	})
}
