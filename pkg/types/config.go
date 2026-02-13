// Package types defines shared interfaces and types for the Crumbs storage system.
// Implements: prd001-cupboard-core (Config, Cupboard, Table interfaces);
//             docs/ARCHITECTURE § Main Interface.
package types

import "errors"

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrDataDirEmpty         = errors.New("data directory must not be empty")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)

// backendSQLite is the recognized backend name.
const backendSQLite = "sqlite"

// Config holds backend selection and backend-specific parameters (prd001-cupboard-core R1.1).
type Config struct {
	Backend      string        `json:"backend" yaml:"backend"`
	DataDir      string        `json:"data_dir" yaml:"data_dir"`
	SQLiteConfig *SQLiteConfig `json:"sqlite_config,omitempty" yaml:"sqlite_config,omitempty"`
}

// SQLiteConfig holds SQLite-specific tuning parameters.
type SQLiteConfig struct {
	SyncStrategy  string `json:"sync_strategy" yaml:"sync_strategy"`
	BatchSize     int    `json:"batch_size" yaml:"batch_size"`
	BatchInterval int    `json:"batch_interval" yaml:"batch_interval"`
}

// Validate checks that Config fields satisfy prd001-cupboard-core R1.2 and R1.3.
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}
	if c.Backend != backendSQLite {
		return ErrBackendUnknown
	}
	if c.Backend == backendSQLite && c.DataDir == "" {
		return ErrDataDirEmpty
	}
	if c.SQLiteConfig != nil {
		if err := c.SQLiteConfig.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// validSyncStrategies lists the recognized sync strategies (prd002-sqlite-backend R16).
var validSyncStrategies = map[string]bool{
	"immediate": true,
	"on_close":  true,
	"batch":     true,
	"":          true, // empty means use default
}

// Validate checks that SQLiteConfig fields are valid.
func (sc SQLiteConfig) Validate() error {
	if !validSyncStrategies[sc.SyncStrategy] {
		return ErrSyncStrategyUnknown
	}
	if sc.BatchSize < 0 {
		return ErrBatchSizeInvalid
	}
	if sc.BatchInterval < 0 {
		return ErrBatchIntervalInvalid
	}
	return nil
}
