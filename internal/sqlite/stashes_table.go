// This file implements the stashes table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd008-stash-interface R1-R3, R7-R12 (Stash entity, types, creation, history,
//             retrieval, filter, querying, deletion, errors, scoping).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var _ types.Table = (*stashesTable)(nil)

type stashesTable struct {
	backend *Backend
}

// Get retrieves a stash by ID (prd008-stash-interface R8, prd002-sqlite-backend R14.7).
func (st *stashesTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := st.backend.db.QueryRow(
		"SELECT stash_id, name, stash_type, value, version, created_at, updated_at FROM stashes WHERE stash_id = ?",
		id,
	)
	s, err := hydrateStash(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting stash %s: %w", id, err)
	}
	return s, nil
}

// Set persists a stash. If id is empty, generates a UUID v7 and creates the
// stash with validation. If id is provided, updates the existing stash.
// On every mutation, a history entry is recorded and appended to stash_history.jsonl
// (prd008-stash-interface R3, R7, prd002-sqlite-backend R13.3, R15).
func (st *stashesTable) Set(id string, data any) (string, error) {
	s, ok := data.(*types.Stash)
	if !ok {
		return "", types.ErrInvalidData
	}

	if s.Name == "" {
		return "", types.ErrInvalidName
	}
	if !types.ValidStashType(s.StashType) {
		return "", types.ErrInvalidStashType
	}

	now := time.Now().UTC()
	isCreate := id == ""

	if isCreate {
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		s.StashID = newID.String()
		s.Version = 1
		s.CreatedAt = now
		s.LastOperation = types.StashOpCreate
		id = s.StashID
	}

	// Check name uniqueness (prd008-stash-interface R1.4).
	var dupID string
	err := st.backend.db.QueryRow(
		"SELECT stash_id FROM stashes WHERE name = ? AND stash_id != ?",
		s.Name, id,
	).Scan(&dupID)
	if err == nil {
		return "", types.ErrDuplicateName
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("checking stash name uniqueness: %w", err)
	}

	var exists bool
	err = st.backend.db.QueryRow(
		"SELECT 1 FROM stashes WHERE stash_id = ?", id,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking stash existence: %w", err)
	}

	// Serialize value to JSON for SQLite storage.
	valueJSON, err := json.Marshal(s.Value)
	if err != nil {
		return "", fmt.Errorf("marshaling stash value: %w", err)
	}

	tx, err := st.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	createdAtStr := s.CreatedAt.Format(time.RFC3339)
	updatedAtStr := now.Format(time.RFC3339)

	if exists {
		_, err = tx.Exec(
			"UPDATE stashes SET name = ?, stash_type = ?, value = ?, version = ?, created_at = ?, updated_at = ? WHERE stash_id = ?",
			s.Name, s.StashType, string(valueJSON), s.Version, createdAtStr, updatedAtStr, id,
		)
	} else {
		_, err = tx.Exec(
			"INSERT INTO stashes (stash_id, name, stash_type, value, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			id, s.Name, s.StashType, string(valueJSON), s.Version, createdAtStr, updatedAtStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting stash: %w", err)
	}

	// Record history entry (prd008-stash-interface R7).
	histID, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generating history UUID v7: %w", err)
	}

	operation := s.LastOperation
	if operation == "" {
		if isCreate {
			operation = types.StashOpCreate
		} else {
			operation = types.StashOpSet
		}
	}

	var changedByVal *string
	if s.ChangedBy != nil && *s.ChangedBy != "" {
		changedByVal = s.ChangedBy
	}

	_, err = tx.Exec(
		"INSERT INTO stash_history (history_id, stash_id, version, value, operation, changed_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		histID.String(), id, s.Version, string(valueJSON), operation, changedByVal, now.Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("recording stash history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing stash: %w", err)
	}

	// Persist stashes.jsonl (prd002-sqlite-backend R5.5).
	if err := st.persistStashesJSONL(); err != nil {
		return "", fmt.Errorf("persisting stashes.jsonl: %w", err)
	}

	// Append history entry to stash_history.jsonl (append-only).
	if err := st.appendStashHistoryJSONL(histID.String(), id, s.Version, valueJSON, operation, changedByVal, now); err != nil {
		return "", fmt.Errorf("appending stash_history.jsonl: %w", err)
	}

	return id, nil
}

// Delete removes a stash and its history (prd008-stash-interface R11).
func (st *stashesTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	// Get the stash to check lock status.
	row := st.backend.db.QueryRow(
		"SELECT stash_id, name, stash_type, value, version, created_at, updated_at FROM stashes WHERE stash_id = ?",
		id,
	)
	s, err := hydrateStash(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("getting stash for deletion: %w", err)
	}

	// Check lock held status (prd008-stash-interface R11.5).
	if s.StashType == types.StashTypeLock && s.Value != nil {
		if m, ok := s.Value.(map[string]any); ok {
			if h, ok := m["holder"].(string); ok && h != "" {
				return types.ErrLockHeld
			}
		}
	}

	tx, err := st.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete history first (FK constraint).
	if _, err := tx.Exec("DELETE FROM stash_history WHERE stash_id = ?", id); err != nil {
		return fmt.Errorf("deleting stash history: %w", err)
	}
	// Delete scoped_to links.
	if _, err := tx.Exec("DELETE FROM links WHERE link_type = 'scoped_to' AND from_id = ?", id); err != nil {
		return fmt.Errorf("deleting stash scope links: %w", err)
	}
	// Delete the stash.
	if _, err := tx.Exec("DELETE FROM stashes WHERE stash_id = ?", id); err != nil {
		return fmt.Errorf("deleting stash: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing stash deletion: %w", err)
	}

	// Persist affected JSONL files (prd002-sqlite-backend R5.5).
	if err := st.persistStashesJSONL(); err != nil {
		return fmt.Errorf("persisting stashes.jsonl: %w", err)
	}
	if err := persistTableJSONL(st.backend, "stash_history", "stash_history.jsonl"); err != nil {
		return fmt.Errorf("persisting stash_history.jsonl: %w", err)
	}
	if err := persistTableJSONL(st.backend, "links", "links.jsonl"); err != nil {
		return fmt.Errorf("persisting links.jsonl: %w", err)
	}

	return nil
}

// FetchStashHistory retrieves all history entries for a stash, ordered by
// version ASC (prd008-stash-interface R7.6).
func (st *stashesTable) FetchStashHistory(stashID string) ([]types.StashHistoryEntry, error) {
	if stashID == "" {
		return nil, types.ErrInvalidID
	}
	if st.backend.db == nil {
		return nil, types.ErrCupboardDetached
	}

	rows, err := st.backend.db.Query(
		"SELECT history_id, stash_id, version, value, operation, changed_by, created_at FROM stash_history WHERE stash_id = ? ORDER BY version ASC",
		stashID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying stash history: %w", err)
	}
	defer rows.Close()

	var entries []types.StashHistoryEntry
	for rows.Next() {
		var e types.StashHistoryEntry
		var valueStr, createdAt string
		if err := rows.Scan(&e.HistoryID, &e.StashID, &e.Version, &valueStr, &e.Operation, &e.ChangedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning history entry: %w", err)
		}
		if err := json.Unmarshal([]byte(valueStr), &e.Value); err != nil {
			return nil, fmt.Errorf("parsing history value: %w", err)
		}
		e.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing history created_at: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating history entries: %w", err)
	}

	// Return empty slice, not nil (consistent with other Fetch methods).
	if entries == nil {
		entries = []types.StashHistoryEntry{}
	}
	return entries, nil
}

// Fetch queries stashes matching the filter, ordered by created_at ASC
// (prd008-stash-interface R9, R10).
func (st *stashesTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT stash_id, name, stash_type, value, version, created_at, updated_at FROM stashes"
	var conditions []string
	var args []any

	if filter != nil {
		if v, ok := filter["stash_type"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "stash_type = ?")
			args = append(args, s)
		}
		if v, ok := filter["name"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "name = ?")
			args = append(args, s)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Order by created_at ASC (prd008-stash-interface R9.6).
	query += " ORDER BY created_at ASC"

	if filter != nil {
		if v, ok := filter["limit"]; ok {
			limit, ok := v.(int)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}
		}
		if v, ok := filter["offset"]; ok {
			offset, ok := v.(int)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			if offset > 0 {
				query += fmt.Sprintf(" OFFSET %d", offset)
			}
		}
	}

	rows, err := st.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching stashes: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		s, err := hydrateStashFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating stash: %w", err)
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stashes: %w", err)
	}

	// Return empty slice, not nil (prd008-stash-interface R10.3).
	if results == nil {
		results = []any{}
	}
	return results, nil
}

// hydrateStash converts a single SQLite row into a *types.Stash
// (prd002-sqlite-backend R14.7).
func hydrateStash(row *sql.Row) (*types.Stash, error) {
	var s types.Stash
	var valueStr, createdAt, updatedAt string
	if err := row.Scan(&s.StashID, &s.Name, &s.StashType, &valueStr, &s.Version, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(valueStr), &s.Value); err != nil {
		return nil, fmt.Errorf("parsing stash value: %w", err)
	}
	var err error
	s.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &s, nil
}

// hydrateStashFromRows converts a row from sql.Rows into a *types.Stash.
func hydrateStashFromRows(rows *sql.Rows) (*types.Stash, error) {
	var s types.Stash
	var valueStr, createdAt, updatedAt string
	if err := rows.Scan(&s.StashID, &s.Name, &s.StashType, &valueStr, &s.Version, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(valueStr), &s.Value); err != nil {
		return nil, fmt.Errorf("parsing stash value: %w", err)
	}
	var err error
	s.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &s, nil
}

// persistStashesJSONL reads all stashes from SQLite and writes them to
// stashes.jsonl using the atomic write pattern.
func (st *stashesTable) persistStashesJSONL() error {
	rows, err := st.backend.db.Query(
		"SELECT stash_id, name, stash_type, value, version, created_at, updated_at FROM stashes ORDER BY created_at ASC",
	)
	if err != nil {
		return fmt.Errorf("querying stashes for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var id, name, stashType, valueStr, createdAt, updatedAt string
		var version int64
		if err := rows.Scan(&id, &name, &stashType, &valueStr, &version, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scanning stash for JSONL: %w", err)
		}
		rec := stashJSONLRecord{
			StashID:   id,
			Name:      name,
			StashType: stashType,
			Value:     json.RawMessage(valueStr),
			Version:   version,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling stash for JSONL: %w", err)
		}
		records = append(records, data)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating stashes for JSONL: %w", err)
	}

	return writeJSONL(
		filepath.Join(st.backend.config.DataDir, "stashes.jsonl"),
		records,
	)
}

// appendStashHistoryJSONL appends a single history entry to stash_history.jsonl.
// This file is append-only per the design decision.
func (st *stashesTable) appendStashHistoryJSONL(historyID, stashID string, version int64, valueJSON []byte, operation string, changedBy *string, createdAt time.Time) error {
	rec := stashHistoryJSONLRecord{
		HistoryID: historyID,
		StashID:   stashID,
		Version:   version,
		Value:     json.RawMessage(valueJSON),
		Operation: operation,
		ChangedBy: changedBy,
		CreatedAt: createdAt.Format(time.RFC3339),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshaling history entry: %w", err)
	}

	// Read existing records and append the new one.
	path := filepath.Join(st.backend.config.DataDir, "stash_history.jsonl")
	existing, err := readJSONL(path)
	if err != nil {
		return fmt.Errorf("reading stash_history.jsonl: %w", err)
	}
	existing = append(existing, data)
	return writeJSONL(path, existing)
}

// stashJSONLRecord matches the JSONL format for stashes (prd002-sqlite-backend R2.9).
type stashJSONLRecord struct {
	StashID   string          `json:"stash_id"`
	Name      string          `json:"name"`
	StashType string          `json:"stash_type"`
	Value     json.RawMessage `json:"value"`
	Version   int64           `json:"version"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// stashHistoryJSONLRecord matches the JSONL format for stash history
// (prd002-sqlite-backend R2.10).
type stashHistoryJSONLRecord struct {
	HistoryID string          `json:"history_id"`
	StashID   string          `json:"stash_id"`
	Version   int64           `json:"version"`
	Value     json.RawMessage `json:"value"`
	Operation string          `json:"operation"`
	ChangedBy *string         `json:"changed_by"`
	CreatedAt string          `json:"created_at"`
}
