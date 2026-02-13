// Create command for the cupboard CLI.
// Implements: prd009-cupboard-cli R5.3; rel02.1-uc001-issue-tracking-cli F2-F4, S1-S2.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
)

var (
	createType        string
	createTitle       string
	createDescription string
	createLabels      string
	createOwner       string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new crumb with issue-tracking fields",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate required --title flag (prd009-cupboard-cli R5.3).
		if createTitle == "" {
			fmt.Fprintln(os.Stderr, "create: --title is required")
			os.Exit(exitUserError)
		}

		// Validate --type against known categories (acceptance criteria).
		validTypes := []string{"task", "epic", "bug", "chore"}
		if createType != "" {
			valid := false
			for _, vt := range validTypes {
				if createType == vt {
					valid = true
					break
				}
			}
			if !valid {
				fmt.Fprintf(os.Stderr, "invalid type %q (valid: %s)\n", createType, strings.Join(validTypes, ", "))
				os.Exit(exitUserError)
			}
		}

		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		table, err := backend.GetTable(types.TableCrumbs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get table:", err)
			os.Exit(exitSysError)
		}

		// Look up property IDs for built-in properties (prd004-properties-interface R8).
		// The backend stores properties by property_id, not by name.
		propTable, err := backend.GetTable(types.TableProperties)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get properties table:", err)
			os.Exit(exitSysError)
		}

		// Fetch all properties and build a name-to-ID map.
		propEntities, err := propTable.Fetch(nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fetch properties:", err)
			os.Exit(exitSysError)
		}

		propNameToID := make(map[string]string)
		for _, entity := range propEntities {
			prop, ok := entity.(*types.Property)
			if !ok {
				continue
			}
			propNameToID[prop.Name] = prop.PropertyID
		}

		// Create a new crumb with state=draft (prd003-crumbs-interface R3.2).
		// Initialize properties map to avoid nil map when setting properties.
		crumb := &types.Crumb{
			Name:       createTitle,
			State:      types.StateDraft,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Properties: make(map[string]any),
		}

		// Set type property if provided (prd009-cupboard-cli R5.3).
		if createType != "" {
			if propID, ok := propNameToID[types.PropertyType]; ok {
				crumb.Properties[propID] = createType
			}
		}

		// Set description property if provided (prd009-cupboard-cli R5.3).
		if createDescription != "" {
			if propID, ok := propNameToID[types.PropertyDescription]; ok {
				crumb.Properties[propID] = createDescription
			}
		}

		// Set owner property if provided.
		if createOwner != "" {
			if propID, ok := propNameToID[types.PropertyOwner]; ok {
				crumb.Properties[propID] = createOwner
			}
		}

		// Set labels property if provided (prd004-properties-interface R3, R9).
		// Labels is a list type; parse comma-separated values.
		if createLabels != "" {
			if propID, ok := propNameToID[types.PropertyLabels]; ok {
				labels := strings.Split(createLabels, ",")
				for i := range labels {
					labels[i] = strings.TrimSpace(labels[i])
				}
				crumb.Properties[propID] = labels
			}
		}

		// Create the crumb via Table.Set with empty ID to generate UUID v7
		// (prd001-cupboard-core R8, prd003-crumbs-interface R3).
		crumbID, err := table.Set("", crumb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create crumb: %s\n", err)
			os.Exit(exitUserError)
		}

		// Retrieve the saved crumb to get the full object with ID and timestamps.
		result, err := table.Get(crumbID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get created crumb:", err)
			os.Exit(exitSysError)
		}

		// Output result based on --json flag (prd009-cupboard-cli R7.1).
		if flagJSON {
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal JSON:", err)
				os.Exit(exitSysError)
			}
			fmt.Println(string(out))
		} else {
			// Human-readable output: "Created <type>: <id>" (prd009-cupboard-cli R5.3).
			typeStr := createType
			if typeStr == "" {
				typeStr = "crumb"
			}
			fmt.Printf("Created %s: %s\n", typeStr, crumbID)
		}

		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createType, "type", "", "crumb type (task, epic, bug, chore)")
	createCmd.Flags().StringVar(&createTitle, "title", "", "crumb title (required)")
	createCmd.Flags().StringVar(&createDescription, "description", "", "crumb description")
	createCmd.Flags().StringVar(&createLabels, "labels", "", "comma-separated labels")
	createCmd.Flags().StringVar(&createOwner, "owner", "", "crumb owner")

	// Mark title as required (prd009-cupboard-cli R5.3).
	createCmd.MarkFlagRequired("title")
}
