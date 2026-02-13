// Get command for the cupboard CLI.
// Implements: prd009-cupboard-cli R3.1; rel01.1-uc004-generic-table-cli S1-S3.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <table> <id>",
	Short: "Retrieve an entity by ID from a table",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName := args[0]
		entityID := args[1]

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "get:", err)
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

		entity, err := table.Get(entityID)
		if err != nil {
			if isEntityNotFound(err) {
				fmt.Fprintf(os.Stderr, "entity %q not found in table %q\n", entityID, tableName)
				os.Exit(exitUserError)
			}
			fmt.Fprintln(os.Stderr, "get entity:", err)
			os.Exit(exitSysError)
		}

		out, err := json.MarshalIndent(entity, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal JSON:", err)
			os.Exit(exitSysError)
		}

		fmt.Println(string(out))
		return nil
	},
}
