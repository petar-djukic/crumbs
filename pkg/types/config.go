package types

import "errors"

// Config holds the parameters needed to initialize a Cupboard backend.
// Implements: prd001-cupboard-core R1.
type Config struct {
	Backend string `json:"backend" yaml:"backend"`
	DataDir string `json:"data_dir" yaml:"data_dir"`
}

// Recognized backend names.
const (
	BackendSQLite = "sqlite"
)

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)

// knownBackends lists the recognized backend names for validation.
var knownBackends = map[string]bool{
	BackendSQLite: true,
}

// Validate checks Config fields and returns the first validation error
// encountered. It returns ErrBackendEmpty when Backend is empty,
// ErrBackendUnknown when Backend is not recognized, and ErrBackendEmpty
// (wrapped) when DataDir is empty for a backend that requires it.
// Implements: prd001-cupboard-core R1.2, R1.3.
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}
	if !knownBackends[c.Backend] {
		return ErrBackendUnknown
	}
	if c.Backend == BackendSQLite && c.DataDir == "" {
		return errors.New("data dir must not be empty for sqlite backend")
	}
	return nil
}
