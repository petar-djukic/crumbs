// Shared helpers for cupboard CLI commands.
// Implements: prd009-cupboard-cli (R3, R8, R9).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mesh-intelligence/crumbs/internal/sqlite"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// validTableNames lists the standard table names for error messages
// (prd001-cupboard-core R2.5).
var validTableNames = []string{
	types.TableCrumbs,
	types.TableTrails,
	types.TableProperties,
	types.TableCategories,
	types.TableMetadata,
	types.TableLinks,
	types.TableStashes,
}

// validTableNamesStr is a comma-separated list of valid table names for error output.
var validTableNamesStr = strings.Join(validTableNames, ", ")

// attachBackend resolves the data directory, creates a SQLite backend, and
// attaches it. The caller must defer backend.Detach(). Returns the attached
// backend or an error suitable for the CLI (prd009-cupboard-cli R3).
func attachBackend() (*sqlite.Backend, error) {
	dataDir, err := resolveDataDir()
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}

	cfg := types.Config{
		Backend: "sqlite",
		DataDir: dataDir,
	}

	backend := sqlite.NewBackend()
	if err := backend.Attach(cfg); err != nil {
		return nil, fmt.Errorf("attach backend: %w", err)
	}

	return backend, nil
}

// parseEntityJSON unmarshals JSON data into the correct entity struct based on
// the table name. Each table maps to a specific entity type per prd001-cupboard-core R2.5.
func parseEntityJSON(tableName string, data []byte) (any, error) {
	switch tableName {
	case types.TableCrumbs:
		var e types.Crumb
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableTrails:
		var e types.Trail
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableProperties:
		var e types.Property
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableCategories:
		var e types.Category
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableMetadata:
		var e types.Metadata
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableLinks:
		var e types.Link
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case types.TableStashes:
		var e types.Stash
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unknown table %q", tableName)
	}
}

// isTableNotFound returns true if the error wraps ErrTableNotFound.
func isTableNotFound(err error) bool {
	return errors.Is(err, types.ErrTableNotFound)
}

// isEntityNotFound returns true if the error wraps ErrNotFound.
func isEntityNotFound(err error) bool {
	return errors.Is(err, types.ErrNotFound)
}
