// This file implements JSONL loading for startup.
// Implements: prd002-sqlite-backend R4 (startup sequence), R4.2 (malformed lines),
//             R4.4 (transactional loading), R7.2 (unknown field tolerance).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// jsonlTableMapping maps JSONL filenames to their SQLite tables and column lists.
// The order matters: tables with foreign keys must load after their referenced tables.
var jsonlTableMapping = []struct {
	file    string
	table   string
	columns []string
}{
	{"crumbs.jsonl", "crumbs", []string{"crumb_id", "name", "state", "created_at", "updated_at"}},
	{"trails.jsonl", "trails", []string{"trail_id", "state", "created_at", "completed_at"}},
	{"properties.jsonl", "properties", []string{"property_id", "name", "description", "value_type", "created_at"}},
	{"categories.jsonl", "categories", []string{"category_id", "property_id", "name", "ordinal"}},
	{"crumb_properties.jsonl", "crumb_properties", []string{"crumb_id", "property_id", "value_type", "value"}},
	{"links.jsonl", "links", []string{"link_id", "link_type", "from_id", "to_id", "created_at"}},
	{"metadata.jsonl", "metadata", []string{"metadata_id", "table_name", "crumb_id", "property_id", "content", "created_at"}},
	{"stashes.jsonl", "stashes", []string{"stash_id", "name", "stash_type", "value", "version", "created_at", "updated_at"}},
	{"stash_history.jsonl", "stash_history", []string{"history_id", "stash_id", "version", "value", "operation", "changed_by", "created_at"}},
}

// loadAllJSONL reads each JSONL file from DataDir and inserts records into the
// corresponding SQLite tables. Loading is transactional: all succeed or the
// database remains empty (prd002-sqlite-backend R4.4). Malformed lines are
// skipped per R4.2. Unknown fields in JSONL records are silently ignored,
// enabling forward compatibility across generations (R7.2).
func loadAllJSONL(db *sql.DB, dataDir string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning load transaction: %w", err)
	}
	defer tx.Rollback()

	// Disable foreign keys during loading, re-enable after.
	if _, err := tx.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disabling foreign keys for load: %w", err)
	}

	for _, mapping := range jsonlTableMapping {
		path := filepath.Join(dataDir, mapping.file)
		records, err := readJSONL(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", mapping.file, err)
		}

		if len(records) == 0 {
			continue
		}

		if err := insertRecords(tx, mapping.table, mapping.columns, records); err != nil {
			return fmt.Errorf("loading %s into %s: %w", mapping.file, mapping.table, err)
		}
	}

	if _, err := tx.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("re-enabling foreign keys: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing load transaction: %w", err)
	}

	return nil
}

// insertRecords inserts parsed JSONL records into a SQLite table. Unknown
// fields in the JSON are silently ignored (forward compatibility per
// prd002-sqlite-backend R7.2). Only columns listed in the mapping are
// extracted; extra fields from future generations do not cause errors.
func insertRecords(tx *sql.Tx, table string, columns []string, records []json.RawMessage) error {
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		joinColumns(columns),
		joinColumns(placeholders),
	)

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("preparing insert for %s: %w", table, err)
	}
	defer stmt.Close()

	for _, rec := range records {
		var obj map[string]any
		if err := json.Unmarshal(rec, &obj); err != nil {
			// Skip malformed records (prd002-sqlite-backend R4.2).
			continue
		}

		args := make([]any, len(columns))
		for i, col := range columns {
			val, ok := obj[col]
			if !ok {
				args[i] = nil
				continue
			}
			// JSON values (stash value, etc.) need to be re-serialized as strings.
			switch v := val.(type) {
			case map[string]any, []any:
				b, err := json.Marshal(v)
				if err != nil {
					args[i] = nil
					continue
				}
				args[i] = string(b)
			default:
				args[i] = val
			}
		}

		if _, err := stmt.Exec(args...); err != nil {
			// Skip records that violate constraints (prd002-sqlite-backend R4.2).
			continue
		}
	}

	return nil
}

// joinColumns joins column names with commas.
func joinColumns(cols []string) string {
	result := ""
	for i, c := range cols {
		if i > 0 {
			result += ", "
		}
		result += c
	}
	return result
}
