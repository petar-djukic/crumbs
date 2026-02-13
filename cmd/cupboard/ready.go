// Ready command for the cupboard CLI.
// Implements: prd009-cupboard-cli R5.2; rel02.1-uc001-issue-tracking-cli F7, S4.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var (
	readyLimit int
	readyType  string
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List crumbs that are ready for work",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "ready:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		table, err := backend.GetTable(types.TableCrumbs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Build filter for state=ready (prd009-cupboard-cli R5.2).
		filter := types.Filter{
			"states": []string{types.StateReady},
		}

		// Add type filter if provided (prd009-cupboard-cli R5.2).
		// Type is a property, so we need to look up the property ID.
		if readyType != "" {
			propTable, err := backend.GetTable(types.TableProperties)
			if err != nil {
				fmt.Fprintln(os.Stderr, "get properties table:", err)
				os.Exit(exitSysError)
			}

			propEntities, err := propTable.Fetch(nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "fetch properties:", err)
				os.Exit(exitSysError)
			}

			// Find the type property ID
			var typePropertyID string
			for _, entity := range propEntities {
				prop, ok := entity.(*types.Property)
				if !ok {
					continue
				}
				if prop.Name == types.PropertyType {
					typePropertyID = prop.PropertyID
					break
				}
			}

			if typePropertyID != "" {
				filter[typePropertyID] = readyType
			}
		}

		// Fetch crumbs with filter (prd003-crumbs-interface R9, R10).
		results, err := table.Fetch(filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch crumbs: %s\n", err)
			os.Exit(exitSysError)
		}

		if results == nil {
			results = []any{}
		}

		// Apply limit if specified (prd009-cupboard-cli R5.2).
		if readyLimit > 0 && len(results) > readyLimit {
			results = results[:readyLimit]
		}

		// Output result based on --json flag (prd009-cupboard-cli R7.1).
		if flagJSON {
			out, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal JSON:", err)
				os.Exit(exitSysError)
			}
			fmt.Println(string(out))
		} else {
			// Human-readable table output (prd009-cupboard-cli R7.2).
			if len(results) == 0 {
				fmt.Println("No ready crumbs found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tNAME\tPRIORITY")
			fmt.Fprintln(w, "--\t----\t----\t--------")

			for _, entity := range results {
				crumb, ok := entity.(*types.Crumb)
				if !ok {
					continue
				}

				// Extract type and priority properties for display
				crumbType := ""
				priority := ""

				// Look up property IDs by iterating properties
				for _, value := range crumb.Properties {
					// We need to determine which property this is
					// For now, we'll display the property values directly
					// A more robust solution would cache the property name-to-ID mapping
					if val, ok := value.(string); ok {
						if crumbType == "" {
							crumbType = val
						} else if priority == "" {
							priority = val
						}
					}
				}

				// Truncate ID for display
				displayID := crumb.CrumbID
				if len(displayID) > 8 {
					displayID = displayID[:8]
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", displayID, crumbType, crumb.Name, priority)
			}

			w.Flush()
			fmt.Printf("\nTotal: %d crumb(s)\n", len(results))
		}

		return nil
	},
}

func init() {
	readyCmd.Flags().IntVarP(&readyLimit, "limit", "n", 0, "maximum number of results (0 = no limit)")
	readyCmd.Flags().StringVar(&readyType, "type", "", "filter by crumb type property (e.g., task, epic)")
}
