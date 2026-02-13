// Delete command for the cupboard CLI.
// Implements: prd009-cupboard-cli R3.3; rel01.1-uc004-generic-table-cli S10-S11.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <table> <id>",
	Short: "Remove an entity by ID from a table",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName := args[0]
		entityID := args[1]

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "delete:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		table, err := backend.GetTable(tableName)
		if err != nil {
			if isTableNotFound(err) {
				fmt.Fprintf(os.Stderr, "unknown table %q (valid: %s)\n", tableName, validTableNamesStr)
				os.Exit(exitUserError)
			}
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		if err := table.Delete(entityID); err != nil {
			if isEntityNotFound(err) {
				fmt.Fprintf(os.Stderr, "entity %q not found in table %q\n", entityID, tableName)
				os.Exit(exitUserError)
			}
			fmt.Fprintf(os.Stderr, "delete entity: %s\n", err)
			os.Exit(exitSysError)
		}

		fmt.Printf("Deleted %s/%s\n", tableName, entityID)
		return nil
	},
}
