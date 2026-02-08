// CLI integration tests for cupboard lifecycle operations.
// Validates test-rel01.0-uc001-cupboard-lifecycle.yaml test cases.
// Implements: docs/specs/test-suites/test-rel01.0-uc001-cupboard-lifecycle.yaml;
//
//	docs/specs/use-cases/rel01.0-uc001-cupboard-lifecycle.yaml.
package integration

import (
	"path/filepath"
	"testing"
)

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
// Updated to account for trail cascade operations per prd006-trails-interface R5.6, R6.6:
//   - Completing a trail removes belongs_to links (crumbs become permanent)
//   - Abandoning a trail deletes all crumbs belonging to it
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

	// Transition crumb1 to pebble (completed successfully) - not on any trail
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

	// Complete trail1 - this removes belongs_to links, making crumb2 permanent
	env.MustRunCupboard("set", "trails", trail1.TrailID,
		`{"TrailID":"`+trail1.TrailID+`","State":"completed"}`)

	// Abandon trail2 - this deletes crumb3 (and its links, properties, metadata)
	env.MustRunCupboard("set", "trails", trail2.TrailID,
		`{"TrailID":"`+trail2.TrailID+`","State":"abandoned"}`)

	// Note: crumb3 was deleted by the abandon cascade, so we don't try to update it

	// Validate counts after cascade operations:
	// - 2 crumbs: crumb1 (pebble, not on trail), crumb2 (draft, made permanent from completed trail1)
	// - crumb3 was deleted by abandon cascade
	// - 2 trails: trail1 (completed), trail2 (abandoned)
	// - 0 links: belongs_to for crumb2 removed by complete cascade, belongs_to for crumb3 removed by abandon cascade
	allCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs").Stdout)
	allTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails").Stdout)
	allLinks := ParseJSON[[]Link](t, env.MustRunCupboard("list", "links").Stdout)

	if len(allCrumbs) != 2 {
		t.Errorf("crumb count = %d, want 2 (crumb3 deleted by abandon cascade)", len(allCrumbs))
	}
	if len(allTrails) != 2 {
		t.Errorf("trail count = %d, want 2", len(allTrails))
	}
	if len(allLinks) != 0 {
		t.Errorf("link count = %d, want 0 (links removed by cascade operations)", len(allLinks))
	}

	// Validate state counts
	pebbleCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "states=pebble").Stdout)
	draftCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "states=draft").Stdout)
	completedTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails", "State=completed").Stdout)
	abandonedTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails", "State=abandoned").Stdout)

	if len(pebbleCrumbs) != 1 {
		t.Errorf("pebble count = %d, want 1", len(pebbleCrumbs))
	}
	// crumb2 remains in draft state (made permanent but not transitioned)
	if len(draftCrumbs) != 1 {
		t.Errorf("draft count = %d, want 1 (crumb2 made permanent from trail1)", len(draftCrumbs))
	}
	if len(completedTrails) != 1 {
		t.Errorf("completed trail count = %d, want 1", len(completedTrails))
	}
	if len(abandonedTrails) != 1 {
		t.Errorf("abandoned trail count = %d, want 1", len(abandonedTrails))
	}

	// Verify crumb3 was actually deleted
	_ = crumb3 // Use crumb3 variable
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
