// Root command for the cupboard CLI.
// Implements: prd009-cupboard-cli (R1, R6); prd010-configuration-directories (R1, R2, R8).
package main

import (
	"github.com/mesh-intelligence/crumbs/internal/paths"
	"github.com/mesh-intelligence/crumbs/pkg/crumbs"
	"github.com/spf13/cobra"
)

// Exit codes per prd009-cupboard-cli R8.
const (
	exitSuccess   = 0
	exitUserError = 1
	exitSysError  = 2
)

// Global flag values.
var (
	flagConfigDir string
	flagDataDir   string
	flagJSON      bool
)

var rootCmd = &cobra.Command{
	Use:     "cupboard",
	Short:   "Cupboard is a local-first issue tracker",
	Version: crumbs.Version,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfigDir, "config-dir", "", "configuration directory (default: $(CWD)/.crumbs)")
	rootCmd.PersistentFlags().StringVar(&flagDataDir, "data-dir", "", "data directory (default: $(CWD)/.crumbs-db)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output as JSON")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
}

// resolveDataDir returns the data directory path following prd010 R2.3 precedence:
// --data-dir flag > config.yaml data_dir > CRUMBS_DATA_DIR env > default $(CWD)/.crumbs-db.
// The configYAMLValue parameter is empty until config.yaml loading is wired in.
func resolveDataDir() (string, error) {
	return paths.ResolveDataDir(flagDataDir, "" /* configYAMLValue */)
}

// resolveConfigDir returns the configuration directory following prd010 R1.3 precedence:
// --config-dir flag > CRUMBS_CONFIG_DIR env > DefaultConfigDir().
func resolveConfigDir() (string, error) {
	return paths.ResolveConfigDir(flagConfigDir)
}
