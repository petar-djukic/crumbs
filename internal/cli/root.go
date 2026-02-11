// Package cli implements the cupboard command-line interface.
// Implements: prd009-cupboard-cli (R1: Root command structure, R6: Global flags,
//             R7: Exit codes, R8: Output modes);
//             docs/ARCHITECTURE § System Components.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Exit codes (prd009-cupboard-cli R7).
const (
	exitSuccess   = 0
	exitUserError = 1
	exitSysError  = 2
)

// rootFlags holds global flag values accessible to all subcommands.
type rootFlags struct {
	configDir string
	dataDir   string
	jsonMode  bool
}

var flags rootFlags

// NewRootCmd creates the top-level "cupboard" command with global flags
// and all subcommands registered.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cupboard",
		Short: "A backend-agnostic storage tool for crumbs",
		Long:  "Cupboard manages crumbs, trails, properties, and related entities\nusing a backend-agnostic storage interface.",
		// Do not print usage on errors returned by subcommands.
		SilenceUsage: true,
	}

	// Global persistent flags (prd009-cupboard-cli R6).
	root.PersistentFlags().StringVar(&flags.configDir, "config-dir", "", "configuration directory (default: .crumbs)")
	root.PersistentFlags().StringVar(&flags.dataDir, "data-dir", "", "data directory (default: .crumbs-db)")
	root.PersistentFlags().BoolVar(&flags.jsonMode, "json", false, "output in JSON format")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd())

	return root
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(exitUserError)
	}
}

// resolveConfigDir returns the config directory from flag, env, or default.
func resolveConfigDir() string {
	if flags.configDir != "" {
		return flags.configDir
	}
	if v := os.Getenv("CRUMBS_CONFIG_DIR"); v != "" {
		return v
	}
	return ".crumbs"
}

// resolveDataDir returns the data directory from flag or default.
// The caller may further override this with a value from config.yaml.
func resolveDataDir() string {
	if flags.dataDir != "" {
		return flags.dataDir
	}
	return ""
}

// exitError prints the error to stderr and returns the given exit code.
func exitError(cmd *cobra.Command, code int, msg string) error {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
	return nil // unreachable
}
