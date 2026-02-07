// Package types defines the public API for the Crumbs storage system.
// Implements: prd-cupboard-core (Config, DoltConfig, DynamoDBConfig);
//
//	docs/ARCHITECTURE § Cupboard API.
package types

import (
	"errors"
	"fmt"
)

// Backend constants identify supported storage backends.
const (
	BackendSQLite = "sqlite"
)

// Config holds configuration for initializing a Cupboard instance.
type Config struct {
	// Backend type: "sqlite"
	Backend string

	// DataDir is the directory for the SQLite backend.
	DataDir string

	// SQLiteConfig holds SQLite-specific settings; nil uses defaults.
	SQLiteConfig *SQLiteConfig
}

// Sync strategy constants for SQLite backend.
const (
	// SyncImmediate syncs every write to JSONL immediately (default).
	// Safest option: JSONL is always current with SQLite.
	SyncImmediate = "immediate"

	// SyncOnClose defers JSONL writes until Detach is called.
	// Higher performance but data loss risk on crash.
	SyncOnClose = "on_close"

	// SyncBatch batches JSONL writes by count or interval.
	// Balance between performance and durability.
	SyncBatch = "batch"
)

// SQLiteConfig holds configuration for the SQLite backend.
type SQLiteConfig struct {
	// SyncStrategy controls when writes are persisted to JSONL files.
	// Options: "immediate" (default), "on_close", "batch".
	// - immediate: every write syncs to JSONL immediately (safest)
	// - on_close: defer JSONL writes until Detach (fastest, risk of data loss)
	// - batch: batch writes by count or time interval
	SyncStrategy string

	// BatchSize is the number of writes to batch before syncing to JSONL.
	// Only used when SyncStrategy is "batch". Default is 100.
	BatchSize int

	// BatchInterval is the maximum time between JSONL syncs.
	// Only used when SyncStrategy is "batch". Default is 5 seconds.
	// Writes sync when either BatchSize or BatchInterval is reached.
	BatchInterval int
}

// Validation errors.
var (
	ErrBackendEmpty         = errors.New("backend cannot be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive when using batch sync strategy")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive when using batch sync strategy")
)

// Validate checks that the Config is well-formed.
// It returns an error if Backend is empty, unrecognized,
// or if required backend-specific config is missing.
func (c Config) Validate() error {
	if c.Backend == "" {
		return ErrBackendEmpty
	}

	switch c.Backend {
	case BackendSQLite:
		if c.SQLiteConfig != nil {
			if err := c.SQLiteConfig.Validate(); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrBackendUnknown, c.Backend)
	}
}

// Validate checks that the SQLiteConfig is well-formed.
func (c SQLiteConfig) Validate() error {
	switch c.SyncStrategy {
	case "", SyncImmediate:
		// Empty defaults to immediate; no additional validation needed
		return nil
	case SyncOnClose:
		// No additional parameters needed
		return nil
	case SyncBatch:
		// Batch mode requires valid size or interval (at least one must be positive)
		if c.BatchSize < 0 {
			return ErrBatchSizeInvalid
		}
		if c.BatchInterval < 0 {
			return ErrBatchIntervalInvalid
		}
		// At least one of BatchSize or BatchInterval must be set for batch mode
		if c.BatchSize == 0 && c.BatchInterval == 0 {
			return fmt.Errorf("%w: must set BatchSize or BatchInterval", ErrBatchSizeInvalid)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrSyncStrategyUnknown, c.SyncStrategy)
	}
}

// GetSyncStrategy returns the effective sync strategy, defaulting to immediate.
func (c *SQLiteConfig) GetSyncStrategy() string {
	if c == nil || c.SyncStrategy == "" {
		return SyncImmediate
	}
	return c.SyncStrategy
}

// GetBatchSize returns the effective batch size, defaulting to 100.
func (c *SQLiteConfig) GetBatchSize() int {
	if c == nil || c.BatchSize <= 0 {
		return 100
	}
	return c.BatchSize
}

// GetBatchInterval returns the effective batch interval in seconds, defaulting to 5.
func (c *SQLiteConfig) GetBatchInterval() int {
	if c == nil || c.BatchInterval <= 0 {
		return 5
	}
	return c.BatchInterval
}
