package types

import "errors"

// Config holds backend selection and parameters.
// Implements: prd001-cupboard-core R1.
type Config struct {
	Backend string `json:"backend" yaml:"backend"`
	DataDir string `json:"data_dir" yaml:"data_dir"`
}

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)

// recognizedBackends lists the backend types that Config.Validate accepts.
var recognizedBackends = map[string]bool{
	"sqlite": true,
}

// Validate checks that the Config fields are consistent and returns the first
// error found. It returns ErrBackendEmpty when Backend is empty,
// ErrBackendUnknown when Backend is not a recognized value, and
// ErrBackendEmpty (wrapping a DataDir message) when DataDir is required but
// empty (prd001-cupboard-core R1.2, R1.3).
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}
	if !recognizedBackends[c.Backend] {
		return ErrBackendUnknown
	}
	if c.Backend == "sqlite" && c.DataDir == "" {
		return errors.New("data_dir must not be empty for sqlite backend")
	}
	return nil
}
