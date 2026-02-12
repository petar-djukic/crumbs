// Package types defines the Cupboard and Table interfaces, entity types, and
// standard errors for the Crumbs storage system.
// Implements: prd001-cupboard-core (Config, Cupboard, Table, errors).
// See docs/ARCHITECTURE.md § Main Interface.
package types

import "errors"

// Config holds backend selection and parameters for Cupboard.Attach.
type Config struct {
	Backend string `json:"backend" yaml:"backend"`
	DataDir string `json:"data_dir" yaml:"data_dir"`
}

// Supported backend names.
const (
	BackendSQLite = "sqlite"
)

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty          = errors.New("backend must not be empty")
	ErrBackendUnknown        = errors.New("unknown backend")
	ErrSyncStrategyUnknown   = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid      = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid  = errors.New("batch interval must be positive")
)

// knownBackends lists the backends that Validate accepts.
var knownBackends = map[string]bool{
	BackendSQLite: true,
}

// Validate checks that the Config is well-formed. It returns a sentinel error
// from this package on failure (prd001-cupboard-core R1.2, R1.3).
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}
	if !knownBackends[c.Backend] {
		return ErrBackendUnknown
	}
	return nil
}
