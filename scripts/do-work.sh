#!/usr/bin/env bash
#
# Pick the top task from beads and invoke Claude to do the work.
#
# The script handles task picking, reservation, and git worktree management.
# Claude receives a clean prompt focused on the work itself.
#
# Usage: do-work.sh [options] [repo-root]
#
# Options:
#   --silence-claude       Suppress Claude's output
#   --make-work-limit N    Number of issues to create when no tasks (default: 5)
#   --cycles N             Number of make-work cycles (default: 0)
#   --generate             Start a new generation: tag main, create branch, delete Go files
#   --reset                Close current generation: tag, merge to main, delete branch
#
# See docs/engineering/eng02-generation-workflow.md for the full workflow.
#
# Workflow:
# 1. Pick and claim a task from beads
# 2. Create a git worktree with a branch for the task
# 3. Run Claude in the worktree
# 4. Merge the branch back to the current branch
# 5. Clean up the worktree
# 6. When no tasks left, call make-work.sh to create more
# 7. Repeat for specified number of cycles
#

set -e

# Parse arguments
SILENCE_CLAUDE=false
MAKE_WORK_LIMIT=5
CYCLES=0
GENERATE=false
RESET=false
REPO_ARG=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --silence-claude)
      SILENCE_CLAUDE=true
      shift
      ;;
    --make-work-limit)
      MAKE_WORK_LIMIT="$2"
      shift 2
      ;;
    --cycles)
      CYCLES="$2"
      shift 2
      ;;
    --generate)
      GENERATE=true
      shift
      ;;
    --reset)
      RESET=true
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

SCRIPT_DIR="$REPO_ROOT/scripts"
PROJECT_NAME=$(basename "$REPO_ROOT")
WORKTREE_BASE="/tmp/${PROJECT_NAME}-worktrees"

# Returns the current git branch name.
current_branch() {
  git rev-parse --abbrev-ref HEAD
}

# Returns 0 if on a generation branch, 1 otherwise.
on_generation_branch() {
  local branch
  branch=$(current_branch)
  [[ "$branch" == generation-* ]]
}

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

  BRANCH_NAME="task/$ISSUE_ID"
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
  echo "Merging $BRANCH_NAME into $(current_branch)..."

  cd "$REPO_ROOT"

  # Merge the task branch into the current branch (main or generation)
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

call_make_work() {
  echo ""
  echo "========================================"
  echo "No tasks available. Creating new work..."
  echo "========================================"
  echo ""

  local make_work_args="--limit $MAKE_WORK_LIMIT"
  if [ "$SILENCE_CLAUDE" = true ]; then
    make_work_args="$make_work_args --silence-claude"
  fi

  "$SCRIPT_DIR/make-work.sh" $make_work_args
}

# Start a new generation (per eng02-generation-workflow).
# Tags main, creates a generation branch, deletes Go files, reinitializes module.
start_generation() {
  local branch
  branch=$(current_branch)

  if [ "$branch" != "main" ]; then
    echo "Error: --generate must be run from main (currently on $branch)."
    exit 1
  fi

  # Check no existing generation branch
  if git branch --list 'generation-*' | grep -q .; then
    echo "Error: a generation branch already exists. Reset it first or delete it."
    git branch --list 'generation-*'
    exit 1
  fi

  local gen_name="generation-$(date +%Y-%m-%d-%H-%M)"

  echo ""
  echo "========================================"
  echo "Starting generation: $gen_name"
  echo "========================================"
  echo ""

  # Tag current main
  echo "Tagging current state as $gen_name..."
  git tag "$gen_name"

  # Create and switch to generation branch
  echo "Creating branch $gen_name..."
  git checkout -b "$gen_name"

  # Delete Go source files
  echo "Deleting Go source files..."
  find . -name '*.go' -not -path './.git/*' -delete 2>/dev/null || true

  # Remove empty directories left behind in Go source dirs
  for dir in cmd/ pkg/ internal/ tests/; do
    if [ -d "$dir" ]; then
      find "$dir" -type d -empty -delete 2>/dev/null || true
    fi
  done

  # Remove build artifacts and dependency lock
  rm -rf bin/ go.sum

  # Reinitialize Go module
  echo "Reinitializing Go module..."
  rm -f go.mod
  go mod init github.com/mesh-intelligence/crumbs

  # Commit the clean state
  echo "Committing clean state..."
  git add -A
  git commit -m "Start generation: $gen_name

Delete Go files, reinitialize module.
Tagged previous state as $gen_name."

  echo ""
  echo "Generation started. Proceeding to make-work/do-work loop."
  echo ""
}

# Close the current generation (per eng02-generation-workflow).
# Tags the branch, merges to main, deletes the branch.
reset_generation() {
  local branch
  branch=$(current_branch)

  if ! on_generation_branch; then
    echo "Error: --reset must be run from a generation branch (currently on $branch)."
    exit 1
  fi

  local closed_tag="${branch}-closed"

  echo ""
  echo "========================================"
  echo "Resetting generation: $branch"
  echo "========================================"
  echo ""

  # Tag the final state
  echo "Tagging final state as $closed_tag..."
  git tag "$closed_tag"

  # Switch to main and merge
  echo "Switching to main..."
  git checkout main

  echo "Merging $branch into main..."
  git merge "$branch" --no-edit

  # Delete the generation branch
  echo "Deleting branch $branch..."
  git branch -d "$branch"

  echo ""
  echo "Generation reset complete. Work is on main."
  echo ""
}

main() {
  local total_tasks=0
  local make_work_calls=0

  while true; do
    # Do all available tasks
    while pick_task; do
      do_one_task
      total_tasks=$((total_tasks + 1))
      echo ""
      echo "----------------------------------------"
      echo ""
    done

    echo "Queue empty. Completed $total_tasks task(s) so far."

    # Check if we should create more work
    if [ "$make_work_calls" -lt "$CYCLES" ]; then
      make_work_calls=$((make_work_calls + 1))
      echo ""
      echo "========================================"
      echo "Make-work call $make_work_calls of $CYCLES"
      echo "========================================"
      call_make_work
    else
      break
    fi
  done

  echo ""
  echo "========================================"
  echo "Done. Total tasks completed: $total_tasks"
  echo "Make-work calls: $make_work_calls"
  echo "========================================"
}

# Handle generation lifecycle flags
if [ "$GENERATE" = true ] && [ "$RESET" = true ]; then
  echo "Error: cannot use --generate and --reset together."
  exit 1
fi

if [ "$GENERATE" = true ]; then
  start_generation
  main
  exit 0
fi

if [ "$RESET" = true ]; then
  reset_generation
  exit 0
fi

main
