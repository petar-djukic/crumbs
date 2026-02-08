// Go API integration tests for link management operations.
// Validates test-rel03.0-uc002-link-management.yaml test cases.
// Implements: docs/specs/test-suites/test-rel03.0-uc002-link-management.yaml;
//
//	docs/specs/use-cases/rel03.0-uc002-link-management.yaml;
//	prd002-sqlite-backend (graph model, R10 graph audit);
//	prd006-trails-interface R7 (crumb membership), R9 (branching);
//	prd008-stash-interface R13 (stash scoping).
package integration

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// --- Test Helpers ---

// isUUIDv7Link validates that the given string matches UUID v7 format.
func isUUIDv7Link(id string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(id))
}

// setupLinkTest creates a cupboard with SQLite backend and returns the
// cupboard and all required tables for testing.
func setupLinkTest(t *testing.T) (*sqlite.Backend, types.Table, types.Table, types.Table, types.Table, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	cupboard := sqlite.NewBackend()
	config := types.Config{
		Backend: types.BackendSQLite,
		DataDir: tmpDir,
	}

	if err := cupboard.Attach(config); err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	linksTable, err := cupboard.GetTable(types.LinksTable)
	if err != nil {
		t.Fatalf("GetTable(links) failed: %v", err)
	}

	crumbsTable, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		t.Fatalf("GetTable(crumbs) failed: %v", err)
	}

	trailsTable, err := cupboard.GetTable(types.TrailsTable)
	if err != nil {
		t.Fatalf("GetTable(trails) failed: %v", err)
	}

	stashesTable, err := cupboard.GetTable(types.StashesTable)
	if err != nil {
		t.Fatalf("GetTable(stashes) failed: %v", err)
	}

	cleanup := func() {
		cupboard.Detach()
	}

	return cupboard, linksTable, crumbsTable, trailsTable, stashesTable, cleanup
}

// createTestCrumbForLink creates a crumb for linking.
func createTestCrumbForLink(t *testing.T, crumbsTable types.Table, name string) string {
	t.Helper()

	crumb := &types.Crumb{
		Name:  name,
		State: types.StateDraft,
	}
	id, err := crumbsTable.Set("", crumb)
	if err != nil {
		t.Fatalf("Create crumb failed: %v", err)
	}
	return id
}

// createTestTrail creates a trail for linking.
func createTestTrail(t *testing.T, trailsTable types.Table) string {
	t.Helper()

	trail := &types.Trail{
		State: types.TrailStateActive,
	}
	id, err := trailsTable.Set("", trail)
	if err != nil {
		t.Fatalf("Create trail failed: %v", err)
	}
	return id
}

// createTestStash creates a stash for linking.
func createTestStash(t *testing.T, stashesTable types.Table, name string) string {
	t.Helper()

	stash := &types.Stash{
		Name:      name,
		StashType: types.StashTypeContext,
		Value:     map[string]any{"test": true},
		Version:   1,
	}
	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}
	return id
}

// --- S1: Link created via Table.Set generates UUID v7 for LinkID ---

func TestLinkManagement_S1_CreateLinkGeneratesUUIDv7(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	crumbID := createTestCrumbForLink(t, crumbsTable, "Test crumb")
	trailID := createTestTrail(t, trailsTable)

	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumbID,
		ToID:     trailID,
	}

	id, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if id == "" {
		t.Error("Set should return generated ID")
	}
	if !isUUIDv7Link(id) {
		t.Errorf("ID %q is not a valid UUID v7", id)
	}
	if link.LinkID != id {
		t.Errorf("Link.LinkID should be set to %q, got %q", id, link.LinkID)
	}
	if link.CreatedAt.IsZero() {
		t.Error("Link.CreatedAt should be set on creation")
	}
}

func TestLinkManagement_S1_SetWithExistingIDUpdatesLink(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	crumb1ID := createTestCrumbForLink(t, crumbsTable, "Crumb 1")
	crumb2ID := createTestCrumbForLink(t, crumbsTable, "Crumb 2")
	trailID := createTestTrail(t, trailsTable)

	// Create initial link
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb1ID,
		ToID:     trailID,
	}
	linkID, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Create link failed: %v", err)
	}

	// Update link to point from crumb2
	link.FromID = crumb2ID
	_, err = linksTable.Set(linkID, link)
	if err != nil {
		t.Fatalf("Update link failed: %v", err)
	}

	// Verify update
	entity, err := linksTable.Get(linkID)
	if err != nil {
		t.Fatalf("Get link failed: %v", err)
	}
	updated := entity.(*types.Link)
	if updated.FromID != crumb2ID {
		t.Errorf("FromID = %q, want %q", updated.FromID, crumb2ID)
	}
}

// --- S2: belongs_to link associates crumb with trail ---

func TestLinkManagement_S2_BelongsToLinkCrumbTrail(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	crumbID := createTestCrumbForLink(t, crumbsTable, "Task crumb")
	trailID := createTestTrail(t, trailsTable)

	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumbID,
		ToID:     trailID,
	}

	id, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Retrieve and verify
	entity, err := linksTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	retrieved := entity.(*types.Link)
	if retrieved.LinkType != types.LinkTypeBelongsTo {
		t.Errorf("LinkType = %q, want %q", retrieved.LinkType, types.LinkTypeBelongsTo)
	}
	if retrieved.FromID != crumbID {
		t.Errorf("FromID = %q, want %q", retrieved.FromID, crumbID)
	}
	if retrieved.ToID != trailID {
		t.Errorf("ToID = %q, want %q", retrieved.ToID, trailID)
	}
}

// --- S3: child_of link establishes crumb hierarchy ---

func TestLinkManagement_S3_ChildOfLinkCrumbHierarchy(t *testing.T) {
	_, linksTable, crumbsTable, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	parentID := createTestCrumbForLink(t, crumbsTable, "Epic parent")
	childID := createTestCrumbForLink(t, crumbsTable, "Task child")

	link := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   childID,
		ToID:     parentID,
	}

	id, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Retrieve and verify
	entity, err := linksTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	retrieved := entity.(*types.Link)
	if retrieved.LinkType != types.LinkTypeChildOf {
		t.Errorf("LinkType = %q, want %q", retrieved.LinkType, types.LinkTypeChildOf)
	}
	if retrieved.FromID != childID {
		t.Errorf("FromID = %q, want %q (child)", retrieved.FromID, childID)
	}
	if retrieved.ToID != parentID {
		t.Errorf("ToID = %q, want %q (parent)", retrieved.ToID, parentID)
	}
}

func TestLinkManagement_S3_QueryChildrenOfParent(t *testing.T) {
	_, linksTable, crumbsTable, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	parentID := createTestCrumbForLink(t, crumbsTable, "Epic parent")
	child1ID := createTestCrumbForLink(t, crumbsTable, "Task 1")
	child2ID := createTestCrumbForLink(t, crumbsTable, "Task 2")

	// Create child_of links
	for _, childID := range []string{child1ID, child2ID} {
		link := &types.Link{
			LinkType: types.LinkTypeChildOf,
			FromID:   childID,
			ToID:     parentID,
		}
		if _, err := linksTable.Set("", link); err != nil {
			t.Fatalf("Create child_of link failed: %v", err)
		}
	}

	// Query children of parent
	filter := map[string]any{"LinkType": types.LinkTypeChildOf, "ToID": parentID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 child links, got %d", len(entities))
	}

	// Verify both children are in results
	childIDs := make(map[string]bool)
	for _, e := range entities {
		link := e.(*types.Link)
		childIDs[link.FromID] = true
	}
	if !childIDs[child1ID] || !childIDs[child2ID] {
		t.Errorf("Expected both children in results, got %v", childIDs)
	}
}

func TestLinkManagement_S3_QueryParentOfChild(t *testing.T) {
	_, linksTable, crumbsTable, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	parentID := createTestCrumbForLink(t, crumbsTable, "Epic parent")
	childID := createTestCrumbForLink(t, crumbsTable, "Task child")

	link := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   childID,
		ToID:     parentID,
	}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create child_of link failed: %v", err)
	}

	// Query parent of child
	filter := map[string]any{"LinkType": types.LinkTypeChildOf, "FromID": childID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 parent link, got %d", len(entities))
	}

	parentLink := entities[0].(*types.Link)
	if parentLink.ToID != parentID {
		t.Errorf("ToID = %q, want %q (parent)", parentLink.ToID, parentID)
	}
}

// --- S4: branches_from link indicates trail branch point ---

func TestLinkManagement_S4_BranchesFromLinkTrailBranchPoint(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	branchPointID := createTestCrumbForLink(t, crumbsTable, "Decision point crumb")
	trailID := createTestTrail(t, trailsTable)

	link := &types.Link{
		LinkType: types.LinkTypeBranchesFrom,
		FromID:   trailID,
		ToID:     branchPointID,
	}

	id, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Retrieve and verify
	entity, err := linksTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	retrieved := entity.(*types.Link)
	if retrieved.LinkType != types.LinkTypeBranchesFrom {
		t.Errorf("LinkType = %q, want %q", retrieved.LinkType, types.LinkTypeBranchesFrom)
	}
	if retrieved.FromID != trailID {
		t.Errorf("FromID = %q, want %q (trail)", retrieved.FromID, trailID)
	}
	if retrieved.ToID != branchPointID {
		t.Errorf("ToID = %q, want %q (crumb)", retrieved.ToID, branchPointID)
	}
}

func TestLinkManagement_S4_QueryBranchPointOfTrail(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	branchPointID := createTestCrumbForLink(t, crumbsTable, "Decision point")
	trailID := createTestTrail(t, trailsTable)

	link := &types.Link{
		LinkType: types.LinkTypeBranchesFrom,
		FromID:   trailID,
		ToID:     branchPointID,
	}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create branches_from link failed: %v", err)
	}

	// Query branch point
	filter := map[string]any{"LinkType": types.LinkTypeBranchesFrom, "FromID": trailID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 branches_from link, got %d", len(entities))
	}

	branchLink := entities[0].(*types.Link)
	if branchLink.ToID != branchPointID {
		t.Errorf("ToID = %q, want %q (branch point)", branchLink.ToID, branchPointID)
	}
}

// --- S5: scoped_to link scopes stash to trail ---

func TestLinkManagement_S5_ScopedToLinkStashTrail(t *testing.T) {
	_, linksTable, _, trailsTable, stashesTable, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	stashID := createTestStash(t, stashesTable, "test-stash")

	link := &types.Link{
		LinkType: types.LinkTypeScopedTo,
		FromID:   stashID,
		ToID:     trailID,
	}

	id, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Retrieve and verify
	entity, err := linksTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	retrieved := entity.(*types.Link)
	if retrieved.LinkType != types.LinkTypeScopedTo {
		t.Errorf("LinkType = %q, want %q", retrieved.LinkType, types.LinkTypeScopedTo)
	}
	if retrieved.FromID != stashID {
		t.Errorf("FromID = %q, want %q (stash)", retrieved.FromID, stashID)
	}
	if retrieved.ToID != trailID {
		t.Errorf("ToID = %q, want %q (trail)", retrieved.ToID, trailID)
	}
}

func TestLinkManagement_S5_QueryStashesScopedToTrail(t *testing.T) {
	_, linksTable, _, trailsTable, stashesTable, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	stash1ID := createTestStash(t, stashesTable, "stash-1")
	stash2ID := createTestStash(t, stashesTable, "stash-2")

	// Create scoped_to links
	for _, stashID := range []string{stash1ID, stash2ID} {
		link := &types.Link{
			LinkType: types.LinkTypeScopedTo,
			FromID:   stashID,
			ToID:     trailID,
		}
		if _, err := linksTable.Set("", link); err != nil {
			t.Fatalf("Create scoped_to link failed: %v", err)
		}
	}

	// Query stashes scoped to trail
	filter := map[string]any{"LinkType": types.LinkTypeScopedTo, "ToID": trailID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 scoped_to links, got %d", len(entities))
	}
}

func TestLinkManagement_S5_QueryTrailScopeOfStash(t *testing.T) {
	_, linksTable, _, trailsTable, stashesTable, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	stashID := createTestStash(t, stashesTable, "scoped-stash")

	link := &types.Link{
		LinkType: types.LinkTypeScopedTo,
		FromID:   stashID,
		ToID:     trailID,
	}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create scoped_to link failed: %v", err)
	}

	// Query trail scope of stash
	filter := map[string]any{"LinkType": types.LinkTypeScopedTo, "FromID": stashID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 scoped_to link, got %d", len(entities))
	}

	scopeLink := entities[0].(*types.Link)
	if scopeLink.ToID != trailID {
		t.Errorf("ToID = %q, want %q (trail)", scopeLink.ToID, trailID)
	}
}

// --- S6: Fetch with link_type filter returns only links of that type ---

func TestLinkManagement_S6_FetchByLinkType(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, stashesTable, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumb1ID := createTestCrumbForLink(t, crumbsTable, "Crumb 1")
	crumb2ID := createTestCrumbForLink(t, crumbsTable, "Crumb 2")
	stashID := createTestStash(t, stashesTable, "test-stash")

	// Create one belongs_to link
	link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1ID, ToID: trailID}
	if _, err := linksTable.Set("", link1); err != nil {
		t.Fatalf("Create belongs_to link failed: %v", err)
	}

	// Create one child_of link
	link2 := &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2ID, ToID: crumb1ID}
	if _, err := linksTable.Set("", link2); err != nil {
		t.Fatalf("Create child_of link failed: %v", err)
	}

	// Create one scoped_to link
	link3 := &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stashID, ToID: trailID}
	if _, err := linksTable.Set("", link3); err != nil {
		t.Fatalf("Create scoped_to link failed: %v", err)
	}

	// Query only belongs_to links
	filter := map[string]any{"LinkType": types.LinkTypeBelongsTo}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 belongs_to link, got %d", len(entities))
	}

	for _, e := range entities {
		link := e.(*types.Link)
		if link.LinkType != types.LinkTypeBelongsTo {
			t.Errorf("Expected LinkType %q, got %q", types.LinkTypeBelongsTo, link.LinkType)
		}
	}
}

func TestLinkManagement_S6_FetchAllLinkTypes(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, stashesTable, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumb1ID := createTestCrumbForLink(t, crumbsTable, "Crumb 1")
	crumb2ID := createTestCrumbForLink(t, crumbsTable, "Crumb 2")
	stashID := createTestStash(t, stashesTable, "test-stash")

	// Create one of each link type
	links := []*types.Link{
		{LinkType: types.LinkTypeBelongsTo, FromID: crumb1ID, ToID: trailID},
		{LinkType: types.LinkTypeChildOf, FromID: crumb2ID, ToID: crumb1ID},
		{LinkType: types.LinkTypeScopedTo, FromID: stashID, ToID: trailID},
	}
	for _, link := range links {
		if _, err := linksTable.Set("", link); err != nil {
			t.Fatalf("Create link failed: %v", err)
		}
	}

	// Fetch all links
	entities, err := linksTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 3 {
		t.Errorf("Expected 3 links, got %d", len(entities))
	}

	// Verify all link types present
	typeCount := make(map[string]int)
	for _, e := range entities {
		link := e.(*types.Link)
		typeCount[link.LinkType]++
	}

	for _, lt := range []string{types.LinkTypeBelongsTo, types.LinkTypeChildOf, types.LinkTypeScopedTo} {
		if typeCount[lt] != 1 {
			t.Errorf("Expected 1 link of type %q, got %d", lt, typeCount[lt])
		}
	}
}

// --- S7: Fetch with from_id filter returns links originating from that entity ---

func TestLinkManagement_S7_FetchByFromID(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumbID := createTestCrumbForLink(t, crumbsTable, "Test crumb")

	link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create link failed: %v", err)
	}

	// Query by from_id
	filter := map[string]any{"FromID": crumbID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 link from crumb, got %d", len(entities))
	}

	result := entities[0].(*types.Link)
	if result.FromID != crumbID {
		t.Errorf("FromID = %q, want %q", result.FromID, crumbID)
	}
}

func TestLinkManagement_S7_FetchByFromIDNoMatches(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumbID := createTestCrumbForLink(t, crumbsTable, "Test crumb")

	link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create link failed: %v", err)
	}

	// Query by nonexistent from_id
	filter := map[string]any{"FromID": "nonexistent-id"}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("Expected 0 links, got %d", len(entities))
	}
}

// --- S8: Fetch with to_id filter returns links targeting that entity ---

func TestLinkManagement_S8_FetchByToID(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumb1ID := createTestCrumbForLink(t, crumbsTable, "Crumb 1")
	crumb2ID := createTestCrumbForLink(t, crumbsTable, "Crumb 2")

	// Create belongs_to links from both crumbs to trail
	for _, crumbID := range []string{crumb1ID, crumb2ID} {
		link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
		if _, err := linksTable.Set("", link); err != nil {
			t.Fatalf("Create link failed: %v", err)
		}
	}

	// Query by to_id
	filter := map[string]any{"ToID": trailID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 links to trail, got %d", len(entities))
	}

	for _, e := range entities {
		link := e.(*types.Link)
		if link.ToID != trailID {
			t.Errorf("ToID = %q, want %q", link.ToID, trailID)
		}
	}
}

func TestLinkManagement_S8_FetchByToIDNoMatches(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumbID := createTestCrumbForLink(t, crumbsTable, "Test crumb")

	link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create link failed: %v", err)
	}

	// Query by nonexistent to_id
	filter := map[string]any{"ToID": "nonexistent-id"}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("Expected 0 links, got %d", len(entities))
	}
}

// --- Combined filter tests ---

func TestLinkManagement_FetchWithLinkTypeAndToID(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumb1ID := createTestCrumbForLink(t, crumbsTable, "Crumb 1")
	crumb2ID := createTestCrumbForLink(t, crumbsTable, "Crumb 2")

	// Create belongs_to link from crumb1 to trail
	link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1ID, ToID: trailID}
	if _, err := linksTable.Set("", link1); err != nil {
		t.Fatalf("Create belongs_to link failed: %v", err)
	}

	// Create child_of link from crumb2 to crumb1
	link2 := &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2ID, ToID: crumb1ID}
	if _, err := linksTable.Set("", link2); err != nil {
		t.Fatalf("Create child_of link failed: %v", err)
	}

	// Query belongs_to links to trail
	filter := map[string]any{"LinkType": types.LinkTypeBelongsTo, "ToID": trailID}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 link, got %d", len(entities))
	}

	result := entities[0].(*types.Link)
	if result.LinkType != types.LinkTypeBelongsTo {
		t.Errorf("LinkType = %q, want %q", result.LinkType, types.LinkTypeBelongsTo)
	}
	if result.ToID != trailID {
		t.Errorf("ToID = %q, want %q", result.ToID, trailID)
	}
}

func TestLinkManagement_FetchWithAllThreeFilters(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumbID := createTestCrumbForLink(t, crumbsTable, "Test crumb")

	link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
	if _, err := linksTable.Set("", link); err != nil {
		t.Fatalf("Create link failed: %v", err)
	}

	// Query with all three filters
	filter := map[string]any{
		"LinkType": types.LinkTypeBelongsTo,
		"FromID":   crumbID,
		"ToID":     trailID,
	}
	entities, err := linksTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 link, got %d", len(entities))
	}
}

// --- S9: Delete removes link; subsequent Get returns ErrNotFound ---

func TestLinkManagement_S9_DeleteRemovesLink(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumbID := createTestCrumbForLink(t, crumbsTable, "Test crumb")

	link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
	linkID, err := linksTable.Set("", link)
	if err != nil {
		t.Fatalf("Create link failed: %v", err)
	}

	// Delete the link
	err = linksTable.Delete(linkID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get should return ErrNotFound
	_, err = linksTable.Get(linkID)
	if err != types.ErrNotFound {
		t.Errorf("Get after delete expected ErrNotFound, got %v", err)
	}
}

func TestLinkManagement_S9_DeleteDoesNotAffectOtherLinks(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, _, cleanup := setupLinkTest(t)
	defer cleanup()

	trailID := createTestTrail(t, trailsTable)
	crumb1ID := createTestCrumbForLink(t, crumbsTable, "Crumb 1")
	crumb2ID := createTestCrumbForLink(t, crumbsTable, "Crumb 2")

	// Create two links
	link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1ID, ToID: trailID}
	link1ID, err := linksTable.Set("", link1)
	if err != nil {
		t.Fatalf("Create link1 failed: %v", err)
	}

	link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2ID, ToID: trailID}
	link2ID, err := linksTable.Set("", link2)
	if err != nil {
		t.Fatalf("Create link2 failed: %v", err)
	}

	// Delete link1
	err = linksTable.Delete(link1ID)
	if err != nil {
		t.Fatalf("Delete link1 failed: %v", err)
	}

	// Link2 should still exist
	_, err = linksTable.Get(link2ID)
	if err != nil {
		t.Errorf("Link2 should still exist, got error: %v", err)
	}
}

// --- S10: Delete of nonexistent link returns ErrNotFound ---

func TestLinkManagement_S10_DeleteNonexistentReturnsErrNotFound(t *testing.T) {
	_, linksTable, _, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	err := linksTable.Delete("nonexistent-uuid-12345")
	if err != types.ErrNotFound {
		t.Errorf("Delete nonexistent expected ErrNotFound, got %v", err)
	}
}

func TestLinkManagement_S10_DeleteEmptyIDReturnsErrInvalidID(t *testing.T) {
	_, linksTable, _, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	err := linksTable.Delete("")
	if err != types.ErrInvalidID {
		t.Errorf("Delete empty ID expected ErrInvalidID, got %v", err)
	}
}

// --- Additional edge cases ---

func TestLinkManagement_GetEmptyIDReturnsErrInvalidID(t *testing.T) {
	_, linksTable, _, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	_, err := linksTable.Get("")
	if err != types.ErrInvalidID {
		t.Errorf("Get empty ID expected ErrInvalidID, got %v", err)
	}
}

func TestLinkManagement_GetNonexistentReturnsErrNotFound(t *testing.T) {
	_, linksTable, _, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	_, err := linksTable.Get("nonexistent-uuid-12345")
	if err != types.ErrNotFound {
		t.Errorf("Get nonexistent expected ErrNotFound, got %v", err)
	}
}

func TestLinkManagement_FetchEmptyTableReturnsEmptySlice(t *testing.T) {
	_, linksTable, _, _, _, cleanup := setupLinkTest(t)
	defer cleanup()

	entities, err := linksTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("Expected 0 links in empty table, got %d", len(entities))
	}
}

// --- Full workflow test ---

func TestLinkManagement_FullWorkflow(t *testing.T) {
	_, linksTable, crumbsTable, trailsTable, stashesTable, cleanup := setupLinkTest(t)
	defer cleanup()

	// Create entities
	trailID := createTestTrail(t, trailsTable)
	parentCrumbID := createTestCrumbForLink(t, crumbsTable, "Epic parent")
	childCrumbID := createTestCrumbForLink(t, crumbsTable, "Task child")
	stashID := createTestStash(t, stashesTable, "shared-context")

	// Create all four link types
	belongsToLink := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: childCrumbID, ToID: trailID}
	belongsToID, err := linksTable.Set("", belongsToLink)
	if err != nil {
		t.Fatalf("Create belongs_to link failed: %v", err)
	}

	childOfLink := &types.Link{LinkType: types.LinkTypeChildOf, FromID: childCrumbID, ToID: parentCrumbID}
	if _, err := linksTable.Set("", childOfLink); err != nil {
		t.Fatalf("Create child_of link failed: %v", err)
	}

	branchesFromLink := &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trailID, ToID: parentCrumbID}
	if _, err := linksTable.Set("", branchesFromLink); err != nil {
		t.Fatalf("Create branches_from link failed: %v", err)
	}

	scopedToLink := &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stashID, ToID: trailID}
	if _, err := linksTable.Set("", scopedToLink); err != nil {
		t.Fatalf("Create scoped_to link failed: %v", err)
	}

	// Verify 4 links exist
	allLinks, err := linksTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch all links failed: %v", err)
	}
	if len(allLinks) != 4 {
		t.Errorf("Expected 4 links, got %d", len(allLinks))
	}

	// Query belongs_to links for trail
	belongsToFilter := map[string]any{"LinkType": types.LinkTypeBelongsTo, "ToID": trailID}
	belongsToLinks, err := linksTable.Fetch(belongsToFilter)
	if err != nil {
		t.Fatalf("Fetch belongs_to links failed: %v", err)
	}
	if len(belongsToLinks) != 1 {
		t.Errorf("Expected 1 belongs_to link, got %d", len(belongsToLinks))
	}

	// Query children of parent
	childrenFilter := map[string]any{"LinkType": types.LinkTypeChildOf, "ToID": parentCrumbID}
	childrenLinks, err := linksTable.Fetch(childrenFilter)
	if err != nil {
		t.Fatalf("Fetch children links failed: %v", err)
	}
	if len(childrenLinks) != 1 {
		t.Errorf("Expected 1 child_of link, got %d", len(childrenLinks))
	}

	// Delete belongs_to link
	if err := linksTable.Delete(belongsToID); err != nil {
		t.Fatalf("Delete belongs_to link failed: %v", err)
	}

	// Verify 3 links remain
	remainingLinks, err := linksTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch remaining links failed: %v", err)
	}
	if len(remainingLinks) != 3 {
		t.Errorf("Expected 3 links after delete, got %d", len(remainingLinks))
	}

	// Query belongs_to links for trail should return 0
	belongsToAfter, err := linksTable.Fetch(belongsToFilter)
	if err != nil {
		t.Fatalf("Fetch belongs_to after delete failed: %v", err)
	}
	if len(belongsToAfter) != 0 {
		t.Errorf("Expected 0 belongs_to links after delete, got %d", len(belongsToAfter))
	}
}
