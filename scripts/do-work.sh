#!/usr/bin/env bash
#
# Pick the top task from beads and invoke Claude to do the work.
#
# The script handles task picking and reservation. Claude receives a clean
# prompt focused on the work itself, without beads-specific instructions.
#

set -e

cd "${1:-$(dirname "$0")/..}" || exit 1

# Globals set by pick_task
ISSUE_JSON=""
ISSUE_ID=""
ISSUE_TITLE=""
ISSUE_DESCRIPTION=""
ISSUE_TYPE=""

pick_task() {
  ISSUE_JSON=$(bd ready -n 1 --json --type "task" 2>/dev/null)

  if [ -z "$ISSUE_JSON" ] || [ "$ISSUE_JSON" = "[]" ]; then
    echo "No tasks available. Run 'bd ready' to see all issues."
    exit 0
  fi

  ISSUE_ID=$(echo "$ISSUE_JSON" | jq -r '.[0].id // empty')
  ISSUE_TITLE=$(echo "$ISSUE_JSON" | jq -r '.[0].title // empty')
  ISSUE_DESCRIPTION=$(echo "$ISSUE_JSON" | jq -r '.[0].description // empty')
  ISSUE_TYPE=$(echo "$ISSUE_JSON" | jq -r '.[0].type // "task"')

  if [ -z "$ISSUE_ID" ]; then
    echo "Failed to parse issue from beads output."
    exit 1
  fi

  echo "Picking up task: $ISSUE_ID - $ISSUE_TITLE"
}

claim_task() {
  bd update "$ISSUE_ID" --status in_progress >/dev/null 2>&1
  echo "Task claimed."
  echo ""
}

build_prompt() {
  cat <<EOF
## Task: $ISSUE_TITLE

**Task ID:** $ISSUE_ID
**Type:** $ISSUE_TYPE

### Description

$ISSUE_DESCRIPTION

---

### Instructions

1. Read VISION.md and ARCHITECTURE.md for context
2. Read any PRDs or docs referenced in the description
3. Complete the task according to the description and acceptance criteria
4. Commit your changes with a message that includes the task ID ($ISSUE_ID)

Do not use beads (bd) commands - task tracking is handled externally.
EOF
}

run_claude() {
  local prompt="$1"
  # --dangerously-skip-permissions: auto-approve all tool use
  # -p: non-interactive mode, exit when done
  # --output-format text: human-readable output
  echo "$prompt" | claude --dangerously-skip-permissions -p --output-format text
}

close_task() {
  echo ""
  echo "Closing task: $ISSUE_ID"
  bd close "$ISSUE_ID" >/dev/null 2>&1

  echo "Committing beads changes..."
  git add .beads/
  git commit -m "Close $ISSUE_ID" --allow-empty >/dev/null 2>&1 || true

  echo "Done."
}

main() {
  pick_task
  claim_task
  run_claude "$(build_prompt)"
  close_task
}

main
