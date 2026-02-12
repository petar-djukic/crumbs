// Root command for the cupboard CLI.
// Implements: prd009-cupboard-cli (R1, R6); prd010-configuration-directories (R1, R2, R8).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesh-intelligence/crumbs/pkg/crumbs"
	"github.com/spf13/cobra"
)

// Exit codes per prd009-cupboard-cli R8.
const (
	exitSuccess   = 0
	exitUserError = 1
	exitSysError  = 2
)

// Default directory names per prd010-configuration-directories R1.2 and R2.2.
const (
	defaultConfigDirName = ".crumbs"
	defaultDataDirName   = ".crumbs-db"
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
}

// resolveDataDir returns the data directory path following prd010 R2.3 precedence:
// --data-dir flag > config.yaml data_dir > default $(CWD)/.crumbs-db.
func resolveDataDir() (string, error) {
	if flagDataDir != "" {
		return filepath.Abs(flagDataDir)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}
	return filepath.Join(cwd, defaultDataDirName), nil
}
