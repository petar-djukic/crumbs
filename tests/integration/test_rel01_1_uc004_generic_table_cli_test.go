// CLI integration tests for generic table operations.
// Validates test-rel01.1-uc004-generic-table-cli.yaml test cases.
// Implements: docs/specs/test-suites/test-rel01.1-uc004-generic-table-cli.yaml;
//
//	docs/specs/use-cases/rel01.1-uc004-generic-table-cli.yaml;
//	prd009-cupboard-cli R3 (generic table commands);
//	prd001-cupboard-core R3 (Table interface).
package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGenericTableCli is the main test for generic table CLI operations.
// It validates get, set, list, and delete commands across multiple tables.
func TestGenericTableCli(t *testing.T) {
	if buildErr != nil {
		t.Fatalf("failed to build cupboard: %v", buildErr)
	}
	if cupboardBin == "" {
		t.Fatal("cupboard binary not built")
	}

	t.Run("GetCommand", testGetCommand)
	t.Run("SetCommand", testSetCommand)
	t.Run("ListCommand", testListCommand)
	t.Run("DeleteCommand", testDeleteCommand)
	t.Run("CrossTableOperations", testCrossTableOperations)
}

// testGetCommand validates the get command behavior.
func testGetCommand(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(env *TestEnv) string // returns entity ID if needed
		table        string
		getID        func(setupID string) string
		wantExitCode int
		wantStdout   string
		wantStderr   string
	}{
		{
			name: "Get crumb by ID returns JSON object",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Test crumb","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				return crumb.CrumbID
			},
			table:        "crumbs",
			getID:        func(id string) string { return id },
			wantExitCode: 0,
			wantStdout:   "Test crumb",
		},
		{
			name:         "Get nonexistent crumb returns exit 1",
			table:        "crumbs",
			getID:        func(_ string) string { return "nonexistent-id-12345" },
			wantExitCode: 1,
			wantStderr:   `entity "nonexistent-id-12345" not found in table "crumbs"`,
		},
		{
			name:         "Get with unknown table returns exit 1",
			table:        "invalid-table",
			getID:        func(_ string) string { return "abc123" },
			wantExitCode: 1,
			wantStderr:   `unknown table "invalid-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			var setupID string
			if tt.setup != nil {
				setupID = tt.setup(env)
			}

			entityID := tt.getID(setupID)
			result := env.RunCupboard("get", tt.table, entityID)

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s",
					result.ExitCode, tt.wantExitCode, result.Stdout, result.Stderr)
			}

			if tt.wantStdout != "" && !strings.Contains(result.Stdout, tt.wantStdout) {
				t.Errorf("stdout should contain %q, got %q", tt.wantStdout, result.Stdout)
			}

			if tt.wantStderr != "" && !strings.Contains(result.Stderr, tt.wantStderr) {
				t.Errorf("stderr should contain %q, got %q", tt.wantStderr, result.Stderr)
			}
		})
	}
}

// testSetCommand validates the set command behavior.
func testSetCommand(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(env *TestEnv) string
		table        string
		id           func(setupID string) string
		json         func(setupID string) string
		wantExitCode int
		wantStdout   string
		wantStderr   string
		checkResult  func(t *testing.T, env *TestEnv, result CmdResult, setupID string)
	}{
		{
			name:         "Set crumb with empty ID creates new entity",
			table:        "crumbs",
			id:           func(_ string) string { return "" },
			json:         func(_ string) string { return `{"Name":"New task","State":"draft"}` },
			wantExitCode: 0,
			wantStdout:   "CrumbID",
			checkResult: func(t *testing.T, env *TestEnv, result CmdResult, _ string) {
				// Verify entity was created
				listResult := env.MustRunCupboard("list", "crumbs")
				var items []map[string]any
				if err := json.Unmarshal([]byte(listResult.Stdout), &items); err != nil {
					t.Fatalf("parse list output: %v", err)
				}
				if len(items) != 1 {
					t.Errorf("expected 1 crumb, got %d", len(items))
				}
				if items[0]["Name"] != "New task" {
					t.Errorf("expected Name='New task', got %v", items[0]["Name"])
				}
			},
		},
		{
			name: "Set crumb with existing ID updates entity",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Original","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				return crumb.CrumbID
			},
			table: "crumbs",
			id:    func(setupID string) string { return setupID },
			json: func(setupID string) string {
				return `{"CrumbID":"` + setupID + `","Name":"Updated","State":"ready"}`
			},
			wantExitCode: 0,
			checkResult: func(t *testing.T, env *TestEnv, result CmdResult, setupID string) {
				// Verify entity was updated
				getResult := env.MustRunCupboard("get", "crumbs", setupID)
				crumb := ParseJSON[Crumb](t, getResult.Stdout)
				if crumb.Name != "Updated" {
					t.Errorf("Name = %q, want 'Updated'", crumb.Name)
				}
				if crumb.State != "ready" {
					t.Errorf("State = %q, want 'ready'", crumb.State)
				}
			},
		},
		{
			name:         "Set trail creates new trail",
			table:        "trails",
			id:           func(_ string) string { return "" },
			json:         func(_ string) string { return `{"State":"active"}` },
			wantExitCode: 0,
			wantStdout:   "TrailID",
		},
		{
			name:         "Set with invalid JSON returns exit 1",
			table:        "crumbs",
			id:           func(_ string) string { return "" },
			json:         func(_ string) string { return `{not valid json}` },
			wantExitCode: 1,
			wantStderr:   "parse JSON",
		},
		{
			name:         "Set with unknown table returns exit 1",
			table:        "unknown-table",
			id:           func(_ string) string { return "" },
			json:         func(_ string) string { return `{"field":"value"}` },
			wantExitCode: 1,
			wantStderr:   `unknown table "unknown-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			var setupID string
			if tt.setup != nil {
				setupID = tt.setup(env)
			}

			id := tt.id(setupID)
			jsonData := tt.json(setupID)
			result := env.RunCupboard("set", tt.table, id, jsonData)

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s",
					result.ExitCode, tt.wantExitCode, result.Stdout, result.Stderr)
			}

			if tt.wantStdout != "" && !strings.Contains(result.Stdout, tt.wantStdout) {
				t.Errorf("stdout should contain %q, got %q", tt.wantStdout, result.Stdout)
			}

			if tt.wantStderr != "" && !strings.Contains(result.Stderr, tt.wantStderr) {
				t.Errorf("stderr should contain %q, got %q", tt.wantStderr, result.Stderr)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, env, result, setupID)
			}
		})
	}
}

// testListCommand validates the list command behavior.
func testListCommand(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(env *TestEnv)
		table        string
		filter       []string
		wantExitCode int
		wantCount    int
		wantStderr   string
		checkResult  func(t *testing.T, items []map[string]any)
	}{
		{
			name: "List crumbs returns JSON array",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Task A","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Task B","State":"ready"}`)
			},
			table:        "crumbs",
			wantExitCode: 0,
			wantCount:    2,
		},
		{
			name:         "List empty table returns empty JSON array",
			table:        "crumbs",
			wantExitCode: 0,
			wantCount:    0,
		},
		{
			name: "List with filter returns matching entities",
			setup: func(env *TestEnv) {
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Draft task","State":"draft"}`)
				env.MustRunCupboard("set", "crumbs", "", `{"Name":"Ready task","State":"ready"}`)
			},
			table:        "crumbs",
			filter:       []string{"states=draft"},
			wantExitCode: 0,
			wantCount:    1,
			checkResult: func(t *testing.T, items []map[string]any) {
				if items[0]["Name"] != "Draft task" {
					t.Errorf("expected Name='Draft task', got %v", items[0]["Name"])
				}
			},
		},
		{
			name:         "List with invalid filter returns exit 1",
			table:        "crumbs",
			filter:       []string{"malformed-filter"},
			wantExitCode: 1,
			wantStderr:   `invalid filter "malformed-filter" (expected key=value)`,
		},
		{
			name:         "List with unknown table returns exit 1",
			table:        "fake-table",
			wantExitCode: 1,
			wantStderr:   `unknown table "fake-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			if tt.setup != nil {
				tt.setup(env)
			}

			args := []string{"list", tt.table}
			args = append(args, tt.filter...)
			result := env.RunCupboard(args...)

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s",
					result.ExitCode, tt.wantExitCode, result.Stdout, result.Stderr)
			}

			if tt.wantStderr != "" && !strings.Contains(result.Stderr, tt.wantStderr) {
				t.Errorf("stderr should contain %q, got %q", tt.wantStderr, result.Stderr)
			}

			if tt.wantExitCode == 0 {
				var items []map[string]any
				if err := json.Unmarshal([]byte(result.Stdout), &items); err != nil {
					t.Fatalf("parse list output: %v\nstdout: %s", err, result.Stdout)
				}

				if len(items) != tt.wantCount {
					t.Errorf("item count = %d, want %d", len(items), tt.wantCount)
				}

				if tt.checkResult != nil && len(items) > 0 {
					tt.checkResult(t, items)
				}
			}
		})
	}
}

// testDeleteCommand validates the delete command behavior.
func testDeleteCommand(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(env *TestEnv) string
		table        string
		getID        func(setupID string) string
		wantExitCode int
		wantStdout   string
		wantStderr   string
		checkResult  func(t *testing.T, env *TestEnv, setupID string)
	}{
		{
			name: "Delete crumb by ID succeeds",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"To delete","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				return crumb.CrumbID
			},
			table:        "crumbs",
			getID:        func(id string) string { return id },
			wantExitCode: 0,
			wantStdout:   "Deleted crumbs/",
			checkResult: func(t *testing.T, env *TestEnv, _ string) {
				// Verify entity was deleted
				listResult := env.MustRunCupboard("list", "crumbs")
				var items []map[string]any
				if err := json.Unmarshal([]byte(listResult.Stdout), &items); err != nil {
					t.Fatalf("parse list output: %v", err)
				}
				if len(items) != 0 {
					t.Errorf("expected 0 crumbs after delete, got %d", len(items))
				}
			},
		},
		{
			name:         "Delete nonexistent crumb returns exit 1",
			table:        "crumbs",
			getID:        func(_ string) string { return "nonexistent-id-xyz" },
			wantExitCode: 1,
			wantStderr:   `entity "nonexistent-id-xyz" not found in table "crumbs"`,
		},
		{
			name:         "Delete with unknown table returns exit 1",
			table:        "bad-table",
			getID:        func(_ string) string { return "some-id" },
			wantExitCode: 1,
			wantStderr:   `unknown table "bad-table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			var setupID string
			if tt.setup != nil {
				setupID = tt.setup(env)
			}

			entityID := tt.getID(setupID)
			result := env.RunCupboard("delete", tt.table, entityID)

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s",
					result.ExitCode, tt.wantExitCode, result.Stdout, result.Stderr)
			}

			if tt.wantStdout != "" && !strings.Contains(result.Stdout, tt.wantStdout) {
				t.Errorf("stdout should contain %q, got %q", tt.wantStdout, result.Stdout)
			}

			if tt.wantStderr != "" && !strings.Contains(result.Stderr, tt.wantStderr) {
				t.Errorf("stderr should contain %q, got %q", tt.wantStderr, result.Stderr)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, env, setupID)
			}
		})
	}
}

// testCrossTableOperations validates that generic commands work across different tables.
func testCrossTableOperations(t *testing.T) {
	t.Run("Create and list trails", func(t *testing.T) {
		env := NewTestEnv(t)
		env.MustRunCupboard("init")

		// Create a trail
		result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
		if !strings.Contains(result.Stdout, "TrailID") {
			t.Error("expected TrailID in output")
		}

		// Parse the trail ID
		trail := ParseJSON[Trail](t, result.Stdout)

		// Get the trail
		getResult := env.MustRunCupboard("get", "trails", trail.TrailID)
		if !strings.Contains(getResult.Stdout, "active") {
			t.Error("expected State=active in get output")
		}

		// List trails
		listResult := env.MustRunCupboard("list", "trails")
		var items []map[string]any
		if err := json.Unmarshal([]byte(listResult.Stdout), &items); err != nil {
			t.Fatalf("parse list output: %v", err)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 trail, got %d", len(items))
		}

		// Delete the trail
		deleteResult := env.MustRunCupboard("delete", "trails", trail.TrailID)
		if !strings.Contains(deleteResult.Stdout, "Deleted trails/") {
			t.Error("expected 'Deleted trails/' in output")
		}
	})

	t.Run("Create and list links", func(t *testing.T) {
		env := NewTestEnv(t)
		env.MustRunCupboard("init")

		// Create a crumb and a trail
		crumbResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Test crumb","State":"draft"}`)
		crumb := ParseJSON[Crumb](t, crumbResult.Stdout)

		trailResult := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
		trail := ParseJSON[Trail](t, trailResult.Stdout)

		// Create a link between crumb and trail
		linkJSON := `{"LinkType":"belongs_to","FromID":"` + crumb.CrumbID + `","ToID":"` + trail.TrailID + `"}`
		linkResult := env.MustRunCupboard("set", "links", "", linkJSON)
		if !strings.Contains(linkResult.Stdout, "LinkID") {
			t.Error("expected LinkID in output")
		}

		// Parse the link
		link := ParseJSON[Link](t, linkResult.Stdout)

		// Get the link
		getResult := env.MustRunCupboard("get", "links", link.LinkID)
		if !strings.Contains(getResult.Stdout, "belongs_to") {
			t.Error("expected LinkType=belongs_to in get output")
		}

		// List links
		listResult := env.MustRunCupboard("list", "links")
		var items []map[string]any
		if err := json.Unmarshal([]byte(listResult.Stdout), &items); err != nil {
			t.Fatalf("parse list output: %v", err)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 link, got %d", len(items))
		}
	})

	t.Run("Filter across multiple crumbs", func(t *testing.T) {
		env := NewTestEnv(t)
		env.MustRunCupboard("init")

		// Create multiple crumbs with different states
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Draft 1","State":"draft"}`)
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Draft 2","State":"draft"}`)
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Ready 1","State":"ready"}`)
		env.MustRunCupboard("set", "crumbs", "", `{"Name":"Taken 1","State":"taken"}`)

		// Filter by draft state
		draftResult := env.MustRunCupboard("list", "crumbs", "states=draft")
		var draftItems []map[string]any
		if err := json.Unmarshal([]byte(draftResult.Stdout), &draftItems); err != nil {
			t.Fatalf("parse list output: %v", err)
		}
		if len(draftItems) != 2 {
			t.Errorf("expected 2 draft crumbs, got %d", len(draftItems))
		}

		// Filter by ready state
		readyResult := env.MustRunCupboard("list", "crumbs", "states=ready")
		var readyItems []map[string]any
		if err := json.Unmarshal([]byte(readyResult.Stdout), &readyItems); err != nil {
			t.Fatalf("parse list output: %v", err)
		}
		if len(readyItems) != 1 {
			t.Errorf("expected 1 ready crumb, got %d", len(readyItems))
		}

		// List all
		allResult := env.MustRunCupboard("list", "crumbs")
		var allItems []map[string]any
		if err := json.Unmarshal([]byte(allResult.Stdout), &allItems); err != nil {
			t.Fatalf("parse list output: %v", err)
		}
		if len(allItems) != 4 {
			t.Errorf("expected 4 total crumbs, got %d", len(allItems))
		}
	})
}
