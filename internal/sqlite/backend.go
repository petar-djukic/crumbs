package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mesh-intelligence/crumbs/pkg/types"

	_ "modernc.org/sqlite"
)

// Compile-time assertion: SQLiteBackend implements types.Cupboard.
var _ types.Cupboard = (*SQLiteBackend)(nil)

const (
	backendSQLite = "sqlite"
	dbFileName    = "cupboard.db"
)

// SQLiteBackend stores data in JSONL files (source of truth) with a SQLite
// query cache rebuilt on each startup.
// Implements: prd002-sqlite-backend R11, prd001-cupboard-core R2.
type SQLiteBackend struct {
	mu       sync.RWMutex
	attached bool
	dataDir  string
	db       *sql.DB
	tables   map[string]*table
}

// NewBackend creates an unattached SQLiteBackend. Call Attach to initialize.
func NewBackend() *SQLiteBackend {
	return &SQLiteBackend{}
}

// GetTable returns the Table for the given name.
// Returns ErrTableNotFound if the name is not a recognized table.
// Returns ErrCupboardDetached if not attached.
// Implements: prd001-cupboard-core R2, prd002-sqlite-backend R12.
func (b *SQLiteBackend) GetTable(name string) (types.Table, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.attached {
		return nil, types.ErrCupboardDetached
	}
	tbl, ok := b.tables[name]
	if !ok {
		return nil, types.ErrTableNotFound
	}
	return tbl, nil
}

// Attach validates the config and initializes the backend.
// Creates DataDir if needed, creates/opens JSONL files, opens SQLite database,
// creates schema, loads JSONL into SQLite, and seeds built-in properties.
// Returns ErrAlreadyAttached if called twice.
// Implements: prd001-cupboard-core R4, prd002-sqlite-backend R4, R11.
func (b *SQLiteBackend) Attach(config types.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.attached {
		return types.ErrAlreadyAttached
	}

	// Validate config (prd001 R1.2-R1.4).
	if config.Backend == "" {
		return types.ErrBackendEmpty
	}
	if config.Backend != backendSQLite {
		return types.ErrBackendUnknown
	}
	if config.DataDir == "" {
		return fmt.Errorf("data directory required for sqlite backend")
	}

	// Create DataDir if needed (prd002 R1.3).
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	// Create empty JSONL files if needed (prd002 R1.4).
	if err := ensureJSONLFiles(config.DataDir); err != nil {
		return fmt.Errorf("ensuring JSONL files: %w", err)
	}

	// Delete stale cupboard.db (prd002 R4.1, prd010 R5.1).
	dbPath := filepath.Join(config.DataDir, dbFileName)
	_ = os.Remove(dbPath)

	// Open SQLite database (prd002 R3.1).
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better read concurrency.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return fmt.Errorf("enabling foreign keys: %w", err)
	}

	// Create schema (prd002 R3).
	if err := createSchema(db); err != nil {
		db.Close()
		return fmt.Errorf("creating schema: %w", err)
	}

	b.db = db
	b.dataDir = config.DataDir

	// Load JSONL files into SQLite (prd002 R4.1).
	if err := b.loadJSONL(); err != nil {
		db.Close()
		b.db = nil
		return fmt.Errorf("loading JSONL: %w", err)
	}

	// Seed built-in properties and categories (prd002 R9, prd004 R9).
	if err := b.seedBuiltins(); err != nil {
		db.Close()
		b.db = nil
		return fmt.Errorf("seeding builtins: %w", err)
	}

	// Create table accessors (prd002 R12.4).
	b.tables = map[string]*table{
		types.TableCrumbs:     {name: types.TableCrumbs, backend: b},
		types.TableTrails:     {name: types.TableTrails, backend: b},
		types.TableProperties: {name: types.TableProperties, backend: b},
		types.TableMetadata:   {name: types.TableMetadata, backend: b},
		types.TableLinks:      {name: types.TableLinks, backend: b},
		types.TableStashes:    {name: types.TableStashes, backend: b},
	}

	b.attached = true
	return nil
}

// Detach releases backend resources. Idempotent.
// Implements: prd001-cupboard-core R5, prd002-sqlite-backend R6, prd010 R7.
func (b *SQLiteBackend) Detach() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.attached {
		return nil
	}

	if b.db != nil {
		b.db.Close()
		b.db = nil
	}

	// Delete cupboard.db; only JSONL remains after orderly shutdown (prd010 R7.2).
	_ = os.Remove(filepath.Join(b.dataDir, dbFileName))

	b.tables = nil
	b.attached = false
	return nil
}

// loadJSONL reads all JSONL files and inserts their records into SQLite.
// Malformed lines are skipped with a warning per prd002 R4.2.
func (b *SQLiteBackend) loadJSONL() error {
	type loader struct {
		file string
		load func(json.RawMessage) error
	}

	loaders := []loader{
		{"properties.jsonl", b.loadPropertyLine},
		{"categories.jsonl", b.loadCategoryLine},
		{"crumbs.jsonl", b.loadCrumbLine},
		{"crumb_properties.jsonl", b.loadCrumbPropertyLine},
		{"trails.jsonl", b.loadTrailLine},
		{"metadata.jsonl", b.loadMetadataLine},
		{"links.jsonl", b.loadLinkLine},
		{"stashes.jsonl", b.loadStashLine},
		{"stash_history.jsonl", b.loadStashHistoryLine},
	}

	for _, l := range loaders {
		path := filepath.Join(b.dataDir, l.file)
		lines, _, err := readJSONLLines(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", l.file, err)
		}
		for _, line := range lines {
			if err := l.load(line); err != nil {
				// Skip malformed; prd002 R4.2 says log and continue.
				continue
			}
		}
	}
	return nil
}

func (b *SQLiteBackend) loadCrumbLine(data json.RawMessage) error {
	c, err := hydrateCrumb(data)
	if err != nil {
		return err
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO crumbs (crumb_id, name, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		c.CrumbID, c.Name, c.State,
		c.CreatedAt.Format(json_time_format),
		c.UpdatedAt.Format(json_time_format))
	return err
}

func (b *SQLiteBackend) loadTrailLine(data json.RawMessage) error {
	t, err := hydrateTrail(data)
	if err != nil {
		return err
	}
	var completedAt sql.NullString
	if t.CompletedAt != nil {
		completedAt = sql.NullString{String: t.CompletedAt.Format(json_time_format), Valid: true}
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO trails (trail_id, state, created_at, completed_at)
		VALUES (?, ?, ?, ?)`,
		t.TrailID, t.State, t.CreatedAt.Format(json_time_format), completedAt)
	return err
}

func (b *SQLiteBackend) loadPropertyLine(data json.RawMessage) error {
	p, err := hydrateProperty(data)
	if err != nil {
		return err
	}
	var desc sql.NullString
	if p.Description != "" {
		desc = sql.NullString{String: p.Description, Valid: true}
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO properties (property_id, name, description, value_type, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		p.PropertyID, p.Name, desc, p.ValueType, p.CreatedAt.Format(json_time_format))
	return err
}

func (b *SQLiteBackend) loadCategoryLine(data json.RawMessage) error {
	cat, err := hydrateCategory(data)
	if err != nil {
		return err
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO categories (category_id, property_id, name, ordinal)
		VALUES (?, ?, ?, ?)`,
		cat.CategoryID, cat.PropertyID, cat.Name, cat.Ordinal)
	return err
}

func (b *SQLiteBackend) loadCrumbPropertyLine(data json.RawMessage) error {
	cp, err := hydrateCrumbProperty(data)
	if err != nil {
		return err
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO crumb_properties (crumb_id, property_id, value)
		VALUES (?, ?, ?)`,
		cp.CrumbID, cp.PropertyID, cp.Value)
	return err
}

func (b *SQLiteBackend) loadMetadataLine(data json.RawMessage) error {
	m, err := hydrateMetadata(data)
	if err != nil {
		return err
	}
	var propID sql.NullString
	if m.PropertyID != nil {
		propID = sql.NullString{String: *m.PropertyID, Valid: true}
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO metadata (metadata_id, table_name, crumb_id, property_id, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.MetadataID, m.TableName, m.CrumbID, propID, m.Content, m.CreatedAt.Format(json_time_format))
	return err
}

func (b *SQLiteBackend) loadLinkLine(data json.RawMessage) error {
	l, err := hydrateLink(data)
	if err != nil {
		return err
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO links (link_id, link_type, from_id, to_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		l.LinkID, l.LinkType, l.FromID, l.ToID, l.CreatedAt.Format(json_time_format))
	return err
}

func (b *SQLiteBackend) loadStashLine(data json.RawMessage) error {
	s, err := hydrateStash(data)
	if err != nil {
		return err
	}
	valueJSON, err := json.Marshal(s.Value)
	if err != nil {
		return err
	}
	var changedBy sql.NullString
	if s.ChangedBy != nil {
		changedBy = sql.NullString{String: *s.ChangedBy, Valid: true}
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO stashes (stash_id, name, stash_type, value, version, created_at, updated_at, last_operation, changed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.StashID, s.Name, s.StashType, string(valueJSON), s.Version,
		s.CreatedAt.Format(json_time_format), s.CreatedAt.Format(json_time_format),
		s.LastOperation, changedBy)
	return err
}

func (b *SQLiteBackend) loadStashHistoryLine(data json.RawMessage) error {
	h, err := hydrateStashHistory(data)
	if err != nil {
		return err
	}
	valueJSON, err := json.Marshal(h.Value)
	if err != nil {
		return err
	}
	var changedBy sql.NullString
	if h.ChangedBy != nil {
		changedBy = sql.NullString{String: *h.ChangedBy, Valid: true}
	}
	_, err = b.db.Exec(`
		INSERT OR REPLACE INTO stash_history (history_id, stash_id, version, value, operation, changed_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		h.HistoryID, h.StashID, h.Version, string(valueJSON), h.Operation, changedBy, h.CreatedAt.Format(json_time_format))
	return err
}

// json_time_format is RFC 3339 as used for JSONL timestamps.
const json_time_format = "2006-01-02T15:04:05Z07:00"
