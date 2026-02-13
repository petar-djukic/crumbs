// This file implements the properties table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd004-properties-interface R1-R6, R10 (Property entity, creation, retrieval,
//             querying, backfill, errors).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var _ types.Table = (*propertiesTable)(nil)

type propertiesTable struct {
	backend *Backend
}

// defaultValueJSON returns the JSON-encoded default value for a property's
// value type (prd004-properties-interface R3.5).
func defaultValueJSON(valueType string) string {
	switch valueType {
	case types.ValueTypeCategorical:
		return "null"
	case types.ValueTypeText:
		return `""`
	case types.ValueTypeInteger:
		return "0"
	case types.ValueTypeBoolean:
		return "false"
	case types.ValueTypeTimestamp:
		return "null"
	case types.ValueTypeList:
		return "[]"
	default:
		return "null"
	}
}

// Get retrieves a property by ID (prd004-properties-interface R5,
// prd002-sqlite-backend R14.4).
func (pt *propertiesTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := pt.backend.db.QueryRow(
		"SELECT property_id, name, description, value_type, created_at FROM properties WHERE property_id = ?",
		id,
	)
	prop, err := hydrateProperty(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting property %s: %w", id, err)
	}
	return prop, nil
}

// Set persists a property. If id is empty, generates a UUID v7 and creates the
// property with validation and backfill to all existing crumbs. If id is
// provided, updates the existing property (prd004-properties-interface R4).
func (pt *propertiesTable) Set(id string, data any) (string, error) {
	prop, ok := data.(*types.Property)
	if !ok {
		return "", types.ErrInvalidData
	}
	if prop.Name == "" {
		return "", types.ErrInvalidName
	}
	if !types.ValidValueType(prop.ValueType) {
		return "", types.ErrInvalidValueType
	}

	now := time.Now().UTC()
	isCreate := id == ""

	if isCreate {
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		prop.PropertyID = newID.String()
		prop.CreatedAt = now
		id = prop.PropertyID
	}

	// Check name uniqueness (prd004-properties-interface R1.3).
	var dupID string
	err := pt.backend.db.QueryRow(
		"SELECT property_id FROM properties WHERE name = ? AND property_id != ?",
		prop.Name, id,
	).Scan(&dupID)
	if err == nil {
		return "", types.ErrDuplicateName
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("checking property name uniqueness: %w", err)
	}

	var exists bool
	err = pt.backend.db.QueryRow(
		"SELECT 1 FROM properties WHERE property_id = ?", id,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking property existence: %w", err)
	}

	tx, err := pt.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	createdAtStr := prop.CreatedAt.Format(time.RFC3339)
	var descPtr *string
	if prop.Description != "" {
		descPtr = &prop.Description
	}

	if exists {
		_, err = tx.Exec(
			"UPDATE properties SET name = ?, description = ?, value_type = ?, created_at = ? WHERE property_id = ?",
			prop.Name, descPtr, prop.ValueType, createdAtStr, id,
		)
	} else {
		_, err = tx.Exec(
			"INSERT INTO properties (property_id, name, description, value_type, created_at) VALUES (?, ?, ?, ?, ?)",
			id, prop.Name, descPtr, prop.ValueType, createdAtStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting property: %w", err)
	}

	// Backfill crumb_properties for all existing crumbs when creating a new
	// property (prd004-properties-interface R4.2, R4.3).
	if isCreate {
		defaultVal := defaultValueJSON(prop.ValueType)
		rows, err := tx.Query("SELECT crumb_id FROM crumbs")
		if err != nil {
			return "", fmt.Errorf("querying crumbs for backfill: %w", err)
		}
		var crumbIDs []string
		for rows.Next() {
			var cid string
			if err := rows.Scan(&cid); err != nil {
				rows.Close()
				return "", fmt.Errorf("scanning crumb id for backfill: %w", err)
			}
			crumbIDs = append(crumbIDs, cid)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("iterating crumbs for backfill: %w", err)
		}

		for _, cid := range crumbIDs {
			_, err := tx.Exec(
				"INSERT OR IGNORE INTO crumb_properties (crumb_id, property_id, value_type, value) VALUES (?, ?, ?, ?)",
				cid, id, prop.ValueType, defaultVal,
			)
			if err != nil {
				return "", fmt.Errorf("backfilling crumb_properties for crumb %s: %w", cid, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing property: %w", err)
	}

	// Persist affected JSONL files.
	if err := persistTableJSONL(pt.backend, "properties", "properties.jsonl"); err != nil {
		return "", fmt.Errorf("persisting properties.jsonl: %w", err)
	}
	if isCreate {
		if err := persistTableJSONL(pt.backend, "crumb_properties", "crumb_properties.jsonl"); err != nil {
			return "", fmt.Errorf("persisting crumb_properties.jsonl: %w", err)
		}
	}

	return id, nil
}

// Delete removes a property by ID (prd004-properties-interface R10).
func (pt *propertiesTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	var exists bool
	err := pt.backend.db.QueryRow(
		"SELECT 1 FROM properties WHERE property_id = ?", id,
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("checking property existence: %w", err)
	}

	tx, err := pt.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Cascade: remove crumb_properties and categories for this property.
	if _, err := tx.Exec("DELETE FROM crumb_properties WHERE property_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb_properties: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM categories WHERE property_id = ?", id); err != nil {
		return fmt.Errorf("deleting categories: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM properties WHERE property_id = ?", id); err != nil {
		return fmt.Errorf("deleting property: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing property deletion: %w", err)
	}

	if err := persistTableJSONL(pt.backend, "properties", "properties.jsonl"); err != nil {
		return fmt.Errorf("persisting properties.jsonl: %w", err)
	}
	if err := persistTableJSONL(pt.backend, "categories", "categories.jsonl"); err != nil {
		return fmt.Errorf("persisting categories.jsonl: %w", err)
	}
	if err := persistTableJSONL(pt.backend, "crumb_properties", "crumb_properties.jsonl"); err != nil {
		return fmt.Errorf("persisting crumb_properties.jsonl: %w", err)
	}

	return nil
}

// Fetch queries properties matching the filter, ordered by created_at ASC
// (oldest first, built-ins first) (prd004-properties-interface R6).
func (pt *propertiesTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT property_id, name, description, value_type, created_at FROM properties"
	var conditions []string
	var args []any

	if filter != nil {
		if v, ok := filter["name"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "name = ?")
			args = append(args, s)
		}
		if v, ok := filter["value_type"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "value_type = ?")
			args = append(args, s)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Order by created_at ASC (prd004-properties-interface R6.4).
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

	rows, err := pt.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching properties: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		prop, err := hydratePropertyFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating property: %w", err)
		}
		results = append(results, prop)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating properties: %w", err)
	}

	if results == nil {
		results = []any{}
	}
	return results, nil
}

// hydrateProperty converts a single SQLite row into a *types.Property
// (prd002-sqlite-backend R14.4).
func hydrateProperty(row *sql.Row) (*types.Property, error) {
	var p types.Property
	var createdAt string
	var desc sql.NullString
	if err := row.Scan(&p.PropertyID, &p.Name, &desc, &p.ValueType, &createdAt); err != nil {
		return nil, err
	}
	if desc.Valid {
		p.Description = desc.String
	}
	var err error
	p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &p, nil
}

// hydratePropertyFromRows converts a row from sql.Rows into a *types.Property.
func hydratePropertyFromRows(rows *sql.Rows) (*types.Property, error) {
	var p types.Property
	var createdAt string
	var desc sql.NullString
	if err := rows.Scan(&p.PropertyID, &p.Name, &desc, &p.ValueType, &createdAt); err != nil {
		return nil, err
	}
	if desc.Valid {
		p.Description = desc.String
	}
	var err error
	p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &p, nil
}

// persistPropertyJSONL reads all properties from SQLite and writes them to
// properties.jsonl using the atomic write pattern.
func (pt *propertiesTable) persistPropertyJSONL() error {
	rows, err := pt.backend.db.Query(
		"SELECT property_id, name, description, value_type, created_at FROM properties ORDER BY created_at ASC",
	)
	if err != nil {
		return fmt.Errorf("querying properties for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var id, name, valueType, createdAt string
		var desc sql.NullString
		if err := rows.Scan(&id, &name, &desc, &valueType, &createdAt); err != nil {
			return fmt.Errorf("scanning property for JSONL: %w", err)
		}
		rec := propertyJSONLRecord{
			PropertyID:  id,
			Name:        name,
			Description: desc.String,
			ValueType:   valueType,
			CreatedAt:   createdAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling property for JSONL: %w", err)
		}
		records = append(records, data)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating properties for JSONL: %w", err)
	}

	return writeJSONL(
		fmt.Sprintf("%s/%s", pt.backend.config.DataDir, "properties.jsonl"),
		records,
	)
}

// propertyJSONLRecord matches the JSONL format for properties
// (prd002-sqlite-backend R2.4).
type propertyJSONLRecord struct {
	PropertyID  string `json:"property_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ValueType   string `json:"value_type"`
	CreatedAt   string `json:"created_at"`
}
