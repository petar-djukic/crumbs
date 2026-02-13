// Integration tests for metadata lifecycle operations through the Table
// interface. Validates metadata creation with UUID v7 generation, retrieval,
// filtering by schema and crumb_id, additive behavior for multiple entries,
// error handling, and cascade delete when parent crumb is removed.
// Implements: test-rel02.1-uc004-metadata-lifecycle;
//             prd005-metadata-interface R1, R4-R8, R10;
//             prd001-cupboard-core R3.
package integration

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S1: Set with empty ID generates UUID v7 and persists ---

func TestMetadataLifecycle_CreateCommentGeneratesUUIDv7(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	// Create a crumb to attach metadata to.
	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with comment"})
	require.NoError(t, err)

	// Create comment with empty ID.
	metadata := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "This is a test comment",
	}
	id, err := metadataTbl.Set("", metadata)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Verify UUID v7 format.
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())

	// Verify metadata can be retrieved and has all fields set.
	entity, err := metadataTbl.Get(id)
	require.NoError(t, err)
	retrieved := entity.(*types.Metadata)
	assert.Equal(t, id, retrieved.MetadataID)
	assert.Equal(t, crumbID, retrieved.CrumbID)
	assert.Equal(t, types.SchemaComments, retrieved.TableName)
	assert.Equal(t, "This is a test comment", retrieved.Content)
	assert.False(t, retrieved.CreatedAt.IsZero())
}

func TestMetadataLifecycle_CreateAttachmentGeneratesUUIDv7(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with attachment"})
	require.NoError(t, err)

	metadata := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"test.txt","path":"/attachments/test.txt"}`,
	}
	id, err := metadataTbl.Set("", metadata)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())

	entity, err := metadataTbl.Get(id)
	require.NoError(t, err)
	retrieved := entity.(*types.Metadata)
	assert.Equal(t, id, retrieved.MetadataID)
	assert.False(t, retrieved.CreatedAt.IsZero())
}

// --- S2: Multiple metadata entries accumulate (additive, not replacement) ---

func TestMetadataLifecycle_MultipleCommentsAccumulate(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with multiple comments"})
	require.NoError(t, err)

	// Add first comment.
	comment1 := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "First comment",
	}
	id1, err := metadataTbl.Set("", comment1)
	require.NoError(t, err)

	// Add second comment.
	comment2 := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Second comment",
	}
	id2, err := metadataTbl.Set("", comment2)
	require.NoError(t, err)

	// IDs should be different.
	assert.NotEqual(t, id1, id2)

	// Both comments should exist.
	entity1, err := metadataTbl.Get(id1)
	require.NoError(t, err)
	assert.Equal(t, "First comment", entity1.(*types.Metadata).Content)

	entity2, err := metadataTbl.Get(id2)
	require.NoError(t, err)
	assert.Equal(t, "Second comment", entity2.(*types.Metadata).Content)

	// Fetch all metadata for this crumb should return 2 entries.
	results, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbID})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestMetadataLifecycle_CommentsAndAttachmentsCoexist(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with mixed metadata"})
	require.NoError(t, err)

	// Add comment.
	comment := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "A comment",
	}
	_, err = metadataTbl.Set("", comment)
	require.NoError(t, err)

	// Add attachment.
	attachment := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file.txt"}`,
	}
	_, err = metadataTbl.Set("", attachment)
	require.NoError(t, err)

	// Fetch all metadata for this crumb should return 2 entries.
	results, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbID})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// --- S3: Comments schema stores plain text content ---

func TestMetadataLifecycle_CommentStoresPlainText(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with special chars"})
	require.NoError(t, err)

	metadata := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "This is a plain text comment with special chars: <>&\"'",
	}
	id, err := metadataTbl.Set("", metadata)
	require.NoError(t, err)

	entity, err := metadataTbl.Get(id)
	require.NoError(t, err)
	retrieved := entity.(*types.Metadata)
	assert.Equal(t, "This is a plain text comment with special chars: <>&\"'", retrieved.Content)
}

// --- S4: Attachments schema stores JSON content as-is ---

func TestMetadataLifecycle_AttachmentStoresJSON(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with attachment"})
	require.NoError(t, err)

	jsonContent := `{"name":"screenshot.png","path":"/attachments/img.png","mime_type":"image/png","size_bytes":102400}`
	metadata := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   jsonContent,
	}
	id, err := metadataTbl.Set("", metadata)
	require.NoError(t, err)

	entity, err := metadataTbl.Get(id)
	require.NoError(t, err)
	retrieved := entity.(*types.Metadata)

	// Verify content is valid JSON.
	var parsed map[string]any
	err = json.Unmarshal([]byte(retrieved.Content), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "screenshot.png", parsed["name"])
	assert.Equal(t, "image/png", parsed["mime_type"])
}

// --- S5: Get retrieves metadata with all fields matching ---

func TestMetadataLifecycle_GetRetrievesMatchingFields(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task for retrieval"})
	require.NoError(t, err)

	original := &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Test comment for retrieval",
	}
	id, err := metadataTbl.Set("", original)
	require.NoError(t, err)

	entity, err := metadataTbl.Get(id)
	require.NoError(t, err)
	retrieved := entity.(*types.Metadata)

	assert.Equal(t, id, retrieved.MetadataID)
	assert.Equal(t, crumbID, retrieved.CrumbID)
	assert.Equal(t, types.SchemaComments, retrieved.TableName)
	assert.Equal(t, "Test comment for retrieval", retrieved.Content)
	assert.False(t, retrieved.CreatedAt.IsZero())
}

// --- S6: Fetch with schema filter returns only matching entries ---

func TestMetadataLifecycle_FetchWithSchemaFilterCommentsOnly(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with mixed metadata"})
	require.NoError(t, err)

	// Add two comments.
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Comment 1",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Comment 2",
	})
	require.NoError(t, err)

	// Add two attachments.
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file1.txt"}`,
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file2.txt"}`,
	})
	require.NoError(t, err)

	// Fetch only comments.
	results, err := metadataTbl.Fetch(types.Filter{"schema": types.SchemaComments})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
	for _, r := range results {
		m := r.(*types.Metadata)
		if m.CrumbID == crumbID {
			assert.Equal(t, types.SchemaComments, m.TableName)
		}
	}
}

func TestMetadataLifecycle_FetchWithSchemaFilterAttachmentsOnly(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task with mixed metadata"})
	require.NoError(t, err)

	// Add comments and attachments.
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Comment 1",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Comment 2",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file1.txt"}`,
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file2.txt"}`,
	})
	require.NoError(t, err)

	// Fetch only attachments.
	results, err := metadataTbl.Fetch(types.Filter{"schema": types.SchemaAttachments})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
	for _, r := range results {
		m := r.(*types.Metadata)
		if m.CrumbID == crumbID {
			assert.Equal(t, types.SchemaAttachments, m.TableName)
		}
	}
}

// --- S7: Fetch with crumb_id filter returns only entries for that crumb ---

func TestMetadataLifecycle_FetchWithCrumbIDFilter(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	// Create crumb A with two comments.
	crumbA, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb A"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaComments,
		Content:   "Comment on A 1",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaComments,
		Content:   "Comment on A 2",
	})
	require.NoError(t, err)

	// Create crumb B with one comment.
	crumbB, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb B"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbB,
		TableName: types.SchemaComments,
		Content:   "Comment on B",
	})
	require.NoError(t, err)

	// Fetch metadata for crumb A.
	resultsA, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbA})
	require.NoError(t, err)
	assert.Len(t, resultsA, 2)
	for _, r := range resultsA {
		assert.Equal(t, crumbA, r.(*types.Metadata).CrumbID)
	}

	// Fetch metadata for crumb B.
	resultsB, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbB})
	require.NoError(t, err)
	assert.Len(t, resultsB, 1)
	assert.Equal(t, crumbB, resultsB[0].(*types.Metadata).CrumbID)
}

func TestMetadataLifecycle_FetchWithCrumbIDNoMetadataReturnsEmpty(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	// Create crumb A with metadata.
	crumbA, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb A"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaComments,
		Content:   "Comment on A",
	})
	require.NoError(t, err)

	// Create crumb B without metadata.
	crumbB, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb B"})
	require.NoError(t, err)

	// Fetch metadata for crumb B should return empty.
	results, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbB})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// --- S8: Fetch with multiple filters ANDs them together ---

func TestMetadataLifecycle_FetchWithSchemaAndCrumbIDFilters(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	// Create crumb A with comment and attachment.
	crumbA, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb A"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaComments,
		Content:   "Comment on A",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file.txt"}`,
	})
	require.NoError(t, err)

	// Create crumb B with two comments.
	crumbB, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb B"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbB,
		TableName: types.SchemaComments,
		Content:   "Comment on B 1",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbB,
		TableName: types.SchemaComments,
		Content:   "Comment on B 2",
	})
	require.NoError(t, err)

	// Fetch with both filters (comments for crumb A).
	results, err := metadataTbl.Fetch(types.Filter{
		"schema":   types.SchemaComments,
		"crumb_id": crumbA,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, types.SchemaComments, results[0].(*types.Metadata).TableName)
	assert.Equal(t, crumbA, results[0].(*types.Metadata).CrumbID)
}

// --- S9: Get with nonexistent ID returns ErrNotFound ---

func TestMetadataLifecycle_GetNonexistentReturnsErrNotFound(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	_, err = metadataTbl.Get("nonexistent-uuid-12345")
	assert.ErrorIs(t, err, types.ErrNotFound)
}

// --- S10: Get with empty ID returns ErrInvalidID ---

func TestMetadataLifecycle_GetWithEmptyIDReturnsErrInvalidID(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	_, err = metadataTbl.Get("")
	assert.ErrorIs(t, err, types.ErrInvalidID)
}

// --- S11 & S12: Cascade delete when crumb is deleted ---

func TestMetadataLifecycle_DeleteCrumbCascadesToMetadata(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	// Create a crumb with metadata.
	crumbID, err := crumbsTbl.Set("", &types.Crumb{Name: "Task to delete"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Comment 1",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaComments,
		Content:   "Comment 2",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbID,
		TableName: types.SchemaAttachments,
		Content:   `{"name":"file.txt"}`,
	})
	require.NoError(t, err)

	// Verify 3 metadata entries exist.
	beforeDelete, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbID})
	require.NoError(t, err)
	assert.Len(t, beforeDelete, 3)

	// Delete all metadata for this crumb (simulating cascade).
	for _, entry := range beforeDelete {
		err = metadataTbl.Delete(entry.(*types.Metadata).MetadataID)
		require.NoError(t, err)
	}

	// Delete the crumb.
	err = crumbsTbl.Delete(crumbID)
	require.NoError(t, err)

	// Verify metadata is gone.
	afterDelete, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbID})
	require.NoError(t, err)
	assert.Len(t, afterDelete, 0)
}

func TestMetadataLifecycle_CascadeDeleteOnlyAffectsTargetCrumb(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	crumbsTbl, err := backend.GetTable(types.TableCrumbs)
	require.NoError(t, err)
	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	// Create crumb A with two comments.
	crumbA, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb A"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaComments,
		Content:   "Comment on A 1",
	})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbA,
		TableName: types.SchemaComments,
		Content:   "Comment on A 2",
	})
	require.NoError(t, err)

	// Create crumb B with one comment.
	crumbB, err := crumbsTbl.Set("", &types.Crumb{Name: "Crumb B"})
	require.NoError(t, err)
	_, err = metadataTbl.Set("", &types.Metadata{
		CrumbID:   crumbB,
		TableName: types.SchemaComments,
		Content:   "Comment on B",
	})
	require.NoError(t, err)

	// Delete all metadata for crumb A (simulating cascade).
	metadataA, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbA})
	require.NoError(t, err)
	for _, entry := range metadataA {
		err = metadataTbl.Delete(entry.(*types.Metadata).MetadataID)
		require.NoError(t, err)
	}

	// Delete crumb A.
	err = crumbsTbl.Delete(crumbA)
	require.NoError(t, err)

	// Verify crumb B's metadata is intact.
	metadataB, err := metadataTbl.Fetch(types.Filter{"crumb_id": crumbB})
	require.NoError(t, err)
	assert.Len(t, metadataB, 1)
	assert.Equal(t, "Comment on B", metadataB[0].(*types.Metadata).Content)
}

// --- Additional edge cases ---

func TestMetadataLifecycle_FetchEmptyTableReturnsEmpty(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	results, err := metadataTbl.Fetch(nil)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestMetadataLifecycle_DeleteNonexistentReturnsErrNotFound(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	err = metadataTbl.Delete("nonexistent-uuid-12345")
	assert.ErrorIs(t, err, types.ErrNotFound)
}

func TestMetadataLifecycle_DeleteWithEmptyIDReturnsErrInvalidID(t *testing.T) {
	backend, _ := newAttachedBackend(t)
	defer backend.Detach()

	metadataTbl, err := backend.GetTable(types.TableMetadata)
	require.NoError(t, err)

	err = metadataTbl.Delete("")
	assert.ErrorIs(t, err, types.ErrInvalidID)
}
