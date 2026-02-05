// Delete command removes an entity by ID from a table.
// Implements: prd-cupboard-core R3 (Table.Delete);
//             docs/ARCHITECTURE § CLI.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/petardjukic/crumbs/pkg/types"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <table> <id>",
	Short: "Delete an entity by ID",
	Long: `Delete removes an entity from the specified table by its ID.

Valid table names: crumbs, trails, properties, metadata, links, stashes

Example:
  crumbs delete crumbs abc123
  crumbs delete trails def456`,
	Args: cobra.ExactArgs(2),
	RunE: runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	tableName := args[0]
	id := args[1]

	table, err := cupboard.GetTable(tableName)
	if err != nil {
		if err == types.ErrTableNotFound {
			return fmt.Errorf("unknown table %q (valid: crumbs, trails, properties, metadata, links, stashes)", tableName)
		}
		return fmt.Errorf("get table: %w", err)
	}

	if err := table.Delete(id); err != nil {
		if err == types.ErrNotFound {
			return fmt.Errorf("entity %q not found in table %q", id, tableName)
		}
		return fmt.Errorf("delete entity: %w", err)
	}

	fmt.Printf("Deleted %s/%s\n", tableName, id)
	return nil
}
