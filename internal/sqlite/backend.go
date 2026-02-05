// Package sqlite implements the SQLite storage backend for Crumbs.
// Implements: prd-sqlite-backend R4, R5, R6, R11, R12;
//
//	prd-cupboard-core R2, R4, R5;
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/dukaforge/crumbs/pkg/types"
)

//go:embed schema.sql
var schemaSQL string

// Backend implements the Cupboard interface using SQLite as the query engine
// and JSON files as the source of truth.
type Backend struct {
	mu       sync.RWMutex
	attached bool
	config   types.Config
	db       *sql.DB
	tables   map[string]*Table
}

// NewBackend creates a new SQLite backend instance.
// The backend is not attached; call Attach with a Config to initialize.
func NewBackend() *Backend {
	return &Backend{
		tables: make(map[string]*Table),
	}
}

// GetTable returns a Table interface for the specified table name.
// Returns ErrTableNotFound if the table name is not recognized.
// Returns ErrCupboardDetached if the backend is not attached.
func (b *Backend) GetTable(name string) (types.Table, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.attached {
		return nil, types.ErrCupboardDetached
	}

	table, ok := b.tables[name]
	if !ok {
		return nil, types.ErrTableNotFound
	}
	return table, nil
}

// Attach initializes the backend with the given configuration.
// Creates DataDir if it does not exist, initializes SQLite schema,
// and creates table accessors.
// Returns ErrAlreadyAttached if already attached.
func (b *Backend) Attach(config types.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.attached {
		return types.ErrAlreadyAttached
	}

	if err := config.Validate(); err != nil {
		return err
	}

	// Create DataDir if needed
	dataDir := config.DataDir
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	// Open SQLite database
	dbPath := filepath.Join(dataDir, "cupboard.db")
	// Remove existing database file to ensure fresh schema (per R4.3)
	_ = os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Execute schema
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return err
	}

	b.db = db
	b.config = config

	// Initialize JSON files if they don't exist (per R1.4)
	if err := b.initJSONFiles(); err != nil {
		db.Close()
		return err
	}

	// Load JSON files into SQLite (per R4.1)
	if err := b.loadAllJSON(); err != nil {
		db.Close()
		return fmt.Errorf("load JSON: %w", err)
	}

	b.attached = true

	// Create table accessors
	b.tables[types.CrumbsTable] = newTable(b, types.CrumbsTable)
	b.tables[types.TrailsTable] = newTable(b, types.TrailsTable)
	b.tables[types.PropertiesTable] = newTable(b, types.PropertiesTable)
	b.tables[types.MetadataTable] = newTable(b, types.MetadataTable)
	b.tables[types.LinksTable] = newTable(b, types.LinksTable)
	b.tables[types.StashesTable] = newTable(b, types.StashesTable)

	return nil
}

// Detach releases all resources held by the backend.
// Closes the SQLite connection. After Detach, all operations return ErrCupboardDetached.
// Detach is idempotent.
func (b *Backend) Detach() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.attached {
		return nil // idempotent
	}

	if b.db != nil {
		if err := b.db.Close(); err != nil {
			return err
		}
		b.db = nil
	}

	b.attached = false
	b.tables = make(map[string]*Table)

	return nil
}

// generateUUID generates a new UUID v7 for entity IDs.
func generateUUID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to UUID v4 if v7 generation fails
		return uuid.New().String()
	}
	return id.String()
}
