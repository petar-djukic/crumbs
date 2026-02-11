package types

import "errors"

// Config holds backend selection and connection parameters.
// Implements: prd001-cupboard-core R1.
type Config struct {
	Backend string // Backend type, e.g. "sqlite".
	DataDir string // Directory for the backend data files.
}

// Config validation errors (prd001-cupboard-core R7).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)
