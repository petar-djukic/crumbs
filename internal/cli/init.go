// Implements: prd009-cupboard-cli (R2.1: init command, R10: Init behavior);
//             prd010-configuration-directories (R1: Config directory, R2: Data directory,
//             R8: Directory resolution);
//             rel01.1-uc001-go-install (F3: init).
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configFile holds the structure written to config.yaml.
type configFile struct {
	Backend string `yaml:"backend"`
	DataDir string `yaml:"data_dir,omitempty"`
}

// defaultDataDir is used when no data directory is specified by flag or config.
const defaultDataDir = ".crumbs-db"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize cupboard storage",
		Long:  "Create configuration and data directories, then initialize the storage backend.",
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	configDir := resolveConfigDir()
	dataDir := resolveDataDir()

	// Load data_dir from existing config.yaml if flag was not provided.
	if dataDir == "" {
		dataDir = loadDataDirFromConfig(configDir)
	}
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	// Create config directory (prd010 R1.6).
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return exitError(cmd, exitSysError, fmt.Sprintf("create config directory: %s", err))
	}

	// Write config.yaml if missing (prd010 R8.3).
	configPath := filepath.Join(configDir, "config.yaml")
	if err := writeConfigIfMissing(configPath, dataDir); err != nil {
		return exitError(cmd, exitSysError, fmt.Sprintf("write config: %s", err))
	}

	// Initialize the data directory via Cupboard.Attach then Detach.
	cfg := types.Config{
		Backend: types.BackendSQLite,
		DataDir: dataDir,
	}

	cupboard := sqlite.NewBackend()
	if err := cupboard.Attach(cfg); err != nil {
		return exitError(cmd, exitSysError, fmt.Sprintf("initialize storage: %s", err))
	}
	if err := cupboard.Detach(); err != nil {
		return exitError(cmd, exitSysError, fmt.Sprintf("finalize storage: %s", err))
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Cupboard initialized successfully")
	return nil
}

// writeConfigIfMissing creates config.yaml with default values if the file
// does not exist. If it already exists, the function returns nil (idempotent).
func writeConfigIfMissing(path, dataDir string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	cfg := configFile{
		Backend: types.BackendSQLite,
		DataDir: dataDir,
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// loadDataDirFromConfig reads data_dir from an existing config.yaml.
// Returns empty string if the file does not exist or cannot be read.
func loadDataDirFromConfig(configDir string) string {
	path := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.DataDir
}
