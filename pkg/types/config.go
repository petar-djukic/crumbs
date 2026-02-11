// Package types defines shared public types and interfaces for the Crumbs system.
// Implements: prd001-cupboard-core (R1: Configuration, R1.4: config validation errors);
//             prd010-configuration-directories (R9: Config Struct).
package types

import "errors"

// Supported backend values.
const BackendSQLite = "sqlite"

// Config holds the parameters needed to attach a Cupboard to a backend.
// The CLI reads config.yaml and constructs a Config to pass to Cupboard.Attach.
type Config struct {
	Backend string `json:"backend" yaml:"backend"`
	DataDir string `json:"data_dir" yaml:"data_dir"`
}

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrDataDirEmpty         = errors.New("data_dir must not be empty")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)

// knownBackends lists the backends that Validate accepts.
var knownBackends = map[string]bool{
	BackendSQLite: true,
}

// Validate checks the Config for missing or invalid fields.
// It returns ErrBackendEmpty if Backend is empty, ErrBackendUnknown if Backend
// is not a recognized value, and ErrDataDirEmpty if DataDir is empty for a
// backend that requires it.
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}
	if !knownBackends[c.Backend] {
		return ErrBackendUnknown
	}
	if c.Backend == BackendSQLite && c.DataDir == "" {
		return ErrDataDirEmpty
	}
	return nil
}
