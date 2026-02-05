// Set command creates or updates an entity in a table.
// Implements: prd-cupboard-core R3 (Table.Set);
//             docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/petardjukic/crumbs/pkg/types"
)

var setCmd = &cobra.Command{
	Use:   "set <table> <id> <json>",
	Short: "Create or update an entity",
	Long: `Set creates or updates an entity in the specified table.

If id is empty string (""), a new UUID v7 is generated.
If id is provided, updates the existing entity or creates if not found.

The json argument should be a valid JSON object with the entity fields.

Valid table names: crumbs, trails, properties, metadata, links, stashes

Example:
  crumbs set crumbs "" '{"Name":"My Task","State":"draft"}'
  crumbs set crumbs abc123 '{"CrumbID":"abc123","Name":"Updated Task","State":"ready"}'
  crumbs set trails "" '{"State":"active"}'`,
	Args: cobra.ExactArgs(3),
	RunE: runSet,
}

func runSet(cmd *cobra.Command, args []string) error {
	tableName := args[0]
	id := args[1]
	jsonData := args[2]

	table, err := cupboard.GetTable(tableName)
	if err != nil {
		if err == types.ErrTableNotFound {
			return fmt.Errorf("unknown table %q (valid: crumbs, trails, properties, metadata, links, stashes)", tableName)
		}
		return fmt.Errorf("get table: %w", err)
	}

	// Parse JSON into entity based on table type
	entity, err := parseEntityJSON(tableName, jsonData)
	if err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	resultID, err := table.Set(id, entity)
	if err != nil {
		return fmt.Errorf("set entity: %w", err)
	}

	// Fetch the saved entity to show the result
	saved, err := table.Get(resultID)
	if err != nil {
		// Entity was saved but retrieval failed; show ID only
		fmt.Printf("{\"id\": %q}\n", resultID)
		return nil
	}

	output, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		fmt.Printf("{\"id\": %q}\n", resultID)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

// parseEntityJSON unmarshals JSON into the appropriate entity type for the table.
func parseEntityJSON(tableName, jsonData string) (any, error) {
	switch tableName {
	case types.CrumbsTable:
		var entity types.Crumb
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	case types.TrailsTable:
		var entity types.Trail
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	case types.PropertiesTable:
		var entity types.Property
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	case types.MetadataTable:
		var entity types.Metadata
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	case types.LinksTable:
		var entity types.Link
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	case types.StashesTable:
		var entity types.Stash
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	default:
		// Fallback to generic map
		var entity map[string]any
		if err := json.Unmarshal([]byte(jsonData), &entity); err != nil {
			return nil, err
		}
		return entity, nil
	}
}
