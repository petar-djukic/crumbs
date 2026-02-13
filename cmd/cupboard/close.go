// Close command for the cupboard CLI.
// Implements: prd009-cupboard-cli R5.6; rel02.1-uc001-issue-tracking-cli F12-F13, S9-S10.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var closeCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a crumb by transitioning to pebble state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		crumbID := args[0]

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "close:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		table, err := backend.GetTable(types.TableCrumbs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Retrieve the crumb (prd003-crumbs-interface R6)
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
			fmt.Fprintln(os.Stderr, "close: entity is not a crumb")
			os.Exit(exitSysError)
		}

		// Transition to pebble state (prd003-crumbs-interface R4.3)
		if err := crumb.Pebble(); err != nil {
			if err == types.ErrInvalidTransition {
				fmt.Fprintf(os.Stderr, "close crumb: invalid state transition (must be in taken state, current: %s)\n", crumb.State)
				os.Exit(exitUserError)
			}
			fmt.Fprintln(os.Stderr, "close crumb:", err)
			os.Exit(exitUserError)
		}

		// Persist changes (prd003-crumbs-interface R4.5)
		_, err = table.Set(crumb.CrumbID, crumb)
		if err != nil {
			fmt.Fprintln(os.Stderr, "close crumb:", err)
			os.Exit(exitUserError)
		}

		// Output result (prd009-cupboard-cli R5.6, R7.1)
		if flagJSON {
			out, err := json.MarshalIndent(crumb, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal JSON:", err)
				os.Exit(exitSysError)
			}
			fmt.Println(string(out))
		} else {
			fmt.Printf("Closed %s\n", crumbID)
		}

		return nil
	},
}
