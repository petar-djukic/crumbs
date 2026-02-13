// Integration tests for link management CRUD operations through the Table
// interface. Validates link creation with UUID v7 generation, all four link
// types (belongs_to, child_of, branches_from, scoped_to), filtering by
// link_type/from_id/to_id, uniqueness constraints, and deletion.
// Implements: test-rel03.0-uc002-link-management;
//             prd007-links-interface R1-R7; prd002-sqlite-backend R3, R12-R15.
package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S1: Link created via Table.Set generates UUID v7 for LinkID ---

func TestLinkManagement_CreateLinkGeneratesUUIDv7(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create prerequisite entities.
	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	// Create belongs_to link.
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb.CrumbID,
		ToID:     trail.TrailID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Verify UUID v7 format.
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())

	// Verify link retrieval.
	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)
	assert.Equal(t, id, gotLink.LinkID)
	assert.Equal(t, types.LinkTypeBelongsTo, gotLink.LinkType)
	assert.Equal(t, crumb.CrumbID, gotLink.FromID)
	assert.Equal(t, trail.TrailID, gotLink.ToID)
	assert.False(t, gotLink.CreatedAt.IsZero())
}

func TestLinkManagement_SetWithExistingIDUpdatesLink(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)

	// Create initial link.
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb1.CrumbID,
		ToID:     trail.TrailID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)

	// Update to point from crumb2.
	link.FromID = crumb2.CrumbID
	_, err = linksTbl.Set(id, link)
	require.NoError(t, err)

	// Verify update.
	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	assert.Equal(t, crumb2.CrumbID, got.(*types.Link).FromID)
}

// --- S2: belongs_to link associates crumb with trail ---

func TestLinkManagement_BelongsToLinkCrumbToTrail(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb.CrumbID,
		ToID:     trail.TrailID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)

	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)
	assert.Equal(t, types.LinkTypeBelongsTo, gotLink.LinkType)
	assert.Equal(t, crumb.CrumbID, gotLink.FromID)
	assert.Equal(t, trail.TrailID, gotLink.ToID)
}

func TestLinkManagement_BelongsToLinkAssociatesCrumbWithTrail(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create prerequisite entities.
	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	// Create belongs_to link (from_id=crumb_id, to_id=trail_id).
	link := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb.CrumbID,
		ToID:     trail.TrailID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)
	assert.NotEmpty(t, id, "Link ID should not be empty")

	// Verify link is retrievable via Table.Get.
	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)

	// Verify all Link struct fields.
	assert.Equal(t, id, gotLink.LinkID, "LinkID should match returned ID")
	assert.Equal(t, types.LinkTypeBelongsTo, gotLink.LinkType, "LinkType should be belongs_to")
	assert.Equal(t, crumb.CrumbID, gotLink.FromID, "FromID should be crumb ID")
	assert.Equal(t, trail.TrailID, gotLink.ToID, "ToID should be trail ID")
	assert.False(t, gotLink.CreatedAt.IsZero(), "CreatedAt should be set")

	// Verify prd007-links-interface R2.1 semantics: crumb membership in trail.
	// FromID is the crumb, ToID is the trail.
	assert.Equal(t, crumb.CrumbID, gotLink.FromID, "belongs_to link FromID should be crumb")
	assert.Equal(t, trail.TrailID, gotLink.ToID, "belongs_to link ToID should be trail")
}

// --- S3: child_of link establishes crumb hierarchy ---

func TestLinkManagement_ChildOfLinkEstablishesHierarchy(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, _ := getTestTables(t, backend)

	parent := createTestCrumb(t, crumbsTbl)
	child := createTestCrumb(t, crumbsTbl)

	link := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   child.CrumbID,
		ToID:     parent.CrumbID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)

	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)
	assert.Equal(t, types.LinkTypeChildOf, gotLink.LinkType)
	assert.Equal(t, child.CrumbID, gotLink.FromID)
	assert.Equal(t, parent.CrumbID, gotLink.ToID)
}

func TestLinkManagement_ChildOfLinkEstablishesCrumbHierarchy(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, _ := getTestTables(t, backend)

	// Create parent and child crumbs.
	parent := createTestCrumb(t, crumbsTbl)
	child := createTestCrumb(t, crumbsTbl)

	// Create child_of link (from_id=child, to_id=parent).
	link := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   child.CrumbID,
		ToID:     parent.CrumbID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)
	assert.NotEmpty(t, id, "Link ID should not be empty")

	// Verify link is retrievable via Table.Get.
	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)

	// Verify all Link struct fields.
	assert.Equal(t, id, gotLink.LinkID, "LinkID should match returned ID")
	assert.Equal(t, types.LinkTypeChildOf, gotLink.LinkType, "LinkType should be child_of")
	assert.Equal(t, child.CrumbID, gotLink.FromID, "FromID should be child crumb ID")
	assert.Equal(t, parent.CrumbID, gotLink.ToID, "ToID should be parent crumb ID")
	assert.False(t, gotLink.CreatedAt.IsZero(), "CreatedAt should be set")

	// Verify prd007-links-interface R2.1 semantics: crumb hierarchy.
	// FromID is the child crumb, ToID is the parent crumb.
	assert.Equal(t, child.CrumbID, gotLink.FromID, "child_of link FromID should be child")
	assert.Equal(t, parent.CrumbID, gotLink.ToID, "child_of link ToID should be parent")
}

func TestLinkManagement_QueryChildrenOfParent(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, _ := getTestTables(t, backend)

	parent := createTestCrumb(t, crumbsTbl)
	child1 := createTestCrumb(t, crumbsTbl)
	child2 := createTestCrumb(t, crumbsTbl)

	// Create child_of links.
	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: child1.CrumbID, ToID: parent.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: child2.CrumbID, ToID: parent.CrumbID})
	require.NoError(t, err)

	// Query children.
	filter := types.Filter{"link_type": types.LinkTypeChildOf, "to_id": parent.CrumbID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	fromIDs := []string{results[0].(*types.Link).FromID, results[1].(*types.Link).FromID}
	assert.Contains(t, fromIDs, child1.CrumbID)
	assert.Contains(t, fromIDs, child2.CrumbID)
}

func TestLinkManagement_QueryParentOfChild(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, _ := getTestTables(t, backend)

	parent := createTestCrumb(t, crumbsTbl)
	child := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: child.CrumbID, ToID: parent.CrumbID})
	require.NoError(t, err)

	// Query parent.
	filter := types.Filter{"link_type": types.LinkTypeChildOf, "from_id": child.CrumbID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, parent.CrumbID, results[0].(*types.Link).ToID)
}

// --- S4: branches_from link indicates trail branch point ---

func TestLinkManagement_BranchesFromLinkIndicatesBranchPoint(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	branchPoint := createTestCrumb(t, crumbsTbl)

	link := &types.Link{
		LinkType: types.LinkTypeBranchesFrom,
		FromID:   trail.TrailID,
		ToID:     branchPoint.CrumbID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)

	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)
	assert.Equal(t, types.LinkTypeBranchesFrom, gotLink.LinkType)
	assert.Equal(t, trail.TrailID, gotLink.FromID)
	assert.Equal(t, branchPoint.CrumbID, gotLink.ToID)
}

func TestLinkManagement_BranchesFromLinkIndicatesTrailBranchPoint(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create trail and branch point crumb.
	trail := createTestTrail(t, trailsTbl)
	branchPoint := createTestCrumb(t, crumbsTbl)

	// Create branches_from link (from_id=trail_id, to_id=crumb_id).
	link := &types.Link{
		LinkType: types.LinkTypeBranchesFrom,
		FromID:   trail.TrailID,
		ToID:     branchPoint.CrumbID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)
	assert.NotEmpty(t, id, "Link ID should not be empty")

	// Verify link is retrievable via Table.Get.
	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)

	// Verify all Link struct fields.
	assert.Equal(t, id, gotLink.LinkID, "LinkID should match returned ID")
	assert.Equal(t, types.LinkTypeBranchesFrom, gotLink.LinkType, "LinkType should be branches_from")
	assert.Equal(t, trail.TrailID, gotLink.FromID, "FromID should be trail ID")
	assert.Equal(t, branchPoint.CrumbID, gotLink.ToID, "ToID should be branch point crumb ID")
	assert.False(t, gotLink.CreatedAt.IsZero(), "CreatedAt should be set")

	// Verify prd007-links-interface R2.1 semantics: trail branch point.
	// FromID is the trail, ToID is the branch point crumb.
	assert.Equal(t, trail.TrailID, gotLink.FromID, "branches_from link FromID should be trail")
	assert.Equal(t, branchPoint.CrumbID, gotLink.ToID, "branches_from link ToID should be branch point crumb")
}

func TestLinkManagement_QueryBranchPointOfTrail(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	branchPoint := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trail.TrailID, ToID: branchPoint.CrumbID})
	require.NoError(t, err)

	filter := types.Filter{"link_type": types.LinkTypeBranchesFrom, "from_id": trail.TrailID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, branchPoint.CrumbID, results[0].(*types.Link).ToID)
}

// --- S5: scoped_to link scopes stash to trail ---

func TestLinkManagement_ScopedToLinkScopesStashToTrail(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	trail := createTestTrail(t, trailsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	link := &types.Link{
		LinkType: types.LinkTypeScopedTo,
		FromID:   stash.StashID,
		ToID:     trail.TrailID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)

	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)
	assert.Equal(t, types.LinkTypeScopedTo, gotLink.LinkType)
	assert.Equal(t, stash.StashID, gotLink.FromID)
	assert.Equal(t, trail.TrailID, gotLink.ToID)
}

func TestLinkManagement_ScopedToLinkScopesStashToTrailFull(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create stash and trail.
	trail := createTestTrail(t, trailsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	// Create scoped_to link (from_id=stash_id, to_id=trail_id).
	link := &types.Link{
		LinkType: types.LinkTypeScopedTo,
		FromID:   stash.StashID,
		ToID:     trail.TrailID,
	}
	id, err := linksTbl.Set("", link)
	require.NoError(t, err)
	assert.NotEmpty(t, id, "Link ID should not be empty")

	// Verify link is retrievable via Table.Get.
	got, err := linksTbl.Get(id)
	require.NoError(t, err)
	gotLink := got.(*types.Link)

	// Verify all Link struct fields.
	assert.Equal(t, id, gotLink.LinkID, "LinkID should match returned ID")
	assert.Equal(t, types.LinkTypeScopedTo, gotLink.LinkType, "LinkType should be scoped_to")
	assert.Equal(t, stash.StashID, gotLink.FromID, "FromID should be stash ID")
	assert.Equal(t, trail.TrailID, gotLink.ToID, "ToID should be trail ID")
	assert.False(t, gotLink.CreatedAt.IsZero(), "CreatedAt should be set")

	// Verify prd007-links-interface R2.1 semantics: stash scoped to trail.
	// FromID is the stash, ToID is the trail.
	assert.Equal(t, stash.StashID, gotLink.FromID, "scoped_to link FromID should be stash")
	assert.Equal(t, trail.TrailID, gotLink.ToID, "scoped_to link ToID should be trail")
}

func TestLinkManagement_QueryStashesScopedToTrail(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	trail := createTestTrail(t, trailsTbl)
	stash1 := createTestStash(t, stashesTbl, types.StashTypeContext)
	stash2 := createTestStash(t, stashesTbl, types.StashTypeContext)

	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stash1.StashID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stash2.StashID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{"link_type": types.LinkTypeScopedTo, "to_id": trail.TrailID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestLinkManagement_QueryTrailScopeOfStash(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	trail := createTestTrail(t, trailsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stash.StashID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{"link_type": types.LinkTypeScopedTo, "from_id": stash.StashID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, trail.TrailID, results[0].(*types.Link).ToID)
}

// --- S6: Fetch with link_type filter returns only links of that type ---

func TestLinkManagement_FetchByLinkTypeReturnsOnlyMatchingLinks(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	// Create links of different types.
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stash.StashID, ToID: trail.TrailID})
	require.NoError(t, err)

	// Fetch belongs_to links only.
	filter := types.Filter{"link_type": types.LinkTypeBelongsTo}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, types.LinkTypeBelongsTo, results[0].(*types.Link).LinkType)
}

func TestLinkManagement_FetchAllLinksWithoutFilter(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stash.StashID, ToID: trail.TrailID})
	require.NoError(t, err)

	// Fetch all links.
	results, err := linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	linkTypes := make(map[string]bool)
	for _, r := range results {
		linkTypes[r.(*types.Link).LinkType] = true
	}
	assert.True(t, linkTypes[types.LinkTypeBelongsTo])
	assert.True(t, linkTypes[types.LinkTypeChildOf])
	assert.True(t, linkTypes[types.LinkTypeScopedTo])
}

// --- S7: Fetch with from_id filter returns links originating from that entity ---

func TestLinkManagement_FetchByFromIDReturnsLinksFromEntity(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{"from_id": crumb.CrumbID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, crumb.CrumbID, results[0].(*types.Link).FromID)
}

func TestLinkManagement_FetchByFromIDWithNoMatchesReturnsEmpty(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{"from_id": "nonexistent-id"}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- S8: Fetch with to_id filter returns links targeting that entity ---

func TestLinkManagement_FetchByToIDReturnsLinksToEntity(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{"to_id": trail.TrailID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	for _, r := range results {
		assert.Equal(t, trail.TrailID, r.(*types.Link).ToID)
	}
}

func TestLinkManagement_FetchByToIDWithNoMatchesReturnsEmpty(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{"to_id": "nonexistent-id"}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- Combined filter tests ---

func TestLinkManagement_FetchWithLinkTypeAndToIDCombined(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)

	filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": trail.TrailID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, types.LinkTypeBelongsTo, results[0].(*types.Link).LinkType)
	assert.Equal(t, trail.TrailID, results[0].(*types.Link).ToID)
}

func TestLinkManagement_FetchWithAllThreeFilters(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	filter := types.Filter{
		"link_type": types.LinkTypeBelongsTo,
		"from_id":   crumb.CrumbID,
		"to_id":     trail.TrailID,
	}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

// --- S9: Delete removes link; subsequent Get returns ErrNotFound ---

func TestLinkManagement_DeleteLinkRemovesIt(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	id, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	err = linksTbl.Delete(id)
	require.NoError(t, err)

	_, err = linksTbl.Get(id)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestLinkManagement_DeleteLinkDoesNotAffectOtherLinks(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)

	id1, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	id2, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)

	err = linksTbl.Delete(id1)
	require.NoError(t, err)

	got, err := linksTbl.Get(id2)
	require.NoError(t, err)
	assert.Equal(t, id2, got.(*types.Link).LinkID)
}

// --- S10: Delete of nonexistent link returns ErrNotFound ---

func TestLinkManagement_DeleteNonexistentLinkReturnsErrNotFound(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, _ := getTestTables(t, backend)

	err := linksTbl.Delete("nonexistent-uuid-12345")
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestLinkManagement_DeleteWithEmptyIDReturnsErrInvalidID(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, _ := getTestTables(t, backend)

	err := linksTbl.Delete("")
	assert.ErrorIs(t, err, types.ErrInvalidID)
}

// --- Additional edge cases ---

func TestLinkManagement_GetWithEmptyIDReturnsErrInvalidID(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, _ := getTestTables(t, backend)

	_, err := linksTbl.Get("")
	assert.ErrorIs(t, err, types.ErrInvalidID)
}

func TestLinkManagement_GetNonexistentLinkReturnsErrNotFound(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, _ := getTestTables(t, backend)

	_, err := linksTbl.Get("nonexistent-uuid-12345")
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestLinkManagement_FetchEmptyTableReturnsEmptySlice(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, _ := getTestTables(t, backend)

	results, err := linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- Full workflow test ---

func TestLinkManagement_FullWorkflow(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	// Create entities.
	trail := createTestTrail(t, trailsTbl)
	parentCrumb := createTestCrumb(t, crumbsTbl)
	childCrumb := createTestCrumb(t, crumbsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	// Create all four link types.
	belongsToID, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: childCrumb.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: childCrumb.CrumbID, ToID: parentCrumb.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trail.TrailID, ToID: parentCrumb.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeScopedTo, FromID: stash.StashID, ToID: trail.TrailID})
	require.NoError(t, err)

	// Verify initial count.
	allLinks, err := linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allLinks, 4)

	// Query belongs_to links for trail.
	filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": trail.TrailID}
	belongsToLinks, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, belongsToLinks, 1)

	// Query child_of links where ToID is parentCrumb.
	filter = types.Filter{"link_type": types.LinkTypeChildOf, "to_id": parentCrumb.CrumbID}
	childOfLinks, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, childOfLinks, 1)

	// Delete belongs_to link.
	err = linksTbl.Delete(belongsToID)
	require.NoError(t, err)

	// Verify final count.
	allLinks, err = linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allLinks, 3)

	// Verify belongs_to links for trail is now 0.
	filter = types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": trail.TrailID}
	belongsToLinks, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, belongsToLinks, 0)
}

// --- Uniqueness constraint tests ---

// TestLinkManagement_UniquenessConstraintEnforced validates that the uniqueness
// constraint on (link_type, from_id, to_id) is enforced. Attempting to create a
// duplicate link with the same combination should return an error
// (prd007-links-interface R5.1, R5.2).
func TestLinkManagement_UniquenessConstraintEnforced(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Test uniqueness for belongs_to link.
	trail := createTestTrail(t, trailsTbl)
	crumb := createTestCrumb(t, crumbsTbl)

	// Create first belongs_to link.
	link1 := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb.CrumbID,
		ToID:     trail.TrailID,
	}
	id1, err := linksTbl.Set("", link1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	// Attempt to create duplicate belongs_to link with same (link_type, from_id, to_id).
	link2 := &types.Link{
		LinkType: types.LinkTypeBelongsTo,
		FromID:   crumb.CrumbID,
		ToID:     trail.TrailID,
	}
	_, err = linksTbl.Set("", link2)
	assert.Error(t, err, "Duplicate link should return an error")
	assert.ErrorIs(t, err, types.ErrDuplicateName, "Error should be ErrDuplicateName")

	// Verify only one link exists.
	filter := types.Filter{
		"link_type": types.LinkTypeBelongsTo,
		"from_id":   crumb.CrumbID,
		"to_id":     trail.TrailID,
	}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Only one link should exist")
}

// TestLinkManagement_UniquenessConstraintPerLinkType validates that uniqueness
// is enforced separately for each link type. The same (from_id, to_id) pair can
// exist for different link types (prd007-links-interface R5.1).
func TestLinkManagement_UniquenessConstraintPerLinkType(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, _ := getTestTables(t, backend)

	// Create two crumbs.
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)

	// Create child_of link from crumb1 to crumb2.
	link1 := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   crumb1.CrumbID,
		ToID:     crumb2.CrumbID,
	}
	id1, err := linksTbl.Set("", link1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	// Attempt to create duplicate child_of link with same (from_id, to_id).
	link2 := &types.Link{
		LinkType: types.LinkTypeChildOf,
		FromID:   crumb1.CrumbID,
		ToID:     crumb2.CrumbID,
	}
	_, err = linksTbl.Set("", link2)
	assert.Error(t, err, "Duplicate child_of link should return an error")
	assert.ErrorIs(t, err, types.ErrDuplicateName)

	// Verify only one child_of link exists.
	filter := types.Filter{
		"link_type": types.LinkTypeChildOf,
		"from_id":   crumb1.CrumbID,
		"to_id":     crumb2.CrumbID,
	}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Only one child_of link should exist")
}

// TestLinkManagement_UniquenessConstraintForBranchesFrom validates that a trail
// can have at most one branches_from link due to the uniqueness constraint
// (prd007-links-interface R5.1, R6.3).
func TestLinkManagement_UniquenessConstraintForBranchesFrom(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	trail := createTestTrail(t, trailsTbl)
	branchPoint := createTestCrumb(t, crumbsTbl)

	// Create first branches_from link.
	link1 := &types.Link{
		LinkType: types.LinkTypeBranchesFrom,
		FromID:   trail.TrailID,
		ToID:     branchPoint.CrumbID,
	}
	id1, err := linksTbl.Set("", link1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	// Attempt to create duplicate branches_from link for the same trail and branch point.
	link2 := &types.Link{
		LinkType: types.LinkTypeBranchesFrom,
		FromID:   trail.TrailID,
		ToID:     branchPoint.CrumbID,
	}
	_, err = linksTbl.Set("", link2)
	assert.Error(t, err, "Duplicate branches_from link should return an error")
	assert.ErrorIs(t, err, types.ErrDuplicateName)
}

// TestLinkManagement_UniquenessConstraintForScopedTo validates that a stash
// can have at most one scoped_to link due to the uniqueness constraint
// (prd007-links-interface R5.1, R6.4).
func TestLinkManagement_UniquenessConstraintForScopedTo(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, _, trailsTbl := getTestTables(t, backend)
	stashesTbl, err := backend.GetTable(types.TableStashes)
	require.NoError(t, err)

	trail := createTestTrail(t, trailsTbl)
	stash := createTestStash(t, stashesTbl, types.StashTypeContext)

	// Create first scoped_to link.
	link1 := &types.Link{
		LinkType: types.LinkTypeScopedTo,
		FromID:   stash.StashID,
		ToID:     trail.TrailID,
	}
	id1, err := linksTbl.Set("", link1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	// Attempt to create duplicate scoped_to link for the same stash and trail.
	link2 := &types.Link{
		LinkType: types.LinkTypeScopedTo,
		FromID:   stash.StashID,
		ToID:     trail.TrailID,
	}
	_, err = linksTbl.Set("", link2)
	assert.Error(t, err, "Duplicate scoped_to link should return an error")
	assert.ErrorIs(t, err, types.ErrDuplicateName)
}

// --- Comprehensive filter AND semantics tests ---

// TestLinkManagement_FetchMultipleLinksWithLinkTypeFilter validates that filtering
// by link_type returns only links of that specific type, even when multiple link
// types exist in the database (prd007-links-interface R4.1, R4.2).
func TestLinkManagement_FetchMultipleLinksWithLinkTypeFilter(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create diverse set of entities.
	trail1 := createTestTrail(t, trailsTbl)
	trail2 := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)
	crumb3 := createTestCrumb(t, crumbsTbl)

	// Create multiple belongs_to links.
	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail1.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2.CrumbID, ToID: trail1.TrailID})
	require.NoError(t, err)

	// Create multiple child_of links.
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb3.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)

	// Create branches_from link.
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trail2.TrailID, ToID: crumb1.CrumbID})
	require.NoError(t, err)

	// Verify total count.
	allLinks, err := linksTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, allLinks, 5)

	// Test link_type filter for belongs_to.
	filter := types.Filter{"link_type": types.LinkTypeBelongsTo}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should return only belongs_to links")
	for _, r := range results {
		assert.Equal(t, types.LinkTypeBelongsTo, r.(*types.Link).LinkType)
	}

	// Test link_type filter for child_of.
	filter = types.Filter{"link_type": types.LinkTypeChildOf}
	results, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should return only child_of links")
	for _, r := range results {
		assert.Equal(t, types.LinkTypeChildOf, r.(*types.Link).LinkType)
	}

	// Test link_type filter for branches_from.
	filter = types.Filter{"link_type": types.LinkTypeBranchesFrom}
	results, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should return only branches_from link")
	assert.Equal(t, types.LinkTypeBranchesFrom, results[0].(*types.Link).LinkType)
}

// TestLinkManagement_FetchMultipleLinksWithFromIDFilter validates that filtering
// by from_id returns all links originating from that entity, regardless of link
// type (prd007-links-interface R4.1, R4.2).
func TestLinkManagement_FetchMultipleLinksWithFromIDFilter(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create entities.
	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)
	crumb3 := createTestCrumb(t, crumbsTbl)

	// Create multiple links from crumb1 to different targets.
	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail.TrailID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb1.CrumbID, ToID: crumb2.CrumbID})
	require.NoError(t, err)

	// Create a link from crumb2 (different from_id).
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb3.CrumbID})
	require.NoError(t, err)

	// Fetch all links from crumb1.
	filter := types.Filter{"from_id": crumb1.CrumbID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should return all links from crumb1")
	for _, r := range results {
		assert.Equal(t, crumb1.CrumbID, r.(*types.Link).FromID)
	}

	// Verify different link types returned.
	linkTypes := make(map[string]bool)
	for _, r := range results {
		linkTypes[r.(*types.Link).LinkType] = true
	}
	assert.True(t, linkTypes[types.LinkTypeBelongsTo])
	assert.True(t, linkTypes[types.LinkTypeChildOf])
}

// TestLinkManagement_FetchMultipleLinksWithToIDFilter validates that filtering
// by to_id returns all links targeting that entity, regardless of link type
// (prd007-links-interface R4.1, R4.2).
func TestLinkManagement_FetchMultipleLinksWithToIDFilter(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create entities.
	trail := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)
	crumb3 := createTestCrumb(t, crumbsTbl)

	// Create multiple links to crumb1 from different sources.
	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb3.CrumbID, ToID: crumb1.CrumbID})
	require.NoError(t, err)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trail.TrailID, ToID: crumb1.CrumbID})
	require.NoError(t, err)

	// Create a link to a different target.
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb3.CrumbID})
	require.NoError(t, err)

	// Fetch all links to crumb1.
	filter := types.Filter{"to_id": crumb1.CrumbID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 3, "Should return all links to crumb1")
	for _, r := range results {
		assert.Equal(t, crumb1.CrumbID, r.(*types.Link).ToID)
	}

	// Verify different link types and from_ids.
	linkTypes := make(map[string]bool)
	fromIDs := make(map[string]bool)
	for _, r := range results {
		linkTypes[r.(*types.Link).LinkType] = true
		fromIDs[r.(*types.Link).FromID] = true
	}
	assert.True(t, linkTypes[types.LinkTypeChildOf])
	assert.True(t, linkTypes[types.LinkTypeBranchesFrom])
	assert.Len(t, fromIDs, 3, "Links should come from three different entities")
}

// TestLinkManagement_FetchWithCombinedFiltersANDSemantics validates that
// multiple filter keys are ANDed together, returning only links that match ALL
// criteria (prd007-links-interface R4.2).
func TestLinkManagement_FetchWithCombinedFiltersANDSemantics(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	linksTbl, crumbsTbl, trailsTbl := getTestTables(t, backend)

	// Create entities.
	trail1 := createTestTrail(t, trailsTbl)
	crumb1 := createTestCrumb(t, crumbsTbl)
	crumb2 := createTestCrumb(t, crumbsTbl)
	crumb3 := createTestCrumb(t, crumbsTbl)

	// Create diverse links.
	// belongs_to: crumb1 -> trail1
	_, err := linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb1.CrumbID, ToID: trail1.TrailID})
	require.NoError(t, err)
	// belongs_to: crumb2 -> trail1 (same trail, different crumb)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumb2.CrumbID, ToID: trail1.TrailID})
	require.NoError(t, err)
	// child_of: crumb1 -> crumb3
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb1.CrumbID, ToID: crumb3.CrumbID})
	require.NoError(t, err)
	// child_of: crumb2 -> crumb3 (same parent, different child)
	_, err = linksTbl.Set("", &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumb2.CrumbID, ToID: crumb3.CrumbID})
	require.NoError(t, err)

	// Test AND semantics: link_type AND from_id.
	// Should return only child_of links from crumb1 (1 result).
	filter := types.Filter{"link_type": types.LinkTypeChildOf, "from_id": crumb1.CrumbID}
	results, err := linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should return only links matching both link_type AND from_id")
	assert.Equal(t, types.LinkTypeChildOf, results[0].(*types.Link).LinkType)
	assert.Equal(t, crumb1.CrumbID, results[0].(*types.Link).FromID)

	// Test AND semantics: link_type AND to_id.
	// Should return only child_of links to crumb3 (2 results).
	filter = types.Filter{"link_type": types.LinkTypeChildOf, "to_id": crumb3.CrumbID}
	results, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should return only links matching both link_type AND to_id")
	for _, r := range results {
		assert.Equal(t, types.LinkTypeChildOf, r.(*types.Link).LinkType)
		assert.Equal(t, crumb3.CrumbID, r.(*types.Link).ToID)
	}

	// Test AND semantics: from_id AND to_id.
	// Should return only links from crumb1 to crumb3 (1 result).
	filter = types.Filter{"from_id": crumb1.CrumbID, "to_id": crumb3.CrumbID}
	results, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should return only links matching both from_id AND to_id")
	assert.Equal(t, crumb1.CrumbID, results[0].(*types.Link).FromID)
	assert.Equal(t, crumb3.CrumbID, results[0].(*types.Link).ToID)

	// Test AND semantics: all three filters.
	// Should return exactly one specific link.
	filter = types.Filter{
		"link_type": types.LinkTypeChildOf,
		"from_id":   crumb1.CrumbID,
		"to_id":     crumb3.CrumbID,
	}
	results, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should return only the link matching all three criteria")
	link := results[0].(*types.Link)
	assert.Equal(t, types.LinkTypeChildOf, link.LinkType)
	assert.Equal(t, crumb1.CrumbID, link.FromID)
	assert.Equal(t, crumb3.CrumbID, link.ToID)

	// Test AND semantics with no matches.
	// belongs_to from crumb1 to crumb3 does not exist.
	filter = types.Filter{
		"link_type": types.LinkTypeBelongsTo,
		"from_id":   crumb1.CrumbID,
		"to_id":     crumb3.CrumbID,
	}
	results, err = linksTbl.Fetch(filter)
	require.NoError(t, err)
	assert.Len(t, results, 0, "Should return empty when no links match all criteria")
}

// --- Helper functions ---

// getTestTables retrieves the links, crumbs, and trails tables from the backend.
func getTestTables(t *testing.T, backend *sqlite.Backend) (types.Table, types.Table, types.Table) {
	t.Helper()

	linksTbl, err := backend.GetTable(types.TableLinks)
	require.NoError(t, err)

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)

	trailsTbl, err := backend.GetTable(types.TableTrails)
	require.NoError(t, err)

	return linksTbl, crumbsTbl, trailsTbl
}

// createTestTrail creates a trail in draft state.
func createTestTrail(t *testing.T, trailsTbl types.Table) *types.Trail {
	t.Helper()

	trail := &types.Trail{State: types.TrailStateDraft}
	id, err := trailsTbl.Set("", trail)
	require.NoError(t, err)

	got, err := trailsTbl.Get(id)
	require.NoError(t, err)
	return got.(*types.Trail)
}

// createTestCrumb creates a crumb in draft state.
func createTestCrumb(t *testing.T, crumbsTbl types.Table) *types.Crumb {
	t.Helper()

	crumb := &types.Crumb{Name: "Test crumb", State: types.StateDraft}
	id, err := crumbsTbl.Set("", crumb)
	require.NoError(t, err)

	got, err := crumbsTbl.Get(id)
	require.NoError(t, err)
	return got.(*types.Crumb)
}

