// Config loading for the cupboard CLI.
// Implements: prd010-configuration-directories (R1.4, R1.5, R1.6, R8);
//
//	rel01.1-uc003-configuration-loading (F4, F6, S6-S8).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	configFileName = "config"
	configFileType = "yaml"
	configFileExt  = "config.yaml"

	// Config keys matching prd010 R1.5.
	cfgKeyBackend = "backend"
	cfgKeyDataDir = "data_dir"

	// Default backend per prd010 R1.5.
	defaultBackend = "sqlite"
)

// defaultConfigYAML is the content written to config.yaml on first run
// per prd010 R1.6.
const defaultConfigYAML = `# Cupboard CLI configuration
# See prd010-configuration-directories for details.

# Backend selection
backend: sqlite

# Data directory (optional; overridable by --data-dir flag)
# data_dir:
`

// loadConfig reads config.yaml from the resolved config directory using Viper.
// It creates the config directory and a default config.yaml on first run.
// A missing config.yaml is not an error (prd010 R8.2).
func loadConfig(configDir string) (*viper.Viper, error) {
	if err := ensureConfigDir(configDir); err != nil {
		return nil, fmt.Errorf("ensure config dir: %w", err)
	}

	if err := ensureDefaultConfigFile(configDir); err != nil {
		return nil, fmt.Errorf("ensure default config: %w", err)
	}

	v := viper.New()
	v.SetDefault(cfgKeyBackend, defaultBackend)
	v.SetConfigName(configFileName)
	v.SetConfigType(configFileType)
	v.AddConfigPath(configDir)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Missing config.yaml is not an error (prd010 R8.2).
			return v, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	return v, nil
}

// ensureConfigDir creates the config directory if it does not exist (prd010 R1.6).
func ensureConfigDir(configDir string) error {
	return os.MkdirAll(configDir, 0o755)
}

// ensureDefaultConfigFile creates a default config.yaml if the file does not
// exist in the config directory (prd010 R1.6, R8.3).
func ensureDefaultConfigFile(configDir string) error {
	path := filepath.Join(configDir, configFileExt)

	_, err := os.Stat(path)
	if err == nil {
		// File already exists.
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat config file: %w", err)
	}

	return os.WriteFile(path, []byte(defaultConfigYAML), 0o644)
}
