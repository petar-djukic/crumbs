// Shared test helpers for integration tests.
package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/stretchr/testify/require"
)

// createTestStash creates a stash with the specified type and optional name.
// If no name is provided, generates a unique name.
// Returns the created *types.Stash with StashID, Version, and CreatedAt populated.
// Uses require.NoError to fail fast on any errors.
func createTestStash(t *testing.T, table types.Table, stashType string, name ...string) *types.Stash {
	t.Helper()

	// Generate unique name if not provided.
	var stashName string
	if len(name) > 0 && name[0] != "" {
		stashName = name[0]
	} else {
		// Use UUID to generate unique stash name to avoid duplicate name errors.
		stashName = "stash-" + uuid.NewString()
	}

	// Initialize stash with appropriate default value based on type.
	var value any
	switch stashType {
	case types.StashTypeResource:
		value = map[string]any{"uri": "https://example.com", "kind": "http"}
	case types.StashTypeArtifact:
		value = map[string]any{"path": "/tmp/artifact", "producer": "test"}
	case types.StashTypeContext:
		value = map[string]any{"key": "value"}
	case types.StashTypeCounter:
		value = map[string]any{"value": int64(0)}
	case types.StashTypeLock:
		value = nil
	default:
		// Default to context type with empty map if unknown type.
		value = map[string]any{}
	}

	stash := &types.Stash{
		Name:      stashName,
		StashType: stashType,
		Value:     value,
	}

	id, err := table.Set("", stash)
	require.NoError(t, err, "Failed to create test stash")

	got, err := table.Get(id)
	require.NoError(t, err, "Failed to retrieve created stash")

	return got.(*types.Stash)
}
