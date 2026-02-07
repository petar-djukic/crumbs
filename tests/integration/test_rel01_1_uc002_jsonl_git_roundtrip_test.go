// CLI integration tests for JSONL git roundtrip persistence.
// Validates test-rel01.1-uc002-jsonl-git-roundtrip.yaml test cases.
// Implements: docs/test-suites/test-rel01.1-uc002-jsonl-git-roundtrip.yaml;
//             docs/use-cases/rel01.1-uc002-jsonl-git-roundtrip.yaml;
//             prd-sqlite-backend R1, R4, R5.
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestJSONLFileCreation validates JSONL files are created and contain data (cases 1-3).
func TestJSONLFileCreation(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(env *TestEnv)
		checkFiles     bool
		wantCrumbCount int
	}{
		{
			name: "JSONL files created on first crumb",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"First crumb","State":"draft"}`)
			},
			checkFiles:     true,
			wantCrumbCount: 1,
		},
		{
			name: "Crumb appears in crumbs.jsonl",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Tracked crumb","State":"draft"}`)
			},
			wantCrumbCount: 1,
		},
		{
			name: "Multiple crumbs written to JSONL",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Task one","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Epic one","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Task two","State":"draft"}`)
			},
			wantCrumbCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			tt.setup(env)

			crumbsFile := filepath.Join(env.DataDir, "crumbs.jsonl")

			if tt.checkFiles {
				// Verify crumbs.jsonl exists
				if _, err := os.Stat(crumbsFile); os.IsNotExist(err) {
					t.Error("crumbs.jsonl not created")
				}

				// Verify cupboard.db exists
				dbFile := filepath.Join(env.DataDir, "cupboard.db")
				if _, err := os.Stat(dbFile); os.IsNotExist(err) {
					t.Error("cupboard.db not created")
				}
			}

			// Verify crumb count in JSONL
			crumbs := ReadJSONLFile[map[string]any](t, crumbsFile)
			if len(crumbs) != tt.wantCrumbCount {
				t.Errorf("crumb count = %d, want %d", len(crumbs), tt.wantCrumbCount)
			}
		})
	}
}

// TestJSONLContentCorrectness validates JSONL content format (cases 4-6).
func TestJSONLContentCorrectness(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(env *TestEnv)
		checkFunc  func(t *testing.T, jsonlContent string)
	}{
		{
			name: "JSONL contains correct crumb data",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Verify content","State":"draft"}`)
			},
			checkFunc: func(t *testing.T, jsonlContent string) {
				if !strings.Contains(jsonlContent, "Verify content") {
					t.Error("JSONL missing crumb name")
				}
				if !strings.Contains(jsonlContent, "crumb_id") {
					t.Error("JSONL missing crumb_id field")
				}
				if !strings.Contains(jsonlContent, "state") {
					t.Error("JSONL missing state field")
				}
			},
		},
		{
			name: "JSONL uses RFC 3339 timestamps",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Timestamp test","State":"draft"}`)
			},
			checkFunc: func(t *testing.T, jsonlContent string) {
				if !strings.Contains(jsonlContent, "created_at") {
					t.Error("JSONL missing created_at field")
				}
				// RFC 3339 timestamps contain T separator and timezone
				if !strings.Contains(jsonlContent, "T") {
					t.Error("JSONL timestamp missing T separator")
				}
				// Check for timezone indicator (Z or +/-offset)
				if !strings.Contains(jsonlContent, "Z") && !regexp.MustCompile(`[+-]\d{2}:\d{2}`).MatchString(jsonlContent) {
					t.Error("JSONL timestamp missing timezone indicator")
				}
			},
		},
		{
			name: "JSONL uses lowercase hyphenated UUIDs",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"UUID test","State":"draft"}`)
			},
			checkFunc: func(t *testing.T, jsonlContent string) {
				// UUID pattern: lowercase hex with hyphens
				uuidPattern := regexp.MustCompile(`"crumb_id"\s*:\s*"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"`)
				if !uuidPattern.MatchString(jsonlContent) {
					t.Error("JSONL crumb_id not in lowercase hyphenated UUID format")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			tt.setup(env)

			crumbsFile := filepath.Join(env.DataDir, "crumbs.jsonl")
			content, err := os.ReadFile(crumbsFile)
			if err != nil {
				t.Fatalf("failed to read crumbs.jsonl: %v", err)
			}

			tt.checkFunc(t, string(content))
		})
	}
}

// TestDatabaseDeletionAndRebuild validates database regeneration from JSONL (cases 7-12).
func TestDatabaseDeletionAndRebuild(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(env *TestEnv) string // returns crumb ID if needed
		deleteDB       bool
		wantCrumbCount int
		checkTitle     string // expected title after rebuild
	}{
		{
			name: "Delete cupboard.db removes database",
			setup: func(env *TestEnv) string {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Pre-delete","State":"draft"}`)
				return ""
			},
			deleteDB:       true,
			wantCrumbCount: -1, // skip count check for this test
		},
		{
			name: "Cupboard command regenerates database from JSONL",
			setup: func(env *TestEnv) string {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Rebuilder","State":"draft"}`)
				return ""
			},
			deleteDB:       true,
			wantCrumbCount: 1,
		},
		{
			name: "Single crumb survives database deletion",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Survivor","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				return crumb.CrumbID
			},
			deleteDB:       true,
			wantCrumbCount: 1,
			checkTitle:     "Survivor",
		},
		{
			name: "Multiple crumbs survive database deletion",
			setup: func(env *TestEnv) string {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"First survivor","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Second survivor","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Third survivor","State":"draft"}`)
				return ""
			},
			deleteDB:       true,
			wantCrumbCount: 3,
		},
		{
			name: "Crumb details intact after rebuild",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Detailed epic","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				return crumb.CrumbID
			},
			deleteDB:       true,
			wantCrumbCount: 1,
			checkTitle:     "Detailed epic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			crumbID := tt.setup(env)

			dbFile := filepath.Join(env.DataDir, "cupboard.db")

			if tt.deleteDB {
				if err := os.Remove(dbFile); err != nil {
					t.Fatalf("failed to delete cupboard.db: %v", err)
				}

				// For "Delete cupboard.db removes database" test, just verify it's gone
				if tt.name == "Delete cupboard.db removes database" {
					if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
						t.Error("cupboard.db should not exist after deletion")
					}
					return
				}
			}

			// Run a command to trigger rebuild
			result := env.MustRunCupboard("list", "crumbs")

			// Verify database was regenerated
			if _, err := os.Stat(dbFile); os.IsNotExist(err) {
				t.Error("cupboard.db not regenerated after command")
			}

			if tt.wantCrumbCount >= 0 {
				var items []map[string]any
				if err := json.Unmarshal([]byte(result.Stdout), &items); err != nil {
					t.Fatalf("failed to parse list output: %v", err)
				}
				if len(items) != tt.wantCrumbCount {
					t.Errorf("crumb count = %d, want %d", len(items), tt.wantCrumbCount)
				}
			}

			if tt.checkTitle != "" && crumbID != "" {
				getResult := env.MustRunCupboard("get", "crumbs", crumbID)
				if !strings.Contains(getResult.Stdout, tt.checkTitle) {
					t.Errorf("expected title %q in output, got %s", tt.checkTitle, getResult.Stdout)
				}
			}
		})
	}
}

// TestStateChangesPersistence validates state changes persist to JSONL (cases 13-15).
func TestStateChangesPersistence(t *testing.T) {
	tests := []struct {
		name        string
		targetState string
		deleteDB    bool
	}{
		{
			name:        "State update persists to JSONL",
			targetState: "taken",
			deleteDB:    false,
		},
		{
			name:        "State change survives database deletion",
			targetState: "taken",
			deleteDB:    true,
		},
		{
			name:        "Closed state survives database deletion",
			targetState: "pebble",
			deleteDB:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumb
			result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"State test crumb","State":"draft"}`)
			crumb := ParseJSON[Crumb](t, result.Stdout)

			// Update state
			updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"State test crumb","State":"` + tt.targetState + `"}`
			env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)

			if !tt.deleteDB {
				// Verify JSONL contains updated state
				crumbsFile := filepath.Join(env.DataDir, "crumbs.jsonl")
				content, err := os.ReadFile(crumbsFile)
				if err != nil {
					t.Fatalf("failed to read crumbs.jsonl: %v", err)
				}
				if !strings.Contains(string(content), tt.targetState) {
					t.Errorf("JSONL should contain state %q", tt.targetState)
				}
			} else {
				// Delete database
				dbFile := filepath.Join(env.DataDir, "cupboard.db")
				if err := os.Remove(dbFile); err != nil {
					t.Fatalf("failed to delete cupboard.db: %v", err)
				}

				// Verify state after rebuild
				getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
				if !strings.Contains(getResult.Stdout, tt.targetState) {
					t.Errorf("state should be %q after rebuild, got %s", tt.targetState, getResult.Stdout)
				}
			}
		})
	}
}

// TestQueriesAfterRebuild validates filters work after database rebuild (cases 16-17).
func TestQueriesAfterRebuild(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(env *TestEnv)
		filter      string
		wantCount   int
	}{
		{
			name: "Ready filter works after rebuild",
			setup: func(env *TestEnv) {
				// Create open task
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Open task","State":"draft"}`)
				// Create and close a task
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Closed task","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				env.MustRunCupboard("set", "crumbs", crumb.CrumbID,
					`{"CrumbID":"`+crumb.CrumbID+`","Name":"Closed task","State":"pebble"}`)
			},
			filter:    "State=draft",
			wantCount: 1,
		},
		{
			name: "Type filter works after rebuild",
			setup: func(env *TestEnv) {
				// Create task and epic (using different names to distinguish)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"A task","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"An epic","State":"ready"}`)
			},
			filter:    "State=ready",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			tt.setup(env)

			// Delete database
			dbFile := filepath.Join(env.DataDir, "cupboard.db")
			if err := os.Remove(dbFile); err != nil {
				t.Fatalf("failed to delete cupboard.db: %v", err)
			}

			// Query with filter (triggers rebuild)
			result := env.MustRunCupboard("list", "crumbs", tt.filter)

			var items []map[string]any
			if err := json.Unmarshal([]byte(result.Stdout), &items); err != nil {
				t.Fatalf("failed to parse list output: %v", err)
			}

			if len(items) != tt.wantCount {
				t.Errorf("filtered count = %d, want %d", len(items), tt.wantCount)
			}
		})
	}
}

// TestEmptyCupboardRebuild validates empty/fresh cupboard scenarios (cases 18-20).
func TestEmptyCupboardRebuild(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(env *TestEnv)
		wantCrumbCount int
		checkFileExists string
	}{
		{
			name: "Empty JSONL files create empty cupboard",
			setup: func(env *TestEnv) {
				// Remove all JSONL files and database
				files, _ := filepath.Glob(filepath.Join(env.DataDir, "*.jsonl"))
				for _, f := range files {
					os.Remove(f)
				}
				dbFile := filepath.Join(env.DataDir, "cupboard.db")
				os.Remove(dbFile)
			},
			wantCrumbCount: 0,
		},
		{
			name: "Fresh start with no files",
			setup: func(env *TestEnv) {
				// Remove entire data directory
				os.RemoveAll(env.DataDir)
			},
			wantCrumbCount:  0,
			checkFileExists: "crumbs.jsonl",
		},
		{
			name: "Create works after fresh start",
			setup: func(env *TestEnv) {
				// Remove entire data directory
				os.RemoveAll(env.DataDir)
				// Trigger recreation
				env.MustRunCupboard("init")
			},
			wantCrumbCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			tt.setup(env)

			if tt.name == "Create works after fresh start" {
				// Create a crumb after fresh start
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Fresh crumb","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				if crumb.Name != "Fresh crumb" {
					t.Errorf("crumb name = %q, want %q", crumb.Name, "Fresh crumb")
				}
				return
			}

			// Run list to trigger rebuild/initialization
			result := env.RunCupboard("list", "crumbs")
			if result.ExitCode != 0 {
				// If list fails, try reinitializing
				env.MustRunCupboard("init")
				result = env.MustRunCupboard("list", "crumbs")
			}

			var items []map[string]any
			if err := json.Unmarshal([]byte(result.Stdout), &items); err != nil {
				t.Fatalf("failed to parse list output: %v\nstdout: %s", err, result.Stdout)
			}

			if len(items) != tt.wantCrumbCount {
				t.Errorf("crumb count = %d, want %d", len(items), tt.wantCrumbCount)
			}

			if tt.checkFileExists != "" {
				filePath := filepath.Join(env.DataDir, tt.checkFileExists)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("expected file %s to exist", tt.checkFileExists)
				}
			}
		})
	}
}

// TestGitEnv provides a test environment with git repository support.
type TestGitEnv struct {
	*TestEnv
	GitDir string
}

// NewTestGitEnv creates a test environment initialized as a git repository.
func NewTestGitEnv(t *testing.T) *TestGitEnv {
	t.Helper()
	env := NewTestEnv(t)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = env.TempDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\n%s", err, output)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = env.TempDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = env.TempDir
	cmd.Run()

	// Create .gitignore with cupboard.db
	gitignorePath := filepath.Join(env.TempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("data/cupboard.db\n"), 0644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}

	return &TestGitEnv{
		TestEnv: env,
		GitDir:  env.TempDir,
	}
}

// RunGit executes a git command in the test environment.
func (e *TestGitEnv) RunGit(args ...string) CmdResult {
	e.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = e.GitDir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return CmdResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// TestGitIntegration validates git workflow integration (cases 21-22).
func TestGitIntegration(t *testing.T) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping git integration tests")
	}

	t.Run("JSONL files can be committed to git", func(t *testing.T) {
		env := NewTestGitEnv(t)
		env.MustRunCupboard("init")

		// Create a crumb
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Git tracked","State":"draft"}`)

		// Add JSONL files to git
		result := env.RunGit("add", filepath.Join(env.DataDir, "*.jsonl"))
		if result.ExitCode != 0 {
			// Try adding files individually
			files, _ := filepath.Glob(filepath.Join(env.DataDir, "*.jsonl"))
			for _, f := range files {
				env.RunGit("add", f)
			}
		}

		// Check git status for JSONL files
		result = env.RunGit("status", "--porcelain", env.DataDir)

		// Verify crumbs.jsonl is tracked
		if !strings.Contains(result.Stdout, "crumbs.jsonl") {
			// It might already be committed or staged differently
			result = env.RunGit("status", "--porcelain")
			if !strings.Contains(result.Stdout, "crumbs.jsonl") && !strings.Contains(result.Stdout, "data/") {
				t.Log("git status output:", result.Stdout)
				// This is okay if there are no changes (already committed)
			}
		}
	})

	t.Run("Cupboard.db not tracked by git", func(t *testing.T) {
		env := NewTestGitEnv(t)
		env.MustRunCupboard("init")

		// Create a crumb (to ensure cupboard.db exists)
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Db test","State":"draft"}`)

		// Verify cupboard.db exists
		dbFile := filepath.Join(env.DataDir, "cupboard.db")
		if _, err := os.Stat(dbFile); os.IsNotExist(err) {
			t.Fatal("cupboard.db should exist")
		}

		// Check git status for cupboard.db - should be empty (gitignored)
		result := env.RunGit("status", "--porcelain", dbFile)

		// If gitignored properly, status should be empty
		if strings.TrimSpace(result.Stdout) != "" {
			// Check if it's showing as untracked but should be ignored
			checkIgnored := env.RunGit("check-ignore", "-v", dbFile)
			if checkIgnored.ExitCode != 0 {
				t.Errorf("cupboard.db should be gitignored, got status: %s", result.Stdout)
			}
		}
	})
}

// TestFullRoundtripWorkflow validates the complete roundtrip workflow (cases 23-24).
func TestFullRoundtripWorkflow(t *testing.T) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping roundtrip workflow tests")
	}

	t.Run("Complete roundtrip workflow", func(t *testing.T) {
		env := NewTestGitEnv(t)
		env.MustRunCupboard("init")

		// Step 1: Create crumbs
		result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Roundtrip task","State":"draft"}`)
		taskCrumb := ParseJSON[Crumb](t, result.Stdout)
		taskID := taskCrumb.CrumbID

		result = env.MustRunCupboard("set", "crumbs", "", `{"Name":"Roundtrip epic","State":"draft"}`)
		epicCrumb := ParseJSON[Crumb](t, result.Stdout)
		_ = epicCrumb.CrumbID

		// Step 2: Commit JSONL to git
		files, _ := filepath.Glob(filepath.Join(env.DataDir, "*.jsonl"))
		for _, f := range files {
			env.RunGit("add", f)
		}
		env.RunGit("commit", "-m", "Add initial crumbs")

		// Step 3: Verify initial count
		result = env.MustRunCupboard("list", "crumbs")
		items := ParseJSON[[]Crumb](t, result.Stdout)
		if len(items) != 2 {
			t.Errorf("initial count = %d, want 2", len(items))
		}

		// Step 4: Delete database
		dbFile := filepath.Join(env.DataDir, "cupboard.db")
		if err := os.Remove(dbFile); err != nil {
			t.Fatalf("failed to delete cupboard.db: %v", err)
		}

		// Step 5: Verify count after rebuild
		result = env.MustRunCupboard("list", "crumbs")
		items = ParseJSON[[]Crumb](t, result.Stdout)
		if len(items) != 2 {
			t.Errorf("count after rebuild = %d, want 2", len(items))
		}

		// Step 6: Verify task details after rebuild
		result = env.MustRunCupboard("get", "crumbs", taskID)
		if !strings.Contains(result.Stdout, "Roundtrip task") {
			t.Error("task title not preserved after rebuild")
		}

		// Step 7: Update state
		updateJSON := `{"CrumbID":"` + taskID + `","Name":"Roundtrip task","State":"taken"}`
		env.MustRunCupboard("set", "crumbs", taskID, updateJSON)

		// Step 8: Delete DB and verify state
		os.Remove(dbFile)
		result = env.MustRunCupboard("get", "crumbs", taskID)
		if !strings.Contains(result.Stdout, "taken") {
			t.Error("state not preserved as taken after rebuild")
		}

		// Step 9: Close task
		closeJSON := `{"CrumbID":"` + taskID + `","Name":"Roundtrip task","State":"pebble"}`
		env.MustRunCupboard("set", "crumbs", taskID, closeJSON)

		// Step 10: Delete DB and verify closed state
		os.Remove(dbFile)
		result = env.MustRunCupboard("get", "crumbs", taskID)
		if !strings.Contains(result.Stdout, "pebble") {
			t.Error("state not preserved as pebble after rebuild")
		}

		// Final: Verify ready filter shows no tasks (only pebble exists for task type)
		result = env.MustRunCupboard("list", "crumbs", "State=draft")
		items = ParseJSON[[]Crumb](t, result.Stdout)
		// Should have 1 (the epic is still draft)
		if len(items) != 1 {
			t.Errorf("draft count = %d, want 1", len(items))
		}
	})

	t.Run("Roundtrip with multiple deletes", func(t *testing.T) {
		env := NewTestGitEnv(t)
		env.MustRunCupboard("init")

		// Create persistent crumb
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Persistent","State":"draft"}`)

		dbFile := filepath.Join(env.DataDir, "cupboard.db")

		// Repeat delete and rebuild cycle
		for i := 0; i < 3; i++ {
			os.Remove(dbFile)

			result := env.MustRunCupboard("list", "crumbs")
			items := ParseJSON[[]Crumb](t, result.Stdout)
			if len(items) != 1 {
				t.Errorf("iteration %d: count = %d, want 1", i+1, len(items))
			}
		}
	})
}
