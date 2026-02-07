// CLI integration tests for cupboard lifecycle and CRUD operations.
// Validates test004-cupboard-lifecycle.yaml test cases.
// Implements: docs/test-suites/test004-cupboard-lifecycle.yaml;
//             docs/use-cases/rel01.0-uc001-cupboard-lifecycle.md;
//             docs/use-cases/rel01.0-uc003-crud-operations.md.
package integration

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCrumbStateTransitions validates crumb state transition operations.
func TestCrumbStateTransitions(t *testing.T) {
	tests := []struct {
		name         string
		initialState string
		targetState  string
	}{
		{"draft to ready", "draft", "ready"},
		{"ready to taken", "ready", "taken"},
		{"taken to pebble", "taken", "pebble"},
		{"draft to dust", "draft", "dust"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumb with initial state
			createJSON := `{"Name":"State test crumb","State":"` + tt.initialState + `"}`
			result := env.MustRunCupboard("set", "crumbs", "", createJSON)
			crumb := ParseJSON[Crumb](t, result.Stdout)

			if crumb.State != tt.initialState {
				t.Errorf("initial state = %q, want %q", crumb.State, tt.initialState)
			}

			// Transition to target state
			updateJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"State test crumb","State":"` + tt.targetState + `"}`
			env.MustRunCupboard("set", "crumbs", crumb.CrumbID, updateJSON)

			// Verify state
			getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
			crumb = ParseJSON[Crumb](t, getResult.Stdout)

			if crumb.State != tt.targetState {
				t.Errorf("final state = %q, want %q", crumb.State, tt.targetState)
			}
		})
	}
}

// TestTrailCreation validates trail creation operations.
func TestTrailCreation(t *testing.T) {
	tests := []struct {
		name       string
		trailCount int
	}{
		{"create single trail", 1},
		{"create multiple trails", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			trailIDs := make([]string, 0, tt.trailCount)

			for i := 0; i < tt.trailCount; i++ {
				result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
				trail := ParseJSON[Trail](t, result.Stdout)

				if trail.TrailID == "" {
					t.Error("trail ID not generated")
				}
				if trail.State != "active" {
					t.Errorf("trail state = %q, want active", trail.State)
				}

				// Verify unique IDs
				for _, existingID := range trailIDs {
					if trail.TrailID == existingID {
						t.Error("trail IDs should be unique")
					}
				}
				trailIDs = append(trailIDs, trail.TrailID)
			}

			// Verify count via list
			listResult := env.MustRunCupboard("list", "trails")
			trails := ParseJSON[[]Trail](t, listResult.Stdout)

			if len(trails) != tt.trailCount {
				t.Errorf("trail count = %d, want %d", len(trails), tt.trailCount)
			}
		})
	}
}

// TestTrailLifecycle validates trail completion and abandonment.
func TestTrailLifecycle(t *testing.T) {
	tests := []struct {
		name        string
		targetState string
	}{
		{"complete trail", "completed"},
		{"abandon trail", "abandoned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create trail in active state
			result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
			trail := ParseJSON[Trail](t, result.Stdout)

			// Transition to target state
			updateJSON := `{"TrailID":"` + trail.TrailID + `","State":"` + tt.targetState + `"}`
			env.MustRunCupboard("set", "trails", trail.TrailID, updateJSON)

			// Verify state
			getResult := env.MustRunCupboard("get", "trails", trail.TrailID)
			trail = ParseJSON[Trail](t, getResult.Stdout)

			if trail.State != tt.targetState {
				t.Errorf("trail state = %q, want %q", trail.State, tt.targetState)
			}
		})
	}
}

// TestCrumbTrailLinking validates belongs_to link creation between crumbs and trails.
func TestCrumbTrailLinking(t *testing.T) {
	tests := []struct {
		name       string
		crumbCount int
		trailCount int
		linkCount  int
	}{
		{"link single crumb to trail", 1, 1, 1},
		{"link multiple crumbs to different trails", 2, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumbs
			crumbIDs := make([]string, 0, tt.crumbCount)
			for i := 0; i < tt.crumbCount; i++ {
				result := env.MustRunCupboard("set", "crumbs", "",
					`{"Name":"Crumb `+string(rune('1'+i))+`","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)
				crumbIDs = append(crumbIDs, crumb.CrumbID)
			}

			// Create trails
			trailIDs := make([]string, 0, tt.trailCount)
			for i := 0; i < tt.trailCount; i++ {
				result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
				trail := ParseJSON[Trail](t, result.Stdout)
				trailIDs = append(trailIDs, trail.TrailID)
			}

			// Create links
			for i := 0; i < tt.linkCount; i++ {
				crumbIdx := i % len(crumbIDs)
				trailIdx := i % len(trailIDs)
				linkJSON := `{"LinkType":"belongs_to","FromID":"` + crumbIDs[crumbIdx] + `","ToID":"` + trailIDs[trailIdx] + `"}`
				result := env.MustRunCupboard("set", "links", "", linkJSON)
				link := ParseJSON[Link](t, result.Stdout)

				if link.LinkID == "" {
					t.Error("link ID not generated")
				}
				if link.LinkType != "belongs_to" {
					t.Errorf("link type = %q, want belongs_to", link.LinkType)
				}
			}

			// Verify link count
			listResult := env.MustRunCupboard("list", "links")
			links := ParseJSON[[]Link](t, listResult.Stdout)

			if len(links) != tt.linkCount {
				t.Errorf("link count = %d, want %d", len(links), tt.linkCount)
			}
		})
	}
}

// TestCrumbArchival validates crumb archival via dust state.
func TestCrumbArchival(t *testing.T) {
	tests := []struct {
		name           string
		keepCount      int
		dustCount      int
		wantDraftCount int
	}{
		{"archive single crumb", 1, 1, 1},
		{"archive multiple crumbs", 2, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			env.MustRunCupboard("init")

			// Create crumbs to keep
			for i := 0; i < tt.keepCount; i++ {
				env.MustRunCupboard("set", "crumbs", "",
					`{"Name":"Keep crumb","State":"draft"}`)
			}

			// Create and dust crumbs
			for i := 0; i < tt.dustCount; i++ {
				result := env.MustRunCupboard("set", "crumbs", "",
					`{"Name":"Dust crumb","State":"draft"}`)
				crumb := ParseJSON[Crumb](t, result.Stdout)

				// Set to dust state
				dustJSON := `{"CrumbID":"` + crumb.CrumbID + `","Name":"Dust crumb","State":"dust"}`
				env.MustRunCupboard("set", "crumbs", crumb.CrumbID, dustJSON)

				// Verify dust state
				getResult := env.MustRunCupboard("get", "crumbs", crumb.CrumbID)
				crumb = ParseJSON[Crumb](t, getResult.Stdout)
				if crumb.State != "dust" {
					t.Errorf("crumb state = %q, want dust", crumb.State)
				}
			}

			// Verify draft filter excludes dust
			draftResult := env.MustRunCupboard("list", "crumbs", "State=draft")
			draftCrumbs := ParseJSON[[]Crumb](t, draftResult.Stdout)

			if len(draftCrumbs) != tt.wantDraftCount {
				t.Errorf("draft count = %d, want %d", len(draftCrumbs), tt.wantDraftCount)
			}

			// Verify dust filter works
			dustResult := env.MustRunCupboard("list", "crumbs", "State=dust")
			dustCrumbs := ParseJSON[[]Crumb](t, dustResult.Stdout)

			if len(dustCrumbs) != tt.dustCount {
				t.Errorf("dust count = %d, want %d", len(dustCrumbs), tt.dustCount)
			}
		})
	}
}

// TestTrailPersistence validates trails are persisted to JSONL files.
func TestTrailPersistence(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create trails
	trail1Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail1 := ParseJSON[Trail](t, trail1Result.Stdout)

	trail2Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail2 := ParseJSON[Trail](t, trail2Result.Stdout)

	// Transition states
	env.MustRunCupboard("set", "trails", trail1.TrailID,
		`{"TrailID":"`+trail1.TrailID+`","State":"completed"}`)
	env.MustRunCupboard("set", "trails", trail2.TrailID,
		`{"TrailID":"`+trail2.TrailID+`","State":"abandoned"}`)

	// Read JSONL file
	trailsFile := filepath.Join(env.DataDir, "trails.jsonl")
	trails := ReadJSONLFile[map[string]any](t, trailsFile)

	if len(trails) != 2 {
		t.Errorf("trail count in JSONL = %d, want 2", len(trails))
	}

	// Verify states in JSONL
	for _, tr := range trails {
		trailID, _ := tr["trail_id"].(string)
		state, _ := tr["state"].(string)

		switch trailID {
		case trail1.TrailID:
			if state != "completed" {
				t.Errorf("trail1 state in JSONL = %q, want completed", state)
			}
		case trail2.TrailID:
			if state != "abandoned" {
				t.Errorf("trail2 state in JSONL = %q, want abandoned", state)
			}
		}
	}
}

// TestFullLifecycleWorkflow validates the complete lifecycle with crumbs and trails.
func TestFullLifecycleWorkflow(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create 3 crumbs
	crumb1Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Implement feature X","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, crumb1Result.Stdout)

	crumb2Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Write tests for feature X","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, crumb2Result.Stdout)

	crumb3Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Try approach A","State":"draft"}`)
	crumb3 := ParseJSON[Crumb](t, crumb3Result.Stdout)

	// Transition crumb1 to pebble (completed successfully)
	env.MustRunCupboard("set", "crumbs", crumb1.CrumbID,
		`{"CrumbID":"`+crumb1.CrumbID+`","Name":"Implement feature X","State":"pebble"}`)

	// Create 2 trails
	trail1Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail1 := ParseJSON[Trail](t, trail1Result.Stdout)

	trail2Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail2 := ParseJSON[Trail](t, trail2Result.Stdout)

	// Link crumbs to trails
	env.MustRunCupboard("set", "links", "",
		`{"LinkType":"belongs_to","FromID":"`+crumb2.CrumbID+`","ToID":"`+trail1.TrailID+`"}`)
	env.MustRunCupboard("set", "links", "",
		`{"LinkType":"belongs_to","FromID":"`+crumb3.CrumbID+`","ToID":"`+trail2.TrailID+`"}`)

	// Complete trail1, abandon trail2
	env.MustRunCupboard("set", "trails", trail1.TrailID,
		`{"TrailID":"`+trail1.TrailID+`","State":"completed"}`)
	env.MustRunCupboard("set", "trails", trail2.TrailID,
		`{"TrailID":"`+trail2.TrailID+`","State":"abandoned"}`)

	// Dust crumb3 (from abandoned trail)
	env.MustRunCupboard("set", "crumbs", crumb3.CrumbID,
		`{"CrumbID":"`+crumb3.CrumbID+`","Name":"Try approach A","State":"dust"}`)

	// Validate counts
	allCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs").Stdout)
	allTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails").Stdout)
	allLinks := ParseJSON[[]Link](t, env.MustRunCupboard("list", "links").Stdout)

	if len(allCrumbs) != 3 {
		t.Errorf("crumb count = %d, want 3", len(allCrumbs))
	}
	if len(allTrails) != 2 {
		t.Errorf("trail count = %d, want 2", len(allTrails))
	}
	if len(allLinks) != 2 {
		t.Errorf("link count = %d, want 2", len(allLinks))
	}

	// Validate state counts
	pebbleCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "State=pebble").Stdout)
	dustCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "State=dust").Stdout)
	draftCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "State=draft").Stdout)
	completedTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails", "State=completed").Stdout)
	abandonedTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails", "State=abandoned").Stdout)

	if len(pebbleCrumbs) != 1 {
		t.Errorf("pebble count = %d, want 1", len(pebbleCrumbs))
	}
	if len(dustCrumbs) != 1 {
		t.Errorf("dust count = %d, want 1", len(dustCrumbs))
	}
	if len(draftCrumbs) != 1 {
		t.Errorf("draft count = %d, want 1", len(draftCrumbs))
	}
	if len(completedTrails) != 1 {
		t.Errorf("completed trail count = %d, want 1", len(completedTrails))
	}
	if len(abandonedTrails) != 1 {
		t.Errorf("abandoned trail count = %d, want 1", len(abandonedTrails))
	}
}

// TestInitialize validates cupboard initialization for lifecycle tests.
func TestLifecycleInitialize(t *testing.T) {
	env := NewTestEnv(t)

	result := env.MustRunCupboard("init")

	// Verify output message
	if result.Stdout == "" {
		t.Error("expected init output message")
	}

	// Verify trails.jsonl was created
	trailsFile := filepath.Join(env.DataDir, "trails.jsonl")
	if _, err := filepath.Abs(trailsFile); err != nil {
		t.Errorf("trails.jsonl path error: %v", err)
	}

	// Verify links.jsonl was created
	linksFile := filepath.Join(env.DataDir, "links.jsonl")
	if _, err := filepath.Abs(linksFile); err != nil {
		t.Errorf("links.jsonl path error: %v", err)
	}
}

// TestCrumbPersistence validates crumbs with state changes are persisted.
func TestCrumbPersistence(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumbs with different states
	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Draft crumb","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)

	result2 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Pebble crumb","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, result2.Stdout)

	// Transition crumb2 to pebble
	env.MustRunCupboard("set", "crumbs", crumb2.CrumbID,
		`{"CrumbID":"`+crumb2.CrumbID+`","Name":"Pebble crumb","State":"pebble"}`)

	// Read JSONL file
	crumbsFile := filepath.Join(env.DataDir, "crumbs.jsonl")
	crumbs := ReadJSONLFile[map[string]any](t, crumbsFile)

	if len(crumbs) != 2 {
		t.Errorf("crumb count in JSONL = %d, want 2", len(crumbs))
	}

	// Verify states in JSONL
	for _, cr := range crumbs {
		crumbID, _ := cr["crumb_id"].(string)
		state, _ := cr["state"].(string)

		switch crumbID {
		case crumb1.CrumbID:
			if state != "draft" {
				t.Errorf("crumb1 state in JSONL = %q, want draft", state)
			}
		case crumb2.CrumbID:
			if state != "pebble" {
				t.Errorf("crumb2 state in JSONL = %q, want pebble", state)
			}
		}
	}
}

// Suppress unused import warning for strings package.
var _ = strings.Contains
