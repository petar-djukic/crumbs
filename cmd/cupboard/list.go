// List command for the cupboard CLI.
// Implements: prd009-cupboard-cli R3.4; rel01.1-uc004-generic-table-cli S7-S9.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <table> [key=value...]",
	Short: "Query entities from a table with optional filters",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName := args[0]

		filter, err := parseFilters(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(exitUserError)
		}

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "list:", err)
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

		results, err := table.Fetch(filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch entities: %s\n", err)
			os.Exit(exitSysError)
		}

		if results == nil {
			results = []any{}
		}

		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal JSON:", err)
			os.Exit(exitSysError)
		}

		fmt.Println(string(out))
		return nil
	},
}

// parseFilters converts key=value positional args into a types.Filter map.
// The "states" and "State" keys wrap the value in a []string for the Fetch filter.
func parseFilters(args []string) (types.Filter, error) {
	if len(args) == 0 {
		return nil, nil
	}

	filter := make(types.Filter, len(args))
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid filter %q (expected key=value)", arg)
		}
		key, value := parts[0], parts[1]

		if key == "states" || key == "State" {
			filter["states"] = []string{value}
		} else {
			filter[key] = value
		}
	}
	return filter, nil
}
