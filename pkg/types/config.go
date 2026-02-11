// Package types defines the Cupboard and Table interfaces, configuration,
// and sentinel errors for the Crumbs storage layer.
// Implements: prd001-cupboard-core (Config, Cupboard, Table, errors);
//             docs/ARCHITECTURE § Main Interfaces.
package types

import "errors"

// Config holds the settings needed to attach a Cupboard to a backend.
// Implements prd001-cupboard-core R1.
type Config struct {
	Backend string `json:"backend" yaml:"backend"`
	DataDir string `json:"data_dir" yaml:"data_dir"`

	// SQLite holds backend-specific settings when Backend is "sqlite".
	SQLite SQLiteConfig `json:"sqlite,omitempty" yaml:"sqlite,omitempty"`
}

// SQLiteConfig holds SQLite-specific configuration.
// Fields will be added by prd002-sqlite-backend.
type SQLiteConfig struct{}

// Recognized backend names.
const (
	BackendSQLite = "sqlite"
)

// Config validation errors (prd001-cupboard-core R1.4, R7).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)

// Validate checks that the Config is well-formed.
// It returns a sentinel error when a field is missing or unrecognized.
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}
	if c.Backend != BackendSQLite {
		return ErrBackendUnknown
	}
	if c.DataDir == "" {
		return ErrDataDirEmpty
	}
	return nil
}

// ErrDataDirEmpty is returned when DataDir is empty for a backend that requires it.
var ErrDataDirEmpty = errors.New("data directory must not be empty")
