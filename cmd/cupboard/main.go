// Package main provides the cupboard CLI.
// Implements: prd-cupboard-core R2, R4, R5;
//
//	docs/ARCHITECTURE § CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dukaforge/crumbs/internal/sqlite"
	"github.com/dukaforge/crumbs/pkg/types"
)

var (
	// configFile is set by the --config flag.
	configFile string

	// cupboard is the global Cupboard instance, initialized on startup.
	cupboard types.Cupboard
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cupboard",
	Short: "Cupboard is a breadcrumb storage system",
	Long: `Cupboard is a storage system for managing development breadcrumbs,
trails, and related metadata. It provides a CLI interface for interacting
with the Cupboard storage backend.`,
	PersistentPreRunE: initCupboard,
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return closeCupboard()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default: .crumbs.yaml or ~/.crumbs/config.yaml)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the cupboard storage",
	Long:  `Initialize the cupboard storage backend using configuration from file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Cupboard is already initialized by PersistentPreRunE
		// Just print confirmation
		fmt.Println("Cupboard initialized successfully")
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cupboard v0.1.0")
	},
}

// initCupboard loads config and initializes the Cupboard.
func initCupboard(cmd *cobra.Command, args []string) error {
	// Skip init for version command
	if cmd.Name() == "version" {
		return nil
	}

	cfg, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create backend based on config
	backend := sqlite.NewBackend()
	if err := backend.Attach(cfg); err != nil {
		return fmt.Errorf("attach cupboard: %w", err)
	}

	cupboard = backend
	return nil
}

// closeCupboard detaches the Cupboard and releases resources.
func closeCupboard() error {
	if cupboard != nil {
		return cupboard.Detach()
	}
	return nil
}
