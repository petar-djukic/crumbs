// Package sqlite implements the SQLite storage backend for Crumbs.
// Implements: prd002-sqlite-backend R4, R5, R6, R11, R12;
//
//	prd010-configuration-directories R3, R4, R5, R6;
//	prd001-cupboard-core R2, R4, R5;
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/mesh-intelligence/crumbs/pkg/types"
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

	// Sync strategy state (prd002-sqlite-backend R16)
	syncStrategy  string         // effective sync strategy: immediate, on_close, batch
	batchSize     int            // number of writes before batch flush
	batchInterval time.Duration  // time between batch flushes
	pendingWrites []pendingWrite // queue of writes pending JSONL persist
	batchTimer    *time.Timer    // timer for interval-based batch flush
	batchMu       sync.Mutex     // protects pendingWrites and batchTimer
}

// pendingWrite represents a deferred JSONL write operation.
// Used by on_close and batch sync strategies.
type pendingWrite struct {
	tableName string       // entity table name (crumbs, trails, etc.)
	operation string       // "save" or "delete"
	persist   func() error // function to execute the JSONL write
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

	// Initialize sync strategy from config (prd002-sqlite-backend R16)
	b.syncStrategy = config.SQLiteConfig.GetSyncStrategy()
	b.batchSize = config.SQLiteConfig.GetBatchSize()
	b.batchInterval = time.Duration(config.SQLiteConfig.GetBatchInterval()) * time.Second
	b.pendingWrites = nil

	// Start batch timer if using batch strategy (prd002-sqlite-backend R16.4)
	if b.syncStrategy == types.SyncBatch && b.batchInterval > 0 {
		b.startBatchTimer()
	}

	// Initialize JSONL files if they don't exist (per prd010-configuration-directories R4.3)
	if err := b.initJSONLFiles(); err != nil {
		db.Close()
		return err
	}

	// Load JSONL files into SQLite (per prd010-configuration-directories R5.1)
	if err := b.loadAllJSONL(); err != nil {
		db.Close()
		return fmt.Errorf("load JSONL: %w", err)
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
// For on_close and batch sync strategies, flushes all pending writes before closing
// (prd002-sqlite-backend R6.1, R16.3).
func (b *Backend) Detach() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.attached {
		return nil // idempotent
	}

	// Stop batch timer if running
	b.stopBatchTimer()

	// Flush all pending writes before closing (prd002-sqlite-backend R6.1, R16.3)
	if err := b.flushPendingWritesLocked(); err != nil {
		return fmt.Errorf("flush pending writes: %w", err)
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

// Sync strategy methods (prd002-sqlite-backend R16)

// shouldPersistImmediately returns true if JSONL writes should happen immediately.
// Returns true for "immediate" strategy (default), false for "on_close" and "batch".
func (b *Backend) shouldPersistImmediately() bool {
	return b.syncStrategy == types.SyncImmediate || b.syncStrategy == ""
}

// queueWrite adds a write operation to the pending queue.
// For "on_close" strategy, writes are queued until Detach.
// For "batch" strategy, writes are queued until batch size or interval is reached.
// The caller must hold b.mu (read or write lock).
func (b *Backend) queueWrite(tableName, operation string, persist func() error) {
	b.batchMu.Lock()
	defer b.batchMu.Unlock()

	b.pendingWrites = append(b.pendingWrites, pendingWrite{
		tableName: tableName,
		operation: operation,
		persist:   persist,
	})

	// For batch strategy, check if we should flush based on batch size (R16.4)
	if b.syncStrategy == types.SyncBatch && b.batchSize > 0 && len(b.pendingWrites) >= b.batchSize {
		// Flush synchronously when batch size reached
		_ = b.flushPendingWritesBatchLocked()
	}
}

// flushPendingWritesLocked flushes all pending writes to JSONL files.
// The caller must hold b.mu write lock.
func (b *Backend) flushPendingWritesLocked() error {
	b.batchMu.Lock()
	defer b.batchMu.Unlock()

	return b.flushPendingWritesBatchLocked()
}

// flushPendingWritesBatchLocked executes all pending writes.
// The caller must hold b.batchMu lock.
func (b *Backend) flushPendingWritesBatchLocked() error {
	if len(b.pendingWrites) == 0 {
		return nil
	}

	// Execute all pending writes
	for _, pw := range b.pendingWrites {
		if err := pw.persist(); err != nil {
			// Continue flushing other writes even if one fails
			// per prd002-sqlite-backend R5.4 (return error to caller, next Attach reconciles)
			return fmt.Errorf("flush %s %s: %w", pw.tableName, pw.operation, err)
		}
	}

	// Clear the queue
	b.pendingWrites = nil

	return nil
}

// startBatchTimer starts the batch interval timer for periodic flushes.
// The caller should ensure this is only called for batch strategy with positive interval.
func (b *Backend) startBatchTimer() {
	b.batchMu.Lock()
	defer b.batchMu.Unlock()

	if b.batchTimer != nil {
		return // already running
	}

	b.batchTimer = time.AfterFunc(b.batchInterval, func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if !b.attached {
			return
		}

		// Flush pending writes
		_ = b.flushPendingWritesLocked()

		// Restart the timer
		b.batchMu.Lock()
		if b.batchTimer != nil && b.attached {
			b.batchTimer.Reset(b.batchInterval)
		}
		b.batchMu.Unlock()
	})
}

// stopBatchTimer stops the batch interval timer if running.
func (b *Backend) stopBatchTimer() {
	b.batchMu.Lock()
	defer b.batchMu.Unlock()

	if b.batchTimer != nil {
		b.batchTimer.Stop()
		b.batchTimer = nil
	}
}
