#!/usr/bin/env bash
# make-work.sh
# Implements: rel02.1-uc003-self-hosting F6; test-rel02.1-uc003-self-hosting "Make-work cycle"
#
# Issue creation workflow automation script.
# Reads a JSON file with proposed issues and creates crumbs via cupboard CLI.

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

# Project root (assumes script is in scripts/ subdirectory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Cupboard binary (assumes it's in PATH or can be built)
CUPBOARD="${CUPBOARD:-cupboard}"

# Dry-run mode flag
DRY_RUN=false

# -----------------------------------------------------------------------------
# Helper functions
# -----------------------------------------------------------------------------

log() {
  echo "[make-work] $*" >&2
}

error() {
  echo "[make-work] ERROR: $*" >&2
  exit 1
}

usage() {
  cat <<EOF
Usage: $0 [OPTIONS] <json-file>

Create work items from a JSON file.

Arguments:
  json-file        Path to JSON file containing proposed issues

Options:
  --dry-run        Show what would be created without creating anything
  -h, --help       Show this help message

JSON Format:
  [
    {
      "index": 0,
      "title": "Issue title",
      "description": "Issue description",
      "dependency": -1
    },
    {
      "index": 1,
      "title": "Another issue",
      "description": "Description",
      "dependency": 0
    }
  ]

  - index: Unique sequential number for this issue
  - title: Issue title (becomes crumb Name)
  - description: Issue description
  - dependency: Index of parent issue, or -1 for no dependency

Example:
  $0 proposed-issues.json
  $0 --dry-run proposed-issues.json
EOF
  exit 0
}

# Parse JSON using basic shell tools (no jq dependency)
# Usage: json_array_length <json_array>
json_array_length() {
  local json="$1"

  # Count objects in array by counting opening braces after initial bracket
  echo "$json" | grep -o '{' | wc -l | tr -d ' '
}

# Extract value from JSON object
# Usage: json_value <json_object> <key>
json_value() {
  local json="$1"
  local key="$2"

  # Extract value for the given key
  echo "$json" | sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\?\([^,}\"]*\)\"\?.*/\1/p" | head -1
}

# Extract array element by index
# Usage: json_array_element <json_array> <index>
json_array_element() {
  local json="$1"
  local index="$2"

  # This is a simple implementation that works for basic cases
  # Extract the nth object from the array
  echo "$json" | sed -n "s/.*{\([^}]*\)}.*/{\1}/p" | sed -n "$((index + 1))p"
}

# Extract CrumbID from cupboard set output
# Usage: extract_crumb_id <json_output>
extract_crumb_id() {
  local json="$1"

  # Extract CrumbID from JSON output
  echo "$json" | sed -n 's/.*"CrumbID"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

# -----------------------------------------------------------------------------
# JSON validation
# -----------------------------------------------------------------------------

validate_json() {
  local json_file="$1"

  log "Validating JSON structure"

  # Check if file exists
  if [[ ! -f "$json_file" ]]; then
    error "File not found: $json_file"
  fi

  # Read JSON file
  local json_content
  json_content=$(cat "$json_file")

  # Basic validation - check if it looks like a JSON array
  if [[ ! "$json_content" =~ ^\[.*\]$ ]]; then
    error "JSON file must contain an array at the top level"
  fi

  # Check if array is empty
  local length
  length=$(json_array_length "$json_content")

  if [[ "$length" -eq 0 ]]; then
    error "JSON array is empty - no issues to create"
  fi

  log "JSON file contains $length issue(s)"

  echo "$json_content"
}

# -----------------------------------------------------------------------------
# Issue creation
# -----------------------------------------------------------------------------

create_issues() {
  local json_content="$1"

  # Use two parallel arrays to map index to crumb ID
  # (bash 3.2 doesn't support associative arrays)
  local -a indices=()
  local -a crumb_ids=()

  # Track created issues for summary
  local created_count=0
  local failed_count=0

  # Parse JSON and create issues
  # Note: This is a simplified parser that expects well-formed JSON
  # In production, consider using jq for robust JSON parsing

  log "Reading JSON file"

  # Simple JSON parsing: convert to single line, then split on object boundaries
  # Replace },<whitespace>{  with }|{
  local raw_objects
  raw_objects=$(echo "$json_content" | tr -d '\n' | sed 's/}[[:space:]]*,[[:space:]]*{/}|{/g' | sed 's/^\[//;s/\]$//')

  # Split into array using | as delimiter
  IFS='|' read -ra issues <<< "$raw_objects"

  log "Found ${#issues[@]} issue(s) to create"

  # Process each issue
  local issue_count=${#issues[@]}
  if [[ "$issue_count" -eq 0 ]]; then
    error "No valid issues found in JSON file"
  fi

  local issue_idx
  for issue_idx in $(seq 0 $((issue_count - 1))); do
    local issue="${issues[$issue_idx]}"

    # Clean up the issue (remove leading/trailing whitespace)
    issue=$(echo "$issue" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    # Extract fields
    local index
    local title
    local description
    local dependency

    index=$(echo "$issue" | sed -n 's/.*"index"[[:space:]]*:[[:space:]]*\([0-9-]*\).*/\1/p')
    title=$(echo "$issue" | sed -n 's/.*"title"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
    description=$(echo "$issue" | sed -n 's/.*"description"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
    dependency=$(echo "$issue" | sed -n 's/.*"dependency"[[:space:]]*:[[:space:]]*\([0-9-]*\).*/\1/p')

    # Validate required fields
    if [[ -z "$index" || -z "$title" ]]; then
      log "WARNING: Skipping malformed issue (missing index or title)"
      ((failed_count++))
      continue
    fi

    # Use empty description if not provided
    if [[ -z "$description" ]]; then
      description=""
    fi

    # Use -1 as default for dependency
    if [[ -z "$dependency" ]]; then
      dependency=-1
    fi

    log "Processing issue $index: $title"

    if [[ "$DRY_RUN" == true ]]; then
      log "[DRY-RUN] Would create: [$index] $title"
      if [[ "$dependency" != -1 ]]; then
        log "[DRY-RUN]   with dependency on issue $dependency"
      fi
      ((created_count++))
      continue
    fi

    # Create crumb via cupboard set command
    # JSON payload: {"Name":"title","State":"draft"}
    # Description will be added via metadata in future implementation

    local json_payload
    json_payload=$(cat <<EOF
{"Name":"$title","State":"draft"}
EOF
)

    # Create the crumb
    local output
    if ! output=$("$CUPBOARD" set crumbs "" "$json_payload" 2>&1); then
      log "ERROR: Failed to create issue $index: $title"
      log "  Output: $output"
      ((failed_count++))
      continue
    fi

    # Extract crumb ID from output
    local crumb_id
    crumb_id=$(extract_crumb_id "$output")

    if [[ -z "$crumb_id" ]]; then
      log "ERROR: Failed to extract CrumbID from output for issue $index"
      log "  Output: $output"
      ((failed_count++))
      continue
    fi

    # Store mapping using parallel arrays
    indices+=("$index")
    crumb_ids+=("$crumb_id")

    log "Created: [$index] $title -> $crumb_id"
    ((created_count++))

    # Handle dependency if present
    if [[ "$dependency" != -1 ]]; then
      # Look up parent crumb ID from parallel arrays
      local parent_id=""
      local i
      for i in "${!indices[@]}"; do
        if [[ "${indices[$i]}" == "$dependency" ]]; then
          parent_id="${crumb_ids[$i]}"
          break
        fi
      done

      if [[ -z "$parent_id" ]]; then
        log "WARNING: Dependency $dependency not found for issue $index"
        log "  Parent issue must be created before child issue"
        continue
      fi

      log "  Creating child_of link: $crumb_id -> $parent_id"

      # Create child_of link
      # Note: Link creation via CLI is not yet implemented
      # For now, we log the intent
      # In future: cupboard link create --type child_of --from "$crumb_id" --to "$parent_id"
      log "  [TODO] child_of link creation not yet implemented in CLI"
    fi
  done

  # Print summary
  echo ""
  log "Summary:"
  log "  Created: $created_count issue(s)"
  if [[ "$failed_count" -gt 0 ]]; then
    log "  Failed:  $failed_count issue(s)"
  fi

  # List created issues
  if [[ "$created_count" -gt 0 && "$DRY_RUN" == false ]]; then
    echo ""
    log "Created issues:"

    local i
    for i in "${!indices[@]}"; do
      echo "  [${indices[$i]}] ${crumb_ids[$i]}"
    done
  fi
}

# -----------------------------------------------------------------------------
# Git commit
# -----------------------------------------------------------------------------

commit_changes() {
  if [[ "$DRY_RUN" == true ]]; then
    log "[DRY-RUN] Would commit JSONL changes to git"
    return 0
  fi

  log "Committing JSONL changes to git"

  # Change to project root
  cd "$PROJECT_ROOT"

  # Check if there are changes to commit
  if ! git diff --quiet HEAD -- "*.jsonl" 2>/dev/null && ! git diff --cached --quiet -- "*.jsonl" 2>/dev/null; then
    log "No JSONL changes to commit"
    return 0
  fi

  # Add JSONL files
  if ! git add .crumbs-db/*.jsonl 2>/dev/null; then
    log "WARNING: Failed to add JSONL files (they may not exist yet)"
    return 0
  fi

  # Commit with descriptive message
  if ! git commit -m "Add work items via make-work.sh

Created issues from JSON input file.

Generated by: scripts/make-work.sh" 2>/dev/null; then
    log "WARNING: Git commit failed (changes may already be committed)"
    return 0
  fi

  log "Changes committed successfully"
}

# -----------------------------------------------------------------------------
# Main workflow
# -----------------------------------------------------------------------------

main() {
  local json_file=""

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case $1 in
      --dry-run)
        DRY_RUN=true
        shift
        ;;
      -h|--help)
        usage
        ;;
      -*)
        error "Unknown option: $1"
        ;;
      *)
        if [[ -z "$json_file" ]]; then
          json_file="$1"
        else
          error "Multiple JSON files specified. Only one file is supported."
        fi
        shift
        ;;
    esac
  done

  # Validate arguments
  if [[ -z "$json_file" ]]; then
    error "Missing required argument: json-file

Use --help for usage information."
  fi

  log "Starting make-work workflow"

  if [[ "$DRY_RUN" == true ]]; then
    log "DRY-RUN mode enabled - no changes will be made"
  fi

  # Validate JSON file
  local json_content
  json_content=$(validate_json "$json_file")

  # List existing issues before creation
  log "Listing existing issues"

  if [[ "$DRY_RUN" == false ]]; then
    local existing_count
    if existing_count=$("$CUPBOARD" list crumbs 2>&1 | grep -c "CrumbID" || echo "0"); then
      log "Found $existing_count existing issue(s)"
    fi
  fi

  # Create issues
  create_issues "$json_content"

  # Commit changes
  commit_changes

  log "Workflow completed successfully"
  exit 0
}

# -----------------------------------------------------------------------------
# Entry point
# -----------------------------------------------------------------------------

main "$@"
