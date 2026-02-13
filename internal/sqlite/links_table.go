// This file implements the links table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd007-links-interface R1-R7 (Link entity, types, uniqueness, cardinality).
package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var _ types.Table = (*linksTable)(nil)

type linksTable struct {
	backend *Backend
}

// Get retrieves a link by ID (prd007-links-interface R3, prd002-sqlite-backend R14.6).
func (lt *linksTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := lt.backend.db.QueryRow(
		"SELECT link_id, link_type, from_id, to_id, created_at FROM links WHERE link_id = ?",
		id,
	)
	link, err := hydrateLink(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting link %s: %w", id, err)
	}
	return link, nil
}

// Set persists a link. If id is empty, generates a UUID v7 and creates the
// link. If id is provided, updates the existing link. Validates link type and
// enforces uniqueness of (link_type, from_id, to_id) (prd007-links-interface
// R2, R5).
func (lt *linksTable) Set(id string, data any) (string, error) {
	link, ok := data.(*types.Link)
	if !ok {
		return "", types.ErrInvalidData
	}

	if !types.ValidLinkType(link.LinkType) {
		return "", types.ErrInvalidData
	}
	if link.FromID == "" || link.ToID == "" {
		return "", types.ErrInvalidData
	}

	now := time.Now().UTC()

	if id == "" {
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		link.LinkID = newID.String()
		link.CreatedAt = now
		id = link.LinkID
	}

	// Check uniqueness of (link_type, from_id, to_id) excluding the current id
	// (prd007-links-interface R5).
	var dupID string
	err := lt.backend.db.QueryRow(
		"SELECT link_id FROM links WHERE link_type = ? AND from_id = ? AND to_id = ? AND link_id != ?",
		link.LinkType, link.FromID, link.ToID, id,
	).Scan(&dupID)
	if err == nil {
		return "", types.ErrDuplicateName
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("checking link uniqueness: %w", err)
	}

	var exists bool
	err = lt.backend.db.QueryRow(
		"SELECT 1 FROM links WHERE link_id = ?", id,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking link existence: %w", err)
	}

	tx, err := lt.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	createdAtStr := link.CreatedAt.Format(time.RFC3339)

	if exists {
		_, err = tx.Exec(
			"UPDATE links SET link_type = ?, from_id = ?, to_id = ?, created_at = ? WHERE link_id = ?",
			link.LinkType, link.FromID, link.ToID, createdAtStr, id,
		)
	} else {
		_, err = tx.Exec(
			"INSERT INTO links (link_id, link_type, from_id, to_id, created_at) VALUES (?, ?, ?, ?, ?)",
			id, link.LinkType, link.FromID, link.ToID, createdAtStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting link: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing link: %w", err)
	}

	if err := persistTableJSONL(lt.backend, "links", "links.jsonl"); err != nil {
		return "", fmt.Errorf("persisting links.jsonl: %w", err)
	}

	return id, nil
}

// Delete removes a link by ID (prd007-links-interface R4).
func (lt *linksTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	var exists bool
	err := lt.backend.db.QueryRow(
		"SELECT 1 FROM links WHERE link_id = ?", id,
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("checking link existence: %w", err)
	}

	tx, err := lt.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM links WHERE link_id = ?", id); err != nil {
		return fmt.Errorf("deleting link: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing link deletion: %w", err)
	}

	if err := persistTableJSONL(lt.backend, "links", "links.jsonl"); err != nil {
		return fmt.Errorf("persisting links.jsonl: %w", err)
	}

	return nil
}

// Fetch queries links matching the filter, ordered by created_at DESC.
// Supported filter keys: link_type, from_id, to_id (prd007-links-interface R4).
func (lt *linksTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT link_id, link_type, from_id, to_id, created_at FROM links"
	var conditions []string
	var args []any

	if filter != nil {
		if v, ok := filter["link_type"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "link_type = ?")
			args = append(args, s)
		}
		if v, ok := filter["from_id"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "from_id = ?")
			args = append(args, s)
		}
		if v, ok := filter["to_id"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "to_id = ?")
			args = append(args, s)
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

	rows, err := lt.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching links: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		link, err := hydrateLinkFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating link: %w", err)
		}
		results = append(results, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating links: %w", err)
	}

	if results == nil {
		results = []any{}
	}
	return results, nil
}

// hydrateLink converts a single SQLite row into a *types.Link
// (prd002-sqlite-backend R14.6).
func hydrateLink(row *sql.Row) (*types.Link, error) {
	var l types.Link
	var createdAt string
	if err := row.Scan(&l.LinkID, &l.LinkType, &l.FromID, &l.ToID, &createdAt); err != nil {
		return nil, err
	}
	var err error
	l.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &l, nil
}

// hydrateLinkFromRows converts a row from sql.Rows into a *types.Link.
func hydrateLinkFromRows(rows *sql.Rows) (*types.Link, error) {
	var l types.Link
	var createdAt string
	if err := rows.Scan(&l.LinkID, &l.LinkType, &l.FromID, &l.ToID, &createdAt); err != nil {
		return nil, err
	}
	var err error
	l.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &l, nil
}
