// Integration tests for trail-crumb lifecycle control and cascade operations.
// Validates trail as a container for crumbs, lifecycle coupling (complete makes
// crumbs permanent, abandon deletes crumbs), cardinality constraints, and query
// patterns for trail membership.
// Implements: test-rel03.0-uc004-trail-crumb-lifecycle;
//             prd006-trails-interface R5-R6 (Complete, Abandon, cascade);
//             prd007-links-interface R2.1, R6.1 (belongs_to links, cardinality);
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

func TestTrailCrumbLifecycle(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, backend *sqlite.Backend) *lifecycleContext
		operation func(t *testing.T, ctx *lifecycleContext)
		assert    func(t *testing.T, ctx *lifecycleContext)
	}{
		// --- S1: Crumbs associate with trails via belongs_to links ---

		{
			name: "Create belongs_to link associates crumb with trail",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				link := &types.Link{
					LinkType: types.LinkTypeBelongsTo,
					FromID:   ctx.crumbID,
					ToID:     ctx.trailID,
				}
				linkID, err := ctx.linksTbl.Set("", link)
				require.NoError(t, err)
				ctx.linkID = linkID
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.linksTbl.Get(ctx.linkID)
				require.NoError(t, err)
				link := got.(*types.Link)
				assert.Equal(t, types.LinkTypeBelongsTo, link.LinkType)
				assert.Equal(t, ctx.crumbID, link.FromID)
				assert.Equal(t, ctx.trailID, link.ToID)
			},
		},

		{
			name: "Crumb without belongs_to link is orphaned or permanent",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create crumb without link.
				crumb := &types.Crumb{Name: "Orphaned crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)

				return &lifecycleContext{
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// No operation needed.
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		// --- S2: Cardinality enforcement - crumb belongs to at most one trail ---

		{
			name: "Crumb can belong to only one trail",
			// SKIP: Cardinality constraint not yet enforced in schema.
			// TODO: Add UNIQUE constraint on (link_type, from_id) for belongs_to links.
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				t.Skip("Cardinality constraint not implemented yet")
				return nil
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create two trails in active state.
				trail1 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID1, err := trailsTbl.Set("", trail1)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID1)
				require.NoError(t, err)
				trail1 = got.(*types.Trail)
				trail1.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID1, trail1)
				require.NoError(t, err)

				trail2 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID2, err := trailsTbl.Set("", trail2)
				require.NoError(t, err)
				got, err = trailsTbl.Get(trailID2)
				require.NoError(t, err)
				trail2 = got.(*types.Trail)
				trail2.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID2, trail2)
				require.NoError(t, err)

				// Create crumb and link to trail1.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID1}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID1,
					trailID2:  trailID2,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// Attempt to create second belongs_to link for same crumb.
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: ctx.crumbID, ToID: ctx.trailID2}
				_, err := ctx.linksTbl.Set("", link)
				assert.Error(t, err, "should fail due to uniqueness constraint")
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Verify only one belongs_to link exists for crumb.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 1)
			},
		},

		{
			name: "Moving crumb requires deleting old link first",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create two trails.
				trail1 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID1, err := trailsTbl.Set("", trail1)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID1)
				require.NoError(t, err)
				trail1 = got.(*types.Trail)
				trail1.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID1, trail1)
				require.NoError(t, err)

				trail2 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID2, err := trailsTbl.Set("", trail2)
				require.NoError(t, err)
				got, err = trailsTbl.Get(trailID2)
				require.NoError(t, err)
				trail2 = got.(*types.Trail)
				trail2.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID2, trail2)
				require.NoError(t, err)

				// Create crumb and link to trail1.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID1}
				linkID, err := linksTbl.Set("", link)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID1,
					trailID2:  trailID2,
					crumbID:   crumbID,
					linkID:    linkID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// Delete old link, create new one.
				err := ctx.linksTbl.Delete(ctx.linkID)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: ctx.crumbID, ToID: ctx.trailID2}
				_, err = ctx.linksTbl.Set("", link)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 1)
				assert.Equal(t, ctx.trailID2, links[0].(*types.Link).ToID)
			},
		},

		// --- S3: Fetch crumbs by trail membership ---

		{
			name: "Fetch all crumbs belonging to a trail",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				// Create three crumbs.
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
					crumbID3:  crumbID3,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// No operation needed.
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 2)

				// Verify crumb1 and crumb2 are in results, crumb3 is not.
				fromIDs := make(map[string]bool)
				for _, l := range links {
					fromIDs[l.(*types.Link).FromID] = true
				}
				assert.True(t, fromIDs[ctx.crumbID])
				assert.True(t, fromIDs[ctx.crumbID2])
				assert.False(t, fromIDs[ctx.crumbID3])
			},
		},

		{
			name: "Fetch returns empty for trail with no crumbs",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create trail in active state (no crumbs).
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// No operation needed.
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		{
			name: "Fetch crumbs across multiple trails",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create two trails.
				trail1 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID1, err := trailsTbl.Set("", trail1)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID1)
				require.NoError(t, err)
				trail1 = got.(*types.Trail)
				trail1.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID1, trail1)
				require.NoError(t, err)

				trail2 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID2, err := trailsTbl.Set("", trail2)
				require.NoError(t, err)
				got, err = trailsTbl.Get(trailID2)
				require.NoError(t, err)
				trail2 = got.(*types.Trail)
				trail2.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID2, trail2)
				require.NoError(t, err)

				// Create crumbs and link to trails.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID1}
				_, err = linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID2}
				_, err = linksTbl.Set("", link2)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID1,
					trailID2:  trailID2,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// No operation needed.
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				filter1 := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links1, err := ctx.linksTbl.Fetch(filter1)
				require.NoError(t, err)
				assert.Len(t, links1, 1)
				assert.Equal(t, ctx.crumbID, links1[0].(*types.Link).FromID)

				filter2 := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID2}
				links2, err := ctx.linksTbl.Fetch(filter2)
				require.NoError(t, err)
				assert.Len(t, links2, 1)
				assert.Equal(t, ctx.crumbID2, links2[0].(*types.Link).FromID)
			},
		},

		// --- S4: Move crumb between trails ---

		{
			name: "Move crumb from one trail to another",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create two trails.
				trail1 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID1, err := trailsTbl.Set("", trail1)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID1)
				require.NoError(t, err)
				trail1 = got.(*types.Trail)
				trail1.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID1, trail1)
				require.NoError(t, err)

				trail2 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID2, err := trailsTbl.Set("", trail2)
				require.NoError(t, err)
				got, err = trailsTbl.Get(trailID2)
				require.NoError(t, err)
				trail2 = got.(*types.Trail)
				trail2.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID2, trail2)
				require.NoError(t, err)

				// Create crumb and link to trail1.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID1}
				linkID, err := linksTbl.Set("", link)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID1,
					trailID2:  trailID2,
					crumbID:   crumbID,
					linkID:    linkID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// Delete old link, create new one.
				err := ctx.linksTbl.Delete(ctx.linkID)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: ctx.crumbID, ToID: ctx.trailID2}
				_, err = ctx.linksTbl.Set("", link)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Verify crumb on trail2, not trail1.
				filter1 := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links1, err := ctx.linksTbl.Fetch(filter1)
				require.NoError(t, err)
				assert.Len(t, links1, 0)

				filter2 := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID2}
				links2, err := ctx.linksTbl.Fetch(filter2)
				require.NoError(t, err)
				assert.Len(t, links2, 1)
			},
		},

		{
			name: "Crumb data unchanged after moving trails",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create two trails.
				trail1 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID1, err := trailsTbl.Set("", trail1)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID1)
				require.NoError(t, err)
				trail1 = got.(*types.Trail)
				trail1.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID1, trail1)
				require.NoError(t, err)

				trail2 := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID2, err := trailsTbl.Set("", trail2)
				require.NoError(t, err)
				got, err = trailsTbl.Get(trailID2)
				require.NoError(t, err)
				trail2 = got.(*types.Trail)
				trail2.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID2, trail2)
				require.NoError(t, err)

				// Create crumb with name and state, link to trail1.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				got, err = crumbsTbl.Get(crumbID)
				require.NoError(t, err)
				crumb = got.(*types.Crumb)
				err = crumb.SetState(types.StateReady)
				require.NoError(t, err)
				_, err = crumbsTbl.Set(crumbID, crumb)
				require.NoError(t, err)

				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID1}
				linkID, err := linksTbl.Set("", link)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID1,
					trailID2:  trailID2,
					crumbID:   crumbID,
					linkID:    linkID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// Delete old link, create new one.
				err := ctx.linksTbl.Delete(ctx.linkID)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: ctx.crumbID, ToID: ctx.trailID2}
				_, err = ctx.linksTbl.Set("", link)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Verify crumb data unchanged.
				got, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)
				crumb := got.(*types.Crumb)
				assert.Equal(t, "Test crumb", crumb.Name)
				assert.Equal(t, types.StateReady, crumb.State)
			},
		},

		// --- S5: Complete removes belongs_to links ---

		{
			name: "Complete trail removes all belongs_to links",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateCompleted, got.(*types.Trail).State)

				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		{
			name: "Complete sets CompletedAt timestamp",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateCompleted, trail.State)
				assert.NotNil(t, trail.CompletedAt)
			},
		},

		// --- S6: Crumbs persist after completion ---

		{
			name: "Crumbs exist after trail completion",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Both crumbs should still exist.
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				require.NoError(t, err)
			},
		},

		{
			name: "Completed crumbs have no belongs_to link",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		{
			name: "Permanent crumbs indistinguishable from never-trailed crumbs",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				// Create standalone crumb (never linked to trail).
				crumbStandalone := &types.Crumb{Name: "Standalone crumb"}
				crumbIDStandalone, err := crumbsTbl.Set("", crumbStandalone)
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

				// Create crumb_completed, link to trail.
				crumbCompleted := &types.Crumb{Name: "Completed crumb"}
				crumbIDCompleted, err := crumbsTbl.Set("", crumbCompleted)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbIDCompleted, ToID: trailID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				// Complete trail.
				got, err = trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbIDStandalone,
					crumbID2:  crumbIDCompleted,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// No operation needed.
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Both crumbs should exist.
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				require.NoError(t, err)

				// Both should have no belongs_to link.
				filter1 := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID}
				links1, err := ctx.linksTbl.Fetch(filter1)
				require.NoError(t, err)
				assert.Len(t, links1, 0)

				filter2 := types.Filter{"link_type": types.LinkTypeBelongsTo, "from_id": ctx.crumbID2}
				links2, err := ctx.linksTbl.Fetch(filter2)
				require.NoError(t, err)
				assert.Len(t, links2, 0)
			},
		},

		// --- S7: Abandon deletes all crumbs belonging to trail ---

		{
			name: "Abandon trail deletes associated crumbs",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
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
			name: "Abandon only deletes crumbs with belongs_to link",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				// Create crumb_linked and link to trail.
				crumbLinked := &types.Crumb{Name: "Linked crumb"}
				crumbIDLinked, err := crumbsTbl.Set("", crumbLinked)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbIDLinked, ToID: trailID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				// Create crumb_unlinked (no belongs_to).
				crumbUnlinked := &types.Crumb{Name: "Unlinked crumb"}
				crumbIDUnlinked, err := crumbsTbl.Set("", crumbUnlinked)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbIDLinked,
					crumbID2:  crumbIDUnlinked,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Linked crumb should be deleted.
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				assert.ErrorIs(t, err, types.ErrNotFound)

				// Unlinked crumb should still exist.
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				require.NoError(t, err)
			},
		},

		{
			name: "Crumb removed from trail before abandon survives",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				// Create crumbs and link both to trail.
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
					linkID:    linkID1,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
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
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Crumb1 should still exist (removed before abandon).
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)

				// Crumb2 should be deleted (was on trail at abandon time).
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},

		// --- S8: Cascade deletion cleans up properties, metadata, links ---

		{
			name: "Abandon deletes crumb properties",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Crumb should be deleted (and properties with it).
				_, err := ctx.crumbsTbl.Get(ctx.crumbID)
				assert.ErrorIs(t, err, types.ErrNotFound)
			},
		},

		{
			name: "Abandon deletes crumb metadata",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)
				metadataTbl, err := backend.GetTable(types.TableMetadata)
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

				// Create crumb with metadata and link to trail.
				crumb := &types.Crumb{Name: "Test crumb"}
				crumbID, err := crumbsTbl.Set("", crumb)
				require.NoError(t, err)
				metadata := &types.Metadata{
					CrumbID:   crumbID,
					TableName: types.SchemaComments,
					Content:   "test comment content",
				}
				_, err = metadataTbl.Set("", metadata)
				require.NoError(t, err)
				link := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID, ToID: trailID}
				_, err = linksTbl.Set("", link)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl:   trailsTbl,
					crumbsTbl:   crumbsTbl,
					linksTbl:    linksTbl,
					metadataTbl: metadataTbl,
					trailID:     trailID,
					crumbID:     crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Metadata should be deleted.
				filter := types.Filter{"crumb_id": ctx.crumbID}
				metadata, err := ctx.metadataTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, metadata, 0)
			},
		},

		{
			name: "Abandon deletes all links involving crumb",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				// Create parent_crumb and child_crumb, link both to trail.
				parentCrumb := &types.Crumb{Name: "Parent crumb"}
				parentCrumbID, err := crumbsTbl.Set("", parentCrumb)
				require.NoError(t, err)
				childCrumb := &types.Crumb{Name: "Child crumb"}
				childCrumbID, err := crumbsTbl.Set("", childCrumb)
				require.NoError(t, err)

				linkBelongs1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: parentCrumbID, ToID: trailID}
				_, err = linksTbl.Set("", linkBelongs1)
				require.NoError(t, err)
				linkBelongs2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: childCrumbID, ToID: trailID}
				_, err = linksTbl.Set("", linkBelongs2)
				require.NoError(t, err)

				// Create child_of link.
				linkChild := &types.Link{LinkType: types.LinkTypeChildOf, FromID: childCrumbID, ToID: parentCrumbID}
				_, err = linksTbl.Set("", linkChild)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   parentCrumbID,
					crumbID2:  childCrumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// All child_of links should be deleted.
				filter := types.Filter{"link_type": types.LinkTypeChildOf}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		{
			name: "Abandon removes belongs_to links for trail",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No belongs_to links should remain for the trail.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		// --- S9: Trail remains in database after abandonment ---

		{
			name: "Abandoned trail still retrievable",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateAbandoned, trail.State)
				assert.NotNil(t, trail.CompletedAt)
			},
		},

		{
			name: "Abandoned trail has no associated crumbs",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
					trailID:   trailID,
					crumbID:   crumbID1,
					crumbID2:  crumbID2,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(ctx.trailID, trail)
				require.NoError(t, err)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateAbandoned, got.(*types.Trail).State)

				// No associated crumbs.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		// --- S10: State validation for Complete and Abandon ---

		{
			name: "Complete requires active state",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No assertion needed; operation validates the error.
			},
		},

		{
			name: "Complete on pending state returns error",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in pending state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStatePending
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No assertion needed.
			},
		},

		{
			name: "Complete on completed state returns error",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Complete()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No assertion needed.
			},
		},

		{
			name: "Abandon requires active state",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No assertion needed.
			},
		},

		{
			name: "Abandon on completed state returns error",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
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

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No assertion needed.
			},
		},

		{
			name: "Abandon on abandoned state returns error",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)

				// Create trail in active state, then abandon it.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					trailID:   trailID,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				err = trail.Abandon()
				assert.ErrorIs(t, err, types.ErrInvalidState)
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// No assertion needed.
			},
		},

		// --- Full lifecycle workflows ---

		{
			name: "Complete lifecycle - trail as container with completion",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// Create trail in draft state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := ctx.trailsTbl.Set("", trail)
				require.NoError(t, err)

				// Transition to active.
				got, err := ctx.trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = ctx.trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumbs and link to trail.
				crumb1 := &types.Crumb{Name: "Crumb 1"}
				crumbID1, err := ctx.crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{Name: "Crumb 2"}
				crumbID2, err := ctx.crumbsTbl.Set("", crumb2)
				require.NoError(t, err)
				crumb3 := &types.Crumb{Name: "Crumb 3"}
				crumbID3, err := ctx.crumbsTbl.Set("", crumb3)
				require.NoError(t, err)

				link1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = ctx.linksTbl.Set("", link1)
				require.NoError(t, err)
				link2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = ctx.linksTbl.Set("", link2)
				require.NoError(t, err)
				link3 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID3, ToID: trailID}
				_, err = ctx.linksTbl.Set("", link3)
				require.NoError(t, err)

				// Verify 3 crumbs on trail.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				require.Len(t, links, 3)

				// Complete trail.
				got, err = ctx.trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				err = trail.Complete()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Store IDs for assertions.
				ctx.trailID = trailID
				ctx.crumbID = crumbID1
				ctx.crumbID2 = crumbID2
				ctx.crumbID3 = crumbID3
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Trail should be completed.
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				assert.Equal(t, types.TrailStateCompleted, got.(*types.Trail).State)

				// All 3 crumbs should still exist.
				_, err = ctx.crumbsTbl.Get(ctx.crumbID)
				require.NoError(t, err)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				require.NoError(t, err)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID3)
				require.NoError(t, err)

				// No belongs_to links should remain.
				filter := types.Filter{"link_type": types.LinkTypeBelongsTo, "to_id": ctx.trailID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)
			},
		},

		{
			name: "Abandon lifecycle - trail as container with cleanup",
			setup: func(t *testing.T, backend *sqlite.Backend) *lifecycleContext {
				trailsTbl, err := backend.GetTable(types.TableTrails)
				require.NoError(t, err)
				crumbsTbl, err := backend.GetTable(types.TableCrumbs)
				require.NoError(t, err)
				linksTbl, err := backend.GetTable(types.TableLinks)
				require.NoError(t, err)

				return &lifecycleContext{
					trailsTbl: trailsTbl,
					crumbsTbl: crumbsTbl,
					linksTbl:  linksTbl,
				}
			},
			operation: func(t *testing.T, ctx *lifecycleContext) {
				// Create trail in active state.
				trail := &types.Trail{CreatedAt: time.Now().UTC()}
				trailID, err := ctx.trailsTbl.Set("", trail)
				require.NoError(t, err)
				got, err := ctx.trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				trail.State = types.TrailStateActive
				_, err = ctx.trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Create crumbs with properties.
				crumb1 := &types.Crumb{
					Name:       "Crumb 1",
					Properties: map[string]any{"prop1": "value1"},
				}
				crumbID1, err := ctx.crumbsTbl.Set("", crumb1)
				require.NoError(t, err)
				crumb2 := &types.Crumb{
					Name:       "Crumb 2",
					Properties: map[string]any{"prop2": "value2"},
				}
				crumbID2, err := ctx.crumbsTbl.Set("", crumb2)
				require.NoError(t, err)

				// Link crumbs to trail.
				linkBelongs1 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID1, ToID: trailID}
				_, err = ctx.linksTbl.Set("", linkBelongs1)
				require.NoError(t, err)
				linkBelongs2 := &types.Link{LinkType: types.LinkTypeBelongsTo, FromID: crumbID2, ToID: trailID}
				_, err = ctx.linksTbl.Set("", linkBelongs2)
				require.NoError(t, err)

				// Create child_of link between crumbs.
				linkChild := &types.Link{LinkType: types.LinkTypeChildOf, FromID: crumbID2, ToID: crumbID1}
				_, err = ctx.linksTbl.Set("", linkChild)
				require.NoError(t, err)

				// Abandon trail.
				got, err = ctx.trailsTbl.Get(trailID)
				require.NoError(t, err)
				trail = got.(*types.Trail)
				err = trail.Abandon()
				require.NoError(t, err)
				_, err = ctx.trailsTbl.Set(trailID, trail)
				require.NoError(t, err)

				// Store IDs for assertions.
				ctx.trailID = trailID
				ctx.crumbID = crumbID1
				ctx.crumbID2 = crumbID2
			},
			assert: func(t *testing.T, ctx *lifecycleContext) {
				// Trail should exist and be abandoned.
				got, err := ctx.trailsTbl.Get(ctx.trailID)
				require.NoError(t, err)
				trail := got.(*types.Trail)
				assert.Equal(t, types.TrailStateAbandoned, trail.State)

				// Crumbs should be deleted.
				_, err = ctx.crumbsTbl.Get(ctx.crumbID)
				assert.ErrorIs(t, err, types.ErrNotFound)
				_, err = ctx.crumbsTbl.Get(ctx.crumbID2)
				assert.ErrorIs(t, err, types.ErrNotFound)

				// No orphan links should remain.
				filter := types.Filter{"from_id": ctx.crumbID}
				links, err := ctx.linksTbl.Fetch(filter)
				require.NoError(t, err)
				assert.Len(t, links, 0)

				filter = types.Filter{"from_id": ctx.crumbID2}
				links, err = ctx.linksTbl.Fetch(filter)
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

// lifecycleContext holds state shared across setup, operation, and assert phases.
type lifecycleContext struct {
	trailsTbl   types.Table
	crumbsTbl   types.Table
	linksTbl    types.Table
	metadataTbl types.Table
	trailID     string
	trailID2    string
	crumbID     string
	crumbID2    string
	crumbID3    string
	linkID      string
}
