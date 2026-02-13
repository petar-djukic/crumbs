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

// configDataDir holds the data_dir value loaded from config.yaml.
// Set by PersistentPreRunE so all subcommands can use it.
var configDataDir string

var rootCmd = &cobra.Command{
	Use:     "cupboard",
	Short:   "Cupboard is a local-first issue tracker",
	Version: crumbs.Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := resolveConfigDir()
		if err != nil {
			return err
		}

		cfg, err := loadConfig(configDir)
		if err != nil {
			return err
		}

		configDataDir = cfg.GetString(cfgKeyDataDir)
		return nil
	},
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
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
}

// resolveDataDir returns the data directory path following prd010 R2.3 precedence:
// --data-dir flag > config.yaml data_dir > CRUMBS_DATA_DIR env > default $(CWD)/.crumbs-db.
func resolveDataDir() (string, error) {
	return paths.ResolveDataDir(flagDataDir, configDataDir)
}

// resolveConfigDir returns the configuration directory following prd010 R1.3 precedence:
// --config-dir flag > CRUMBS_CONFIG_DIR env > DefaultConfigDir().
func resolveConfigDir() (string, error) {
	return paths.ResolveConfigDir(flagConfigDir)
}
