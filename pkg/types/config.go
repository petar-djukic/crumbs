// Package types defines the public interfaces and entity structs for the
// Crumbs storage system. It contains no implementation; backends in
// internal/ satisfy these contracts.
//
// Implements: prd001-cupboard-core (Config, Cupboard, Table, errors, UUID v7);
//
//	docs/ARCHITECTURE § System Overview, § Main Interfaces.
package types

import "errors"

// Config selects a backend and provides backend-specific parameters.
// Implements prd001-cupboard-core R1.
type Config struct {
	Backend      string        `json:"backend" yaml:"backend"`
	DataDir      string        `json:"data_dir" yaml:"data_dir"`
	SQLiteConfig *SQLiteConfig `json:"sqlite_config,omitempty" yaml:"sqlite_config,omitempty"`
}

// SQLiteConfig holds parameters for the SQLite backend.
type SQLiteConfig struct {
	SyncStrategy  string `json:"sync_strategy,omitempty" yaml:"sync_strategy,omitempty"`
	BatchSize     int    `json:"batch_size,omitempty" yaml:"batch_size,omitempty"`
	BatchInterval int    `json:"batch_interval,omitempty" yaml:"batch_interval,omitempty"`
}

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty          = errors.New("backend must not be empty")
	ErrBackendUnknown        = errors.New("unknown backend")
	ErrSyncStrategyUnknown   = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid      = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid  = errors.New("batch interval must be positive")
)
