// Version command for the cupboard CLI.
// Implements: prd009-cupboard-cli R2.2; rel01.0-uc004-scaffolding-validation S3.
package main

import (
	"fmt"

	"github.com/mesh-intelligence/crumbs/pkg/crumbs"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the cupboard version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cupboard", crumbs.Version)
	},
}
