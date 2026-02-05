#!/usr/bin/env bash
#
# Pick the top task from beads and invoke Claude to do the work.
#

set -e

cd "${1:-$(dirname "$0")/..}" || exit 1

# Get the top task issue as JSON
issue_json=$(bd ready -n 1 --json --type "task" 2>/dev/null)

if [ -z "$issue_json" ] || [ "$issue_json" = "[]" ]; then
  echo "No tasks available. Run 'bd ready' to see all issues."
  exit 0
fi

# Extract issue ID and title for display
issue_id=$(echo "$issue_json" | jq -r '.[0].id // empty')
issue_title=$(echo "$issue_json" | jq -r '.[0].title // empty')

if [ -z "$issue_id" ]; then
  echo "Failed to parse issue from beads output."
  exit 1
fi

echo "Picking up task: $issue_id - $issue_title"
echo ""

# Build the prompt for Claude
prompt=$(cat <<EOF
/do-work

Work on the following issue from beads:

Issue ID: $issue_id
Issue JSON:
$issue_json

Use 'bd show $issue_id' to get the full issue details, then claim it with 'bd update $issue_id --status in_progress' and complete the work.
EOF
)

# Invoke Claude with the prompt
# --dangerously-skip-permissions: auto-approve all tool use
# --print: non-interactive mode, exit when done
# --verbose: show full turn-by-turn output
exec claude --dangerously-skip-permissions --print --verbose "$prompt"
