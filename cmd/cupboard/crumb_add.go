// Crumb add command creates a new work item.
// Implements: prd003-crumbs-interface R3 (creating crumbs);
//
//	docs/ARCHITECTURE § CLI.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var (
	crumbName  string
	crumbState string
)

var crumbAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a new crumb",
	Long: `Add creates a new crumb (work item) with the specified name.

The crumb is created with state "draft" by default.

Example:
  cupboard crumb add --name "Implement feature X"
  cupboard crumb add --name "Fix bug" --state pending
  cupboard crumb add --name "Review PR" --json`,
	RunE: runCrumbAdd,
}

func init() {
	crumbAddCmd.Flags().StringVar(&crumbName, "name", "", "name for the crumb (required)")
	crumbAddCmd.Flags().StringVar(&crumbState, "state", "", "initial state (default: draft)")
	_ = crumbAddCmd.MarkFlagRequired("name")
}

func runCrumbAdd(cmd *cobra.Command, args []string) error {
	table, err := cupboard.GetTable(types.CrumbsTable)
	if err != nil {
		return fmt.Errorf("get crumbs table: %w", err)
	}

	crumb := &types.Crumb{
		Name:  crumbName,
		State: types.StateDraft, // Default state per prd003-crumbs-interface R2.2
	}

	// Override state if provided
	if crumbState != "" {
		if err := crumb.SetState(crumbState); err != nil {
			return fmt.Errorf("invalid state %q: %w", crumbState, err)
		}
	}

	id, err := table.Set("", crumb)
	if err != nil {
		return fmt.Errorf("create crumb: %w", err)
	}

	// Fetch the created crumb to get full details
	entity, err := table.Get(id)
	if err != nil {
		// Created but couldn't fetch; print ID only
		if jsonOutput {
			fmt.Printf("{\"CrumbID\": %q}\n", id)
		} else {
			fmt.Printf("Created crumb: %s\n", id)
		}
		return nil
	}

	savedCrumb, ok := entity.(*types.Crumb)
	if !ok {
		return fmt.Errorf("unexpected entity type")
	}

	if jsonOutput {
		output, err := json.MarshalIndent(savedCrumb, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal crumb: %w", err)
		}
		fmt.Println(string(output))
	} else {
		fmt.Printf("Created crumb: %s\n", savedCrumb.CrumbID)
	}

	return nil
}
