// This file implements the metadata table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd005-metadata-interface R1, R4-R8 (Metadata entity, creation, retrieval,
//             lifecycle, filter, querying).
package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var _ types.Table = (*metadataTable)(nil)

// builtInSchemas contains the pre-registered metadata schemas
// (prd005-metadata-interface R3.1, R3.2).
var builtInSchemas = map[string]types.Schema{
	types.SchemaComments: {
		SchemaName:  types.SchemaComments,
		Description: "User comments and notes on crumbs",
		ContentType: types.ContentTypeText,
	},
	types.SchemaAttachments: {
		SchemaName:  types.SchemaAttachments,
		Description: "File attachments with name, path, and mime type",
		ContentType: types.ContentTypeJSON,
	},
}

type metadataTable struct {
	backend *Backend
}

// Get retrieves a metadata entry by ID (prd005-metadata-interface R5,
// prd002-sqlite-backend R14.5).
func (mt *metadataTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := mt.backend.db.QueryRow(
		"SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata WHERE metadata_id = ?",
		id,
	)
	m, err := hydrateMetadata(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting metadata %s: %w", id, err)
	}
	return m, nil
}

// Set persists a metadata entry. If id is empty, generates a UUID v7 and creates
// the entry with validation. If id is provided, updates the existing entry.
// Validates schema, crumb existence, content, and optional property reference
// (prd005-metadata-interface R4, prd002-sqlite-backend R13.3, R15).
func (mt *metadataTable) Set(id string, data any) (string, error) {
	m, ok := data.(*types.Metadata)
	if !ok {
		return "", types.ErrInvalidData
	}

	// Validate schema name (prd005-metadata-interface R4.2).
	if _, ok := builtInSchemas[m.TableName]; !ok {
		return "", types.ErrSchemaNotFound
	}

	// Validate content (prd005-metadata-interface R4.2).
	if m.Content == "" {
		return "", types.ErrInvalidContent
	}

	// Validate crumb exists (prd005-metadata-interface R4.2).
	var crumbExists bool
	err := mt.backend.db.QueryRow(
		"SELECT 1 FROM crumbs WHERE crumb_id = ?", m.CrumbID,
	).Scan(&crumbExists)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", types.ErrNotFound
		}
		return "", fmt.Errorf("checking crumb existence: %w", err)
	}

	// Validate property exists if set (prd005-metadata-interface R4.2).
	if m.PropertyID != nil && *m.PropertyID != "" {
		var propExists bool
		err := mt.backend.db.QueryRow(
			"SELECT 1 FROM properties WHERE property_id = ?", *m.PropertyID,
		).Scan(&propExists)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", types.ErrPropertyNotFound
			}
			return "", fmt.Errorf("checking property existence: %w", err)
		}
	}

	now := time.Now().UTC()

	if id == "" {
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		m.MetadataID = newID.String()
		m.CreatedAt = now
		id = m.MetadataID
	}

	var exists bool
	err = mt.backend.db.QueryRow(
		"SELECT 1 FROM metadata WHERE metadata_id = ?", id,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking metadata existence: %w", err)
	}

	tx, err := mt.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	createdAtStr := m.CreatedAt.Format(time.RFC3339)
	var propIDVal *string
	if m.PropertyID != nil && *m.PropertyID != "" {
		propIDVal = m.PropertyID
	}

	if exists {
		_, err = tx.Exec(
			"UPDATE metadata SET table_name = ?, crumb_id = ?, property_id = ?, content = ?, created_at = ? WHERE metadata_id = ?",
			m.TableName, m.CrumbID, propIDVal, m.Content, createdAtStr, id,
		)
	} else {
		_, err = tx.Exec(
			"INSERT INTO metadata (metadata_id, table_name, crumb_id, property_id, content, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			id, m.TableName, m.CrumbID, propIDVal, m.Content, createdAtStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting metadata: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing metadata: %w", err)
	}

	// Persist to metadata.jsonl (prd002-sqlite-backend R5.5).
	if err := persistTableJSONL(mt.backend, "metadata", "metadata.jsonl"); err != nil {
		return "", fmt.Errorf("persisting metadata.jsonl: %w", err)
	}

	return id, nil
}

// Delete removes a metadata entry by ID (prd005-metadata-interface R6.3, R6.4).
func (mt *metadataTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	var exists bool
	err := mt.backend.db.QueryRow(
		"SELECT 1 FROM metadata WHERE metadata_id = ?", id,
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("checking metadata existence: %w", err)
	}

	tx, err := mt.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM metadata WHERE metadata_id = ?", id); err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing metadata deletion: %w", err)
	}

	if err := persistTableJSONL(mt.backend, "metadata", "metadata.jsonl"); err != nil {
		return fmt.Errorf("persisting metadata.jsonl: %w", err)
	}

	return nil
}

// Fetch queries metadata matching the filter, ordered by created_at ASC
// (prd005-metadata-interface R7, R8).
func (mt *metadataTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata"
	var conditions []string
	var args []any

	if filter != nil {
		if v, ok := filter["schema"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "table_name = ?")
			args = append(args, s)
		}
		if v, ok := filter["crumb_id"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "crumb_id = ?")
			args = append(args, s)
		}
		if v, ok := filter["property_id"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "property_id = ?")
			args = append(args, s)
		}
		if v, ok := filter["content_contains"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "content LIKE ? COLLATE NOCASE")
			args = append(args, "%"+s+"%")
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Order by created_at ASC (prd005-metadata-interface R7.6).
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

	rows, err := mt.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		m, err := hydrateMetadataFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating metadata: %w", err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating metadata: %w", err)
	}

	// Return empty slice, not nil (prd005-metadata-interface R8.3).
	if results == nil {
		results = []any{}
	}
	return results, nil
}

// hydrateMetadata converts a single SQLite row into a *types.Metadata
// (prd002-sqlite-backend R14.5).
func hydrateMetadata(row *sql.Row) (*types.Metadata, error) {
	var m types.Metadata
	var createdAt string
	var propID sql.NullString
	if err := row.Scan(&m.MetadataID, &m.TableName, &m.CrumbID, &propID, &m.Content, &createdAt); err != nil {
		return nil, err
	}
	if propID.Valid {
		m.PropertyID = &propID.String
	}
	var err error
	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &m, nil
}

// hydrateMetadataFromRows converts a row from sql.Rows into a *types.Metadata.
func hydrateMetadataFromRows(rows *sql.Rows) (*types.Metadata, error) {
	var m types.Metadata
	var createdAt string
	var propID sql.NullString
	if err := rows.Scan(&m.MetadataID, &m.TableName, &m.CrumbID, &propID, &m.Content, &createdAt); err != nil {
		return nil, err
	}
	if propID.Valid {
		m.PropertyID = &propID.String
	}
	var err error
	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &m, nil
}
