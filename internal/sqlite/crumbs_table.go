// This file implements the crumbs table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd003-crumbs-interface R3 (creation), R6-R10 (retrieval/update/delete/filter);
//             prd004-properties-interface R3.5, R4.2 (auto-init on crumb creation);
//             prd001-cupboard-core R3 (Table interface), R8 (UUID v7).
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

// Compile-time interface check: crumbsTable must implement Table.
var _ types.Table = (*crumbsTable)(nil)

// crumbsTable implements the Table interface for the crumbs entity type.
// Each operation hydrates/dehydrates between SQLite rows and *types.Crumb
// structs, and persists changes to crumbs.jsonl atomically.
type crumbsTable struct {
	backend *Backend
}

// Get retrieves a crumb by ID, hydrates the row to *types.Crumb, and returns it
// (prd003-crumbs-interface R6, prd002-sqlite-backend R13.2, R14.2).
func (ct *crumbsTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := ct.backend.db.QueryRow(
		"SELECT crumb_id, name, state, created_at, updated_at FROM crumbs WHERE crumb_id = ?",
		id,
	)
	crumb, err := hydrateCrumb(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting crumb %s: %w", id, err)
	}
	if err := ct.hydrateProperties(crumb); err != nil {
		return nil, fmt.Errorf("hydrating properties for crumb %s: %w", id, err)
	}
	return crumb, nil
}

// Set persists a crumb. If id is empty, generates a UUID v7 and creates
// the crumb with defaults. If id is provided, updates the existing crumb.
// Returns the actual ID and any error (prd003-crumbs-interface R3, R7,
// prd002-sqlite-backend R13.3, R15).
func (ct *crumbsTable) Set(id string, data any) (string, error) {
	crumb, ok := data.(*types.Crumb)
	if !ok {
		return "", types.ErrInvalidData
	}
	if crumb.Name == "" {
		return "", types.ErrInvalidName
	}

	now := time.Now().UTC()

	isCreate := id == ""

	if isCreate {
		// Create: generate UUID v7, set defaults (prd003-crumbs-interface R3.2).
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		crumb.CrumbID = newID.String()
		crumb.State = types.StateDraft
		crumb.CreatedAt = now
		crumb.UpdatedAt = now
		if crumb.Properties == nil {
			crumb.Properties = make(map[string]any)
		}
		id = crumb.CrumbID
	}

	// Determine INSERT vs UPDATE (prd002-sqlite-backend R15.6).
	var exists bool
	err := ct.backend.db.QueryRow(
		"SELECT 1 FROM crumbs WHERE crumb_id = ?", id,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking crumb existence: %w", err)
	}

	tx, err := ct.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	createdAtStr := crumb.CreatedAt.Format(time.RFC3339)
	updatedAtStr := crumb.UpdatedAt.Format(time.RFC3339)

	if exists {
		// UPDATE existing crumb.
		_, err = tx.Exec(
			"UPDATE crumbs SET name = ?, state = ?, created_at = ?, updated_at = ? WHERE crumb_id = ?",
			crumb.Name, crumb.State, createdAtStr, updatedAtStr, id,
		)
	} else {
		// INSERT new crumb.
		_, err = tx.Exec(
			"INSERT INTO crumbs (crumb_id, name, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			id, crumb.Name, crumb.State, createdAtStr, updatedAtStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting crumb: %w", err)
	}

	// Auto-initialize all defined properties on new crumbs
	// (prd004-properties-interface R3.5, R4.2).
	if isCreate {
		if err := ct.autoInitProperties(tx, crumb); err != nil {
			return "", fmt.Errorf("auto-initializing properties: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing crumb: %w", err)
	}

	// Persist to crumbs.jsonl atomically (prd002-sqlite-backend R5.1, R5.2).
	if err := ct.persistAllCrumbsJSONL(); err != nil {
		return "", fmt.Errorf("persisting crumbs.jsonl: %w", err)
	}
	if isCreate {
		if err := persistTableJSONL(ct.backend, "crumb_properties", "crumb_properties.jsonl"); err != nil {
			return "", fmt.Errorf("persisting crumb_properties.jsonl: %w", err)
		}
	}

	return id, nil
}

// Delete removes a crumb and cascades to crumb_properties, metadata, and links
// (prd003-crumbs-interface R8, prd002-sqlite-backend R5.5, R13.4).
func (ct *crumbsTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	// Verify the crumb exists.
	var exists bool
	err := ct.backend.db.QueryRow(
		"SELECT 1 FROM crumbs WHERE crumb_id = ?", id,
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("checking crumb existence: %w", err)
	}

	tx, err := ct.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Cascade deletes (prd003-crumbs-interface R8.3).
	// Delete crumb_properties for this crumb.
	if _, err := tx.Exec("DELETE FROM crumb_properties WHERE crumb_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb properties: %w", err)
	}
	// Delete metadata for this crumb.
	if _, err := tx.Exec("DELETE FROM metadata WHERE crumb_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb metadata: %w", err)
	}
	// Delete links where this crumb is from_id or to_id.
	if _, err := tx.Exec("DELETE FROM links WHERE from_id = ? OR to_id = ?", id, id); err != nil {
		return fmt.Errorf("deleting crumb links: %w", err)
	}
	// Delete the crumb itself.
	if _, err := tx.Exec("DELETE FROM crumbs WHERE crumb_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing crumb deletion: %w", err)
	}

	// Persist affected JSONL files (prd002-sqlite-backend R5.5).
	if err := ct.persistAllCrumbsJSONL(); err != nil {
		return fmt.Errorf("persisting crumbs.jsonl: %w", err)
	}
	if err := persistTableJSONL(ct.backend, "crumb_properties", "crumb_properties.jsonl"); err != nil {
		return fmt.Errorf("persisting crumb_properties.jsonl: %w", err)
	}
	if err := persistTableJSONL(ct.backend, "metadata", "metadata.jsonl"); err != nil {
		return fmt.Errorf("persisting metadata.jsonl: %w", err)
	}
	if err := persistTableJSONL(ct.backend, "links", "links.jsonl"); err != nil {
		return fmt.Errorf("persisting links.jsonl: %w", err)
	}

	return nil
}

// Fetch queries crumbs matching the filter, ordered by created_at DESC
// (prd003-crumbs-interface R9, R10, prd002-sqlite-backend R13.5).
func (ct *crumbsTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT crumbs.crumb_id, crumbs.name, crumbs.state, crumbs.created_at, crumbs.updated_at FROM crumbs"
	var conditions []string
	var args []any
	var joins []string

	if filter != nil {
		// Filter by states (prd003-crumbs-interface R9.2).
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
				conditions = append(conditions, "crumbs.state IN ("+strings.Join(placeholders, ", ")+")")
			}
		}

		// Filter by trail_id via belongs_to link (prd003-crumbs-interface R9.2).
		if v, ok := filter["trail_id"]; ok {
			trailID, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			joins = append(joins, "INNER JOIN links AS lt ON lt.from_id = crumbs.crumb_id AND lt.link_type = 'belongs_to'")
			conditions = append(conditions, "lt.to_id = ?")
			args = append(args, trailID)
		}

		// Filter by parent_id via child_of link (prd003-crumbs-interface R9.2).
		if v, ok := filter["parent_id"]; ok {
			parentID, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			joins = append(joins, "INNER JOIN links AS lp ON lp.from_id = crumbs.crumb_id AND lp.link_type = 'child_of'")
			conditions = append(conditions, "lp.to_id = ?")
			args = append(args, parentID)
		}
	}

	// Build the final query.
	for _, j := range joins {
		query += " " + j
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Order by created_at DESC (prd003-crumbs-interface R9.6).
	query += " ORDER BY crumbs.created_at DESC"

	// Apply limit and offset after ordering (prd003-crumbs-interface R10.4).
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

	rows, err := ct.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching crumbs: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		crumb, err := hydrateCrumbFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating crumb: %w", err)
		}
		results = append(results, crumb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating crumbs: %w", err)
	}

	// Hydrate properties for each crumb.
	for _, r := range results {
		crumb := r.(*types.Crumb)
		if err := ct.hydrateProperties(crumb); err != nil {
			return nil, fmt.Errorf("hydrating properties for crumb %s: %w", crumb.CrumbID, err)
		}
	}

	// Return empty slice, not nil (prd003-crumbs-interface R10.3).
	if results == nil {
		results = []any{}
	}
	return results, nil
}

// autoInitProperties inserts a crumb_properties row for every defined property,
// using the type's default value. Explicit values already in crumb.Properties
// are preserved (prd004-properties-interface R3.5, R4.2).
func (ct *crumbsTable) autoInitProperties(tx *sql.Tx, crumb *types.Crumb) error {
	rows, err := tx.Query("SELECT property_id, value_type FROM properties")
	if err != nil {
		return fmt.Errorf("querying properties for auto-init: %w", err)
	}
	type propDef struct {
		id        string
		valueType string
	}
	var props []propDef
	for rows.Next() {
		var pd propDef
		if err := rows.Scan(&pd.id, &pd.valueType); err != nil {
			rows.Close()
			return fmt.Errorf("scanning property for auto-init: %w", err)
		}
		props = append(props, pd)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating properties for auto-init: %w", err)
	}

	for _, pd := range props {
		// If the caller already set an explicit value, persist it instead of
		// the type default.
		if explicitVal, ok := crumb.Properties[pd.id]; ok {
			valJSON, err := json.Marshal(explicitVal)
			if err != nil {
				return fmt.Errorf("marshaling explicit property %s: %w", pd.id, err)
			}
			_, err = tx.Exec(
				"INSERT OR IGNORE INTO crumb_properties (crumb_id, property_id, value_type, value) VALUES (?, ?, ?, ?)",
				crumb.CrumbID, pd.id, pd.valueType, string(valJSON),
			)
			if err != nil {
				return fmt.Errorf("inserting explicit property %s: %w", pd.id, err)
			}
			continue
		}
		defaultVal := defaultValueJSON(pd.valueType)
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO crumb_properties (crumb_id, property_id, value_type, value) VALUES (?, ?, ?, ?)",
			crumb.CrumbID, pd.id, pd.valueType, defaultVal,
		)
		if err != nil {
			return fmt.Errorf("inserting default property %s: %w", pd.id, err)
		}
		crumb.Properties[pd.id] = parseDefaultValue(pd.valueType)
	}
	return nil
}

// hydrateProperties loads crumb_properties rows into the crumb's Properties map.
func (ct *crumbsTable) hydrateProperties(crumb *types.Crumb) error {
	rows, err := ct.backend.db.Query(
		"SELECT property_id, value_type, value FROM crumb_properties WHERE crumb_id = ?",
		crumb.CrumbID,
	)
	if err != nil {
		return fmt.Errorf("querying crumb_properties: %w", err)
	}
	defer rows.Close()

	props := make(map[string]any)
	for rows.Next() {
		var propID, valueType, rawValue string
		if err := rows.Scan(&propID, &valueType, &rawValue); err != nil {
			return fmt.Errorf("scanning crumb_property: %w", err)
		}
		var parsed any
		if err := json.Unmarshal([]byte(rawValue), &parsed); err != nil {
			return fmt.Errorf("parsing property value for %s: %w", propID, err)
		}
		props[propID] = parsed
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating crumb_properties: %w", err)
	}

	crumb.Properties = props
	return nil
}

// parseDefaultValue returns the Go value corresponding to a value type's
// default (prd004-properties-interface R3.5).
func parseDefaultValue(valueType string) any {
	switch valueType {
	case types.ValueTypeCategorical:
		return nil
	case types.ValueTypeText:
		return ""
	case types.ValueTypeInteger:
		return float64(0) // JSON numbers decode as float64
	case types.ValueTypeBoolean:
		return false
	case types.ValueTypeTimestamp:
		return nil
	case types.ValueTypeList:
		return []any{}
	default:
		return nil
	}
}

// hydrateCrumb converts a single SQLite row into a *types.Crumb
// (prd002-sqlite-backend R14.2).
func hydrateCrumb(row *sql.Row) (*types.Crumb, error) {
	var c types.Crumb
	var createdAt, updatedAt string
	if err := row.Scan(&c.CrumbID, &c.Name, &c.State, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var err error
	c.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	c.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}
	return &c, nil
}

// hydrateCrumbFromRows converts a row from sql.Rows into a *types.Crumb.
func hydrateCrumbFromRows(rows *sql.Rows) (*types.Crumb, error) {
	var c types.Crumb
	var createdAt, updatedAt string
	if err := rows.Scan(&c.CrumbID, &c.Name, &c.State, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var err error
	c.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	c.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}
	return &c, nil
}

// persistAllCrumbsJSONL reads all crumbs from SQLite and writes them to
// crumbs.jsonl using the atomic write pattern.
func (ct *crumbsTable) persistAllCrumbsJSONL() error {
	rows, err := ct.backend.db.Query(
		"SELECT crumb_id, name, state, created_at, updated_at FROM crumbs ORDER BY created_at ASC",
	)
	if err != nil {
		return fmt.Errorf("querying crumbs for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var id, name, state, createdAt, updatedAt string
		if err := rows.Scan(&id, &name, &state, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scanning crumb for JSONL: %w", err)
		}
		rec := crumbJSONLRecord{
			CrumbID:   id,
			Name:      name,
			State:     state,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling crumb for JSONL: %w", err)
		}
		records = append(records, data)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating crumbs for JSONL: %w", err)
	}

	return persistCrumbsJSONL(ct.backend.config.DataDir, records)
}

// persistTableJSONL reads all rows from the given SQLite table and writes
// them as JSONL to the given filename, using the atomic write pattern.
// Shared across all table accessors.
func persistTableJSONL(b *Backend, tableName, fileName string) error {
	rows, err := b.db.Query("SELECT * FROM " + tableName)
	if err != nil {
		return fmt.Errorf("querying %s for JSONL: %w", tableName, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("getting columns for %s: %w", tableName, err)
	}

	var records []json.RawMessage
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("scanning %s row: %w", tableName, err)
		}
		rec := make(map[string]any, len(cols))
		for i, col := range cols {
			rec[col] = values[i]
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling %s row: %w", tableName, err)
		}
		records = append(records, data)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating %s for JSONL: %w", tableName, err)
	}

	return writeJSONL(
		fmt.Sprintf("%s/%s", b.config.DataDir, fileName),
		records,
	)
}

// crumbJSONLRecord matches the JSONL format for crumbs (prd002-sqlite-backend R2.2).
type crumbJSONLRecord struct {
	CrumbID   string `json:"crumb_id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
