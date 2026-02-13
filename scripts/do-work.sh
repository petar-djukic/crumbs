#!/usr/bin/env bash
# do-work.sh
# Implements: rel02.1-uc003-self-hosting F5; test-rel02.1-uc003-self-hosting "Do-work cycle"
#
# Agent workflow automation script.
# Picks a ready task from cupboard, creates a git worktree, invokes agent, merges work back, and closes task.

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

# Project root (assumes script is in scripts/ subdirectory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Worktree location (per eng01-git-integration Table 3)
WORKTREE_BASE="${TMPDIR:-/tmp}/crumbs-worktrees"

# Cupboard binary (assumes it's in PATH or can be built)
CUPBOARD="${CUPBOARD:-cupboard}"

# Agent command (placeholder - can be overridden via environment)
AGENT_CMD="${AGENT_CMD:-echo 'Agent placeholder - set AGENT_CMD to invoke real agent'}"

# -----------------------------------------------------------------------------
# Cleanup handler
# -----------------------------------------------------------------------------

WORKTREE_PATH=""
TASK_BRANCH=""

cleanup() {
  local exit_code=$?

  if [[ -n "$WORKTREE_PATH" && -d "$WORKTREE_PATH" ]]; then
    echo "[do-work] Cleaning up worktree: $WORKTREE_PATH"
    cd "$PROJECT_ROOT"
    git worktree remove "$WORKTREE_PATH" --force 2>/dev/null || true
  fi

  if [[ -n "$TASK_BRANCH" ]]; then
    echo "[do-work] Cleaning up branch: $TASK_BRANCH"
    git branch -D "$TASK_BRANCH" 2>/dev/null || true
  fi

  exit "$exit_code"
}

trap cleanup EXIT

# -----------------------------------------------------------------------------
# Helper functions
# -----------------------------------------------------------------------------

log() {
  echo "[do-work] $*" >&2
}

error() {
  echo "[do-work] ERROR: $*" >&2
  exit 1
}

# Parse JSON using basic shell tools (no jq dependency)
# Usage: json_value <json_string> <key>
json_value() {
  local json="$1"
  local key="$2"

  # Extract value for the given key
  # This is a simple implementation that works for basic cases
  echo "$json" | sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -1
}

# Extract CrumbID from JSON array (first element)
json_first_crumb_id() {
  local json="$1"

  # Remove leading/trailing brackets and whitespace, extract first CrumbID
  echo "$json" | sed -n 's/.*"CrumbID"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

# -----------------------------------------------------------------------------
# Main workflow
# -----------------------------------------------------------------------------

main() {
  log "Starting do-work workflow"

  # Verify we're in a git repository
  if ! git rev-parse --git-dir &>/dev/null; then
    error "Not in a git repository"
  fi

  # Store original directory
  ORIGINAL_DIR="$(pwd)"

  # Get current branch (base branch for worktree naming)
  BASE_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
  log "Base branch: $BASE_BRANCH"

  # -----------------------------------------------------------------------------
  # Step 1: Pick a ready task
  # -----------------------------------------------------------------------------

  log "Fetching ready tasks from cupboard"

  # Use cupboard ready with --json flag (prd009-cupboard-cli R5.2, R7.1)
  READY_JSON=$("$CUPBOARD" ready -n 1 --json --type task 2>/dev/null) || {
    error "Failed to run cupboard ready command. Is cupboard CLI built and in PATH?"
  }

  # Check if we got any tasks
  if [[ -z "$READY_JSON" || "$READY_JSON" == "[]" || "$READY_JSON" == "null" ]]; then
    log "No ready tasks found"
    exit 0
  fi

  # Extract task ID from JSON
  TASK_ID=$(json_first_crumb_id "$READY_JSON")

  if [[ -z "$TASK_ID" ]]; then
    error "Failed to parse task ID from cupboard output"
  fi

  log "Picked task: $TASK_ID"

  # -----------------------------------------------------------------------------
  # Step 2: Validate task exists and is in ready state
  # -----------------------------------------------------------------------------

  log "Validating task state"

  TASK_SHOW=$("$CUPBOARD" show "$TASK_ID" 2>/dev/null) || {
    error "Task $TASK_ID not found"
  }

  # Check if task is in ready state (basic validation)
  if ! echo "$TASK_SHOW" | grep -q "ready"; then
    error "Task $TASK_ID is not in ready state"
  fi

  # -----------------------------------------------------------------------------
  # Step 3: Create git worktree for the task
  # -----------------------------------------------------------------------------

  # Task branch naming per eng01-git-integration Table 3
  TASK_BRANCH="${BASE_BRANCH}/task/${TASK_ID}"
  WORKTREE_PATH="${WORKTREE_BASE}/${TASK_ID}"

  log "Creating worktree at $WORKTREE_PATH with branch $TASK_BRANCH"

  # Ensure worktree base directory exists
  mkdir -p "$WORKTREE_BASE"

  # Remove existing worktree/branch if it exists (recovery case)
  if [[ -d "$WORKTREE_PATH" ]]; then
    log "Removing existing worktree (recovery)"
    git worktree remove "$WORKTREE_PATH" --force 2>/dev/null || true
  fi

  if git show-ref --verify --quiet "refs/heads/$TASK_BRANCH"; then
    log "Removing existing branch (recovery)"
    git branch -D "$TASK_BRANCH" 2>/dev/null || true
  fi

  # Create worktree with new branch from current HEAD
  git worktree add -b "$TASK_BRANCH" "$WORKTREE_PATH" HEAD || {
    error "Failed to create git worktree"
  }

  log "Worktree created successfully"

  # -----------------------------------------------------------------------------
  # Step 4: Transition task to taken state
  # -----------------------------------------------------------------------------

  log "Transitioning task to taken state"

  "$CUPBOARD" update "$TASK_ID" --status taken || {
    error "Failed to update task status to taken"
  }

  log "Task status updated to taken"

  # -----------------------------------------------------------------------------
  # Step 5: Invoke agent with context
  # -----------------------------------------------------------------------------

  log "Invoking agent in worktree"

  # Change to worktree directory
  cd "$WORKTREE_PATH"

  # Invoke agent (placeholder for now)
  # In production, this would be something like:
  # claude --working-dir "$WORKTREE_PATH" --task-id "$TASK_ID"
  eval "$AGENT_CMD" || {
    log "Agent invocation failed or returned non-zero"
    # Don't error here - agent might have done partial work
    # Leave the task in taken state and worktree for manual inspection
    WORKTREE_PATH=""  # Prevent cleanup
    TASK_BRANCH=""
    exit 1
  }

  log "Agent completed successfully"

  # -----------------------------------------------------------------------------
  # Step 6: Verify changes
  # -----------------------------------------------------------------------------

  log "Verifying changes"

  # Check if there are any changes (committed or uncommitted)
  if ! git diff --quiet HEAD || ! git diff --cached --quiet || [[ -n "$(git status --porcelain)" ]]; then
    log "Changes detected in worktree"

    # If there are uncommitted changes, warn (agent should have committed)
    if [[ -n "$(git status --porcelain)" ]]; then
      log "WARNING: Uncommitted changes found. Agent should have committed all work."
      log "Committing changes automatically"
      git add -A
      git commit -m "Auto-commit: Complete task $TASK_ID

Agent completed work but did not commit changes.

Task: $TASK_ID" || true
    fi
  else
    log "No changes detected (agent may have only read files)"
  fi

  # -----------------------------------------------------------------------------
  # Step 7: Merge worktree back to base branch
  # -----------------------------------------------------------------------------

  log "Merging worktree back to $BASE_BRANCH"

  # Return to original directory
  cd "$ORIGINAL_DIR"

  # Merge the task branch into the base branch
  git merge --no-ff "$TASK_BRANCH" -m "Merge task $TASK_ID

Completed task: $TASK_ID" || {
    error "Failed to merge task branch. Manual merge required."
  }

  log "Merge completed successfully"

  # -----------------------------------------------------------------------------
  # Step 8: Close task
  # -----------------------------------------------------------------------------

  log "Closing task"

  "$CUPBOARD" close "$TASK_ID" || {
    error "Failed to close task"
  }

  log "Task closed successfully"

  # -----------------------------------------------------------------------------
  # Step 9: Cleanup (handled by trap)
  # -----------------------------------------------------------------------------

  log "Workflow completed successfully for task $TASK_ID"
  exit 0
}

# -----------------------------------------------------------------------------
# Entry point
# -----------------------------------------------------------------------------

main "$@"
