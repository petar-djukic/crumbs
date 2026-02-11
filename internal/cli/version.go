// Implements: prd009-cupboard-cli (R2.2: version command);
//             rel01.0-uc004-scaffolding-validation (S2: version output).
package cli

import (
	"fmt"

	"github.com/mesh-intelligence/crumbs/pkg/crumbs"
	"github.com/spf13/cobra"
)

const modulePath = "github.com/mesh-intelligence/crumbs"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the cupboard version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "cupboard v%s\nmodule: %s\n", crumbs.Version, modulePath)
			return nil
		},
	}
}
