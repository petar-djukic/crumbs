package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"
)

//go:embed prompts/measure.tmpl
var measurePromptTmpl string

// measureConfig holds options for the Measure target.
type measureConfig struct {
	cobblerConfig
}

func parseMeasureFlags() measureConfig {
	var cfg measureConfig
	fs := flag.NewFlagSet("cobbler:measure", flag.ContinueOnError)
	registerCobblerFlags(fs, &cfg.cobblerConfig)
	parseTargetFlags(fs)
	resolveCobblerBranch(&cfg.cobblerConfig, fs)
	return cfg
}

// Measure assesses project state and proposes new tasks via Claude.
//
// Claude writes proposed tasks as JSON; we import them into beads.
//
// Flags:
//
//	--silence-agent          suppress Claude output (default true)
//	--max-issues N           max issues to propose (default 10)
//	--user-prompt TEXT       user prompt text
//	--generation-branch NAME generation branch to work on
func (Cobbler) Measure() error {
	return measure(parseMeasureFlags())
}

func measure(cfg measureConfig) error {
	branch, err := resolveBranch(cfg.generationBranch)
	if err != nil {
		return err
	}
	if err := ensureOnBranch(branch); err != nil {
		return fmt.Errorf("switching to branch: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	outputFile := filepath.Join("docs", fmt.Sprintf("proposed-issues-%s.json", timestamp))

	// Clean up old proposed-issues files.
	matches, _ := filepath.Glob("docs/proposed-issues-*.json")
	for _, f := range matches {
		os.Remove(f)
	}

	// Get existing issues.
	fmt.Println("Querying existing issues...")
	existingIssues := getExistingIssues()

	issueCount := countJSONArray(existingIssues)
	fmt.Printf("Found %d existing issue(s).\n", issueCount)
	fmt.Printf("Max issues: %d\n", cfg.maxIssues)
	fmt.Printf("Output file: %s\n", outputFile)
	fmt.Println()

	// Build and run prompt.
	prompt := buildMeasurePrompt(cfg.userPrompt, existingIssues, cfg.maxIssues, "docs/"+filepath.Base(outputFile))

	if err := runClaude(prompt, "", cfg.silenceAgent); err != nil {
		return fmt.Errorf("running Claude: %w", err)
	}

	// Import proposed issues.
	fmt.Println()
	if _, statErr := os.Stat(outputFile); statErr != nil {
		fmt.Println("No proposed issues file created.")
		return nil
	}

	if err := importIssues(outputFile); err != nil {
		return fmt.Errorf("importing issues: %w", err)
	}
	os.Remove(outputFile)

	fmt.Println()
	fmt.Println("Done.")
	return nil
}

func getExistingIssues() string {
	if _, err := exec.LookPath(binBd); err != nil {
		return "[]"
	}
	out, err := bdListJSON()
	if err != nil {
		return "[]"
	}
	return string(out)
}

func countJSONArray(jsonStr string) int {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return 0
	}
	return len(arr)
}

type measurePromptData struct {
	ExistingIssues string
	Limit          int
	OutputPath     string
	UserInput      string
}

func buildMeasurePrompt(userInput, existingIssues string, limit int, outputPath string) string {
	tmpl := template.Must(template.New("measure").Parse(measurePromptTmpl))
	data := measurePromptData{
		ExistingIssues: existingIssues,
		Limit:          limit,
		OutputPath:     outputPath,
		UserInput:      userInput,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("measure prompt template: %v", err))
	}
	return buf.String()
}

type proposedIssue struct {
	Index       int    `json:"index"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Dependency  int    `json:"dependency"`
}

func importIssues(jsonFile string) error {
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("reading JSON file: %w", err)
	}

	var issues []proposedIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	fmt.Printf("Importing %d task(s)...\n", len(issues))

	// Pass 1: create all issues and collect their beads IDs.
	createdIDs := make(map[int]string)
	for _, issue := range issues {
		fmt.Printf("  Creating: %s\n", issue.Title)
		out, err := bdCreateTask(issue.Title, issue.Description)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    Warning: Failed to create task\n")
			continue
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &created); err == nil && created.ID != "" {
			createdIDs[issue.Index] = created.ID
		}
	}

	// Pass 2: wire up dependencies.
	for _, issue := range issues {
		if issue.Dependency < 0 {
			continue
		}
		childID, hasChild := createdIDs[issue.Index]
		parentID, hasParent := createdIDs[issue.Dependency]
		if !hasChild || !hasParent {
			fmt.Fprintf(os.Stderr, "  Warning: skipping dependency %d -> %d (missing ID)\n", issue.Index, issue.Dependency)
			continue
		}
		fmt.Printf("  Linking: %s depends on %s\n", childID, parentID)
		if err := bdAddDep(childID, parentID); err != nil {
			fmt.Fprintf(os.Stderr, "    Warning: Failed to add dependency\n")
		}
	}

	beadsCommit("Add issues from measure")
	fmt.Println("Issues imported.")

	return nil
}
