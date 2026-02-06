// Package sqlite provides the public API for the SQLite Cupboard backend.
// This package exposes the factory function for creating SQLite backends
// while keeping implementation details internal.
//
// Implements: prd-cupboard-core R2, R4 (backend factory);
//
//	docs/ARCHITECTURE § Public API.
package sqlite

import (
	"github.com/petar-djukic/crumbs/internal/sqlite"
	"github.com/petar-djukic/crumbs/pkg/types"
)

// NewBackend creates a new SQLite backend instance.
// The backend is not attached; call Attach with a Config to initialize.
//
// Example:
//
//	backend := sqlite.NewBackend()
//	err := backend.Attach(types.Config{
//	    Backend: types.BackendSQLite,
//	    DataDir: ".crumbs",
//	})
//	defer backend.Detach()
func NewBackend() types.Cupboard {
	return sqlite.NewBackend()
}
