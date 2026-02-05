#!/bin/bash
# Validation script for the self-hosting milestone (uc002-self-hosting).
# Verifies that crumbs can manage its own development workflow.
#
# Tests:
#   1. Trail creation for feature exploration
#   2. Adding crumbs to trails
#   3. Trail completion (makes crumbs permanent)
#   4. Trail abandonment (cleanup failed exploration)
#   5. Querying crumbs with filters
#   6. JSON persistence verification
#
# Usage: ./scripts/validate-self-hosting.sh
# Returns: 0 on success, 1 on failure

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Test directory
TEST_DIR=$(mktemp -d)
CONFIG_FILE="$TEST_DIR/.crumbs.yaml"
DATA_DIR="$TEST_DIR/.crumbs-data"
CUPBOARD_BIN="${CUPBOARD_BIN:-./bin/cupboard}"

# Counters
TESTS_RUN=0
TESTS_PASSED=0

cleanup() {
    rm -rf "$TEST_DIR"
}
trap cleanup EXIT

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    exit 1
}

pass() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    echo -e "${GREEN}[PASS]${NC} $1"
}

run_test() {
    TESTS_RUN=$((TESTS_RUN + 1))
}

cupboard() {
    "$CUPBOARD_BIN" --config "$CONFIG_FILE" "$@"
}

# Ensure binary exists
if [[ ! -x "$CUPBOARD_BIN" ]]; then
    log "Building cupboard binary..."
    go build -o ./bin/cupboard ./cmd/cupboard || fail "Failed to build cupboard"
    CUPBOARD_BIN="./bin/cupboard"
fi

log "Using cupboard binary: $CUPBOARD_BIN"
log "Test directory: $TEST_DIR"

# Create config file
cat > "$CONFIG_FILE" << EOF
backend: sqlite
datadir: $DATA_DIR
EOF

# =============================================================================
# Test 1: Initialize cupboard
# =============================================================================
run_test
log "Test 1: Initialize cupboard"
cupboard init >/dev/null || fail "Failed to initialize cupboard"
[[ -d "$DATA_DIR" ]] || fail "Data directory not created"
[[ -f "$DATA_DIR/crumbs.json" ]] || fail "crumbs.json not created"
pass "Cupboard initialized"

# =============================================================================
# Test 2: Create crumbs (Phase 2 - Basic Self-Hosting)
# =============================================================================
run_test
log "Test 2: Create crumbs for work tracking"

# Create first crumb - a task in draft state
CRUMB1_JSON=$(cupboard set crumbs "" '{"Name":"Implement feature X","State":"draft"}')
CRUMB1_ID=$(echo "$CRUMB1_JSON" | jq -r '.CrumbID')
[[ -n "$CRUMB1_ID" && "$CRUMB1_ID" != "null" ]] || fail "Failed to create crumb 1"

# Create second crumb
CRUMB2_JSON=$(cupboard set crumbs "" '{"Name":"Write tests for feature X","State":"draft"}')
CRUMB2_ID=$(echo "$CRUMB2_JSON" | jq -r '.CrumbID')
[[ -n "$CRUMB2_ID" && "$CRUMB2_ID" != "null" ]] || fail "Failed to create crumb 2"

# Create third crumb for abandonment test
CRUMB3_JSON=$(cupboard set crumbs "" '{"Name":"Try approach A","State":"draft"}')
CRUMB3_ID=$(echo "$CRUMB3_JSON" | jq -r '.CrumbID')
[[ -n "$CRUMB3_ID" && "$CRUMB3_ID" != "null" ]] || fail "Failed to create crumb 3"

pass "Created 3 crumbs: $CRUMB1_ID, $CRUMB2_ID, $CRUMB3_ID"

# =============================================================================
# Test 3: Track crumb progress (state transitions)
# =============================================================================
run_test
log "Test 3: Track crumb progress through states"

# Transition crumb1: draft -> ready -> taken -> completed
cupboard set crumbs "$CRUMB1_ID" "{\"CrumbID\":\"$CRUMB1_ID\",\"Name\":\"Implement feature X\",\"State\":\"ready\"}" >/dev/null
STATE=$(cupboard get crumbs "$CRUMB1_ID" | jq -r '.State')
[[ "$STATE" == "ready" ]] || fail "Expected state ready, got $STATE"

cupboard set crumbs "$CRUMB1_ID" "{\"CrumbID\":\"$CRUMB1_ID\",\"Name\":\"Implement feature X\",\"State\":\"taken\"}" >/dev/null
STATE=$(cupboard get crumbs "$CRUMB1_ID" | jq -r '.State')
[[ "$STATE" == "taken" ]] || fail "Expected state taken, got $STATE"

cupboard set crumbs "$CRUMB1_ID" "{\"CrumbID\":\"$CRUMB1_ID\",\"Name\":\"Implement feature X\",\"State\":\"completed\"}" >/dev/null
STATE=$(cupboard get crumbs "$CRUMB1_ID" | jq -r '.State')
[[ "$STATE" == "completed" ]] || fail "Expected state completed, got $STATE"

pass "Crumb state transitions work: draft -> ready -> taken -> completed"

# =============================================================================
# Test 4: Query crumbs with filters
# =============================================================================
run_test
log "Test 4: Query crumbs with filters"

# Count draft crumbs (should be 2: crumb2 and crumb3)
DRAFT_COUNT=$(cupboard list crumbs State=draft | jq 'length')
[[ "$DRAFT_COUNT" -eq 2 ]] || fail "Expected 2 draft crumbs, got $DRAFT_COUNT"

# Count completed crumbs (should be 1: crumb1)
COMPLETED_COUNT=$(cupboard list crumbs State=completed | jq 'length')
[[ "$COMPLETED_COUNT" -eq 1 ]] || fail "Expected 1 completed crumb, got $COMPLETED_COUNT"

# List all crumbs (should be 3)
TOTAL_COUNT=$(cupboard list crumbs | jq 'length')
[[ "$TOTAL_COUNT" -eq 3 ]] || fail "Expected 3 total crumbs, got $TOTAL_COUNT"

pass "Query filters work: 2 draft, 1 completed, 3 total"

# =============================================================================
# Test 5: Create trail for exploration (Phase 3)
# =============================================================================
run_test
log "Test 5: Create trail for feature exploration"

# Create a trail in active state
TRAIL1_JSON=$(cupboard set trails "" '{"State":"active"}')
TRAIL1_ID=$(echo "$TRAIL1_JSON" | jq -r '.TrailID')
[[ -n "$TRAIL1_ID" && "$TRAIL1_ID" != "null" ]] || fail "Failed to create trail 1"

# Create a second trail for abandonment test
TRAIL2_JSON=$(cupboard set trails "" '{"State":"active"}')
TRAIL2_ID=$(echo "$TRAIL2_JSON" | jq -r '.TrailID')
[[ -n "$TRAIL2_ID" && "$TRAIL2_ID" != "null" ]] || fail "Failed to create trail 2"

pass "Created trails: $TRAIL1_ID, $TRAIL2_ID"

# =============================================================================
# Test 6: Add crumbs to trails (via links)
# =============================================================================
run_test
log "Test 6: Add crumbs to trails via belongs_to links"

# Link crumb2 to trail1 (the good exploration)
LINK1_JSON=$(cupboard set links "" "{\"LinkType\":\"belongs_to\",\"FromID\":\"$CRUMB2_ID\",\"ToID\":\"$TRAIL1_ID\"}")
LINK1_ID=$(echo "$LINK1_JSON" | jq -r '.LinkID')
[[ -n "$LINK1_ID" && "$LINK1_ID" != "null" ]] || fail "Failed to create link 1"

# Link crumb3 to trail2 (the bad exploration)
LINK2_JSON=$(cupboard set links "" "{\"LinkType\":\"belongs_to\",\"FromID\":\"$CRUMB3_ID\",\"ToID\":\"$TRAIL2_ID\"}")
LINK2_ID=$(echo "$LINK2_JSON" | jq -r '.LinkID')
[[ -n "$LINK2_ID" && "$LINK2_ID" != "null" ]] || fail "Failed to create link 2"

# Verify links exist
LINK_COUNT=$(cupboard list links | jq 'length')
[[ "$LINK_COUNT" -eq 2 ]] || fail "Expected 2 links, got $LINK_COUNT"

pass "Created belongs_to links: crumb2 -> trail1, crumb3 -> trail2"

# =============================================================================
# Test 7: Complete trail (successful exploration)
# =============================================================================
run_test
log "Test 7: Complete trail (successful exploration)"

# Complete trail1
cupboard set trails "$TRAIL1_ID" "{\"TrailID\":\"$TRAIL1_ID\",\"State\":\"completed\"}" >/dev/null
STATE=$(cupboard get trails "$TRAIL1_ID" | jq -r '.State')
[[ "$STATE" == "completed" ]] || fail "Expected trail state completed, got $STATE"

pass "Trail completed successfully"

# =============================================================================
# Test 8: Abandon trail (failed exploration)
# =============================================================================
run_test
log "Test 8: Abandon trail (failed exploration)"

# Abandon trail2
cupboard set trails "$TRAIL2_ID" "{\"TrailID\":\"$TRAIL2_ID\",\"State\":\"abandoned\"}" >/dev/null
STATE=$(cupboard get trails "$TRAIL2_ID" | jq -r '.State')
[[ "$STATE" == "abandoned" ]] || fail "Expected trail state abandoned, got $STATE"

pass "Trail abandoned successfully"

# =============================================================================
# Test 9: Archive crumb (soft delete abandoned exploration)
# =============================================================================
run_test
log "Test 9: Archive crumb from abandoned trail"

# Archive crumb3 (the one linked to the abandoned trail)
cupboard set crumbs "$CRUMB3_ID" "{\"CrumbID\":\"$CRUMB3_ID\",\"Name\":\"Try approach A\",\"State\":\"archived\"}" >/dev/null
STATE=$(cupboard get crumbs "$CRUMB3_ID" | jq -r '.State')
[[ "$STATE" == "archived" ]] || fail "Expected crumb state archived, got $STATE"

# Verify archived crumb not in draft list
DRAFT_COUNT=$(cupboard list crumbs State=draft | jq 'length')
[[ "$DRAFT_COUNT" -eq 1 ]] || fail "Expected 1 draft crumb after archive, got $DRAFT_COUNT"

pass "Crumb archived (soft delete)"

# =============================================================================
# Test 10: Verify JSON persistence
# =============================================================================
run_test
log "Test 10: Verify JSON persistence"

# Check crumbs.json
CRUMBS_JSON_COUNT=$(jq 'length' "$DATA_DIR/crumbs.json")
[[ "$CRUMBS_JSON_COUNT" -eq 3 ]] || fail "Expected 3 crumbs in JSON, got $CRUMBS_JSON_COUNT"

# Check trails.json
TRAILS_JSON_COUNT=$(jq 'length' "$DATA_DIR/trails.json")
[[ "$TRAILS_JSON_COUNT" -eq 2 ]] || fail "Expected 2 trails in JSON, got $TRAILS_JSON_COUNT"

# Check links.json
LINKS_JSON_COUNT=$(jq 'length' "$DATA_DIR/links.json")
[[ "$LINKS_JSON_COUNT" -eq 2 ]] || fail "Expected 2 links in JSON, got $LINKS_JSON_COUNT"

# Verify completed trail state in JSON
TRAIL1_STATE_JSON=$(jq -r ".[] | select(.trail_id==\"$TRAIL1_ID\") | .state" "$DATA_DIR/trails.json")
[[ "$TRAIL1_STATE_JSON" == "completed" ]] || fail "Expected completed in JSON, got $TRAIL1_STATE_JSON"

# Verify abandoned trail state in JSON
TRAIL2_STATE_JSON=$(jq -r ".[] | select(.trail_id==\"$TRAIL2_ID\") | .state" "$DATA_DIR/trails.json")
[[ "$TRAIL2_STATE_JSON" == "abandoned" ]] || fail "Expected abandoned in JSON, got $TRAIL2_STATE_JSON"

pass "JSON persistence verified: all data persisted correctly"

# =============================================================================
# Test 11: Full workflow summary
# =============================================================================
run_test
log "Test 11: Full workflow validation"

# Final counts
FINAL_CRUMBS=$(cupboard list crumbs | jq 'length')
FINAL_TRAILS=$(cupboard list trails | jq 'length')
FINAL_LINKS=$(cupboard list links | jq 'length')

# Summary state counts
COMPLETED_CRUMBS=$(cupboard list crumbs State=completed | jq 'length')
ARCHIVED_CRUMBS=$(cupboard list crumbs State=archived | jq 'length')
DRAFT_CRUMBS=$(cupboard list crumbs State=draft | jq 'length')

COMPLETED_TRAILS=$(cupboard list trails State=completed | jq 'length')
ABANDONED_TRAILS=$(cupboard list trails State=abandoned | jq 'length')

[[ "$FINAL_CRUMBS" -eq 3 ]] || fail "Expected 3 crumbs, got $FINAL_CRUMBS"
[[ "$FINAL_TRAILS" -eq 2 ]] || fail "Expected 2 trails, got $FINAL_TRAILS"
[[ "$COMPLETED_CRUMBS" -eq 1 ]] || fail "Expected 1 completed crumb, got $COMPLETED_CRUMBS"
[[ "$ARCHIVED_CRUMBS" -eq 1 ]] || fail "Expected 1 archived crumb, got $ARCHIVED_CRUMBS"
[[ "$DRAFT_CRUMBS" -eq 1 ]] || fail "Expected 1 draft crumb, got $DRAFT_CRUMBS"
[[ "$COMPLETED_TRAILS" -eq 1 ]] || fail "Expected 1 completed trail, got $COMPLETED_TRAILS"
[[ "$ABANDONED_TRAILS" -eq 1 ]] || fail "Expected 1 abandoned trail, got $ABANDONED_TRAILS"

pass "Full workflow validated"

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "=============================================="
echo -e "${GREEN}Self-Hosting Milestone Validation: PASSED${NC}"
echo "=============================================="
echo "Tests run:    $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo ""
echo "Validated capabilities:"
echo "  - Cupboard initialization with SQLite backend"
echo "  - Crumb creation and state transitions"
echo "  - Crumb filtering and queries"
echo "  - Trail creation for exploration"
echo "  - Linking crumbs to trails (belongs_to)"
echo "  - Trail completion (successful exploration)"
echo "  - Trail abandonment (failed exploration)"
echo "  - Crumb archival (soft delete)"
echo "  - JSON persistence to disk"
echo ""
echo "Milestone: Crumbs can manage its own development workflow."
echo "=============================================="

exit 0
