package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mesh-intelligence/crumbs/pkg/types"

	_ "modernc.org/sqlite"
)

// Compile-time interface check: Backend must implement Cupboard.
var _ types.Cupboard = (*Backend)(nil)

// JSONL filenames within DataDir (prd002-sqlite-backend R1.2).
var jsonlFiles = []string{
	"crumbs.jsonl",
	"trails.jsonl",
	"links.jsonl",
	"properties.jsonl",
	"categories.jsonl",
	"crumb_properties.jsonl",
	"metadata.jsonl",
	"stashes.jsonl",
	"stash_history.jsonl",
}

// Backend implements the Cupboard interface using SQLite as a query cache
// over JSONL files (prd002-sqlite-backend R11).
type Backend struct {
	mu       sync.RWMutex
	attached bool
	config   types.Config
	db       *sql.DB
	tables   map[string]types.Table
}

// NewBackend returns an unattached SQLite backend. Call Attach to initialize
// the connection and schema (prd002-sqlite-backend R11.1).
func NewBackend() *Backend {
	return &Backend{}
}

// GetTable returns the table accessor for the given name. It returns
// ErrCupboardDetached if the backend is not attached, or ErrTableNotFound if
// the name is not a standard table name (prd002-sqlite-backend R12).
func (b *Backend) GetTable(name string) (types.Table, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.attached {
		return nil, types.ErrCupboardDetached
	}
	t, ok := b.tables[name]
	if !ok {
		return nil, types.ErrTableNotFound
	}
	return t, nil
}

// Attach validates the config, creates DataDir and JSONL files if needed,
// deletes any existing cupboard.db, creates a new database with the full
// schema, and marks the backend as attached (prd002-sqlite-backend R4, R11.2,
// R11.3).
func (b *Backend) Attach(config types.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.attached {
		return types.ErrAlreadyAttached
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Create DataDir if it does not exist (R1.3).
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	// Create empty JSONL files if they do not exist (R1.4).
	for _, name := range jsonlFiles {
		p := filepath.Join(config.DataDir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			f, err := os.Create(p)
			if err != nil {
				return fmt.Errorf("creating %s: %w", name, err)
			}
			f.Close()
		}
	}

	// Delete existing cupboard.db (ephemeral cache, R4.1).
	dbPath := filepath.Join(config.DataDir, "cupboard.db")
	_ = os.Remove(dbPath)

	// Open new SQLite database.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening sqlite database: %w", err)
	}

	// Enable foreign keys.
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return fmt.Errorf("enabling foreign keys: %w", err)
	}

	// Create schema (R3.2).
	for _, ddl := range schemaDDL {
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return fmt.Errorf("creating table: %w", err)
		}
	}

	// Create indexes (R3.3).
	for _, ddl := range indexDDL {
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return fmt.Errorf("creating index: %w", err)
		}
	}

	b.db = db
	b.config = config

	// Load JSONL files into SQLite tables (prd002-sqlite-backend R4).
	if err := loadAllJSONL(db, config.DataDir); err != nil {
		db.Close()
		return fmt.Errorf("loading JSONL files: %w", err)
	}

	// Seed built-in properties and categories on first run (prd002-sqlite-backend R9).
	if err := seedBuiltInProperties(db, config.DataDir); err != nil {
		db.Close()
		return fmt.Errorf("seeding built-in properties: %w", err)
	}

	b.tables = buildTables(b)
	b.attached = true

	return nil
}

// Detach closes the SQLite connection and marks the backend as detached.
// Subsequent operations return ErrCupboardDetached. Detach is idempotent
// (prd002-sqlite-backend R6.2, prd001-cupboard-core R5.2).
func (b *Backend) Detach() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.attached {
		return nil
	}

	if b.db != nil {
		b.db.Close()
		b.db = nil
	}

	b.tables = nil
	b.attached = false

	return nil
}

// buildTables creates a table accessor for each standard table name
// (prd002-sqlite-backend R12.1, R12.4).
func buildTables(b *Backend) map[string]types.Table {
	return map[string]types.Table{
		types.TableCrumbs:     &crumbsTable{backend: b},
		types.TableTrails:     &trailsTable{backend: b},
		types.TableProperties: &propertiesTable{backend: b},
		types.TableCategories: &categoriesTable{backend: b},
		types.TableMetadata:   &metadataTable{backend: b},
		types.TableLinks:      &linksTable{backend: b},
		types.TableStashes:    &stashesTable{backend: b},
	}
}
