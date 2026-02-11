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
	measureStart := time.Now()
	logf("measure: starting")
	cfg.logConfig("measure")

	if err := requireBeads(); err != nil {
		logf("measure: beads not initialized: %v", err)
		return err
	}

	branch, err := resolveBranch(cfg.generationBranch)
	if err != nil {
		logf("measure: resolveBranch failed: %v", err)
		return err
	}
	logf("measure: resolved branch=%s", branch)

	if err := ensureOnBranch(branch); err != nil {
		logf("measure: ensureOnBranch failed: %v", err)
		return fmt.Errorf("switching to branch: %w", err)
	}

	_ = os.MkdirAll(cobblerDir, 0o755)
	timestamp := time.Now().Format("20060102-150405")
	outputFile := filepath.Join(cobblerDir, fmt.Sprintf("proposed-issues-%s.json", timestamp))

	// Clean up old proposed-issues files.
	matches, _ := filepath.Glob(cobblerDir + "proposed-issues-*.json")
	if len(matches) > 0 {
		logf("measure: cleaning %d old proposed-issues file(s)", len(matches))
	}
	for _, f := range matches {
		os.Remove(f)
	}

	// Get existing issues.
	logf("measure: querying existing issues via bd list")
	existingIssues := getExistingIssues()

	issueCount := countJSONArray(existingIssues)
	logf("measure: found %d existing issue(s), maxIssues=%d", issueCount, cfg.maxIssues)
	logf("measure: outputFile=%s", outputFile)

	// Build and run prompt.
	prompt := buildMeasurePrompt(cfg.userPrompt, existingIssues, cfg.maxIssues, outputFile)
	logf("measure: prompt built, length=%d bytes", len(prompt))

	logf("measure: invoking Claude")
	claudeStart := time.Now()
	if err := runClaude(prompt, "", cfg.silenceAgent, cfg.tokenFile, cfg.noContainer); err != nil {
		logf("measure: Claude failed after %s: %v", time.Since(claudeStart).Round(time.Second), err)
		return fmt.Errorf("running Claude: %w", err)
	}
	logf("measure: Claude completed in %s", time.Since(claudeStart).Round(time.Second))

	// Import proposed issues.
	if _, statErr := os.Stat(outputFile); statErr != nil {
		logf("measure: output file not found at %s (Claude may not have written it)", outputFile)
		fmt.Println("No proposed issues file created.")
		return nil
	}

	fileInfo, _ := os.Stat(outputFile)
	logf("measure: output file found, size=%d bytes", fileInfo.Size())

	logf("measure: importing issues from %s", outputFile)
	importStart := time.Now()
	imported, err := importIssues(outputFile)
	if err != nil {
		logf("measure: import failed after %s: %v", time.Since(importStart).Round(time.Second), err)
		return fmt.Errorf("importing issues: %w", err)
	}
	logf("measure: imported %d issue(s) in %s", imported, time.Since(importStart).Round(time.Second))

	if imported == 0 {
		logf("measure: no issues imported, keeping %s for inspection", outputFile)
	} else {
		logf("measure: removing %s (import successful)", outputFile)
		os.Remove(outputFile)
	}

	logf("measure: completed in %s", time.Since(measureStart).Round(time.Second))
	return nil
}

func getExistingIssues() string {
	if _, err := exec.LookPath(binBd); err != nil {
		logf("getExistingIssues: bd not on PATH: %v", err)
		return "[]"
	}
	out, err := bdListJSON()
	if err != nil {
		logf("getExistingIssues: bd list failed: %v", err)
		return "[]"
	}
	logf("getExistingIssues: got %d bytes", len(out))
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

func importIssues(jsonFile string) (int, error) {
	logf("importIssues: reading %s", jsonFile)
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return 0, fmt.Errorf("reading JSON file: %w", err)
	}
	logf("importIssues: read %d bytes", len(data))

	var issues []proposedIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		logf("importIssues: JSON parse error: %v", err)
		return 0, fmt.Errorf("parsing JSON: %w", err)
	}

	logf("importIssues: parsed %d proposed issue(s)", len(issues))
	for i, issue := range issues {
		logf("importIssues: [%d] title=%q dep=%d", i, issue.Title, issue.Dependency)
	}

	// Pass 1: create all issues and collect their beads IDs.
	createdIDs := make(map[int]string)
	for _, issue := range issues {
		logf("importIssues: creating task %d: %s", issue.Index, issue.Title)
		out, err := bdCreateTask(issue.Title, issue.Description)
		if err != nil {
			logf("importIssues: bd create failed for %q: %v", issue.Title, err)
			continue
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &created); err == nil && created.ID != "" {
			createdIDs[issue.Index] = created.ID
			logf("importIssues: created task %d -> beads id=%s", issue.Index, created.ID)
		} else {
			logf("importIssues: bd create returned unparseable output for %q: %s", issue.Title, string(out))
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
			logf("importIssues: skipping dependency %d->%d (child=%v parent=%v)", issue.Index, issue.Dependency, hasChild, hasParent)
			continue
		}
		logf("importIssues: linking %s (task %d) depends on %s (task %d)", childID, issue.Index, parentID, issue.Dependency)
		if err := bdAddDep(childID, parentID); err != nil {
			logf("importIssues: bd dep add failed: %s -> %s: %v", childID, parentID, err)
		}
	}

	if len(createdIDs) > 0 {
		beadsCommit("Add issues from measure")
	}
	logf("importIssues: %d of %d issue(s) imported", len(createdIDs), len(issues))

	return len(createdIDs), nil
}
