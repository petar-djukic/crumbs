// Update command for the cupboard CLI.
// Implements: prd009-cupboard-cli R5.5; rel02.1-uc001-issue-tracking-cli F8, S5.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var (
	updateStatusFlag string
	updateTitleFlag  string
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update crumb fields",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		crumbID := args[0]

		// At least one flag must be provided
		if updateStatusFlag == "" && updateTitleFlag == "" {
			fmt.Fprintln(os.Stderr, "update: at least one of --status or --title must be provided")
			os.Exit(exitUserError)
		}

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "update:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		table, err := backend.GetTable(types.TableCrumbs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Retrieve the crumb
		entity, err := table.Get(crumbID)
		if err != nil {
			if isEntityNotFound(err) {
				fmt.Fprintf(os.Stderr, "crumb %q not found\n", crumbID)
				os.Exit(exitUserError)
			}
			fmt.Fprintln(os.Stderr, "get crumb:", err)
			os.Exit(exitSysError)
		}

		crumb, ok := entity.(*types.Crumb)
		if !ok {
			fmt.Fprintln(os.Stderr, "update: entity is not a crumb")
			os.Exit(exitSysError)
		}

		// Apply changes
		if updateStatusFlag != "" {
			if err := crumb.SetState(updateStatusFlag); err != nil {
				if err == types.ErrInvalidState {
					fmt.Fprintf(os.Stderr, "invalid state %q (valid: draft, pending, ready, taken, pebble, dust)\n", updateStatusFlag)
					os.Exit(exitUserError)
				}
				fmt.Fprintln(os.Stderr, "set state:", err)
				os.Exit(exitUserError)
			}
		}

		if updateTitleFlag != "" {
			crumb.Name = updateTitleFlag
		}

		// Persist changes
		_, err = table.Set(crumb.CrumbID, crumb)
		if err != nil {
			fmt.Fprintln(os.Stderr, "update crumb:", err)
			os.Exit(exitUserError)
		}

		// Output result
		if flagJSON {
			out, err := json.MarshalIndent(crumb, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal JSON:", err)
				os.Exit(exitSysError)
			}
			fmt.Println(string(out))
		} else {
			fmt.Printf("Updated %s\n", crumbID)
		}

		return nil
	},
}

func init() {
	updateCmd.Flags().StringVar(&updateStatusFlag, "status", "", "set crumb state (draft, pending, ready, taken, pebble, dust)")
	updateCmd.Flags().StringVar(&updateTitleFlag, "title", "", "set crumb name")
}

// validStatesForError returns a comma-separated list of valid states for error messages.
func validStatesForError() string {
	states := []string{"draft", "pending", "ready", "taken", "pebble", "dust"}
	return strings.Join(states, ", ")
}
