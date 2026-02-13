// Init command for the cupboard CLI.
// Implements: prd009-cupboard-cli R2.1, R10;
//
//	prd010-configuration-directories R1.2, R1.6, R2, R5.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cupboard storage",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve config directory (flag > env > default) and ensure it exists
		// with a default config.yaml (prd010 R1.6).
		configDir, err := resolveConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			os.Exit(exitSysError)
		}
		if err := ensureConfigDir(configDir); err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			os.Exit(exitSysError)
		}
		if err := ensureDefaultConfigFile(configDir); err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			os.Exit(exitSysError)
		}

		// Attach backend (creates data directory via SQLite Attach).
		backend, err := attachBackend()
		if err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			os.Exit(exitSysError)
		}
		defer backend.Detach()

		dataDir, err := resolveDataDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			os.Exit(exitSysError)
		}

		fmt.Println("Cupboard initialized successfully")
		fmt.Println("  config:", configDir)
		fmt.Println("  data:  ", dataDir)
		return nil
	},
}
