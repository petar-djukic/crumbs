// Get command retrieves an entity by ID from a table.
// Implements: prd-cupboard-core R3 (Table.Get);
//             docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/petardjukic/crumbs/pkg/types"
)

var getCmd = &cobra.Command{
	Use:   "get <table> <id>",
	Short: "Get an entity by ID",
	Long: `Get retrieves an entity from the specified table by its ID.

Valid table names: crumbs, trails, properties, metadata, links, stashes

Example:
  crumbs get crumbs abc123
  crumbs get trails def456`,
	Args: cobra.ExactArgs(2),
	RunE: runGet,
}

func runGet(cmd *cobra.Command, args []string) error {
	tableName := args[0]
	id := args[1]

	table, err := cupboard.GetTable(tableName)
	if err != nil {
		if err == types.ErrTableNotFound {
			return fmt.Errorf("unknown table %q (valid: crumbs, trails, properties, metadata, links, stashes)", tableName)
		}
		return fmt.Errorf("get table: %w", err)
	}

	entity, err := table.Get(id)
	if err != nil {
		if err == types.ErrNotFound {
			return fmt.Errorf("entity %q not found in table %q", id, tableName)
		}
		return fmt.Errorf("get entity: %w", err)
	}

	output, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal entity: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
