// Show command for the cupboard CLI.
// Implements: prd009-cupboard-cli R5.4; rel02.1-uc001-issue-tracking-cli F6, S3, S8.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display a crumb with full details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		crumbID := args[0]

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "show:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		table, err := backend.GetTable(types.TableCrumbs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Retrieve the crumb (prd001-cupboard-core R3.2, prd003-crumbs-interface R6).
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
			fmt.Fprintln(os.Stderr, "show: entity is not a crumb")
			os.Exit(exitSysError)
		}

		// Fetch metadata (comments) for this crumb (prd002-sqlite-backend R2.8).
		metadataTable, err := backend.GetTable(types.TableMetadata)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get metadata table:", err)
			os.Exit(exitSysError)
		}

		metadataFilter := types.Filter{
			"crumb_id": crumbID,
		}
		metadataResults, err := metadataTable.Fetch(metadataFilter)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fetch metadata:", err)
			os.Exit(exitSysError)
		}

		// Output result based on --json flag (prd009-cupboard-cli R7.1).
		if flagJSON {
			// For JSON output, include the crumb and metadata
			output := map[string]any{
				"crumb":    crumb,
				"metadata": metadataResults,
			}
			out, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal JSON:", err)
				os.Exit(exitSysError)
			}
			fmt.Println(string(out))
		} else {
			// Human-readable output (prd009-cupboard-cli R5.4).
			fmt.Printf("ID:        %s\n", crumb.CrumbID)
			fmt.Printf("Name:      %s\n", crumb.Name)
			fmt.Printf("State:     %s\n", crumb.State)
			fmt.Printf("Created:   %s\n", crumb.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:   %s\n", crumb.UpdatedAt.Format("2006-01-02 15:04:05"))

			// Display properties with names
			if len(crumb.Properties) > 0 {
				fmt.Println("\nProperties:")

				// Fetch property definitions to map IDs to names
				propTable, err := backend.GetTable(types.TableProperties)
				if err == nil {
					propEntities, err := propTable.Fetch(nil)
					if err == nil {
						propIDToName := make(map[string]string)
						for _, entity := range propEntities {
							prop, ok := entity.(*types.Property)
							if ok {
								propIDToName[prop.PropertyID] = prop.Name
							}
						}

						// Display each property with its name
						for propID, value := range crumb.Properties {
							propName := propIDToName[propID]
							if propName == "" {
								propName = propID
							}

							// Format value based on type
							var valueStr string
							switch v := value.(type) {
							case []interface{}:
								// List property (e.g., labels)
								strs := make([]string, len(v))
								for i, item := range v {
									strs[i] = fmt.Sprintf("%v", item)
								}
								valueStr = strings.Join(strs, ", ")
							case []string:
								valueStr = strings.Join(v, ", ")
							default:
								valueStr = fmt.Sprintf("%v", value)
							}

							fmt.Printf("  %s: %s\n", propName, valueStr)
						}
					}
				}
			}

			// Display metadata (comments)
			if len(metadataResults) > 0 {
				fmt.Println("\nComments:")
				for _, entity := range metadataResults {
					metadata, ok := entity.(*types.Metadata)
					if !ok {
						continue
					}
					fmt.Printf("  [%s] %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"), metadata.Content)
				}
			}
		}

		return nil
	},
}
