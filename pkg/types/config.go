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
	BackendSQLite   = "sqlite"
	BackendDolt     = "dolt"
	BackendDynamoDB = "dynamodb"
)

// Config holds configuration for initializing a Cupboard instance.
// The Backend field selects the storage backend; backend-specific
// configs provide additional parameters.
type Config struct {
	// Backend type: "sqlite", "dolt", "dynamodb"
	Backend string

	// DataDir is the directory for local backends (sqlite, dolt);
	// ignored for cloud backends.
	DataDir string

	// DoltConfig holds Dolt-specific settings; nil if not using Dolt.
	DoltConfig *DoltConfig

	// DynamoDBConfig holds DynamoDB-specific settings; nil if not using DynamoDB.
	DynamoDBConfig *DynamoDBConfig
}

// DoltConfig holds configuration for the Dolt backend.
type DoltConfig struct {
	// DSN is the data source name (connection string).
	DSN string

	// Branch is the Git branch for versioning; defaults to "main".
	Branch string
}

// DynamoDBConfig holds configuration for the DynamoDB backend.
type DynamoDBConfig struct {
	// TableName is the DynamoDB table name.
	TableName string

	// Region is the AWS region.
	Region string

	// Endpoint is an optional endpoint override for local testing.
	Endpoint string
}

// Validation errors.
var (
	ErrBackendEmpty       = errors.New("backend cannot be empty")
	ErrBackendUnknown     = errors.New("unknown backend")
	ErrDoltConfigRequired = errors.New("dolt backend requires DoltConfig")
	ErrDynamoDBRequired   = errors.New("dynamodb backend requires DynamoDBConfig")
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
		// SQLite only requires DataDir, which can be empty (defaults to cwd)
		return nil
	case BackendDolt:
		if c.DoltConfig == nil {
			return ErrDoltConfigRequired
		}
		return nil
	case BackendDynamoDB:
		if c.DynamoDBConfig == nil {
			return ErrDynamoDBRequired
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrBackendUnknown, c.Backend)
	}
}
