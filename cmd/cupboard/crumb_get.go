// Crumb get command retrieves a work item by ID.
// Implements: prd003-crumbs-interface R6 (retrieving crumbs);
//
//	docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var crumbGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Retrieve a crumb by ID",
	Long: `Get retrieves a crumb (work item) by its ID.

Example:
  cupboard crumb get abc123
  cupboard crumb get abc123 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runCrumbGet,
}

func runCrumbGet(cmd *cobra.Command, args []string) error {
	id := args[0]

	table, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		return fmt.Errorf("get crumbs table: %w", err)
	}

	entity, err := table.Get(id)
	if err != nil {
		if err == types.ErrNotFound {
			return fmt.Errorf("crumb %q not found", id)
		}
		return fmt.Errorf("get crumb: %w", err)
	}

	crumb, ok := entity.(*types.Crumb)
	if !ok {
		return fmt.Errorf("unexpected entity type")
	}

	if jsonOutput {
		output, err := json.MarshalIndent(crumb, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal crumb: %w", err)
		}
		fmt.Println(string(output))
	} else {
		printCrumbDetails(crumb)
	}

	return nil
}

// printCrumbDetails prints crumb fields in human-readable format.
func printCrumbDetails(c *types.Crumb) {
	fmt.Printf("ID:        %s\n", c.CrumbID)
	fmt.Printf("Name:      %s\n", c.Name)
	fmt.Printf("State:     %s\n", c.State)
	fmt.Printf("Created:   %s\n", c.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:   %s\n", c.UpdatedAt.Format("2006-01-02 15:04:05"))
	if len(c.Properties) > 0 {
		fmt.Printf("Properties:\n")
		for k, v := range c.Properties {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
}
