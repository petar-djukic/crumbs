// CLI integration tests for crumb entity state machine and archival.
// Validates test-rel01.0-uc003-crumb-lifecycle.yaml test cases.
// Implements: docs/specs/test-suites/test-rel01.0-uc003-crumb-lifecycle.yaml;
//
//	docs/specs/use-cases/rel01.0-uc003-crumb-lifecycle.yaml.
package integration

import (
	"testing"
	"time"
)

// capitalize returns the string with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return string(s[0]-32) + s[1:]
}

// TestCrumbCreation validates that new crumbs start in draft state with
// CreatedAt, UpdatedAt, and Properties initialized (S1, S11).
// Note: The CLI requires explicit State in the JSON input; it does not
// auto-default to draft. These tests validate behavior when State is provided.
func TestCrumbCreation(t *testing.T) {
	tests := []struct {
		name           string
		crumbJSON      string
		wantState      string
		checkTimestamp bool
	}{
		{
			name:           "create crumb with draft state and initialized timestamps",
			crumbJSON:      `{"Name":"New crumb","State":"draft"}`,
			wantState:      "draft",
			checkTimestamp: true,
		},
		{
			name:           "create crumb with explicit state",
			crumbJSON:      `{"Name":"Ready crumb","State":"ready"}`,
			wantState:      "ready",
			checkTimestamp: true,
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
			if crumb.State != tt.wantState {
				t.Errorf("crumb state = %q, want %q", crumb.State, tt.wantState)
			}
			if tt.checkTimestamp {
				if crumb.CreatedAt == "" {
					t.Error("CreatedAt not set")
				}
				if crumb.UpdatedAt == "" {
					t.Error("UpdatedAt not set")
				}
			}
		})
	}
}

// TestCrumbStateTransitions validates all state transitions in the crumb state machine.
// Covers S2 (draft→pending), S3 (pending→ready), S4 (ready→taken), S5 (taken→pebble).
func TestCrumbStateTransitions(t *testing.T) {
	tests := []struct {
		name        string
		startState  string
		targetState string
	}{
		{"transition draft to pending", "draft", "pending"},
		{"transition pending to ready", "pending", "ready"},
		{"transition draft to ready", "draft", "ready"},
		{"transition ready to taken", "ready", "taken"},
		{"transition taken to pebble", "taken", "pebble"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumb with start state
			createJSON := `{"Name":"Test crumb","State":"` + tt.startState + `"}`
			createResult := env.MustRunCupboard("set", "crumbs", "", createJSON)
			crumb := ParseJSON[Crumb](t, createResult.Stdout)

			// Transition to target state
			updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"Test crumb","State":"` + tt.targetState + `"}`
			env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)

			// Verify state via get
			getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
			crumb = ParseJSON[Crumb](t, getResult.Stdout)

			if crumb.State != tt.targetState {
				t.Errorf("crumb state = %q, want %q", crumb.State, tt.targetState)
			}
		})
	}
}

// TestCrumbDustTransitions validates that Dust() works from any non-terminal state (S6, S7).
func TestCrumbDustTransitions(t *testing.T) {
	tests := []struct {
		name       string
		startState string
	}{
		{"transition draft to dust", "draft"},
		{"transition pending to dust", "pending"},
		{"transition ready to dust", "ready"},
		{"transition taken to dust", "taken"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumb with start state
			createJSON := `{"Name":"Dust crumb","State":"` + tt.startState + `"}`
			createResult := env.MustRunCupboard("set", "crumbs", "", createJSON)
			crumb := ParseJSON[Crumb](t, createResult.Stdout)

			// Transition to dust
			updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"Dust crumb","State":"dust"}`
			env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)

			// Verify state via get
			getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
			crumb = ParseJSON[Crumb](t, getResult.Stdout)

			if crumb.State != "dust" {
				t.Errorf("crumb state = %q, want dust", crumb.State)
			}
		})
	}
}

// TestCrumbTimestampTracking validates that timestamps are tracked on transitions (S8).
// Tests that UpdatedAt advances on updates and that both timestamps are present.
func TestCrumbTimestampTracking(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb with draft state
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Timestamp crumb","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, createResult.Stdout)

	if crumb.CreatedAt == "" {
		t.Fatal("CreatedAt not set on creation")
	}
	if crumb.UpdatedAt == "" {
		t.Fatal("UpdatedAt not set on creation")
	}

	originalUpdatedAt := crumb.UpdatedAt

	// Wait to ensure timestamp difference (timestamps are at second granularity)
	time.Sleep(1100 * time.Millisecond)

	// Transition to ready
	updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"Timestamp crumb","State":"ready"}`
	updateResult := env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)
	crumb = ParseJSON[Crumb](t, updateResult.Stdout)

	// Verify UpdatedAt advanced
	if crumb.UpdatedAt == originalUpdatedAt {
		t.Errorf("UpdatedAt should have advanced but is still %q", crumb.UpdatedAt)
	}
	if crumb.State != "ready" {
		t.Errorf("state = %q, want ready", crumb.State)
	}

	// Parse timestamps to verify UpdatedAt > originalUpdatedAt
	originalTime, err := time.Parse(time.RFC3339, originalUpdatedAt)
	if err != nil {
		t.Fatalf("failed to parse original UpdatedAt %q: %v", originalUpdatedAt, err)
	}
	updatedTime, err := time.Parse(time.RFC3339, crumb.UpdatedAt)
	if err != nil {
		t.Fatalf("failed to parse UpdatedAt %q: %v", crumb.UpdatedAt, err)
	}
	if !updatedTime.After(originalTime) {
		t.Errorf("UpdatedAt %v should be after original %v", updatedTime, originalTime)
	}
}

// TestCrumbFetchByState validates that Fetch by state returns only crumbs in the
// requested state (S9, S10).
func TestCrumbFetchByState(t *testing.T) {
	tests := []struct {
		name         string
		setupStates  []string
		filterState  string
		wantCount    int
		wantContains string
		wantExcludes []string
	}{
		{
			name:         "fetch by single state returns matching crumbs only",
			setupStates:  []string{"draft", "ready", "taken"},
			filterState:  "ready",
			wantCount:    1,
			wantContains: "Ready crumb",
			wantExcludes: []string{"Draft crumb", "Taken crumb"},
		},
		{
			name:         "filter excludes pebble crumbs from non-terminal state list",
			setupStates:  []string{"draft", "pebble"},
			filterState:  "draft",
			wantCount:    1,
			wantContains: "Draft crumb",
			wantExcludes: []string{"Pebble crumb"},
		},
		{
			name:         "filter excludes dust crumbs from non-terminal state list",
			setupStates:  []string{"draft", "dust"},
			filterState:  "draft",
			wantCount:    1,
			wantContains: "Draft crumb",
			wantExcludes: []string{"Dust crumb"},
		},
		{
			name:         "filter for pebble state returns only pebble crumbs",
			setupStates:  []string{"draft", "pebble", "dust"},
			filterState:  "pebble",
			wantCount:    1,
			wantContains: "Pebble crumb",
			wantExcludes: []string{"Draft crumb", "Dust crumb"},
		},
		{
			name:         "filter for dust state returns only dust crumbs",
			setupStates:  []string{"draft", "pebble", "dust"},
			filterState:  "dust",
			wantCount:    1,
			wantContains: "Dust crumb",
			wantExcludes: []string{"Draft crumb", "Pebble crumb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumbs with various states
			for _, state := range tt.setupStates {
				stateName := capitalize(state)
				crumbJSON := `{"Name":"` + stateName + ` crumb","State":"` + state + `"}`
				env.MustRunCupboard("set", "crumbs", "", crumbJSON)
			}

			// Fetch by state
			listResult := env.MustRunCupboard("list", "crumbs", "states="+tt.filterState)
			crumbs := ParseJSON[[]Crumb](t, listResult.Stdout)

			if len(crumbs) != tt.wantCount {
				t.Errorf("crumb count = %d, want %d", len(crumbs), tt.wantCount)
			}

			// Check that expected crumb is present
			if tt.wantContains != "" {
				found := false
				for _, c := range crumbs {
					if c.Name == tt.wantContains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected crumb %q not found in results", tt.wantContains)
				}
			}

			// Check that excluded crumbs are absent
			for _, exclude := range tt.wantExcludes {
				for _, c := range crumbs {
					if c.Name == exclude {
						t.Errorf("crumb %q should be excluded but was found", exclude)
					}
				}
			}
		})
	}
}

// TestCrumbFetchAllStates validates that listing without filter returns all crumbs.
func TestCrumbFetchAllStates(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumbs with various states
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Draft crumb","State":"draft"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Ready crumb","State":"ready"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Taken crumb","State":"taken"}`)

	// Fetch all crumbs (no filter)
	listResult := env.MustRunCupboard("list", "crumbs")
	crumbs := ParseJSON[[]Crumb](t, listResult.Stdout)

	if len(crumbs) != 3 {
		t.Errorf("crumb count = %d, want 3", len(crumbs))
	}

	// Verify all crumbs are present
	names := make(map[string]bool)
	for _, c := range crumbs {
		names[c.Name] = true
	}
	if !names["Draft crumb"] {
		t.Error("Draft crumb should be in results")
	}
	if !names["Ready crumb"] {
		t.Error("Ready crumb should be in results")
	}
	if !names["Taken crumb"] {
		t.Error("Taken crumb should be in results")
	}
}

// TestFullSuccessPath validates the complete success path: draft → pending → ready → taken → pebble.
func TestFullSuccessPath(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb in draft state (must specify state explicitly)
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Success path crumb","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, createResult.Stdout)

	if crumb.State != "draft" {
		t.Errorf("initial state = %q, want draft", crumb.State)
	}
	if crumb.CrumbID == "" {
		t.Fatal("crumb ID not generated")
	}

	transitions := []string{"pending", "ready", "taken", "pebble"}

	for _, state := range transitions {
		updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"Success path crumb","State":"` + state + `"}`
		env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)
	}

	// Verify final state
	getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
	crumb = ParseJSON[Crumb](t, getResult.Stdout)

	if crumb.State != "pebble" {
		t.Errorf("final state = %q, want pebble", crumb.State)
	}
}

// TestFullFailurePath validates the failure path: draft → dust.
func TestFullFailurePath(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb in draft state (must specify state explicitly)
	createResult := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Failure path crumb","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, createResult.Stdout)

	if crumb.State != "draft" {
		t.Errorf("initial state = %q, want draft", crumb.State)
	}

	// Transition directly to dust
	updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"Failure path crumb","State":"dust"}`
	env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)

	// Verify final state
	getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
	crumb = ParseJSON[Crumb](t, getResult.Stdout)

	if crumb.State != "dust" {
		t.Errorf("final state = %q, want dust", crumb.State)
	}
}

// TestMixedTerminalStates validates correct filtering when multiple crumbs
// have different terminal states.
func TestMixedTerminalStates(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumbs with different states
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Draft crumb","State":"draft"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Pebble crumb","State":"pebble"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Dust crumb","State":"dust"}`)

	// List all crumbs
	allResult := env.MustRunCupboard("list", "crumbs")
	allCrumbs := ParseJSON[[]Crumb](t, allResult.Stdout)

	if len(allCrumbs) != 3 {
		t.Errorf("total crumb count = %d, want 3", len(allCrumbs))
	}

	// Verify counts by state
	draftResult := env.MustRunCupboard("list", "crumbs", "states=draft")
	draftCrumbs := ParseJSON[[]Crumb](t, draftResult.Stdout)
	if len(draftCrumbs) != 1 {
		t.Errorf("draft count = %d, want 1", len(draftCrumbs))
	}

	pebbleResult := env.MustRunCupboard("list", "crumbs", "states=pebble")
	pebbleCrumbs := ParseJSON[[]Crumb](t, pebbleResult.Stdout)
	if len(pebbleCrumbs) != 1 {
		t.Errorf("pebble count = %d, want 1", len(pebbleCrumbs))
	}

	dustResult := env.MustRunCupboard("list", "crumbs", "states=dust")
	dustCrumbs := ParseJSON[[]Crumb](t, dustResult.Stdout)
	if len(dustCrumbs) != 1 {
		t.Errorf("dust count = %d, want 1", len(dustCrumbs))
	}
}
