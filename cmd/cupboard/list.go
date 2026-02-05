// List command queries entities from a table with optional filtering.
// Implements: prd-cupboard-core R3 (Table.Fetch);
//             docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/petardjukic/crumbs/pkg/types"
)

var listCmd = &cobra.Command{
	Use:   "list <table> [filter...]",
	Short: "List entities with optional filter",
	Long: `List queries entities from the specified table with optional filters.

Filters are specified as key=value pairs. Multiple filters are ANDed together.
An empty filter returns all entities in the table.

Valid table names: crumbs, trails, properties, metadata, links, stashes

Example:
  crumbs list crumbs
  crumbs list crumbs State=ready
  crumbs list crumbs State=ready Name=MyTask
  crumbs list trails State=active`,
	Args: cobra.MinimumNArgs(1),
	RunE: runList,
}

func runList(cmd *cobra.Command, args []string) error {
	tableName := args[0]
	filterArgs := args[1:]

	table, err := cupboard.GetTable(tableName)
	if err != nil {
		if err == types.ErrTableNotFound {
			return fmt.Errorf("unknown table %q (valid: crumbs, trails, properties, metadata, links, stashes)", tableName)
		}
		return fmt.Errorf("get table: %w", err)
	}

	// Parse filter arguments
	filter := make(map[string]any)
	for _, arg := range filterArgs {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid filter %q (expected key=value)", arg)
		}
		key := parts[0]
		value := parts[1]

		// Try to parse as JSON for structured values, otherwise use as string
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			parsed = value // Use raw string if not valid JSON
		}
		filter[key] = parsed
	}

	entities, err := table.Fetch(filter)
	if err != nil {
		return fmt.Errorf("fetch entities: %w", err)
	}

	output, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal entities: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
