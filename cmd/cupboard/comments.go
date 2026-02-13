// Comments command for the cupboard CLI.
// Implements: prd009-cupboard-cli R5.7; rel02.1-uc001-issue-tracking-cli F10-F11, S7-S8.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var commentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "Manage comments on crumbs",
}

var commentsAddCmd = &cobra.Command{
	Use:   "add <id> <text>",
	Short: "Add a comment to a crumb",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		crumbID := args[0]
		text := args[1]

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "comments add:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		// Get crumbs table to validate the crumb exists (prd005-metadata-interface R4.2).
		crumbsTable, err := backend.GetTable(types.TableCrumbs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Validate that the crumb exists before creating the comment.
		_, err = crumbsTable.Get(crumbID)
		if err != nil {
			if isEntityNotFound(err) {
				fmt.Fprintf(os.Stderr, "crumb %q not found\n", crumbID)
				os.Exit(exitUserError)
			}
			fmt.Fprintln(os.Stderr, "get crumb:", err)
			os.Exit(exitSysError)
		}

		// Get metadata table to create the comment entry (prd005-metadata-interface R4.1).
		metadataTable, err := backend.GetTable(types.TableMetadata)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Create metadata entry with TableName="comments" (prd005-metadata-interface R3.4).
		metadata := &types.Metadata{
			CrumbID:   crumbID,
			TableName: types.SchemaComments,
			Content:   text,
			CreatedAt: time.Now(),
		}

		// Set with empty ID to generate UUID v7 (prd005-metadata-interface R4.1, R4.2).
		_, err = metadataTable.Set("", metadata)
		if err != nil {
			fmt.Fprintln(os.Stderr, "add comment:", err)
			os.Exit(exitUserError)
		}

		// Output success message (prd009-cupboard-cli R5.7).
		fmt.Printf("Added comment to %s\n", crumbID)
		return nil
	},
}

func init() {
	commentsCmd.AddCommand(commentsAddCmd)
}
