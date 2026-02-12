// Set command for the cupboard CLI.
// Implements: prd009-cupboard-cli R3.2; rel01.1-uc004-generic-table-cli S4-S6.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <table> <id> <json>",
	Short: "Create or update an entity in a table",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName := args[0]
		entityID := args[1]
		jsonPayload := args[2]

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "set:", err)
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

		entity, err := parseEntityJSON(tableName, []byte(jsonPayload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse JSON: %s\n", err)
			os.Exit(exitUserError)
		}

		savedID, err := table.Set(entityID, entity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "set entity: %s\n", err)
			os.Exit(exitUserError)
		}

		result, err := table.Get(savedID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get saved entity:", err)
			os.Exit(exitSysError)
		}

		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal JSON:", err)
			os.Exit(exitSysError)
		}

		fmt.Println(string(out))
		return nil
	},
}

// parseEntityJSON unmarshals JSON data into the correct entity struct based on
// the table name. Each table maps to a specific entity type per prd001-cupboard-core R2.5.
func parseEntityJSON(tableName string, data []byte) (any, error) {
	switch tableName {
	case types.TableCrumbs:
		var e types.Crumb
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableTrails:
		var e types.Trail
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableProperties:
		var e types.Property
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableMetadata:
		var e types.Metadata
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableLinks:
		var e types.Link
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableStashes:
		var e types.Stash
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unknown table %q", tableName)
	}
}
