// CLI integration tests for cupboard.
// Validates self-hosting milestone (uc002-self-hosting).
// Implements: crumbs-ag8.1 (convert validation script to Go tests).
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain builds the cupboard binary once before running tests.
func TestMain(m *testing.M) {
	// Find project root by looking for go.mod
	projectRoot, err := FindProjectRoot()
	if err != nil {
		SetBuildErr(err)
		os.Exit(1)
	}

	// Build cupboard binary into a temp directory
	tmpDir, err := os.MkdirTemp("", "cupboard-test-*")
	if err != nil {
		SetBuildErr(err)
		os.Exit(1)
	}
	binPath := filepath.Join(tmpDir, "cupboard")
	SetCupboardBin(binPath)

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cupboard")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		SetBuildErr(&BuildError{
			Err:    err,
			Output: string(output),
		})
		os.Exit(1)
	}

	code := m.Run()

	// Cleanup binary
	os.RemoveAll(tmpDir)

	os.Exit(code)
}

// Test1_InitializeCupboard verifies cupboard initialization.
func Test1_InitializeCupboard(t *testing.T) {
	env := NewTestEnv(t)

	result := env.MustRunCupboard("init")

	// Verify output message
	if result.Stdout == "" {
		t.Error("expected init output message")
	}

	// Verify data directory was created
	if _, err := os.Stat(env.DataDir); os.IsNotExist(err) {
		t.Error("data directory not created")
	}

	// Verify crumbs.json was created
	crumbsFile := filepath.Join(env.DataDir, "crumbs.json")
	if _, err := os.Stat(crumbsFile); os.IsNotExist(err) {
		t.Error("crumbs.json not created")
	}
}

// Test2_CreateCrumbs verifies crumb creation with draft state.
func Test2_CreateCrumbs(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create first crumb
	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Implement feature X","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)
	if crumb1.CrumbID == "" {
		t.Error("crumb1 ID not generated")
	}
	if crumb1.Name != "Implement feature X" {
		t.Errorf("crumb1 name mismatch: got %q", crumb1.Name)
	}
	if crumb1.State != "draft" {
		t.Errorf("crumb1 state mismatch: got %q", crumb1.State)
	}

	// Create second crumb
	result2 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Write tests for feature X","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, result2.Stdout)
	if crumb2.CrumbID == "" {
		t.Error("crumb2 ID not generated")
	}

	// Create third crumb
	result3 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Try approach A","State":"draft"}`)
	crumb3 := ParseJSON[Crumb](t, result3.Stdout)
	if crumb3.CrumbID == "" {
		t.Error("crumb3 ID not generated")
	}

	// Verify all three are different
	if crumb1.CrumbID == crumb2.CrumbID || crumb2.CrumbID == crumb3.CrumbID {
		t.Error("crumb IDs should be unique")
	}
}

// Test3_CrumbStateTransitions verifies state transitions (draft -> ready -> taken -> completed).
func Test3_CrumbStateTransitions(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumb in draft state
	result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Implement feature X","State":"draft"}`)
	crumb := ParseJSON[Crumb](t, result.Stdout)
	crumbID := crumb.CrumbID

	// Transition to ready
	env.MustRunCupboard("set", "crumbs", crumbID, `{"CrumbID":"`+crumbID+`","Name":"Implement feature X","State":"ready"}`)
	getResult := env.MustRunCupboard("get", "crumbs", crumbID)
	crumb = ParseJSON[Crumb](t, getResult.Stdout)
	if crumb.State != "ready" {
		t.Errorf("expected state ready, got %q", crumb.State)
	}

	// Transition to taken
	env.MustRunCupboard("set", "crumbs", crumbID, `{"CrumbID":"`+crumbID+`","Name":"Implement feature X","State":"taken"}`)
	getResult = env.MustRunCupboard("get", "crumbs", crumbID)
	crumb = ParseJSON[Crumb](t, getResult.Stdout)
	if crumb.State != "taken" {
		t.Errorf("expected state taken, got %q", crumb.State)
	}

	// Transition to completed
	env.MustRunCupboard("set", "crumbs", crumbID, `{"CrumbID":"`+crumbID+`","Name":"Implement feature X","State":"completed"}`)
	getResult = env.MustRunCupboard("get", "crumbs", crumbID)
	crumb = ParseJSON[Crumb](t, getResult.Stdout)
	if crumb.State != "completed" {
		t.Errorf("expected state completed, got %q", crumb.State)
	}
}

// Test4_QueryCrumbsWithFilters verifies filtering by state.
func Test4_QueryCrumbsWithFilters(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumbs in various states
	result1 := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb 1","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, result1.Stdout)

	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb 2","State":"draft"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb 3","State":"draft"}`)

	// Transition crumb1 to completed
	env.MustRunCupboard("set", "crumbs", crumb1.CrumbID,
		`{"CrumbID":"`+crumb1.CrumbID+`","Name":"Crumb 1","State":"completed"}`)

	// Query draft crumbs (should be 2)
	draftResult := env.MustRunCupboard("list", "crumbs", "State=draft")
	draftCrumbs := ParseJSON[[]Crumb](t, draftResult.Stdout)
	if len(draftCrumbs) != 2 {
		t.Errorf("expected 2 draft crumbs, got %d", len(draftCrumbs))
	}

	// Query completed crumbs (should be 1)
	completedResult := env.MustRunCupboard("list", "crumbs", "State=completed")
	completedCrumbs := ParseJSON[[]Crumb](t, completedResult.Stdout)
	if len(completedCrumbs) != 1 {
		t.Errorf("expected 1 completed crumb, got %d", len(completedCrumbs))
	}

	// Query all crumbs (should be 3)
	allResult := env.MustRunCupboard("list", "crumbs")
	allCrumbs := ParseJSON[[]Crumb](t, allResult.Stdout)
	if len(allCrumbs) != 3 {
		t.Errorf("expected 3 total crumbs, got %d", len(allCrumbs))
	}
}

// Test5_CreateTrails verifies trail creation.
func Test5_CreateTrails(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create first trail in active state
	result1 := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail1 := ParseJSON[Trail](t, result1.Stdout)
	if trail1.TrailID == "" {
		t.Error("trail1 ID not generated")
	}
	if trail1.State != "active" {
		t.Errorf("trail1 state mismatch: got %q", trail1.State)
	}

	// Create second trail
	result2 := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail2 := ParseJSON[Trail](t, result2.Stdout)
	if trail2.TrailID == "" {
		t.Error("trail2 ID not generated")
	}

	// Verify different IDs
	if trail1.TrailID == trail2.TrailID {
		t.Error("trail IDs should be unique")
	}
}

// Test6_LinkCrumbsToTrails verifies belongs_to link creation.
func Test6_LinkCrumbsToTrails(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumbs
	crumb1Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb for trail 1","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, crumb1Result.Stdout)

	crumb2Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb for trail 2","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, crumb2Result.Stdout)

	// Create trails
	trail1Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail1 := ParseJSON[Trail](t, trail1Result.Stdout)

	trail2Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail2 := ParseJSON[Trail](t, trail2Result.Stdout)

	// Link crumb1 to trail1
	link1JSON := `{"LinkType":"belongs_to","FromID":"` + crumb1.CrumbID + `","ToID":"` + trail1.TrailID + `"}`
	link1Result := env.MustRunCupboard("set", "links", "", link1JSON)
	link1 := ParseJSON[Link](t, link1Result.Stdout)
	if link1.LinkID == "" {
		t.Error("link1 ID not generated")
	}

	// Link crumb2 to trail2
	link2JSON := `{"LinkType":"belongs_to","FromID":"` + crumb2.CrumbID + `","ToID":"` + trail2.TrailID + `"}`
	link2Result := env.MustRunCupboard("set", "links", "", link2JSON)
	link2 := ParseJSON[Link](t, link2Result.Stdout)
	if link2.LinkID == "" {
		t.Error("link2 ID not generated")
	}

	// Verify links exist
	linksResult := env.MustRunCupboard("list", "links")
	links := ParseJSON[[]Link](t, linksResult.Stdout)
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
}

// Test7_CompleteTrail verifies trail completion (successful exploration).
func Test7_CompleteTrail(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create trail in active state
	trailResult := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail := ParseJSON[Trail](t, trailResult.Stdout)

	// Complete trail
	env.MustRunCupboard("set", "trails", trail.TrailID,
		`{"TrailID":"`+trail.TrailID+`","State":"completed"}`)

	// Verify state
	getResult := env.MustRunCupboard("get", "trails", trail.TrailID)
	trail = ParseJSON[Trail](t, getResult.Stdout)
	if trail.State != "completed" {
		t.Errorf("expected trail state completed, got %q", trail.State)
	}
}

// Test8_AbandonTrail verifies trail abandonment (failed exploration).
func Test8_AbandonTrail(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create trail in active state
	trailResult := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail := ParseJSON[Trail](t, trailResult.Stdout)

	// Abandon trail
	env.MustRunCupboard("set", "trails", trail.TrailID,
		`{"TrailID":"`+trail.TrailID+`","State":"abandoned"}`)

	// Verify state
	getResult := env.MustRunCupboard("get", "trails", trail.TrailID)
	trail = ParseJSON[Trail](t, getResult.Stdout)
	if trail.State != "abandoned" {
		t.Errorf("expected trail state abandoned, got %q", trail.State)
	}
}

// Test9_ArchiveCrumb verifies crumb archival (soft delete).
func Test9_ArchiveCrumb(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create crumbs
	crumb1Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb to keep","State":"draft"}`)
	ParseJSON[Crumb](t, crumb1Result.Stdout)

	crumb2Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb to archive","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, crumb2Result.Stdout)

	// Archive crumb2
	env.MustRunCupboard("set", "crumbs", crumb2.CrumbID,
		`{"CrumbID":"`+crumb2.CrumbID+`","Name":"Crumb to archive","State":"archived"}`)

	// Verify state
	getResult := env.MustRunCupboard("get", "crumbs", crumb2.CrumbID)
	crumb2 = ParseJSON[Crumb](t, getResult.Stdout)
	if crumb2.State != "archived" {
		t.Errorf("expected crumb state archived, got %q", crumb2.State)
	}

	// Verify archived crumb not in draft list
	draftResult := env.MustRunCupboard("list", "crumbs", "State=draft")
	draftCrumbs := ParseJSON[[]Crumb](t, draftResult.Stdout)
	if len(draftCrumbs) != 1 {
		t.Errorf("expected 1 draft crumb after archive, got %d", len(draftCrumbs))
	}
}

// Test10_JSONPersistence verifies data is persisted to JSON files.
func Test10_JSONPersistence(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create test data
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb 1","State":"draft"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb 2","State":"draft"}`)
	env.MustRunCupboard("set", "crumbs", "", `{"Name":"Crumb 3","State":"draft"}`)

	trail1Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail1 := ParseJSON[Trail](t, trail1Result.Stdout)

	trail2Result := env.MustRunCupboard("set", "trails", "", `{"State":"active"}`)
	trail2 := ParseJSON[Trail](t, trail2Result.Stdout)

	// Complete trail1, abandon trail2
	env.MustRunCupboard("set", "trails", trail1.TrailID,
		`{"TrailID":"`+trail1.TrailID+`","State":"completed"}`)
	env.MustRunCupboard("set", "trails", trail2.TrailID,
		`{"TrailID":"`+trail2.TrailID+`","State":"abandoned"}`)

	// Verify crumbs.json
	crumbsFile := filepath.Join(env.DataDir, "crumbs.json")
	crumbsData, err := os.ReadFile(crumbsFile)
	if err != nil {
		t.Fatalf("failed to read crumbs.json: %v", err)
	}
	var crumbs []map[string]any
	if err := json.Unmarshal(crumbsData, &crumbs); err != nil {
		t.Fatalf("failed to parse crumbs.json: %v", err)
	}
	if len(crumbs) != 3 {
		t.Errorf("expected 3 crumbs in JSON, got %d", len(crumbs))
	}

	// Verify trails.json
	trailsFile := filepath.Join(env.DataDir, "trails.json")
	trailsData, err := os.ReadFile(trailsFile)
	if err != nil {
		t.Fatalf("failed to read trails.json: %v", err)
	}
	var trails []map[string]any
	if err := json.Unmarshal(trailsData, &trails); err != nil {
		t.Fatalf("failed to parse trails.json: %v", err)
	}
	if len(trails) != 2 {
		t.Errorf("expected 2 trails in JSON, got %d", len(trails))
	}

	// Verify trail states in JSON
	for _, tr := range trails {
		trailID, _ := tr["trail_id"].(string)
		state, _ := tr["state"].(string)
		if trailID == trail1.TrailID && state != "completed" {
			t.Errorf("expected trail1 state completed in JSON, got %q", state)
		}
		if trailID == trail2.TrailID && state != "abandoned" {
			t.Errorf("expected trail2 state abandoned in JSON, got %q", state)
		}
	}
}

// Test11_FullWorkflowValidation verifies the complete self-hosting workflow.
func Test11_FullWorkflowValidation(t *testing.T) {
	env := NewTestEnv(t)
	env.MustRunCupboard("init")

	// Create 3 crumbs
	crumb1Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Implement feature X","State":"draft"}`)
	crumb1 := ParseJSON[Crumb](t, crumb1Result.Stdout)

	crumb2Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Write tests for feature X","State":"draft"}`)
	crumb2 := ParseJSON[Crumb](t, crumb2Result.Stdout)

	crumb3Result := env.MustRunCupboard("set", "crumbs", "", `{"Name":"Try approach A","State":"draft"}`)
	crumb3 := ParseJSON[Crumb](t, crumb3Result.Stdout)

	// Transition crumb1 through states to completed
	env.MustRunCupboard("set", "crumbs", crumb1.CrumbID,
		`{"CrumbID":"`+crumb1.CrumbID+`","Name":"Implement feature X","State":"completed"}`)

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

	// Archive crumb3 (from abandoned trail)
	env.MustRunCupboard("set", "crumbs", crumb3.CrumbID,
		`{"CrumbID":"`+crumb3.CrumbID+`","Name":"Try approach A","State":"archived"}`)

	// Final validation - counts
	allCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs").Stdout)
	allTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails").Stdout)
	allLinks := ParseJSON[[]Link](t, env.MustRunCupboard("list", "links").Stdout)

	if len(allCrumbs) != 3 {
		t.Errorf("expected 3 crumbs, got %d", len(allCrumbs))
	}
	if len(allTrails) != 2 {
		t.Errorf("expected 2 trails, got %d", len(allTrails))
	}
	if len(allLinks) != 2 {
		t.Errorf("expected 2 links, got %d", len(allLinks))
	}

	// State counts
	completedCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "State=completed").Stdout)
	archivedCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "State=archived").Stdout)
	draftCrumbs := ParseJSON[[]Crumb](t, env.MustRunCupboard("list", "crumbs", "State=draft").Stdout)
	completedTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails", "State=completed").Stdout)
	abandonedTrails := ParseJSON[[]Trail](t, env.MustRunCupboard("list", "trails", "State=abandoned").Stdout)

	if len(completedCrumbs) != 1 {
		t.Errorf("expected 1 completed crumb, got %d", len(completedCrumbs))
	}
	if len(archivedCrumbs) != 1 {
		t.Errorf("expected 1 archived crumb, got %d", len(archivedCrumbs))
	}
	if len(draftCrumbs) != 1 {
		t.Errorf("expected 1 draft crumb, got %d", len(draftCrumbs))
	}
	if len(completedTrails) != 1 {
		t.Errorf("expected 1 completed trail, got %d", len(completedTrails))
	}
	if len(abandonedTrails) != 1 {
		t.Errorf("expected 1 abandoned trail, got %d", len(abandonedTrails))
	}
}
