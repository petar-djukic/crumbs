package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// measureConfig holds options for the Measure target.
type measureConfig struct {
	silence      bool
	keepFiles    bool
	limit        int
	showImport   bool
	autoImport   bool
	issuesFile   string
	appendPrompt string
	promptArg    string
}

func parseMeasureEnv() measureConfig {
	cfg := measureConfig{
		limit:      10,
		autoImport: true,
	}
	if os.Getenv("MEASURE_SILENCE") == "true" {
		cfg.silence = true
	}
	if os.Getenv("MEASURE_KEEP") == "true" {
		cfg.keepFiles = true
	}
	if v := os.Getenv("MEASURE_LIMIT"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.limit = n
		}
	}
	if os.Getenv("MEASURE_SHOW_IMPORT") == "true" {
		cfg.showImport = true
	}
	if os.Getenv("MEASURE_NO_AUTO_IMPORT") == "true" {
		cfg.autoImport = false
	}
	if v := os.Getenv("MEASURE_ISSUES_FILE"); v != "" {
		cfg.issuesFile = v
	}
	if v := os.Getenv("MEASURE_APPEND_PROMPT"); v != "" {
		cfg.appendPrompt = v
	}
	if v := os.Getenv("MEASURE_PROMPT"); v != "" {
		cfg.promptArg = v
	}
	return cfg
}

// Measure assesses project state and proposes new tasks via Claude.
func Measure() error {
	cfg := parseMeasureEnv()

	timestamp := time.Now().Format("20060102-150405")
	outputFile := filepath.Join("docs", fmt.Sprintf("proposed-issues-%s.json", timestamp))
	outputFilename := filepath.Base(outputFile)

	// Clean up old proposed-issues files unless keep is set.
	if !cfg.keepFiles {
		cleanupProposedIssues()
	}

	// Get existing issues.
	fmt.Println("Querying existing issues...")
	existingIssues, err := getExistingIssues(cfg.issuesFile)
	if err != nil {
		return fmt.Errorf("getting existing issues: %w", err)
	}

	issueCount := countJSONArray(existingIssues)
	fmt.Printf("Found %d existing issue(s).\n", issueCount)
	fmt.Printf("Issue limit: %d\n", cfg.limit)
	fmt.Printf("Output file: %s\n", outputFile)
	fmt.Println()

	// Read append prompt file if specified.
	var appendContent string
	if cfg.appendPrompt != "" {
		data, readErr := os.ReadFile(cfg.appendPrompt)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Append prompt file not found: %s\n", cfg.appendPrompt)
		} else {
			appendContent = string(data)
			fmt.Printf("Appending prompt from: %s\n", cfg.appendPrompt)
		}
	}

	// Build and run prompt.
	prompt := buildMeasurePrompt(cfg.promptArg, existingIssues, cfg.limit, "docs/"+outputFilename, appendContent)

	if err := runClaude(prompt, cfg.silence); err != nil {
		return fmt.Errorf("running Claude: %w", err)
	}

	fmt.Println()
	if _, statErr := os.Stat(outputFile); statErr == nil {
		fmt.Printf("Proposed issues written to: %s\n", outputFile)
		fmt.Println()
		fmt.Printf("To review:\n  cat %s | jq\n", outputFile)

		if cfg.autoImport {
			fmt.Println()
			if importErr := importIssues(outputFile); importErr != nil {
				return fmt.Errorf("importing issues: %w", importErr)
			}
			os.Remove(outputFile)
			fmt.Printf("Removed: %s\n", outputFile)
		} else if cfg.showImport {
			fmt.Println()
			fmt.Println("To import into bd (after review):")
			fmt.Println("  # Manual import - bd create commands for each issue")
		}
	} else {
		fmt.Println("No proposed issues file created.")
	}

	fmt.Println()
	fmt.Println("Done.")
	return nil
}

func cleanupProposedIssues() {
	matches, _ := filepath.Glob("docs/proposed-issues-*.json")
	for _, f := range matches {
		os.Remove(f)
	}
}

func getExistingIssues(issuesFile string) (string, error) {
	if issuesFile != "" {
		data, err := os.ReadFile(issuesFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Issues file not found: %s\n", issuesFile)
			return "[]", nil
		}
		return string(data), nil
	}

	if _, err := exec.LookPath("bd"); err != nil {
		return "[]", nil
	}

	out, err := exec.Command("bd", "list", "--json").Output()
	if err != nil {
		return "[]", nil
	}
	return string(out), nil
}

func countJSONArray(jsonStr string) int {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return 0
	}
	return len(arr)
}

func buildMeasurePrompt(userInput, existingIssues string, limit int, outputPath, appendContent string) string {
	var b strings.Builder

	b.WriteString(`# Make Work

Read VISION.md, ARCHITECTURE.md, road-map.yaml, docs/specs/product-requirements/, docs/specs/use-cases/, docs/specs/test-suites/, and docs/engineering/ if they exist.

## Existing Work

The following issues already exist in the system:

` + "```json\n")
	b.WriteString(existingIssues)
	b.WriteString("\n```\n")

	b.WriteString(`
Review what's in progress, what's completed, and what's pending.

## Instructions

Summarize:

1. What problem this project solves
2. The high-level architecture (major components and how they fit together)
3. The current state of implementation (what's done, what's in progress)
4. **Current release**: Which release we are working on and which use cases remain (check road-map.yaml)
5. Current repo size: run ` + "`mage stats`" + ` and include its output (Go production/test LOC, doc words)

Based on this, propose next steps using **release priority**:

1. **Focus on earliest incomplete release**: Prioritize completing use cases from the current release in road-map.yaml
2. **Early preview allowed**: Later use cases can be partially implemented if they share functionality with the current release
3. **Assign issues to releases**: Each issue should map to a use case in road-map.yaml; if uncertain, use release 99.0 (unscheduled)
4. If epics exist: suggest new issues to add to existing epics, or identify what to work on next
5. If no epics exist: suggest epics to create and initial issues for each
6. Identify dependencies - what should be built first and why?

When proposing issues (per crumb-format rule):

1. **Type**: Say whether each issue is **documentation** (markdown in ` + "`docs/`" + `) or **code** (implementation).
2. **Required Reading**: List files the agent must read before starting (PRDs, ARCHITECTURE sections, existing code). This is mandatory for all issues.
3. **Files to Create/Modify**: Explicit list of files the issue will produce or change. For docs: output path. For code: packages/files to create or edit.
4. **Structure** (all issues): Requirements, Design Decisions (optional), Acceptance Criteria.
5. **Documentation issues**: Add **format rule** reference and **required sections** (PRD: Problem, Goals, Requirements, Non-Goals, Acceptance Criteria; use case: Summary, Actor/trigger, Flow, Success criteria).
6. **Code issues**: Requirements, Design Decisions, Acceptance Criteria (tests/behavior); no PRD-style Problem/Goals/Non-Goals.

**Code task sizing**: Target 300-700 lines of production code per task, touching no more than 5 files. This keeps tasks completable in a single session while being substantial enough to make meaningful progress. Split larger features into multiple tasks; combine trivial changes into one task.

`)
	fmt.Fprintf(&b, "**Task limit**: Create no more than %d tasks. If more work is needed, create additional tasks in a future session.\n", limit)

	b.WriteString(`
## Output

After analyzing the project and proposing work, output the new issues as a JSON file.

`)
	fmt.Fprintf(&b, "**IMPORTANT**: Do NOT use bd commands. Instead, write the proposed issues to `%s` using the Write tool.\n", outputPath)

	b.WriteString(`
The JSON format should be an array of issue objects:

` + "```json\n" + `[
  {
    "type": "task",
    "title": "Task title",
    "description": "Full task description with Required Reading, Files to Create/Modify, Requirements, Design Decisions, Acceptance Criteria",
    "labels": ["code"]
  }
]
` + "```\n" + `
Field notes:
- ` + "`type`" + `: "epic" or "task"
- ` + "`title`" + `: Short descriptive title
- ` + "`description`" + `: Full issue description following crumb-format rule
- ` + "`parent`" + `: (tasks only, optional) Reference to parent epic by title slug (lowercase, hyphenated). Only use if creating a NEW epic in the same JSON.
- ` + "`labels`" + `: Optional array, use "documentation" for doc tasks, "code" for code tasks

**Epics are optional.** Only create a new epic if there is a clear need for grouping multiple related tasks. Most of the time, standalone tasks are sufficient. Do NOT create an epic just to have one - if you have only 1-2 tasks, just create the tasks without an epic.

The issues will be automatically imported into bd.
`)

	if userInput != "" {
		fmt.Fprintf(&b, "\n## Additional Context from User\n\n%s\n", userInput)
	}

	if appendContent != "" {
		fmt.Fprintf(&b, "\n## Appended Instructions\n\n%s\n", appendContent)
	}

	return b.String()
}

func runClaude(prompt string, silence bool) error {
	fmt.Println("Running Claude with measure...")
	fmt.Println()

	args := []string{"--dangerously-skip-permissions", "-p", "--verbose", "--output-format", "stream-json"}
	cmd := exec.Command("claude", args...)
	cmd.Stdin = strings.NewReader(prompt)

	if silence {
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
		// Pipe through jq for readability.
		jq := exec.Command("jq")
		jq.Stdin, _ = cmd.StdoutPipe()
		jq.Stdout = os.Stdout
		jq.Stderr = os.Stderr
		if err := jq.Start(); err != nil {
			// Fall back to raw output if jq not available.
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			defer jq.Wait()
		}
	}

	return cmd.Run()
}

// proposedIssue represents one issue from the proposed-issues JSON file.
type proposedIssue struct {
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Parent      string   `json:"parent,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

func importIssues(jsonFile string) error {
	if _, err := exec.LookPath("bd"); err != nil {
		return fmt.Errorf("bd command not found, cannot import")
	}

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("reading JSON file: %w", err)
	}

	var issues []proposedIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	fmt.Printf("Importing issues from %s...\n", jsonFile)

	// Track epic IDs by slug for parent references.
	epicMap := map[string]string{}

	// First pass: create epics.
	var epicCount int
	for _, issue := range issues {
		if issue.Type == "epic" {
			epicCount++
		}
	}
	fmt.Printf("Creating %d epic(s)...\n", epicCount)

	for _, issue := range issues {
		if issue.Type != "epic" {
			continue
		}
		fmt.Printf("  Creating epic: %s\n", issue.Title)

		args := []string{"create", "--type", "epic", issue.Title, "--description", issue.Description, "--json"}
		out, err := exec.Command("bd", args...).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "    Warning: Failed to create epic\n")
			continue
		}

		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &result); err == nil && result.ID != "" {
			slug := titleToSlug(issue.Title)
			epicMap[slug] = result.ID
			fmt.Printf("    Created: %s\n", result.ID)
		}
	}

	// Second pass: create tasks.
	var taskCount int
	for _, issue := range issues {
		if issue.Type == "task" {
			taskCount++
		}
	}
	fmt.Printf("Creating %d task(s)...\n", taskCount)

	for _, issue := range issues {
		if issue.Type != "task" {
			continue
		}
		fmt.Printf("  Creating task: %s\n", issue.Title)

		args := []string{"create", "--type", "task", issue.Title, "--description", issue.Description}

		if issue.Parent != "" {
			if parentID, ok := epicMap[issue.Parent]; ok {
				args = append(args, "--parent", parentID)
			} else {
				fmt.Fprintf(os.Stderr, "    Warning: Parent epic '%s' not found\n", issue.Parent)
			}
		}

		if len(issue.Labels) > 0 {
			args = append(args, "--labels", strings.Join(issue.Labels, ","))
		}

		if err := exec.Command("bd", args...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "    Warning: Failed to create task\n")
		} else {
			fmt.Println("    Created")
		}
	}

	fmt.Println("Import complete.")

	// Sync beads and commit.
	fmt.Println("Syncing and committing beads changes...")
	exec.Command("bd", "sync").Run()
	exec.Command("git", "add", ".beads/").Run()
	exec.Command("git", "commit", "-m", "Add issues from measure", "--allow-empty").Run()
	fmt.Println("Changes committed.")

	return nil
}

func titleToSlug(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
