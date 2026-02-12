// Package types defines shared interfaces, structs, and error sentinels
// for the Crumbs system.
// Implements: prd001-cupboard-core (Config, Cupboard, Table interfaces, errors, table names);
//             prd003-crumbs-interface (Crumb struct);
//             prd004-properties-interface (Property, Category structs);
//             prd005-metadata-interface (Metadata struct);
//             prd006-trails-interface (Trail struct);
//             prd007-links-interface (Link struct, link type constants);
//             prd008-stash-interface (Stash struct).
//             See docs/ARCHITECTURE.md § Main Interfaces.
package types

import "errors"

// Config holds the configuration for attaching a Cupboard to a backend.
// Implements prd001-cupboard-core R1.
type Config struct {
	Backend      string        // Backend type selector (e.g. "sqlite")
	DataDir      string        // Directory for backend storage
	SQLiteConfig *SQLiteConfig // Backend-specific configuration (optional)
}

// SQLiteConfig holds SQLite-specific configuration options.
type SQLiteConfig struct {
	SyncStrategy  string // Sync strategy: "immediate", "on_close", "batch"
	BatchSize     int    // Number of operations per batch (batch strategy)
	BatchInterval int    // Milliseconds between batch flushes (batch strategy)
}

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)
