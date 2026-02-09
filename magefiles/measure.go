package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

//go:embed prompts/measure.tmpl
var measurePromptTmpl string

// measureConfig holds options for the Measure target.
type measureConfig struct {
	silence      bool
	limit        int
	appendPrompt string
	promptArg    string
	branch       string
}

func parseMeasureFlags() measureConfig {
	cfg := measureConfig{limit: 10}
	fs := flag.NewFlagSet("cobbler:measure", flag.ContinueOnError)
	fs.BoolVar(&cfg.silence, "silence", false, "suppress Claude output")
	fs.IntVar(&cfg.limit, "limit", 10, "max issues to propose")
	fs.StringVar(&cfg.appendPrompt, "append-prompt", "", "path to additional prompt file")
	fs.StringVar(&cfg.promptArg, "prompt", "", "user prompt text")
	fs.StringVar(&cfg.branch, "branch", "", "generation branch to work on")
	parseTargetFlags(fs)
	if cfg.branch == "" && fs.NArg() > 0 {
		cfg.branch = fs.Arg(0)
	}
	return cfg
}

// Measure assesses project state and proposes new tasks via Claude.
//
// Claude creates issues directly in beads using bd commands.
//
// Flags:
//
//	--silence          suppress Claude output
//	--limit N          max issues to propose (default 10)
//	--append-prompt F  path to additional prompt file
//	--prompt TEXT      user prompt text
//	--branch NAME      generation branch to work on
func (Cobbler) Measure() error {
	return measure(parseMeasureFlags())
}

func measure(cfg measureConfig) error {
	branch, err := resolveBranch(cfg.branch)
	if err != nil {
		return err
	}
	if err := ensureOnBranch(branch); err != nil {
		return fmt.Errorf("switching to branch: %w", err)
	}

	// Get existing issues.
	fmt.Println("Querying existing issues...")
	existingIssues := getExistingIssues()

	issueCount := countJSONArray(existingIssues)
	fmt.Printf("Found %d existing issue(s).\n", issueCount)
	fmt.Printf("Issue limit: %d\n", cfg.limit)
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
	prompt := buildMeasurePrompt(cfg.promptArg, existingIssues, cfg.limit, appendContent)

	if err := runClaude(prompt, cfg.silence); err != nil {
		return fmt.Errorf("running Claude: %w", err)
	}

	// Sync beads and commit.
	fmt.Println()
	fmt.Println("Syncing beads...")
	_ = exec.Command("bd", "sync").Run()
	_ = exec.Command("git", "add", ".beads/").Run()
	_ = exec.Command("git", "commit", "-m", "Add issues from measure", "--allow-empty").Run()

	fmt.Println()
	fmt.Println("Done.")
	return nil
}

func getExistingIssues() string {
	if _, err := exec.LookPath("bd"); err != nil {
		return "[]"
	}
	out, err := exec.Command("bd", "list", "--json").Output()
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
	UserInput      string
	AppendContent  string
}

func buildMeasurePrompt(userInput, existingIssues string, limit int, appendContent string) string {
	tmpl := template.Must(template.New("measure").Parse(measurePromptTmpl))
	data := measurePromptData{
		ExistingIssues: existingIssues,
		Limit:          limit,
		UserInput:      userInput,
		AppendContent:  appendContent,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("measure prompt template: %v", err))
	}
	return buf.String()
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
			defer func() { _ = jq.Wait() }()
		}
	}

	return cmd.Run()
}
