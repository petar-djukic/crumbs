// Config file loading for crumbs CLI.
// Implements: prd010-configuration-directories R1, R2, R7;
//
//	prd001-cupboard-core R1;
//	docs/ARCHITECTURE § Configuration.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/mesh-intelligence/crumbs/internal/paths"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// Environment variables for directory overrides (per R1.3, R2.3).
const (
	envConfigDir = "CRUMBS_CONFIG_DIR"
	envDataDir   = "CRUMBS_DATA_DIR"
)

// loadConfig loads configuration with precedence rules.
// Per prd010-configuration-directories R7:
//  1. Determine configuration directory (flag > env > platform default)
//  2. Load config.yaml if it exists; use defaults otherwise
//  3. Determine data directory (flag > config > platform default)
//  4. Pass data directory to Cupboard via Config.DataDir
func loadConfig(configDirFlag, dataDirFlag string) (types.Config, error) {
	// Step 1: Resolve configuration directory (per R1.3)
	configDir, err := paths.ResolveConfigDir(configDirFlag, envConfigDir)
	if err != nil {
		return types.Config{}, fmt.Errorf("resolve config dir: %w", err)
	}

	// Step 2: Load config.yaml from config directory (per R7.1, R7.2)
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("config")
	v.AddConfigPath(configDir)

	// Also check current directory for .crumbs.yaml (backwards compatibility)
	v.AddConfigPath(".")
	v.SetConfigName(".crumbs")

	var configFromFile bool
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return types.Config{}, fmt.Errorf("read config: %w", err)
		}
		// Config file not found; use defaults (per R7.2)
	} else {
		configFromFile = true
	}

	// Parse into Config struct
	cfg := types.Config{
		Backend: v.GetString("backend"),
	}

	// Apply defaults if not set
	if cfg.Backend == "" {
		cfg.Backend = types.BackendSQLite
	}

	// Step 3: Resolve data directory with precedence (per R2.3)
	// flag > config > platform default
	configDataDir := v.GetString("data_dir")
	if configDataDir == "" {
		// Also check legacy key
		configDataDir = v.GetString("datadir")
	}

	dataDir, err := paths.ResolveDataDir(dataDirFlag, configDataDir)
	if err != nil {
		return types.Config{}, fmt.Errorf("resolve data dir: %w", err)
	}
	cfg.DataDir = dataDir

	// Ensure config directory exists for future writes (per R1.6)
	if !configFromFile {
		if err := paths.EnsureDir(configDir); err != nil {
			// Log warning but don't fail - config dir creation is best effort
			fmt.Fprintf(os.Stderr, "warning: could not create config directory %s: %v\n", configDir, err)
		}
	}

	return cfg, nil
}
