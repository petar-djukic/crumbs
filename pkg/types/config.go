// Package types defines the Cupboard and Table interfaces, entity structs,
// and sentinel errors for the Crumbs storage system.
// Implements: prd001-cupboard-core (Config, Cupboard, Table interfaces, errors);
//             prd003-crumbs-interface (Crumb entity);
//             prd006-trails-interface (Trail entity);
//             prd007-links-interface (Link entity);
//             prd004-properties-interface (Property, Category entities);
//             prd005-metadata-interface (Metadata entity);
//             prd008-stash-interface (Stash entity).
//             See docs/ARCHITECTURE § Main Interfaces, § Entity Types.
package types

import "errors"

// Supported backend names.
const (
	BackendSQLite = "sqlite"
)

// Config validation errors (prd001-cupboard-core R1.4).
var (
	ErrBackendEmpty         = errors.New("backend must not be empty")
	ErrBackendUnknown       = errors.New("unknown backend")
	ErrDataDirEmpty         = errors.New("data directory must not be empty")
	ErrSyncStrategyUnknown  = errors.New("unknown sync strategy")
	ErrBatchSizeInvalid     = errors.New("batch size must be positive")
	ErrBatchIntervalInvalid = errors.New("batch interval must be positive")
)

// Supported sync strategies for SQLiteConfig.
const (
	SyncImmediate = "immediate"
	SyncOnClose   = "on_close"
	SyncBatch     = "batch"
)

// Config selects a backend and provides backend-specific parameters
// (prd001-cupboard-core R1.1).
type Config struct {
	Backend      string        `json:"backend"       yaml:"backend"`
	DataDir      string        `json:"data_dir"      yaml:"data_dir"`
	SQLiteConfig *SQLiteConfig `json:"sqlite_config"  yaml:"sqlite_config"`
}

// SQLiteConfig holds SQLite-backend-specific parameters.
type SQLiteConfig struct {
	SyncStrategy  string `json:"sync_strategy"   yaml:"sync_strategy"`
	BatchSize     int    `json:"batch_size"      yaml:"batch_size"`
	BatchInterval int    `json:"batch_interval"  yaml:"batch_interval"`
}

// Validate checks that the Config is well-formed (prd001-cupboard-core R1.2–R1.4).
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
	if c.SQLiteConfig != nil {
		if err := c.SQLiteConfig.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks that SQLiteConfig values are valid.
func (sc SQLiteConfig) Validate() error {
	if sc.SyncStrategy != "" {
		switch sc.SyncStrategy {
		case SyncImmediate, SyncOnClose, SyncBatch:
			// valid
		default:
			return ErrSyncStrategyUnknown
		}
	}
	if sc.SyncStrategy == SyncBatch {
		if sc.BatchSize <= 0 {
			return ErrBatchSizeInvalid
		}
		if sc.BatchInterval <= 0 {
			return ErrBatchIntervalInvalid
		}
	}
	return nil
}
