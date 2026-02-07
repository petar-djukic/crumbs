// Crumb delete command removes a work item by ID.
// Implements: prd-crumbs-interface R8 (deleting crumbs);
//
//	docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var crumbDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a crumb by ID",
	Long: `Delete removes a crumb (work item) by its ID.

This is a hard delete that removes the crumb and all associated data.

Example:
  cupboard crumb delete abc123
  cupboard crumb delete abc123 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runCrumbDelete,
}

func runCrumbDelete(cmd *cobra.Command, args []string) error {
	id := args[0]

	table, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		return fmt.Errorf("get crumbs table: %w", err)
	}

	if err := table.Delete(id); err != nil {
		if err == types.ErrNotFound {
			return fmt.Errorf("crumb %q not found", id)
		}
		return fmt.Errorf("delete crumb: %w", err)
	}

	if jsonOutput {
		result := map[string]string{
			"deleted": id,
			"status":  "success",
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
		fmt.Println(string(output))
	} else {
		fmt.Printf("Deleted crumb: %s\n", id)
	}

	return nil
}
