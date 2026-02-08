// Go API integration tests for stash operations.
// Validates test-rel03.0-uc003-stash-operations.yaml test cases.
// Implements: docs/specs/test-suites/test-rel03.0-uc003-stash-operations.yaml;
//
//	docs/specs/use-cases/rel03.0-uc003-stash-operations.yaml;
//	prd008-stash-interface R1-R12.
package integration

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// --- Test Helpers ---

// isUUIDv7Stash validates that the given string matches UUID v7 format.
func isUUIDv7Stash(id string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(id))
}

// setupStashTest creates a cupboard with SQLite backend and returns the
// cupboard and stashes table for testing.
func setupStashTest(t *testing.T) (*sqlite.Backend, types.Table, func()) {
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

	stashesTable, err := cupboard.GetTable(types.StashesTable)
	if err != nil {
		t.Fatalf("GetTable(stashes) failed: %v", err)
	}

	cleanup := func() {
		cupboard.Detach()
	}

	return cupboard, stashesTable, cleanup
}

// --- S1: Stash created via Table.Set generates UUID v7 for StashID ---

func TestStashOperations_S1_CreateStashGeneratesUUIDv7(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "test-stash",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "value"},
		Version:   1,
	}

	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if id == "" {
		t.Error("Set should return generated ID")
	}
	if !isUUIDv7Stash(id) {
		t.Errorf("ID %q is not a valid UUID v7", id)
	}
	if stash.StashID != id {
		t.Errorf("Stash.StashID should be set to %q, got %q", id, stash.StashID)
	}
	if stash.CreatedAt.IsZero() {
		t.Error("Stash.CreatedAt should be set on creation")
	}
}

func TestStashOperations_S1_SetWithExistingIDUpdatesStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "update-test",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"original": true},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Update the stash value
	stash.Value = map[string]any{"updated": true}
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Update stash failed: %v", err)
	}

	// Verify update
	entity, err := stashesTable.Get(stashID)
	if err != nil {
		t.Fatalf("Get stash failed: %v", err)
	}
	updated := entity.(*types.Stash)
	value, ok := updated.Value.(map[string]any)
	if !ok {
		t.Fatalf("Value type assertion failed")
	}
	if _, exists := value["updated"]; !exists {
		t.Error("Value should have 'updated' key after update")
	}
}

// --- S2: All five stash types can be created ---

func TestStashOperations_S2_CreateResourceStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "api-connection",
		StashType: types.StashTypeResource,
		Value:     map[string]any{"uri": "https://api.example.com", "kind": "http"},
		Version:   1,
	}

	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entity, err := stashesTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.StashType != types.StashTypeResource {
		t.Errorf("StashType = %q, want %q", retrieved.StashType, types.StashTypeResource)
	}
}

func TestStashOperations_S2_CreateArtifactStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "build-output",
		StashType: types.StashTypeArtifact,
		Value:     map[string]any{"path": "/tmp/build.zip", "producer": "build-agent"},
		Version:   1,
	}

	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entity, err := stashesTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.StashType != types.StashTypeArtifact {
		t.Errorf("StashType = %q, want %q", retrieved.StashType, types.StashTypeArtifact)
	}
}

func TestStashOperations_S2_CreateContextStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "shared-config",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"timeout": 30, "retries": 3},
		Version:   1,
	}

	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entity, err := stashesTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.StashType != types.StashTypeContext {
		t.Errorf("StashType = %q, want %q", retrieved.StashType, types.StashTypeContext)
	}
}

func TestStashOperations_S2_CreateCounterStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "request-counter",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
		Version:   1,
	}

	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entity, err := stashesTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.StashType != types.StashTypeCounter {
		t.Errorf("StashType = %q, want %q", retrieved.StashType, types.StashTypeCounter)
	}
}

func TestStashOperations_S2_CreateLockStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "resource-lock",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}

	id, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entity, err := stashesTable.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.StashType != types.StashTypeLock {
		t.Errorf("StashType = %q, want %q", retrieved.StashType, types.StashTypeLock)
	}
	if retrieved.Value != nil {
		t.Errorf("Lock stash Value should be nil, got %v", retrieved.Value)
	}
}

// --- S3: SetValue updates value and increments Version ---

func TestStashOperations_S3_SetValueUpdatesValueAndVersion(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "setvalue-test",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"timeout": 30},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// SetValue should update and increment version
	err = stash.SetValue(map[string]any{"timeout": 60})
	if err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}

	if stash.Version != 2 {
		t.Errorf("Version = %d, want 2", stash.Version)
	}

	// Persist and verify
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}

	entity, err := stashesTable.Get(stashID)
	if err != nil {
		t.Fatalf("Get stash failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	value := retrieved.Value.(map[string]any)
	timeout, ok := value["timeout"].(float64) // JSON numbers are float64
	if !ok {
		t.Fatalf("timeout type assertion failed")
	}
	if timeout != 60 {
		t.Errorf("timeout = %v, want 60", timeout)
	}
}

func TestStashOperations_S3_SetValueOnLockReturnsErrInvalidStashType(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-setvalue-test",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	err = stash.SetValue(map[string]any{"invalid": true})
	if err != types.ErrInvalidStashType {
		t.Errorf("SetValue on lock expected ErrInvalidStashType, got %v", err)
	}
}

// --- S4: GetValue returns current value ---

func TestStashOperations_S4_GetValueReturnsCurrent(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "getvalue-test",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "original"},
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	value := stash.GetValue()
	if value == nil {
		t.Fatal("GetValue returned nil")
	}
	v := value.(map[string]any)
	if v["key"] != "original" {
		t.Errorf("GetValue key = %v, want 'original'", v["key"])
	}
}

func TestStashOperations_S4_GetValueReturnsNilForEmpty(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "empty-value-test",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	value := stash.GetValue()
	if value != nil {
		t.Errorf("GetValue expected nil for empty lock, got %v", value)
	}
}

// --- S5, S6: Increment adds delta to counter value ---

func TestStashOperations_S5_IncrementWithPositiveDelta(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "counter-positive",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	newVal, err := stash.Increment(5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	if newVal != 5 {
		t.Errorf("Increment returned %d, want 5", newVal)
	}
	if stash.Version != 2 {
		t.Errorf("Version = %d, want 2", stash.Version)
	}

	// Persist and verify
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}
}

func TestStashOperations_S6_IncrementWithNegativeDelta(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "counter-negative",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(10)},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	newVal, err := stash.Increment(-3)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	if newVal != 7 {
		t.Errorf("Increment returned %d, want 7", newVal)
	}

	// Persist and verify
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}
}

func TestStashOperations_S5_MultipleIncrementsAccumulate(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "counter-accumulate",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Increment 10
	val, err := stash.Increment(10)
	if err != nil {
		t.Fatalf("Increment(10) failed: %v", err)
	}
	if val != 10 {
		t.Errorf("After +10: got %d, want 10", val)
	}

	// Increment -3
	val, err = stash.Increment(-3)
	if err != nil {
		t.Fatalf("Increment(-3) failed: %v", err)
	}
	if val != 7 {
		t.Errorf("After -3: got %d, want 7", val)
	}

	// Increment 5
	val, err = stash.Increment(5)
	if err != nil {
		t.Fatalf("Increment(5) failed: %v", err)
	}
	if val != 12 {
		t.Errorf("After +5: got %d, want 12", val)
	}

	if stash.Version != 4 {
		t.Errorf("Version = %d, want 4 (1 initial + 3 increments)", stash.Version)
	}

	// Persist and verify
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}
}

func TestStashOperations_S5_IncrementOnNonCounterReturnsErrInvalidStashType(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "context-increment",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "value"},
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	_, err = stash.Increment(1)
	if err != types.ErrInvalidStashType {
		t.Errorf("Increment on context expected ErrInvalidStashType, got %v", err)
	}
}

// --- S7, S8, S9: Lock operations ---

func TestStashOperations_S7_AcquireLockSetsHolder(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-acquire",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	err = stash.Acquire("worker-1")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	if stash.Version != 2 {
		t.Errorf("Version = %d, want 2", stash.Version)
	}

	// Verify holder is set
	lockData, ok := stash.Value.(map[string]any)
	if !ok {
		t.Fatal("Lock value should be a map")
	}
	if lockData["holder"] != "worker-1" {
		t.Errorf("holder = %v, want 'worker-1'", lockData["holder"])
	}
	if lockData["acquired_at"] == nil {
		t.Error("acquired_at should be set")
	}

	// Persist
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}
}

func TestStashOperations_S8_AcquireBySameHolderSucceeds(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-reentrant",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// First acquire
	err = stash.Acquire("worker-1")
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	// Reentrant acquire by same holder
	err = stash.Acquire("worker-1")
	if err != nil {
		t.Errorf("Reentrant acquire should succeed, got error: %v", err)
	}

	// Holder should still be worker-1
	lockData := stash.Value.(map[string]any)
	if lockData["holder"] != "worker-1" {
		t.Errorf("holder = %v, want 'worker-1'", lockData["holder"])
	}
}

func TestStashOperations_S7_AcquireByDifferentHolderReturnsErrLockHeld(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-contention",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// First acquire by worker-1
	err = stash.Acquire("worker-1")
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	// Contending acquire by worker-2
	err = stash.Acquire("worker-2")
	if err != types.ErrLockHeld {
		t.Errorf("Acquire by different holder expected ErrLockHeld, got %v", err)
	}
}

func TestStashOperations_S7_AcquireWithEmptyHolderReturnsErrInvalidHolder(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-empty-holder",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	err = stash.Acquire("")
	if err != types.ErrInvalidHolder {
		t.Errorf("Acquire with empty holder expected ErrInvalidHolder, got %v", err)
	}
}

func TestStashOperations_S7_AcquireOnNonLockReturnsErrInvalidStashType(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "context-acquire",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "value"},
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	err = stash.Acquire("worker-1")
	if err != types.ErrInvalidStashType {
		t.Errorf("Acquire on context expected ErrInvalidStashType, got %v", err)
	}
}

func TestStashOperations_S9_ReleaseLockClearsValue(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-release",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Acquire
	err = stash.Acquire("worker-1")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	versionAfterAcquire := stash.Version

	// Release
	err = stash.Release("worker-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if stash.Value != nil {
		t.Errorf("Value should be nil after release, got %v", stash.Value)
	}
	if stash.Version != versionAfterAcquire+1 {
		t.Errorf("Version = %d, want %d", stash.Version, versionAfterAcquire+1)
	}

	// Persist
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}
}

func TestStashOperations_S9_ReleaseByWrongHolderReturnsErrNotLockHolder(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-wrong-release",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Acquire by worker-1
	err = stash.Acquire("worker-1")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Release by worker-2
	err = stash.Release("worker-2")
	if err != types.ErrNotLockHolder {
		t.Errorf("Release by wrong holder expected ErrNotLockHolder, got %v", err)
	}
}

func TestStashOperations_S9_ReleaseOnUnheldLockReturnsErrNotLockHolder(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "lock-unheld-release",
		StashType: types.StashTypeLock,
		Value:     nil,
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	err = stash.Release("worker-1")
	if err != types.ErrNotLockHolder {
		t.Errorf("Release on unheld lock expected ErrNotLockHolder, got %v", err)
	}
}

func TestStashOperations_S9_ReleaseOnNonLockReturnsErrInvalidStashType(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "context-release",
		StashType: types.StashTypeContext,
		Value:     map[string]any{"key": "value"},
		Version:   1,
	}
	_, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	err = stash.Release("worker-1")
	if err != types.ErrInvalidStashType {
		t.Errorf("Release on context expected ErrInvalidStashType, got %v", err)
	}
}

// --- S10: Version tracking ---

func TestStashOperations_S10_VersionStartsAtOne(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "version-test",
		StashType: types.StashTypeContext,
		Value:     map[string]any{},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	entity, err := stashesTable.Get(stashID)
	if err != nil {
		t.Fatalf("Get stash failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.Version != 1 {
		t.Errorf("Version = %d, want 1", retrieved.Version)
	}
}

func TestStashOperations_S10_VersionIncrementsOnEachMutation(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "version-increment-test",
		StashType: types.StashTypeCounter,
		Value:     map[string]any{"value": int64(0)},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Three increments = three version bumps
	stash.Increment(1)
	stash.Increment(1)
	stash.Increment(1)

	if stash.Version != 4 {
		t.Errorf("Version = %d, want 4", stash.Version)
	}

	// Persist and verify
	_, err = stashesTable.Set(stashID, stash)
	if err != nil {
		t.Fatalf("Save stash failed: %v", err)
	}

	entity, err := stashesTable.Get(stashID)
	if err != nil {
		t.Fatalf("Get stash failed: %v", err)
	}
	retrieved := entity.(*types.Stash)
	if retrieved.Version != 4 {
		t.Errorf("Persisted Version = %d, want 4", retrieved.Version)
	}
}

// --- S11, S12: Fetch with filters ---

func TestStashOperations_S11_FetchByStashTypeReturnsOnlyMatching(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	// Create stashes of different types
	stashes := []*types.Stash{
		{Name: "counter-1", StashType: types.StashTypeCounter, Value: map[string]any{"value": int64(0)}, Version: 1},
		{Name: "context-1", StashType: types.StashTypeContext, Value: map[string]any{}, Version: 1},
		{Name: "lock-1", StashType: types.StashTypeLock, Value: nil, Version: 1},
	}
	for _, s := range stashes {
		if _, err := stashesTable.Set("", s); err != nil {
			t.Fatalf("Create stash failed: %v", err)
		}
	}

	// Fetch only counters
	filter := map[string]any{"StashType": types.StashTypeCounter}
	entities, err := stashesTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 counter stash, got %d", len(entities))
	}
	for _, e := range entities {
		s := e.(*types.Stash)
		if s.StashType != types.StashTypeCounter {
			t.Errorf("StashType = %q, want counter", s.StashType)
		}
	}
}

func TestStashOperations_S12_FetchByNameReturnsMatchingStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	// Create stashes with different names
	stashes := []*types.Stash{
		{Name: "shared-config", StashType: types.StashTypeContext, Value: map[string]any{}, Version: 1},
		{Name: "other-config", StashType: types.StashTypeContext, Value: map[string]any{}, Version: 1},
	}
	for _, s := range stashes {
		if _, err := stashesTable.Set("", s); err != nil {
			t.Fatalf("Create stash failed: %v", err)
		}
	}

	// Fetch by name
	filter := map[string]any{"Name": "shared-config"}
	entities, err := stashesTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 stash, got %d", len(entities))
	}
	result := entities[0].(*types.Stash)
	if result.Name != "shared-config" {
		t.Errorf("Name = %q, want 'shared-config'", result.Name)
	}
}

func TestStashOperations_S11_FetchWithNoFilterReturnsAll(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	// Create one of each stash type
	stashes := []*types.Stash{
		{Name: "resource-1", StashType: types.StashTypeResource, Value: map[string]any{}, Version: 1},
		{Name: "artifact-1", StashType: types.StashTypeArtifact, Value: map[string]any{}, Version: 1},
		{Name: "context-1", StashType: types.StashTypeContext, Value: map[string]any{}, Version: 1},
		{Name: "counter-1", StashType: types.StashTypeCounter, Value: map[string]any{"value": int64(0)}, Version: 1},
		{Name: "lock-1", StashType: types.StashTypeLock, Value: nil, Version: 1},
	}
	for _, s := range stashes {
		if _, err := stashesTable.Set("", s); err != nil {
			t.Fatalf("Create stash failed: %v", err)
		}
	}

	// Fetch all
	entities, err := stashesTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 5 {
		t.Errorf("Expected 5 stashes, got %d", len(entities))
	}
}

func TestStashOperations_S11_FetchWithNoMatchesReturnsEmptySlice(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	// Create a context stash
	stash := &types.Stash{
		Name:      "only-context",
		StashType: types.StashTypeContext,
		Value:     map[string]any{},
		Version:   1,
	}
	if _, err := stashesTable.Set("", stash); err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Fetch by nonexistent type
	filter := map[string]any{"StashType": "nonexistent"}
	entities, err := stashesTable.Fetch(filter)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("Expected 0 stashes, got %d", len(entities))
	}
}

// --- S13: Delete stash ---

func TestStashOperations_S13_DeleteRemovesStash(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	stash := &types.Stash{
		Name:      "delete-test",
		StashType: types.StashTypeContext,
		Value:     map[string]any{},
		Version:   1,
	}
	stashID, err := stashesTable.Set("", stash)
	if err != nil {
		t.Fatalf("Create stash failed: %v", err)
	}

	// Delete
	err = stashesTable.Delete(stashID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get should return ErrNotFound
	_, err = stashesTable.Get(stashID)
	if err != types.ErrNotFound {
		t.Errorf("Get after delete expected ErrNotFound, got %v", err)
	}
}

func TestStashOperations_S13_DeleteNonexistentReturnsErrNotFound(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	err := stashesTable.Delete("nonexistent-uuid-12345")
	if err != types.ErrNotFound {
		t.Errorf("Delete nonexistent expected ErrNotFound, got %v", err)
	}
}

func TestStashOperations_S13_DeleteEmptyIDReturnsErrInvalidID(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	err := stashesTable.Delete("")
	if err != types.ErrInvalidID {
		t.Errorf("Delete empty ID expected ErrInvalidID, got %v", err)
	}
}

// --- Additional edge cases ---

func TestStashOperations_GetEmptyIDReturnsErrInvalidID(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	_, err := stashesTable.Get("")
	if err != types.ErrInvalidID {
		t.Errorf("Get empty ID expected ErrInvalidID, got %v", err)
	}
}

func TestStashOperations_GetNonexistentReturnsErrNotFound(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	_, err := stashesTable.Get("nonexistent-uuid-12345")
	if err != types.ErrNotFound {
		t.Errorf("Get nonexistent expected ErrNotFound, got %v", err)
	}
}

func TestStashOperations_FetchEmptyTableReturnsEmptySlice(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	entities, err := stashesTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("Expected 0 stashes in empty table, got %d", len(entities))
	}
}

// --- Full workflow test ---

func TestStashOperations_FullWorkflow(t *testing.T) {
	_, stashesTable, cleanup := setupStashTest(t)
	defer cleanup()

	// Create stashes of each type
	resourceStash := &types.Stash{Name: "api-conn", StashType: types.StashTypeResource, Value: map[string]any{"uri": "https://api.example.com"}, Version: 1}
	artifactStash := &types.Stash{Name: "build-out", StashType: types.StashTypeArtifact, Value: map[string]any{"path": "/tmp/build"}, Version: 1}
	contextStash := &types.Stash{Name: "shared-cfg", StashType: types.StashTypeContext, Value: map[string]any{"timeout": 30}, Version: 1}
	counterStash := &types.Stash{Name: "req-counter", StashType: types.StashTypeCounter, Value: map[string]any{"value": int64(0)}, Version: 1}
	lockStash := &types.Stash{Name: "mutex", StashType: types.StashTypeLock, Value: nil, Version: 1}

	stashes := []*types.Stash{resourceStash, artifactStash, contextStash, counterStash, lockStash}
	for _, s := range stashes {
		_, err := stashesTable.Set("", s)
		if err != nil {
			t.Fatalf("Create stash failed: %v", err)
		}
	}

	// Verify 5 stashes exist
	all, err := stashesTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch all failed: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("Expected 5 stashes, got %d", len(all))
	}

	// SetValue on context
	err = contextStash.SetValue(map[string]any{"timeout": 60})
	if err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}
	if _, err = stashesTable.Set(contextStash.StashID, contextStash); err != nil {
		t.Fatalf("Save context stash failed: %v", err)
	}

	// Increment counter: +10, -3, +5 = 12
	counterStash.Increment(10)
	counterStash.Increment(-3)
	val, err := counterStash.Increment(5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 12 {
		t.Errorf("Counter value = %d, want 12", val)
	}
	if _, err = stashesTable.Set(counterStash.StashID, counterStash); err != nil {
		t.Fatalf("Save counter stash failed: %v", err)
	}

	// Lock operations: worker-1 acquires, worker-2 fails, worker-1 releases, worker-2 acquires
	err = lockStash.Acquire("worker-1")
	if err != nil {
		t.Fatalf("Acquire by worker-1 failed: %v", err)
	}

	err = lockStash.Acquire("worker-2")
	if err != types.ErrLockHeld {
		t.Errorf("worker-2 acquire expected ErrLockHeld, got %v", err)
	}

	err = lockStash.Release("worker-1")
	if err != nil {
		t.Fatalf("Release by worker-1 failed: %v", err)
	}

	err = lockStash.Acquire("worker-2")
	if err != nil {
		t.Fatalf("Acquire by worker-2 after release failed: %v", err)
	}

	lockData := lockStash.Value.(map[string]any)
	if lockData["holder"] != "worker-2" {
		t.Errorf("Lock holder = %v, want worker-2", lockData["holder"])
	}

	if _, err = stashesTable.Set(lockStash.StashID, lockStash); err != nil {
		t.Fatalf("Save lock stash failed: %v", err)
	}

	// Fetch by stash_type
	counters, err := stashesTable.Fetch(map[string]any{"StashType": types.StashTypeCounter})
	if err != nil {
		t.Fatalf("Fetch counters failed: %v", err)
	}
	if len(counters) != 1 {
		t.Errorf("Expected 1 counter, got %d", len(counters))
	}

	locks, err := stashesTable.Fetch(map[string]any{"StashType": types.StashTypeLock})
	if err != nil {
		t.Fatalf("Fetch locks failed: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("Expected 1 lock, got %d", len(locks))
	}

	// Delete context stash
	err = stashesTable.Delete(contextStash.StashID)
	if err != nil {
		t.Fatalf("Delete context stash failed: %v", err)
	}

	// Verify 4 stashes remain
	remaining, err := stashesTable.Fetch(nil)
	if err != nil {
		t.Fatalf("Fetch remaining failed: %v", err)
	}
	if len(remaining) != 4 {
		t.Errorf("Expected 4 stashes after delete, got %d", len(remaining))
	}
}
