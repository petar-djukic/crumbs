// CLI integration tests for Table interface CRUD operations.
// Validates test-rel01.0-uc002-table-crud.yaml test cases.
// Implements: docs/specs/test-suites/test-rel01.0-uc002-table-crud.yaml;
//
//	docs/specs/use-cases/rel01.0-uc002-table-crud.yaml.
package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// isUUIDv7 validates that the given string matches UUID v7 format.
// UUID v7 has version nibble 7 in the 13th character position.
func isUUIDv7(id string) bool {
	// UUID format: xxxxxxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(id))
}

// --- S1: Set with empty ID returns UUID v7 and persists to JSONL ---

// TestUC002_S1_SetEmptyIDGeneratesUUIDv7 validates that Set with empty ID generates
// a UUID v7 identifier and persists the entity to JSONL.
func TestUC002_S1_SetEmptyIDGeneratesUUIDv7(t *testing.T) {
	tests := []struct {
		name        string
		crumbJSON   string
		wantName    string
		wantState   string
		checkJSONL  bool
		jsonlString string
	}{
		{
			name:        "create crumb with empty ID generates UUID v7",
			crumbJSON:   `{"Name":"Test crumb","State":"draft"}`,
			wantName:    "Test crumb",
			wantState:   "draft",
			checkJSONL:  false,
			jsonlString: "",
		},
		{
			name:        "created crumb persists to JSONL file",
			crumbJSON:   `{"Name":"Persisted crumb","State":"draft"}`,
			wantName:    "Persisted crumb",
			wantState:   "draft",
			checkJSONL:  true,
			jsonlString: `"name":"Persisted crumb"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			result := env.MustRunCupboard("set", "crumbs", "", tt.crumbJSON)
			crumb := ParseJSON[Crumb](t, result.Stdout)

			if crumb.CrumbID == "" {
				t.Error("crumb ID not generated")
			}
			if !isUUIDv7(crumb.CrumbID) {
				t.Errorf("crumb ID %q is not a valid UUID v7", crumb.CrumbID)
			}
			if crumb.Name != tt.wantName {
				t.Errorf("crumb name = %q, want %q", crumb.Name, tt.wantName)
			}
			if crumb.State != tt.wantState {
				t.Errorf("crumb state = %q, want %q", crumb.State, tt.wantState)
			}

			if tt.checkJSONL {
				jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
				data, err := os.ReadFile(jsonlPath)
				if err != nil {
					t.Fatalf("failed to read JSONL file: %v", err)
				}
				if !strings.Contains(string(data), tt.jsonlString) {
					t.Errorf("JSONL does not contain %q", tt.jsonlString)
				}
			}
		})
	}
}

// TestUC002_S1_TwoCreatesGenerateUniqueUUIDs validates that each Set with empty ID
// generates a unique UUID v7.
func TestUC002_S1_TwoCreatesGenerateUniqueUUIDs(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"First crumb","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)

	result2 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Second crumb","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, result2.Stdout)

	if crumb1.CrumbID == crumb2.CrumbID {
		t.Error("two creates should generate unique UUIDs")
	}
	if !isUUIDv7(crumb1.CrumbID) {
		t.Errorf("first crumb ID %q is not a valid UUID v7", crumb1.CrumbID)
	}
	if !isUUIDv7(crumb2.CrumbID) {
		t.Errorf("second crumb ID %q is not a valid UUID v7", crumb2.CrumbID)
	}

	// Verify count via list
	listResult := env.MustRunCupboard("list", "crumbs")
	crumbs := ParseJSON[[]Crumb](t, listResult.Stdout)
	if len(crumbs) != 2 {
		t.Errorf("crumb count = %d, want 2", len(crumbs))
	}
}

// --- S2: Get(id) returns entity with all fields matching ---

// TestUC002_S2_GetRetrievesEntityWithMatchingFields validates that Get returns
// the crumb with all fields matching what was created.
func TestUC002_S2_GetRetrievesEntityWithMatchingFields(t *testing.T) {
	tests := []struct {
		name      string
		crumbJSON string
		wantName  string
		wantState string
	}{
		{
			name:      "get retrieves entity with matching fields",
			crumbJSON: `{"Name":"Retrieve me","State":"ready"}`,
			wantName:  "Retrieve me",
			wantState: "ready",
		},
		{
			name:      "round-trip fidelity for crumb fields",
			crumbJSON: `{"Name":"Fidelity test","State":"pending"}`,
			wantName:  "Fidelity test",
			wantState: "pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumb
			createResult := env.MustRunCupboard("set", "crumbs", "", tt.crumbJSON)
			created := ParseJSON[Crumb](t, createResult.Stdout)

			// Get crumb by ID
			getResult := env.MustRunCupboard("get", "crumbs", created.CrumbID)
			retrieved := ParseJSON[Crumb](t, getResult.Stdout)

			// Verify fields match
			if retrieved.CrumbID != created.CrumbID {
				t.Errorf("CrumbID = %q, want %q", retrieved.CrumbID, created.CrumbID)
			}
			if retrieved.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", retrieved.Name, tt.wantName)
			}
			if retrieved.State != tt.wantState {
				t.Errorf("State = %q, want %q", retrieved.State, tt.wantState)
			}
			if retrieved.CreatedAt == "" {
				t.Error("CreatedAt not set")
			}
			if retrieved.UpdatedAt == "" {
				t.Error("UpdatedAt not set")
			}
		})
	}
}

// --- S3: Set(id, entity) updates entity; Get confirms change ---

// TestUC002_S3_UpdateEntityViaSet validates that Set with existing ID updates
// the entity and Get confirms the change.
func TestUC002_S3_UpdateEntityViaSet(t *testing.T) {
	tests := []struct {
		name          string
		initialJSON   string
		initialName   string
		updatedName   string
		updatedState  string
		checkJSONL    bool
		jsonlContains []string
	}{
		{
			name:         "update entity via Set with existing ID",
			initialJSON:  `{"Name":"Original name","State":"draft"}`,
			initialName:  "Original name",
			updatedName:  "Updated name",
			updatedState: "draft",
			checkJSONL:   false,
		},
		{
			name:          "update persists to JSONL",
			initialJSON:   `{"Name":"JSONL update test","State":"draft"}`,
			initialName:   "JSONL update test",
			updatedName:   "JSONL updated",
			updatedState:  "taken",
			checkJSONL:    true,
			jsonlContains: []string{`"name":"JSONL updated"`, `"state":"taken"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumb
			createResult := env.MustRunCupboard("set", "crumbs", "", tt.initialJSON)
			created := ParseJSON[Crumb](t, createResult.Stdout)

			// Update crumb
			updateJSON := `{"CrumbID":"` + created.CrumbID + `","Name":"` + tt.updatedName + `","State":"` + tt.updatedState + `"}`
			env.MustRunCupboard("set", "crumbs", created.CrumbID, updateJSON)

			// Get and verify
			getResult := env.MustRunCupboard("get", "crumbs", created.CrumbID)
			retrieved := ParseJSON[Crumb](t, getResult.Stdout)

			if retrieved.Name != tt.updatedName {
				t.Errorf("Name = %q, want %q", retrieved.Name, tt.updatedName)
			}
			if retrieved.State != tt.updatedState {
				t.Errorf("State = %q, want %q", retrieved.State, tt.updatedState)
			}

			if tt.checkJSONL {
				jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
				data, err := os.ReadFile(jsonlPath)
				if err != nil {
					t.Fatalf("failed to read JSONL file: %v", err)
				}
				for _, s := range tt.jsonlContains {
					if !strings.Contains(string(data), s) {
						t.Errorf("JSONL does not contain %q", s)
					}
				}
			}
		})
	}
}

// TestUC002_S3_UpdatedEntityConfirmedViaGet validates that after update,
// Get returns the modified field values.
func TestUC002_S3_UpdatedEntityConfirmedViaGet(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Before update","State":"draft"}`)
	created := ParseJSON[Crumb](t, createResult.Stdout)

	// Update crumb
	updateJSON := `{"CrumbID":"` + created.CrumbID + `","Name":"After update","State":"ready"}`
	env.MustRunCupboard("set", "crumbs", created.CrumbID, updateJSON)

	// Verify via Get
	getResult := env.MustRunCupboard("get", "crumbs", created.CrumbID)
	retrieved := ParseJSON[Crumb](t, getResult.Stdout)

	if retrieved.Name != "After update" {
		t.Errorf("Name = %q, want After update", retrieved.Name)
	}
	if retrieved.State != "ready" {
		t.Errorf("State = %q, want ready", retrieved.State)
	}
}

// TestUC002_S3_UpdatedAtChangesOnUpdate validates that UpdatedAt timestamp changes on update.
func TestUC002_S3_UpdatedAtChangesOnUpdate(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Timestamp test","State":"draft"}`)
	created := ParseJSON[Crumb](t, createResult.Stdout)

	originalUpdatedAt := created.UpdatedAt

	// Wait to ensure timestamp difference
	time.Sleep(1100 * time.Millisecond)

	// Update crumb
	updateJSON := `{"CrumbID":"` + created.CrumbID + `","Name":"Timestamp updated","State":"draft"}`
	updateResult := env.MustRunCupboard("set", "crumbs", created.CrumbID, updateJSON)
	updated := ParseJSON[Crumb](t, updateResult.Stdout)

	// Parse and compare timestamps
	originalTime, err := time.Parse(time.RFC3339, originalUpdatedAt)
	if err != nil {
		t.Fatalf("failed to parse original UpdatedAt %q: %v", originalUpdatedAt, err)
	}
	updatedTime, err := time.Parse(time.RFC3339, updated.UpdatedAt)
	if err != nil {
		t.Fatalf("failed to parse updated UpdatedAt %q: %v", updated.UpdatedAt, err)
	}

	if !updatedTime.After(originalTime) && !updatedTime.Equal(originalTime) {
		t.Errorf("UpdatedAt should be >= original; got %v, original %v", updatedTime, originalTime)
	}
}

// --- S4: Fetch with empty filter returns all entities ---

// TestUC002_S4_FetchEmptyFilterReturnsAll validates that Fetch with empty filter
// returns all crumbs in the table.
func TestUC002_S4_FetchEmptyFilterReturnsAll(t *testing.T) {
	tests := []struct {
		name       string
		setupCount int
		wantCount  int
	}{
		{
			name:       "fetch with empty filter returns all crumbs",
			setupCount: 3,
			wantCount:  3,
		},
		{
			name:       "fetch empty table returns empty list",
			setupCount: 0,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			states := []string{"draft", "ready", "taken"}
			for i := 0; i < tt.setupCount; i++ {
				state := states[i%len(states)]
				crumbJSON := `{"Name":"Crumb ` + string(rune('A'+i)) + `","State":"` + state + `"}`
				env.MustRunCupboard("set", "crumbs", "", crumbJSON)
			}

			// Fetch all
			listResult := env.MustRunCupboard("list", "crumbs")
			crumbs := ParseJSON[[]Crumb](t, listResult.Stdout)

			if len(crumbs) != tt.wantCount {
				t.Errorf("crumb count = %d, want %d", len(crumbs), tt.wantCount)
			}
		})
	}
}

// --- S5: Fetch with filter returns only matching entities ---

// TestUC002_S5_FetchWithFilterReturnsMatching validates that Fetch with filter
// returns only matching entities.
func TestUC002_S5_FetchWithFilterReturnsMatching(t *testing.T) {
	tests := []struct {
		name        string
		setupStates []string
		filterState string
		wantCount   int
	}{
		{
			name:        "fetch with state filter returns matching crumbs",
			setupStates: []string{"draft", "ready", "ready", "taken"},
			filterState: "ready",
			wantCount:   2,
		},
		{
			name:        "fetch with filter returns no matches",
			setupStates: []string{"draft"},
			filterState: "pebble",
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			for i, state := range tt.setupStates {
				crumbJSON := `{"Name":"Crumb ` + string(rune('A'+i)) + `","State":"` + state + `"}`
				env.MustRunCupboard("set", "crumbs", "", crumbJSON)
			}

			// Fetch with filter
			listResult := env.MustRunCupboard("list", "crumbs", "states="+tt.filterState)
			crumbs := ParseJSON[[]Crumb](t, listResult.Stdout)

			if len(crumbs) != tt.wantCount {
				t.Errorf("crumb count = %d, want %d", len(crumbs), tt.wantCount)
			}

			// Verify all returned crumbs match the filter
			for _, c := range crumbs {
				if c.State != tt.filterState {
					t.Errorf("returned crumb has state %q, want %q", c.State, tt.filterState)
				}
			}
		})
	}
}

// --- S6: Delete(id) removes entity; subsequent Get returns error ---

// TestUC002_S6_DeleteRemovesEntity validates that Delete removes the entity
// from SQLite and subsequent Get fails.
func TestUC002_S6_DeleteRemovesEntity(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Delete me","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, createResult.Stdout)

	// Delete crumb
	deleteResult := env.RunCupboard("delete", "crumbs", crumb.CrumbID)
	if deleteResult.ExitCode != 0 {
		t.Fatalf("delete failed with exit code %d: %s", deleteResult.ExitCode, deleteResult.Stderr)
	}

	// Verify Get fails
	getResult := env.RunCupboard("get", "crumbs", crumb.CrumbID)
	if getResult.ExitCode == 0 {
		t.Error("get after delete should fail")
	}
}

// TestUC002_S6_DeleteRemovesFromJSONL validates that Delete removes the entity
// from JSONL file.
func TestUC002_S6_DeleteRemovesFromJSONL(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"JSONL delete test","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, createResult.Stdout)

	// Delete crumb
	env.MustRunCupboard("delete", "crumbs", crumb.CrumbID)

	// Verify JSONL does not contain the crumb
	jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}
	if strings.Contains(string(data), `"name":"JSONL delete test"`) {
		t.Error("JSONL should not contain deleted crumb")
	}
}

// TestUC002_S6_FetchAfterDeleteExcludesDeleted validates that Fetch no longer
// returns the deleted entity.
func TestUC002_S6_FetchAfterDeleteExcludesDeleted(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create two crumbs
	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Keep this","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)

	result2 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Delete this","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, result2.Stdout)

	// Delete second crumb
	env.MustRunCupboard("delete", "crumbs", crumb2.CrumbID)

	// Fetch all
	listResult := env.MustRunCupboard("list", "crumbs")
	crumbs := ParseJSON[[]Crumb](t, listResult.Stdout)

	if len(crumbs) != 1 {
		t.Errorf("crumb count = %d, want 1", len(crumbs))
	}
	if len(crumbs) > 0 && crumbs[0].CrumbID != crumb1.CrumbID {
		t.Errorf("remaining crumb ID = %q, want %q", crumbs[0].CrumbID, crumb1.CrumbID)
	}
}

// --- S7: Get and Delete on nonexistent IDs return errors ---

// TestUC002_S7_GetNonexistentReturnsError validates that Get with nonexistent ID
// returns an error.
func TestUC002_S7_GetNonexistentReturnsError(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		entityID  string
	}{
		{
			name:      "get nonexistent crumb returns error",
			tableName: "crumbs",
			entityID:  "nonexistent-uuid-12345",
		},
		{
			name:      "get nonexistent trail returns error",
			tableName: "trails",
			entityID:  "nonexistent-uuid-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			result := env.RunCupboard("get", tt.tableName, tt.entityID)
			if result.ExitCode == 0 {
				t.Error("get nonexistent entity should fail")
			}
			if !strings.Contains(strings.ToLower(result.Stderr), "not found") {
				t.Errorf("stderr should contain 'not found', got: %s", result.Stderr)
			}
		})
	}
}

// TestUC002_S7_DeleteNonexistentReturnsError validates that Delete with nonexistent ID
// returns an error.
func TestUC002_S7_DeleteNonexistentReturnsError(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		entityID  string
	}{
		{
			name:      "delete nonexistent crumb returns error",
			tableName: "crumbs",
			entityID:  "nonexistent-uuid-12345",
		},
		{
			name:      "delete nonexistent trail returns error",
			tableName: "trails",
			entityID:  "nonexistent-uuid-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			result := env.RunCupboard("delete", tt.tableName, tt.entityID)
			if result.ExitCode == 0 {
				t.Error("delete nonexistent entity should fail")
			}
			if !strings.Contains(strings.ToLower(result.Stderr), "not found") {
				t.Errorf("stderr should contain 'not found', got: %s", result.Stderr)
			}
		})
	}
}

// --- S8: Same operations work on crumbs and trails tables ---

// TestUC002_S8_TrailSetEmptyIDGeneratesUUIDv7 validates that Set on trails table
// with empty ID generates a UUID v7.
func TestUC002_S8_TrailSetEmptyIDGeneratesUUIDv7(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail := ParseJSON[Trail](t, result.Stdout)

	if trail.TrailID == "" {
		t.Error("trail ID not generated")
	}
	if !isUUIDv7(trail.TrailID) {
		t.Errorf("trail ID %q is not a valid UUID v7", trail.TrailID)
	}
	if trail.State != "active" {
		t.Errorf("trail state = %q, want active", trail.State)
	}

	// Verify count
	listResult := env.MustRunCupboard("list", "trails")
	trails := ParseJSON[[]Trail](t, listResult.Stdout)
	if len(trails) != 1 {
		t.Errorf("trail count = %d, want 1", len(trails))
	}
}

// TestUC002_S8_TrailGetReturnsMatchingFields validates that Get trail returns
// entity with matching fields.
func TestUC002_S8_TrailGetReturnsMatchingFields(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create trail
	createResult := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	created := ParseJSON[Trail](t, createResult.Stdout)

	// Get trail
	getResult := env.MustRunCupboard("get", "trails", created.TrailID)
	retrieved := ParseJSON[Trail](t, getResult.Stdout)

	if retrieved.TrailID != created.TrailID {
		t.Errorf("TrailID = %q, want %q", retrieved.TrailID, created.TrailID)
	}
	if retrieved.State != "active" {
		t.Errorf("State = %q, want active", retrieved.State)
	}
}

// TestUC002_S8_TrailUpdateViaSet validates that Set with existing ID updates the trail.
func TestUC002_S8_TrailUpdateViaSet(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create trail
	createResult := env.MustRunCupboard("set", "trails", "", `{"State":"draft"}`)
	created := ParseJSON[Trail](t, createResult.Stdout)

	// Update trail
	updateJSON := `{"TrailID":"` + created.TrailID + `","State":"active"}`
	env.MustRunCupboard("set", "trails", created.TrailID, updateJSON)

	// Verify via Get
	getResult := env.MustRunCupboard("get", "trails", created.TrailID)
	retrieved := ParseJSON[Trail](t, getResult.Stdout)

	if retrieved.State != "active" {
		t.Errorf("State = %q, want active", retrieved.State)
	}
}

// TestUC002_S8_TrailFetchEmptyFilter validates that Fetch trails with empty filter
// returns all trails.
func TestUC002_S8_TrailFetchEmptyFilter(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	env.MustRunCupboard("set", "trails", "", `{"State":"completed"}`)

	listResult := env.MustRunCupboard("list", "trails")
	trails := ParseJSON[[]Trail](t, listResult.Stdout)

	if len(trails) != 2 {
		t.Errorf("trail count = %d, want 2", len(trails))
	}
}

// TestUC002_S8_TrailFetchWithFilter validates that Fetch trails with filter
// returns matching trails.
func TestUC002_S8_TrailFetchWithFilter(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	env.MustRunCupboard("set", "trails", "", `{"State":"completed"}`)

	listResult := env.MustRunCupboard("list", "trails", "State=active")
	trails := ParseJSON[[]Trail](t, listResult.Stdout)

	if len(trails) != 2 {
		t.Errorf("trail count = %d, want 2", len(trails))
	}
	for _, tr := range trails {
		if tr.State != "active" {
			t.Errorf("trail state = %q, want active", tr.State)
		}
	}
}

// TestUC002_S8_TrailDeleteRemovesFromStorage validates that Delete trail removes
// from storage.
func TestUC002_S8_TrailDeleteRemovesFromStorage(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create trail
	createResult := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail := ParseJSON[Trail](t, createResult.Stdout)

	// Delete trail
	env.MustRunCupboard("delete", "trails", trail.TrailID)

	// Verify Get fails
	getResult := env.RunCupboard("get", "trails", trail.TrailID)
	if getResult.ExitCode == 0 {
		t.Error("get after delete should fail")
	}
}

// TestUC002_S8_TrailPersistsToJSONL validates that trails persist to JSONL file.
func TestUC002_S8_TrailPersistsToJSONL(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	env.MustRunCupboard("set", "trails", "", `{"State":"pending"}`)

	jsonlPath := filepath.Join(env.DataDir, "trails.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}
	if !strings.Contains(string(data), `"state":"pending"`) {
		t.Error("JSONL should contain trail state")
	}
}

// --- S9: JSONL file reflects create, update, delete operations ---

// TestUC002_S9_JSONLReflectsCreateOperation validates that JSONL reflects create operation.
func TestUC002_S9_JSONLReflectsCreateOperation(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Create JSONL test","State":"draft"}`)

	jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}
	if !strings.Contains(string(data), `"name":"Create JSONL test"`) {
		t.Error("JSONL should contain created crumb name")
	}
}

// TestUC002_S9_JSONLReflectsUpdateOperation validates that JSONL reflects update operation.
func TestUC002_S9_JSONLReflectsUpdateOperation(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Before JSONL update","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, createResult.Stdout)

	// Update crumb
	updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"After JSONL update","State":"ready"}`
	env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)

	// Verify JSONL
	jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}
	if !strings.Contains(string(data), `"name":"After JSONL update"`) {
		t.Error("JSONL should contain updated crumb name")
	}
	if !strings.Contains(string(data), `"state":"ready"`) {
		t.Error("JSONL should contain updated crumb state")
	}
	if strings.Contains(string(data), `"name":"Before JSONL update"`) {
		t.Error("JSONL should not contain old crumb name")
	}
}

// TestUC002_S9_JSONLReflectsDeleteOperation validates that JSONL reflects delete operation.
func TestUC002_S9_JSONLReflectsDeleteOperation(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create two crumbs
	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"To be deleted","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)

	env.MustRunCupboard("set", "crumbs", "", `{"Name":"To be kept","State":"draft"}`)

	// Delete first crumb
	env.MustRunCupboard("delete", "crumbs", crumb1.CrumbID)

	// Verify JSONL
	jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}
	if strings.Contains(string(data), `"name":"To be deleted"`) {
		t.Error("JSONL should not contain deleted crumb")
	}
	if !strings.Contains(string(data), `"name":"To be kept"`) {
		t.Error("JSONL should contain kept crumb")
	}
}

// TestUC002_S9_MultipleOperationsReflectedInJSONL validates that a sequence of
// create, update, delete operations are correctly reflected in JSONL.
func TestUC002_S9_MultipleOperationsReflectedInJSONL(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create two crumbs
	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Multi op 1","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)

	result2 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Multi op 2","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, result2.Stdout)

	// Update first crumb
	updateJSON := `{"CrumbID":"` + crumb1.CrumbID + `","Name":"Multi op 1 updated","State":"ready"}`
	env.MustRunCupboard("set", "crumbs", crumb1.CrumbID, updateJSON)

	// Delete second crumb
	env.MustRunCupboard("delete", "crumbs", crumb2.CrumbID)

	// Verify JSONL
	jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}
	if !strings.Contains(string(data), `"name":"Multi op 1 updated"`) {
		t.Error("JSONL should contain updated crumb name")
	}
	if !strings.Contains(string(data), `"state":"ready"`) {
		t.Error("JSONL should contain updated crumb state")
	}
	if strings.Contains(string(data), `"name":"Multi op 2"`) {
		t.Error("JSONL should not contain deleted crumb")
	}
}

// TestUC002_S9_JSONLLinesAreValidJSON validates that each line in JSONL file
// is valid JSON.
func TestUC002_S9_JSONLLinesAreValidJSON(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create multiple crumbs
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Valid JSON 1","State":"draft"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Valid JSON 2","State":"ready"}`)

	// Read and validate JSONL
	jsonlPath := filepath.Join(env.DataDir, "crumbs.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("failed to read JSONL file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
	}
}

// --- S10: Detach prevents further operations with ErrCupboardDetached ---
// NOTE: The detach command is not yet implemented in the CLI. These tests are
// skipped until the detach command is added. The test cases mirror the YAML spec
// (test-rel01.0-uc002-table-crud.yaml) and should be enabled when the command exists.

// TestUC002_S10_OperationsAfterDetachReturnError validates that Table operations
// after Detach return ErrCupboardDetached.
// SKIPPED: The 'detach' command is not yet implemented in the CLI.
func TestUC002_S10_OperationsAfterDetachReturnError(t *testing.T) {
	t.Skip("detach command not yet implemented in CLI")

	tests := []struct {
		name    string
		setup   func(env *TestEnv) string // Returns crumb ID if needed
		command func(env *TestEnv, crumbID string) CmdResult
	}{
		{
			name: "get after detach returns error",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Pre-detach crumb","State":"draft"}`)
				crumb := ParseJSON[Crumb](env.t, result.Stdout)
				env.MustRunCupboard("detach")
				return crumb.CrumbID
			},
			command: func(env *TestEnv, crumbID string) CmdResult {
				return env.RunCupboard("get", "crumbs", crumbID)
			},
		},
		{
			name: "set after detach returns error",
			setup: func(env *TestEnv) string {
				env.MustRunCupboard("detach")
				return ""
			},
			command: func(env *TestEnv, _ string) CmdResult {
				return env.RunCupboard("set", "crumbs", "", `{"Name":"After detach","State":"draft"}`)
			},
		},
		{
			name: "list after detach returns error",
			setup: func(env *TestEnv) string {
				env.MustRunCupboard("detach")
				return ""
			},
			command: func(env *TestEnv, _ string) CmdResult {
				return env.RunCupboard("list", "crumbs")
			},
		},
		{
			name: "delete after detach returns error",
			setup: func(env *TestEnv) string {
				result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Detach delete test","State":"draft"}`)
				crumb := ParseJSON[Crumb](env.t, result.Stdout)
				env.MustRunCupboard("detach")
				return crumb.CrumbID
			},
			command: func(env *TestEnv, crumbID string) CmdResult {
				return env.RunCupboard("delete", "crumbs", crumbID)
			},
		},
		{
			name: "getTable after detach returns error",
			setup: func(env *TestEnv) string {
				env.MustRunCupboard("detach")
				return ""
			},
			command: func(env *TestEnv, _ string) CmdResult {
				return env.RunCupboard("get", "crumbs", "some-id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			crumbID := tt.setup(env)
			result := tt.command(env, crumbID)

			if result.ExitCode == 0 {
				t.Error("operation after detach should fail")
			}
			if !strings.Contains(strings.ToLower(result.Stderr), "detach") {
				t.Errorf("stderr should contain 'detach', got: %s", result.Stderr)
			}
		})
	}
}
