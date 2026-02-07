// Crumb list command queries all work items.
// Implements: prd-crumbs-interface R10 (querying crumbs);
//
//	docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var (
	listState string
	listLimit int
)

var crumbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all crumbs",
	Long: `List fetches all crumbs (work items) and displays them.

Use --state to filter by state.

Example:
  cupboard crumb list
  cupboard crumb list --state ready
  cupboard crumb list --limit 10
  cupboard crumb list --json`,
	RunE: runCrumbList,
}

func init() {
	crumbListCmd.Flags().StringVar(&listState, "state", "", "filter by state (draft, pending, ready, taken, completed, failed, archived)")
	crumbListCmd.Flags().IntVar(&listLimit, "limit", 0, "maximum number of results (0 = no limit)")
}

func runCrumbList(cmd *cobra.Command, args []string) error {
	table, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		return fmt.Errorf("get crumbs table: %w", err)
	}

	// Build filter using struct field names (backend uses crumbFieldToColumn mapping)
	filter := make(map[string]any)
	if listState != "" {
		filter["State"] = listState
	}
	// Note: limit not yet implemented in backend

	entities, err := table.Fetch(filter)
	if err != nil {
		return fmt.Errorf("fetch crumbs: %w", err)
	}

	// Convert to crumbs
	crumbs := make([]*types.Crumb, len(entities))
	for i, entity := range entities {
		crumbs[i] = entity.(*types.Crumb)
	}

	if jsonOutput {
		output, err := json.MarshalIndent(crumbs, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal crumbs: %w", err)
		}
		fmt.Println(string(output))
	} else {
		printCrumbTable(crumbs)
	}

	return nil
}

// printCrumbTable prints crumbs in a human-readable table format.
func printCrumbTable(crumbs []*types.Crumb) {
	if len(crumbs) == 0 {
		fmt.Println("No crumbs found.")
		return
	}

	var sb strings.Builder
	w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "ID\tNAME\tSTATE\tCREATED")
	fmt.Fprintln(w, "--\t----\t-----\t-------")
	for _, c := range crumbs {
		// Truncate name if too long
		name := c.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		// Truncate ID to first 8 chars for readability
		shortID := c.CrumbID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			shortID,
			name,
			c.State,
			c.CreatedAt.Format("2006-01-02"),
		)
	}
	w.Flush()

	// Print output, trimming trailing whitespace from each line
	output := sb.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fmt.Println(strings.TrimRight(line, " "))
	}

	fmt.Printf("Total: %d crumb(s)\n", len(crumbs))
}
