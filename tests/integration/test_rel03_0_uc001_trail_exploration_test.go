// Integration tests for trail-based exploration and lifecycle operations.
// Validates trail creation, belongs_to links, trail completion (making crumbs
// permanent), and trail abandonment (atomic deletion of associated crumbs).
// Implements: test-rel03.0-uc001-trail-exploration;
//             prd006-trails-interface R1-R6 (Trail entity, states, complete, abandon);
//             prd007-links-interface R2 (belongs_to, branches_from links);
//             prd002-sqlite-backend R5.6-R5.7 (cascade on completed/abandoned).
package integration

import (
	"testing"
	"time"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrailExploration(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, backend *sqlite.Backend) *testContext
		operation func(t *testing.T, ctx *testContext)
		assert    func(t *testing.T, ctx *testContext)
	}{
		// --- S1: Trail created via Table.Set starts in draft state by default ---
		{
			name: "Create trail defaults to draft state",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				return &testContext{trailsTbl: trailsTbl}
			},
			operation: func(t *testing.T, ctx *testContext) {
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				id, err := ctx.trailsTbl.Set("", trail)
				require.NoError(t, err)
				ctx.trailID = id
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateDraft, trail.State)
				assert.NotEmpty(t, trail.TrailID)
				assert.False(t, trail.CreatedAt.IsZero())
			},
		},
		{
			name: "Create trail with explicit active state",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				return &testContext{trailsTbl: trailsTbl}
			},
			operation: func(t *testing.T, ctx *testContext) {
				// Create in draft, then update to active.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				id, err := ctx.trailsTbl.Set("", trail)
				require.NoError(t, err)
				ctx.trailID = id

				// Update to active state.
				got, err := ctx.trailsTbl.Get(id)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = ctx.trailsTbl.Set(id, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateActive, trail.State)
				assert.NotEmpty(t, trail.TrailID)
				assert.False(t, trail.CreatedAt.IsZero())
			},
		},
		{
			name: "Create trail in draft state explicitly",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				return &testContext{trailsTbl: trailsTbl}
			},
			operation: func(t *testing.T, ctx *testContext) {
				trail := &types.Trail{State: types.TrailStateDraft, CreatedAt: time.Now().UTC()}
				id, err := ctx.trailsTbl.Set("", trail)
				require.NoError(t, err)
				ctx.trailID = id
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateDraft, got.(*types.Trail).State)
			},
		},
		{
			name: "Transition trail from draft to active via SetState",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				id, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				return &testContext{trailsTbl: trailsTbl, trailID: id}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateActive, got.(*types.Trail).State)
			},
		},
		{
			name: "Transition trail from draft to pending via SetState",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				id, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				return &testContext{trailsTbl: trailsTbl, trailID: id}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				trail.State = types.TrailStatePending
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStatePending, got.(*types.Trail).State)
			},
		},
		{
			name: "SetState rejects invalid transitions from draft",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				id, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				return &testContext{trailsTbl: trailsTbl, trailID: id}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				// Attempting to transition draft -> completed should fail via Complete().
				err = trail.Complete()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// No assertion needed; operation validates the error.
			},
		},
		{
			name: "SetState rejects transitions from terminal states",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in active state, then complete it.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				id, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(id)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(id, trail)
				require.NoError(t, err)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = trailsTbl.Set(id, trail)
				require.NoError(t, err)

				return &testContext{trailsTbl: trailsTbl, trailID: id}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				// Trail is already completed (terminal state).
				// Attempting to transition to another state via Abandon should fail.
				err = trail.Abandon()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// Verify trail is still completed.
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateCompleted, got.(*types.Trail).State)
			},
		},

		// --- S2: belongs_to links associate crumbs with trails ---
		{
			name: "Create belongs_to link associates crumb with trail",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumb.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				link := &types.Link{
					LinkType: types.LinkTypeBelongsTo,
					FromID:   ctx.crumbID,
					ToID:     ctx.trailID,
				}
				linkID, err := ctx.linksTbl.Set("", link)
				require.NoError(t, err)
				ctx.linkID = linkID
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.linksTbl.Get(ctx.linkID)
				require.NoError(t, err)
				link := got.(*types.Link)
				assert.Equal(t, types.LinkTypeBelongsTo, link.LinkType)
				assert.Equal(t, ctx.crumbID, link.FromID)
				assert.Equal(t, ctx.trailID, link.ToID)
			},
		},
		{
			name: "Multiple crumbs can belong to same trail",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create two crumbs.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				link1 := &types.Link{
					LinkType: types.LinkTypeBelongsTo,
					FromID:   ctx.crumbID,
					ToID:     ctx.trailID,
				}
				_, err := ctx.linksTbl.Set("", link1)
				require.NoError(t, err)

				link2 := &types.Link{
					LinkType: types.LinkTypeBelongsTo,
					FromID:   ctx.crumbID2,
					ToID:     ctx.trailID,
				}
				_, err = ctx.linksTbl.Set("", link2)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 2)
			},
		},

		// --- S3: Querying links table with belongs_to filter returns crumbs on that trail ---
		{
			name: "Fetch crumbs on trail via belongs_to filter",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumbs.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)
				crumb3 := &types.Crumb{Name: "Crumb 3"}
				crumbID3, err := crumbsTbl.Set("", crumb3)
				require.NoError(t, err)

				// Link crumb1 and crumb2 to trail.
				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = linksTbl.Set("", link2)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
					crumbID3:  crumbID3,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				// No operation needed; setup is complete.
			},
			assert: func(t *testing.T, ctx *testContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 2)

				// Verify that crumb1 and crumb2 are in results, crumb3 is not.
				fromIDs := make(map[string]bool)
				for _, l := range links {
					fromIDs[l.(*types.Link).FromID] = true
				}
				assert.True(t, fromIDs[ctx.crumbID])
				assert.True(t, fromIDs[ctx.crumbID2])
				assert.False(t, fromIDs[ctx.crumbID3])
			},
		},

		// --- S4: Deleting belongs_to link disassociates without deleting the crumb ---
		{
			name: "Delete belongs_to link keeps crumb intact",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumb and link to trail.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
				linkID, err := linksTbl.Set("", link)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
					linkID:    linkID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				err := ctx.linksTbl.Delete(ctx.linkID)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// Crumb should still exist.
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)

				// Link should not exist.
				_, err = ctx.linksTbl.Get(ctx.linkID)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},

		// --- S5: trail.Abandon() then Table.Set deletes trail crumbs atomically ---
		{
			name: "Abandon trail deletes associated crumbs",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create two crumbs and link to trail.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = linksTbl.Set("", link2)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateAbandoned, got.(*types.Trail).State)

				// Both crumbs should be deleted.
				_, err = ctx.crumbsTbl.Get(ctx.crumbID)
				assert.ErrorIs(t, err, types.ErrNotFound)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},
		{
			name: "Abandon trail removes belongs_to links",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumb and link to trail.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateAbandoned, got.(*types.Trail).State)

				// No belongs_to links should remain.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		// --- S6: Crumbs removed from trail before abandon survive ---
		{
			name: "Crumb removed before abandon survives",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create two crumbs and link both to trail.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				linkID1, err := linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = linksTbl.Set("", link2)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
					linkID:    linkID1,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				// Remove crumb1 from trail by deleting its belongs_to link.
				err := ctx.linksTbl.Delete(ctx.linkID)
				require.NoError(t, err)

				// Abandon the trail.
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// Crumb1 should still exist (removed before abandon).
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)

				// Crumb2 should be deleted (was on trail at abandon time).
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},

		// --- S7: trail.Complete() then Table.Set removes belongs_to links ---
		{
			name: "Complete trail removes belongs_to links",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumbs and link to trail.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = linksTbl.Set("", link2)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateCompleted, got.(*types.Trail).State)

				// No belongs_to links should remain.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		// --- S8: Completed trail crumbs remain queryable ---
		{
			name: "Completed trail crumbs remain accessible",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumbs and link to trail.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = linksTbl.Set("", link2)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// Both crumbs should still exist.
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				require.NoError(t, err)
			},
		},
		{
			name: "Completed trail crumbs are indistinguishable from permanent crumbs",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumb and link to trail.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// Crumb should exist.
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)

				// No belongs_to link should exist.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		// --- S9: Trail state is completed or abandoned after respective operations ---
		{
			name: "Trail state is completed after Complete",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &testContext{trailsTbl: trailsTbl, trailID: trailID}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateCompleted, trail.State)
				assert.NotNil(t, trail.CompletedAt)
			},
		},
		{
			name: "Trail state is abandoned after Abandon",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &testContext{trailsTbl: trailsTbl, trailID: trailID}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateAbandoned, trail.State)
				assert.NotNil(t, trail.CompletedAt)
			},
		},
		{
			name: "Complete on non-active trail returns ErrInvalidState",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)

				return &testContext{trailsTbl: trailsTbl, trailID: trailID}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// No assertion needed; operation validates the error.
			},
		},
		{
			name: "Abandon on non-active trail returns ErrInvalidState",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in active state, then complete it.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &testContext{trailsTbl: trailsTbl, trailID: trailID}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// No assertion needed; operation validates the error.
			},
		},

		// --- S10: No orphan crumbs or links after abandonment ---
		{
			name: "No orphan links after abandonment",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumbs and link to trail.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				linkBelongs1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = linksTbl.Set("", linkBelongs1)
				require.NoError(t, err)
				linkBelongs2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = linksTbl.Set("", linkBelongs2)
				require.NoError(t, err)

				// Create child_of link between crumbs.
				linkChild := &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumbID2, ToID: crumbID1}
				_, err = linksTbl.Set("", linkChild)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// No links referencing the crumbs should exist.
				filter1 := types.Filter{"from_id": ctx.crumbID}
				links1, err := ctx.linksTbl.Fetch(filter1)
				require.NoError(t, err)
				assert.Len(t, links1, 0)

				filter2 := types.Filter{"to_id": ctx.crumbID}
				links2, err := ctx.linksTbl.Fetch(filter2)
				require.NoError(t, err)
				assert.Len(t, links2, 0)

				filter3 := types.Filter{"from_id": ctx.crumbID2}
				links3, err := ctx.linksTbl.Fetch(filter3)
				require.NoError(t, err)
				assert.Len(t, links3, 0)

				filter4 := types.Filter{"to_id": ctx.crumbID2}
				links4, err := ctx.linksTbl.Fetch(filter4)
				require.NoError(t, err)
				assert.Len(t, links4, 0)
			},
		},
		{
			name: "Crumb properties deleted on trail abandonment",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumb with properties and link to trail.
				crumb := &types.Crumb{
					Name:       "Test crumb",
					Properties: map[string]any{"test_prop": "test_value"},
				}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *testContext) {
				// Crumb should be deleted (and properties with it).
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},

		// --- branches_from link tests ---
		{
			name: "Create branches_from link indicates branch point",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create parent crumb.
				parentCrumb := &types.Crumb{Name: "Parent task"}
				parentCrumbID, err := crumbsTbl.Set("", parentCrumb)
				require.NoError(t, err)

				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   parentCrumbID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				link := &types.Link{
					LinkType: types.LinkTypeBranchesFrom,
					FromID:   ctx.trailID,
					ToID:     ctx.crumbID,
				}
				linkID, err := ctx.linksTbl.Set("", link)
				require.NoError(t, err)
				ctx.linkID = linkID
			},
			assert: func(t *testing.T, ctx *testContext) {
				got, err := ctx.linksTbl.Get(ctx.linkID)
				require.NoError(t, err)
				link := got.(*types.Link)
				assert.Equal(t, types.LinkTypeBranchesFrom, link.LinkType)
				assert.Equal(t, ctx.trailID, link.FromID)
				assert.Equal(t, ctx.crumbID, link.ToID)
			},
		},
		{
			name: "Query branches_from link to find branch point",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create parent crumb.
				parentCrumb := &types.Crumb{Name: "Parent task"}
				parentCrumbID, err := crumbsTbl.Set("", parentCrumb)
				require.NoError(t, err)

				// Create trail and link via branches_from.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				link := &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trailID, ToID: parentCrumbID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   parentCrumbID,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				// No operation needed; setup is complete.
			},
			assert: func(t *testing.T, ctx *testContext) {
				filter := types.Filter{"link_type": types.LinkTypeBranchesFrom, "from_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 1)
				assert.Equal(t, ctx.crumbID, links[0].(*types.Link).ToID)
			},
		},

		// --- Full workflow ---
		{
			name: "Full trail exploration workflow",
			setup: func(t *testing.T, backend *sqlite.Backend) *testContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				return &testContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
				}
			},
			operation: func(t *testing.T, ctx *testContext) {
				// Create parent crumb in ready state.
				parentCrumb := &types.Crumb{Name: "Parent task"}
				parentCrumbID, err := ctx.crumbsTbl.Set("", parentCrumb)
				require.NoError(t, err)
				got, err := ctx.crumbsTbl.Get(parentCrumbID)
				require.NoError(t, err)
				parentCrumb = got.(*types.Crumb)
				err = parentCrumb.SetState(types.StateReady)
				require.NoError(t, err)
				_, err = ctx.crumbsTbl.Set(parentCrumbID, parentCrumb)
				require.NoError(t, err)

				// Create trail1 in active state.
				trail1 := &types.Trail{CreatedAt: time.Now().UTC()}
				trail1ID, err := ctx.trailsTbl.Set("", trail1)
				require.NoError(t, err)
				got2, err := ctx.trailsTbl.Get(trail1ID)
				require.NoError(t, err)
				trail1 = got2.(*types.Trail)
				trail1.State = types.TrailStateActive
				_, err = ctx.trailsTbl.Set(trail1ID, trail1)
				require.NoError(t, err)

				// Create branches_from link.
				branchLink := &types.Link{LinkType: types.LinkTypeBranchesFrom, FromID: trail1ID, ToID: parentCrumbID}
				_, err = ctx.linksTbl.Set("", branchLink)
				require.NoError(t, err)

				// Create explore_crumb1 and explore_crumb2, link both to trail1.
				explore1 := &types.Crumb{Name: "Explore 1"}
				explore1ID, err := ctx.crumbsTbl.Set("", explore1)
				require.NoError(t, err)
				explore2 := &types.Crumb{Name: "Explore 2"}
				explore2ID, err := ctx.crumbsTbl.Set("", explore2)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: explore1ID, ToID: trail1ID}
				link1ID, err := ctx.linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: explore2ID, ToID: trail1ID}
				_, err = ctx.linksTbl.Set("", link2)
				require.NoError(t, err)

				// Remove explore_crumb1 from trail1.
				err = ctx.linksTbl.Delete(link1ID)
				require.NoError(t, err)

				// Abandon trail1.
				got3, err := ctx.trailsTbl.Get(trail1ID)
				require.NoError(t, err)
				trail1 = got3.(*types.Trail)
				err = trail1.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(trail1ID, trail1)
				require.NoError(t, err)

				// Store IDs for assertions.
				ctx.trailID = trail1ID
				ctx.crumbID = explore1ID
				ctx.crumbID2 = explore2ID

				// Create trail2 in active state.
				trail2 := &types.Trail{CreatedAt: time.Now().UTC()}
				trail2ID, err := ctx.trailsTbl.Set("", trail2)
				require.NoError(t, err)
				got4, err := ctx.trailsTbl.Get(trail2ID)
				require.NoError(t, err)
				trail2 = got4.(*types.Trail)
				trail2.State = types.TrailStateActive
				_, err = ctx.trailsTbl.Set(trail2ID, trail2)
				require.NoError(t, err)

				// Create explore_crumb3, link to trail2.
				explore3 := &types.Crumb{Name: "Explore 3"}
				explore3ID, err := ctx.crumbsTbl.Set("", explore3)
				require.NoError(t, err)
				link3 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: explore3ID, ToID: trail2ID}
				_, err = ctx.linksTbl.Set("", link3)
				require.NoError(t, err)

				// Complete trail2.
				got5, err := ctx.trailsTbl.Get(trail2ID)
				require.NoError(t, err)
				trail2 = got5.(*types.Trail)
				err = trail2.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(trail2ID, trail2)
				require.NoError(t, err)

				ctx.trailID2 = trail2ID
				ctx.crumbID3 = explore3ID
			},
			assert: func(t *testing.T, ctx *testContext) {
				// trail1 state is abandoned.
				got1, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateAbandoned, got1.(*types.Trail).State)

				// trail2 state is completed.
				got2, err := ctx.trailsTbl.Get(ctx.trailID2)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateCompleted, got2.(*types.Trail).State)

				// explore_crumb1 exists (removed before abandon).
				_, err = ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)

				// explore_crumb2 does not exist (was on trail at abandon time).
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				assert.ErrorIs(t, err, types.ErrNotFound)

				// explore_crumb3 exists (completion preserves crumbs).
				_, err = ctx.crumbsTbl.Get(ctx.crumbID3)
				require.NoError(t, err)

				// explore_crumb3 has no belongs_to link.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID3}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := newAttachedBackend(t)
			defer backend.Detach()

			ctx := tt.setup(t, backend)
			tt.operation(t, ctx)
			tt.assert(t, ctx)
		})
	}
}

// testContext holds state shared across setup, operation, and assert phases.
type testContext struct {
	trailsTbl types.Table
	crumbsTbl types.Table
	linksTbl  types.Table
	trailID   string
	trailID2  string
	crumbID   string
	crumbID2  string
	crumbID3  string
	linkID    string
}
