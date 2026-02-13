// This file implements the trails table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd006-trails-interface R1-R6 (Trail entity, states, complete, abandon);
//             prd002-sqlite-backend R5.6-R5.7 (cascade on completed/abandoned).
package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var _ types.Table = (*trailsTable)(nil)

type trailsTable struct {
	backend *Backend
}

// Get retrieves a trail by ID (prd006-trails-interface R3, prd002-sqlite-backend R14.3).
func (tt *trailsTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := tt.backend.db.QueryRow(
		"SELECT trail_id, state, created_at, completed_at FROM trails WHERE trail_id = ?",
		id,
	)
	trail, err := hydrateTrail(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting trail %s: %w", id, err)
	}
	return trail, nil
}

// Set persists a trail. If id is empty, generates a UUID v7 and creates the
// trail with defaults. If id is provided, updates the existing trail. When the
// state changes to completed, all belongs_to links are removed. When the state
// changes to abandoned, all crumbs on the trail and their associated data are
// deleted (prd002-sqlite-backend R5.6, R5.7).
func (tt *trailsTable) Set(id string, data any) (string, error) {
	trail, ok := data.(*types.Trail)
	if !ok {
		return "", types.ErrInvalidData
	}

	now := time.Now().UTC()

	if id == "" {
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		trail.TrailID = newID.String()
		trail.State = types.TrailStateDraft
		trail.CreatedAt = now
		id = trail.TrailID
	}

	// Determine INSERT vs UPDATE and detect state change for cascade.
	var prevState string
	err := tt.backend.db.QueryRow(
		"SELECT state FROM trails WHERE trail_id = ?", id,
	).Scan(&prevState)
	isUpdate := err == nil
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking trail existence: %w", err)
	}

	tx, err := tt.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	createdAtStr := trail.CreatedAt.Format(time.RFC3339)
	var completedAtStr *string
	if trail.CompletedAt != nil {
		s := trail.CompletedAt.Format(time.RFC3339)
		completedAtStr = &s
	}

	if isUpdate {
		_, err = tx.Exec(
			"UPDATE trails SET state = ?, created_at = ?, completed_at = ? WHERE trail_id = ?",
			trail.State, createdAtStr, completedAtStr, id,
		)
	} else {
		_, err = tx.Exec(
			"INSERT INTO trails (trail_id, state, created_at, completed_at) VALUES (?, ?, ?, ?)",
			id, trail.State, createdAtStr, completedAtStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting trail: %w", err)
	}

	// Cascade on state change (prd002-sqlite-backend R5.6, R5.7).
	stateChanged := isUpdate && prevState != trail.State
	if stateChanged && trail.State == types.TrailStateCompleted {
		if err := cascadeTrailCompleted(tx, id); err != nil {
			return "", fmt.Errorf("cascade on trail completed: %w", err)
		}
	}
	if stateChanged && trail.State == types.TrailStateAbandoned {
		if err := cascadeTrailAbandoned(tx, id); err != nil {
			return "", fmt.Errorf("cascade on trail abandoned: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing trail: %w", err)
	}

	// Persist affected JSONL files.
	if err := persistTableJSONL(tt.backend, "trails", "trails.jsonl"); err != nil {
		return "", fmt.Errorf("persisting trails.jsonl: %w", err)
	}
	if stateChanged && trail.State == types.TrailStateCompleted {
		if err := persistTableJSONL(tt.backend, "links", "links.jsonl"); err != nil {
			return "", fmt.Errorf("persisting links.jsonl after trail completion: %w", err)
		}
	}
	if stateChanged && trail.State == types.TrailStateAbandoned {
		for _, pair := range []struct{ table, file string }{
			{"crumbs", "crumbs.jsonl"},
			{"crumb_properties", "crumb_properties.jsonl"},
			{"metadata", "metadata.jsonl"},
			{"links", "links.jsonl"},
		} {
			if err := persistTableJSONL(tt.backend, pair.table, pair.file); err != nil {
				return "", fmt.Errorf("persisting %s after trail abandonment: %w", pair.file, err)
			}
		}
	}

	return id, nil
}

// Delete removes a trail by ID (prd006-trails-interface R4).
func (tt *trailsTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	var exists bool
	err := tt.backend.db.QueryRow(
		"SELECT 1 FROM trails WHERE trail_id = ?", id,
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("checking trail existence: %w", err)
	}

	tx, err := tt.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove links referencing this trail.
	if _, err := tx.Exec("DELETE FROM links WHERE from_id = ? OR to_id = ?", id, id); err != nil {
		return fmt.Errorf("deleting trail links: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM trails WHERE trail_id = ?", id); err != nil {
		return fmt.Errorf("deleting trail: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing trail deletion: %w", err)
	}

	if err := persistTableJSONL(tt.backend, "trails", "trails.jsonl"); err != nil {
		return fmt.Errorf("persisting trails.jsonl: %w", err)
	}
	if err := persistTableJSONL(tt.backend, "links", "links.jsonl"); err != nil {
		return fmt.Errorf("persisting links.jsonl: %w", err)
	}

	return nil
}

// Fetch queries trails matching the filter, ordered by created_at DESC
// (prd006-trails-interface R6).
func (tt *trailsTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT trail_id, state, created_at, completed_at FROM trails"
	var conditions []string
	var args []any

	if filter != nil {
		if v, ok := filter["state"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "state = ?")
			args = append(args, s)
		}
		if v, ok := filter["states"]; ok {
			states, ok := v.([]string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			if len(states) > 0 {
				placeholders := make([]string, len(states))
				for i, s := range states {
					placeholders[i] = "?"
					args = append(args, s)
				}
				conditions = append(conditions, "state IN ("+strings.Join(placeholders, ", ")+")")
			}
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

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

	rows, err := tt.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching trails: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		trail, err := hydrateTrailFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating trail: %w", err)
		}
		results = append(results, trail)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating trails: %w", err)
	}

	if results == nil {
		results = []any{}
	}
	return results, nil
}

// cascadeTrailCompleted removes all belongs_to links pointing to the trail
// (prd002-sqlite-backend R5.6).
func cascadeTrailCompleted(tx *sql.Tx, trailID string) error {
	_, err := tx.Exec(
		"DELETE FROM links WHERE link_type = ? AND to_id = ?",
		types.LinkTypeBelongsTo, trailID,
	)
	return err
}

// cascadeTrailAbandoned deletes all crumbs belonging to the trail and their
// associated crumb_properties, metadata, and links (prd002-sqlite-backend R5.7).
func cascadeTrailAbandoned(tx *sql.Tx, trailID string) error {
	// Find all crumbs that belong to this trail via belongs_to links.
	rows, err := tx.Query(
		"SELECT from_id FROM links WHERE link_type = ? AND to_id = ?",
		types.LinkTypeBelongsTo, trailID,
	)
	if err != nil {
		return fmt.Errorf("querying crumbs for abandoned trail: %w", err)
	}
	var crumbIDs []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			rows.Close()
			return fmt.Errorf("scanning crumb id: %w", err)
		}
		crumbIDs = append(crumbIDs, cid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating crumb ids: %w", err)
	}

	// Cascade delete each crumb's associated data, then the crumb itself.
	for _, cid := range crumbIDs {
		if _, err := tx.Exec("DELETE FROM crumb_properties WHERE crumb_id = ?", cid); err != nil {
			return fmt.Errorf("deleting crumb_properties for %s: %w", cid, err)
		}
		if _, err := tx.Exec("DELETE FROM metadata WHERE crumb_id = ?", cid); err != nil {
			return fmt.Errorf("deleting metadata for %s: %w", cid, err)
		}
		if _, err := tx.Exec("DELETE FROM links WHERE from_id = ? OR to_id = ?", cid, cid); err != nil {
			return fmt.Errorf("deleting links for %s: %w", cid, err)
		}
		if _, err := tx.Exec("DELETE FROM crumbs WHERE crumb_id = ?", cid); err != nil {
			return fmt.Errorf("deleting crumb %s: %w", cid, err)
		}
	}

	// Remove any remaining links referencing the trail (e.g. branches_from).
	if _, err := tx.Exec("DELETE FROM links WHERE from_id = ? OR to_id = ?", trailID, trailID); err != nil {
		return fmt.Errorf("deleting remaining trail links: %w", err)
	}

	return nil
}

// hydrateTrail converts a single SQLite row into a *types.Trail
// (prd002-sqlite-backend R14.3).
func hydrateTrail(row *sql.Row) (*types.Trail, error) {
	var t types.Trail
	var createdAt string
	var completedAt sql.NullString
	if err := row.Scan(&t.TrailID, &t.State, &createdAt, &completedAt); err != nil {
		return nil, err
	}
	var err error
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	if completedAt.Valid {
		ct, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parsing completed_at: %w", err)
		}
		t.CompletedAt = &ct
	}
	return &t, nil
}

// hydrateTrailFromRows converts a row from sql.Rows into a *types.Trail.
func hydrateTrailFromRows(rows *sql.Rows) (*types.Trail, error) {
	var t types.Trail
	var createdAt string
	var completedAt sql.NullString
	if err := rows.Scan(&t.TrailID, &t.State, &createdAt, &completedAt); err != nil {
		return nil, err
	}
	var err error
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	if completedAt.Valid {
		ct, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parsing completed_at: %w", err)
		}
		t.CompletedAt = &ct
	}
	return &t, nil
}
