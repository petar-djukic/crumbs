#!/usr/bin/env bash
#
# Pick tasks from beads and invoke Claude to do the work until the queue is empty.
#
# The script handles task picking, reservation, and git worktree management.
# Claude receives a clean prompt focused on the work itself.
#
# Task branches are namespaced under the base branch so they are traceable.
# For example, if started on generation-2026-02-08-09-30, task branches are
# named generation-2026-02-08-09-30/task/<issue-id>.
#
# On startup, the script recovers from a previous interrupted run:
# - Removes any worktrees for task branches under the base branch
# - Deletes those task branches
# - Resets any in_progress issues back to ready
#
# Usage: do-work.sh [options] [repo-root]
#
# Options:
#   --silence-claude       Suppress Claude's output
#
# See docs/engineering/eng02-generation-workflow.md for the full workflow.
#

set -e

# Parse arguments
SILENCE_CLAUDE=false
REPO_ARG=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --silence-claude)
      SILENCE_CLAUDE=true
      shift
      ;;
    *)
      REPO_ARG="$1"
      shift
      ;;
  esac
done

REPO_ROOT="${REPO_ARG:-$(dirname "$0")/..}"
cd "$REPO_ROOT" || exit 1
REPO_ROOT=$(pwd)

PROJECT_NAME=$(basename "$REPO_ROOT")
WORKTREE_BASE="/tmp/${PROJECT_NAME}-worktrees"

# Record the base branch at startup. All task branches are namespaced under it
# and merged back to it when done.
BASE_BRANCH=$(git rev-parse --abbrev-ref HEAD)

echo "Base branch: $BASE_BRANCH"

# ---------------------------------------------------------------------------
# Recovery: clean up stale task branches and in_progress issues from a
# previous interrupted run. Single-threaded, so anything left over is stale.
# ---------------------------------------------------------------------------

recover() {
  local recovered=0

  # 1. Find stale task branches under <base>/task/*
  local stale_branches
  stale_branches=$(git branch --list "$BASE_BRANCH/task/*" 2>/dev/null | sed 's/^[* ]*//')

  for branch in $stale_branches; do
    recovered=1
    echo "Recovering stale branch: $branch"

    # Extract issue ID from branch name: <base>/task/<issue-id>
    local issue_id="${branch##*/task/}"
    local worktree_dir="$WORKTREE_BASE/$issue_id"

    # Remove worktree if it exists
    if [ -d "$worktree_dir" ]; then
      echo "  Removing worktree: $worktree_dir"
      git worktree remove "$worktree_dir" --force 2>/dev/null || true
    fi

    # Delete the branch
    echo "  Deleting branch: $branch"
    git branch -D "$branch" 2>/dev/null || true

    # Reset the issue to ready if it exists in beads
    if [ -n "$issue_id" ]; then
      echo "  Resetting issue to ready: $issue_id"
      bd update "$issue_id" --status ready >/dev/null 2>&1 || true
    fi
  done

  # 2. Reset any in_progress issues that have no task branch (orphaned state)
  local in_progress
  in_progress=$(bd list --json --status in_progress --type task 2>/dev/null || echo "[]")

  if [ -n "$in_progress" ] && [ "$in_progress" != "[]" ]; then
    local ids
    ids=$(echo "$in_progress" | jq -r '.[].id // empty')
    for id in $ids; do
      # Only reset if there is no matching branch (already handled above)
      if ! git show-ref --verify --quiet "refs/heads/$BASE_BRANCH/task/$id"; then
        recovered=1
        echo "Resetting orphaned in_progress issue: $id"
        bd update "$id" --status ready >/dev/null 2>&1 || true
      fi
    done
  fi

  # Prune worktree references for directories that no longer exist
  git worktree prune 2>/dev/null || true

  if [ "$recovered" = 1 ]; then
    # Sync beads and commit recovery changes
    bd sync >/dev/null 2>&1 || true
    git add .beads/ 2>/dev/null || true
    git commit -m "Recover stale tasks from interrupted run" --allow-empty >/dev/null 2>&1 || true
    echo "Recovery complete."
    echo ""
  fi
}

# ---------------------------------------------------------------------------
# Task execution
# ---------------------------------------------------------------------------

# Globals set by pick_task
ISSUE_JSON=""
ISSUE_ID=""
ISSUE_TITLE=""
ISSUE_DESCRIPTION=""
ISSUE_TYPE=""
BRANCH_NAME=""
WORKTREE_DIR=""

pick_task() {
  ISSUE_JSON=$(bd ready -n 1 --json --type "task" 2>/dev/null)

  if [ -z "$ISSUE_JSON" ] || [ "$ISSUE_JSON" = "[]" ]; then
    return 1  # No tasks available
  fi

  ISSUE_ID=$(echo "$ISSUE_JSON" | jq -r '.[0].id // empty')
  ISSUE_TITLE=$(echo "$ISSUE_JSON" | jq -r '.[0].title // empty')
  ISSUE_DESCRIPTION=$(echo "$ISSUE_JSON" | jq -r '.[0].description // empty')
  ISSUE_TYPE=$(echo "$ISSUE_JSON" | jq -r '.[0].type // "task"')

  if [ -z "$ISSUE_ID" ]; then
    echo "Failed to parse issue from beads output."
    return 1
  fi

  # Namespace task branches under the base branch
  BRANCH_NAME="$BASE_BRANCH/task/$ISSUE_ID"
  WORKTREE_DIR="$WORKTREE_BASE/$ISSUE_ID"

  echo "Picking up task: $ISSUE_ID - $ISSUE_TITLE"
  return 0
}

claim_task() {
  bd update "$ISSUE_ID" --status in_progress >/dev/null 2>&1
  echo "Task claimed."
}

create_worktree() {
  echo "Creating worktree at $WORKTREE_DIR..."

  mkdir -p "$WORKTREE_BASE"

  # Create branch from current HEAD if it doesn't exist
  if ! git show-ref --verify --quiet "refs/heads/$BRANCH_NAME"; then
    git branch "$BRANCH_NAME"
  fi

  # Create worktree
  git worktree add "$WORKTREE_DIR" "$BRANCH_NAME"

  echo "Worktree created on branch $BRANCH_NAME"
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
2. Read any PRDs, use cases, test suites, or engineering guidelines referenced in the description
3. Complete the task according to the description and acceptance criteria
4. Commit your changes with a message that includes the task ID ($ISSUE_ID)

Do not use beads (bd) commands - task tracking is handled externally.
EOF
}

run_claude() {
  local prompt="$1"

  echo "Running Claude in worktree..."
  cd "$WORKTREE_DIR"

  # --dangerously-skip-permissions: auto-approve all tool use
  # -p: non-interactive mode, exit when done
  # --verbose --output-format stream-json: stream events, pipe to jq for readability
  if [ "$SILENCE_CLAUDE" = true ]; then
    echo "$prompt" | claude --dangerously-skip-permissions -p --verbose --output-format stream-json >/dev/null 2>&1
  else
    echo "$prompt" | claude --dangerously-skip-permissions -p --verbose --output-format stream-json | jq
  fi

  cd "$REPO_ROOT"
}

merge_branch() {
  echo ""
  echo "Merging $BRANCH_NAME into $BASE_BRANCH..."

  cd "$REPO_ROOT"
  git checkout "$BASE_BRANCH"

  git merge "$BRANCH_NAME" --no-edit

  echo "Branch merged."
}

cleanup_worktree() {
  echo "Cleaning up worktree..."

  git worktree remove "$WORKTREE_DIR" --force 2>/dev/null || true
  git branch -d "$BRANCH_NAME" 2>/dev/null || true

  echo "Worktree removed."
}

close_task() {
  echo ""
  echo "Closing task: $ISSUE_ID"
  bd close "$ISSUE_ID" >/dev/null 2>&1
  bd sync >/dev/null 2>&1

  echo "Committing beads changes..."
  git add .beads/
  git commit -m "Close $ISSUE_ID" --allow-empty >/dev/null 2>&1 || true

  echo "Done."
}

do_one_task() {
  claim_task
  create_worktree
  run_claude "$(build_prompt)"
  merge_branch
  cleanup_worktree
  close_task
}

main() {
  recover

  local total_tasks=0

  while pick_task; do
    do_one_task
    total_tasks=$((total_tasks + 1))
    echo ""
    echo "----------------------------------------"
    echo ""
  done

  echo ""
  echo "========================================"
  echo "Done. Completed $total_tasks task(s)."
  echo "========================================"
}

main
