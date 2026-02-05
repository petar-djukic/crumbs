// Config file loading for crumbs CLI.
// Implements: prd-cupboard-core R1;
//             docs/ARCHITECTURE § Configuration.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/petardjukic/crumbs/pkg/types"
)

// loadConfig loads configuration from file.
// Search order: --config flag, ./.crumbs.yaml, ~/.crumbs/config.yaml
func loadConfig(configPath string) (types.Config, error) {
	v := viper.New()

	v.SetConfigType("yaml")

	if configPath != "" {
		// Use explicit config file from --config flag
		v.SetConfigFile(configPath)
	} else {
		// Search for config in standard locations
		v.SetConfigName(".crumbs")
		v.AddConfigPath(".")

		homeDir, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(homeDir, ".crumbs"))
			v.SetConfigName("config")
		}
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; return default config
			return defaultConfig(), nil
		}
		return types.Config{}, fmt.Errorf("read config: %w", err)
	}

	// Parse into Config struct
	cfg := types.Config{
		Backend: v.GetString("backend"),
		DataDir: v.GetString("datadir"),
	}

	// Parse Dolt config if present
	if v.IsSet("dolt") {
		cfg.DoltConfig = &types.DoltConfig{
			DSN:    v.GetString("dolt.dsn"),
			Branch: v.GetString("dolt.branch"),
		}
	}

	// Parse DynamoDB config if present
	if v.IsSet("dynamodb") {
		cfg.DynamoDBConfig = &types.DynamoDBConfig{
			TableName: v.GetString("dynamodb.tablename"),
			Region:    v.GetString("dynamodb.region"),
			Endpoint:  v.GetString("dynamodb.endpoint"),
		}
	}

	return cfg, nil
}

// defaultConfig returns the default configuration using SQLite backend.
func defaultConfig() types.Config {
	return types.Config{
		Backend: types.BackendSQLite,
		DataDir: ".crumbs",
	}
}
