// Table implements the Table interface for a specific entity type.
// Implements: prd-sqlite-backend R13, R14, R15;
//
//	prd-configuration-directories R6;
//	prd-cupboard-core R3;
//	prd-crumbs-interface R3.7 (property initialization on crumb creation);
//	docs/ARCHITECTURE § Table Interfaces.
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// Table provides CRUD operations for a specific entity type.
type Table struct {
	backend   *Backend
	tableName string
}

// newTable creates a table accessor for the specified table.
func newTable(b *Backend, name string) *Table {
	return &Table{
		backend:   b,
		tableName: name,
	}
}

// Get retrieves an entity by ID.
// Returns ErrNotFound if the entity does not exist.
// Returns ErrCupboardDetached if the backend is not attached.
func (t *Table) Get(id string) (any, error) {
	t.backend.mu.RLock()
	defer t.backend.mu.RUnlock()

	if !t.backend.attached {
		return nil, types.ErrCupboardDetached
	}

	if id == "" {
		return nil, types.ErrInvalidID
	}

	switch t.tableName {
	case types.CrumbsTable:
		return t.getCrumb(id)
	case types.TrailsTable:
		return t.getTrail(id)
	case types.PropertiesTable:
		return t.getProperty(id)
	case types.MetadataTable:
		return t.getMetadata(id)
	case types.LinksTable:
		return t.getLink(id)
	case types.StashesTable:
		return t.getStash(id)
	default:
		return nil, types.ErrTableNotFound
	}
}

// Set persists an entity.
// If id is empty, generates a new UUID v7.
// Returns the actual ID used.
// Returns ErrCupboardDetached if the backend is not attached.
func (t *Table) Set(id string, data any) (string, error) {
	t.backend.mu.Lock()
	defer t.backend.mu.Unlock()

	if !t.backend.attached {
		return "", types.ErrCupboardDetached
	}

	switch t.tableName {
	case types.CrumbsTable:
		crumb, ok := data.(*types.Crumb)
		if !ok {
			return "", types.ErrInvalidData
		}
		return t.setCrumb(id, crumb)
	case types.TrailsTable:
		trail, ok := data.(*types.Trail)
		if !ok {
			return "", types.ErrInvalidData
		}
		return t.setTrail(id, trail)
	case types.PropertiesTable:
		prop, ok := data.(*types.Property)
		if !ok {
			return "", types.ErrInvalidData
		}
		return t.setProperty(id, prop)
	case types.MetadataTable:
		meta, ok := data.(*types.Metadata)
		if !ok {
			return "", types.ErrInvalidData
		}
		return t.setMetadata(id, meta)
	case types.LinksTable:
		link, ok := data.(*types.Link)
		if !ok {
			return "", types.ErrInvalidData
		}
		return t.setLink(id, link)
	case types.StashesTable:
		stash, ok := data.(*types.Stash)
		if !ok {
			return "", types.ErrInvalidData
		}
		return t.setStash(id, stash)
	default:
		return "", types.ErrTableNotFound
	}
}

// Delete removes an entity by ID.
// Returns ErrNotFound if the entity does not exist.
// Returns ErrCupboardDetached if the backend is not attached.
func (t *Table) Delete(id string) error {
	t.backend.mu.Lock()
	defer t.backend.mu.Unlock()

	if !t.backend.attached {
		return types.ErrCupboardDetached
	}

	if id == "" {
		return types.ErrInvalidID
	}

	var query string
	var deleteFromJSON func(string) error
	switch t.tableName {
	case types.CrumbsTable:
		query = "DELETE FROM crumbs WHERE crumb_id = ?"
		deleteFromJSON = t.backend.deleteCrumbFromJSONL
	case types.TrailsTable:
		query = "DELETE FROM trails WHERE trail_id = ?"
		deleteFromJSON = t.backend.deleteTrailFromJSONL
	case types.PropertiesTable:
		query = "DELETE FROM properties WHERE property_id = ?"
		deleteFromJSON = t.backend.deletePropertyFromJSONL
	case types.MetadataTable:
		query = "DELETE FROM metadata WHERE metadata_id = ?"
		deleteFromJSON = t.backend.deleteMetadataFromJSONL
	case types.LinksTable:
		query = "DELETE FROM links WHERE link_id = ?"
		deleteFromJSON = t.backend.deleteLinkFromJSONL
	case types.StashesTable:
		query = "DELETE FROM stashes WHERE stash_id = ?"
		deleteFromJSON = t.backend.deleteStashFromJSONL
	default:
		return types.ErrTableNotFound
	}

	result, err := t.backend.db.Exec(query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return types.ErrNotFound
	}

	// Persist deletion to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := deleteFromJSON(id); err != nil {
			return fmt.Errorf("persist deletion to JSONL: %w", err)
		}
	} else {
		// Capture id for deferred delete
		idCopy := id
		deleteFunc := deleteFromJSON
		t.backend.queueWrite(t.tableName, "delete", func() error {
			return deleteFunc(idCopy)
		})
	}

	return nil
}

// Fetch queries entities matching the filter.
// An empty filter returns all entities.
// Returns ErrCupboardDetached if the backend is not attached.
func (t *Table) Fetch(filter map[string]any) ([]any, error) {
	t.backend.mu.RLock()
	defer t.backend.mu.RUnlock()

	if !t.backend.attached {
		return nil, types.ErrCupboardDetached
	}

	switch t.tableName {
	case types.CrumbsTable:
		return t.fetchCrumbs(filter)
	case types.TrailsTable:
		return t.fetchTrails(filter)
	case types.PropertiesTable:
		return t.fetchProperties(filter)
	case types.MetadataTable:
		return t.fetchMetadata(filter)
	case types.LinksTable:
		return t.fetchLinks(filter)
	case types.StashesTable:
		return t.fetchStashes(filter)
	default:
		return nil, types.ErrTableNotFound
	}
}

// Crumb operations

func (t *Table) getCrumb(id string) (*types.Crumb, error) {
	row := t.backend.db.QueryRow(
		"SELECT crumb_id, name, state, created_at, updated_at FROM crumbs WHERE crumb_id = ?",
		id,
	)
	crumb, err := hydrateCrumb(row)
	if err != nil {
		return nil, err
	}
	if err := t.loadCrumbProperties(crumb); err != nil {
		return nil, fmt.Errorf("load crumb properties: %w", err)
	}
	return crumb, nil
}

func (t *Table) setCrumb(id string, crumb *types.Crumb) (string, error) {
	isNewCrumb := id == ""
	if isNewCrumb {
		id = generateUUID()
	}
	crumb.CrumbID = id

	if crumb.CreatedAt.IsZero() {
		crumb.CreatedAt = time.Now()
	}
	crumb.UpdatedAt = time.Now()

	_, err := t.backend.db.Exec(
		`INSERT INTO crumbs (crumb_id, name, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(crumb_id) DO UPDATE SET
		 name = excluded.name,
		 state = excluded.state,
		 updated_at = excluded.updated_at`,
		crumb.CrumbID,
		crumb.Name,
		crumb.State,
		crumb.CreatedAt.Format(time.RFC3339),
		crumb.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}

	// Persist to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := t.backend.saveCrumbToJSONL(crumb); err != nil {
			return "", fmt.Errorf("persist crumb to JSONL: %w", err)
		}
	} else {
		// Capture crumb state for deferred write
		crumbCopy := *crumb
		t.backend.queueWrite(types.CrumbsTable, "save", func() error {
			return t.backend.saveCrumbToJSONL(&crumbCopy)
		})
	}

	// Initialize all defined properties with type-based defaults on crumb creation
	// (per prd-crumbs-interface R3.7, prd-properties-interface R3.5)
	if isNewCrumb {
		if err := t.initializeCrumbProperties(crumb); err != nil {
			return "", fmt.Errorf("initialize crumb properties: %w", err)
		}
	}

	return id, nil
}

func (t *Table) fetchCrumbs(filter map[string]any) ([]any, error) {
	query, args := buildCrumbQuery(filter)
	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		crumb, err := hydrateCrumbRow(rows)
		if err != nil {
			return nil, err
		}
		if err := t.loadCrumbProperties(crumb); err != nil {
			return nil, fmt.Errorf("load crumb properties: %w", err)
		}
		results = append(results, crumb)
	}
	return results, rows.Err()
}

func buildCrumbQuery(filter map[string]any) (string, []any) {
	query := "SELECT crumb_id, name, state, created_at, updated_at FROM crumbs"
	var conditions []string
	var args []any

	for field, value := range filter {
		col := crumbFieldToColumn(field)
		if col != "" {
			conditions = append(conditions, col+" = ?")
			args = append(args, value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func crumbFieldToColumn(field string) string {
	mapping := map[string]string{
		"CrumbID":   "crumb_id",
		"Name":      "name",
		"State":     "state",
		"CreatedAt": "created_at",
		"UpdatedAt": "updated_at",
	}
	return mapping[field]
}

func hydrateCrumb(row *sql.Row) (*types.Crumb, error) {
	var crumb types.Crumb
	var createdAt, updatedAt string
	err := row.Scan(&crumb.CrumbID, &crumb.Name, &crumb.State, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	crumb.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	crumb.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &crumb, nil
}

func hydrateCrumbRow(rows *sql.Rows) (*types.Crumb, error) {
	var crumb types.Crumb
	var createdAt, updatedAt string
	err := rows.Scan(&crumb.CrumbID, &crumb.Name, &crumb.State, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	crumb.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	crumb.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &crumb, nil
}

// loadCrumbProperties populates the Properties map on a crumb by querying the crumb_properties table.
// Per prd-crumbs-interface R6, Table.Get must return crumbs with their Properties map populated.
func (t *Table) loadCrumbProperties(crumb *types.Crumb) error {
	rows, err := t.backend.db.Query(
		"SELECT property_id, value_type, value FROM crumb_properties WHERE crumb_id = ?",
		crumb.CrumbID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	if crumb.Properties == nil {
		crumb.Properties = make(map[string]any)
	}

	for rows.Next() {
		var propertyID, valueType, valueJSON string
		if err := rows.Scan(&propertyID, &valueType, &valueJSON); err != nil {
			return err
		}
		var value any
		if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
			return fmt.Errorf("unmarshal property %s: %w", propertyID, err)
		}
		crumb.Properties[propertyID] = value
	}
	return rows.Err()
}

// Trail operations

func (t *Table) getTrail(id string) (*types.Trail, error) {
	row := t.backend.db.QueryRow(
		"SELECT trail_id, state, created_at, completed_at FROM trails WHERE trail_id = ?",
		id,
	)
	return hydrateTrail(row)
}

func (t *Table) setTrail(id string, trail *types.Trail) (string, error) {
	if id == "" {
		id = generateUUID()
	}
	trail.TrailID = id

	if trail.CreatedAt.IsZero() {
		trail.CreatedAt = time.Now()
	}

	var completedAt interface{}
	if trail.CompletedAt != nil {
		completedAt = trail.CompletedAt.Format(time.RFC3339)
	}

	_, err := t.backend.db.Exec(
		`INSERT INTO trails (trail_id, state, created_at, completed_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(trail_id) DO UPDATE SET
		 state = excluded.state,
		 completed_at = excluded.completed_at`,
		trail.TrailID,
		trail.State,
		trail.CreatedAt.Format(time.RFC3339),
		completedAt,
	)
	if err != nil {
		return "", err
	}

	// Persist to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := t.backend.saveTrailToJSONL(trail); err != nil {
			return "", fmt.Errorf("persist trail to JSONL: %w", err)
		}
	} else {
		// Capture trail state for deferred write
		trailCopy := *trail
		t.backend.queueWrite(types.TrailsTable, "save", func() error {
			return t.backend.saveTrailToJSONL(&trailCopy)
		})
	}

	return id, nil
}

func (t *Table) fetchTrails(filter map[string]any) ([]any, error) {
	query, args := buildTrailQuery(filter)
	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		trail, err := hydrateTrailRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, trail)
	}
	return results, rows.Err()
}

func buildTrailQuery(filter map[string]any) (string, []any) {
	query := "SELECT trail_id, state, created_at, completed_at FROM trails"
	var conditions []string
	var args []any

	for field, value := range filter {
		col := trailFieldToColumn(field)
		if col != "" {
			conditions = append(conditions, col+" = ?")
			args = append(args, value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func trailFieldToColumn(field string) string {
	mapping := map[string]string{
		"TrailID":     "trail_id",
		"State":       "state",
		"CreatedAt":   "created_at",
		"CompletedAt": "completed_at",
	}
	return mapping[field]
}

func hydrateTrail(row *sql.Row) (*types.Trail, error) {
	var trail types.Trail
	var completedAt sql.NullString
	var createdAt string
	err := row.Scan(&trail.TrailID, &trail.State, &createdAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	trail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		trail.CompletedAt = &t
	}
	return &trail, nil
}

func hydrateTrailRow(rows *sql.Rows) (*types.Trail, error) {
	var trail types.Trail
	var completedAt sql.NullString
	var createdAt string
	err := rows.Scan(&trail.TrailID, &trail.State, &createdAt, &completedAt)
	if err != nil {
		return nil, err
	}
	trail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		trail.CompletedAt = &t
	}
	return &trail, nil
}

// Property operations

func (t *Table) getProperty(id string) (*types.Property, error) {
	row := t.backend.db.QueryRow(
		"SELECT property_id, name, description, value_type, created_at FROM properties WHERE property_id = ?",
		id,
	)
	return hydrateProperty(row)
}

func (t *Table) setProperty(id string, prop *types.Property) (string, error) {
	isNewProperty := id == ""
	if isNewProperty {
		id = generateUUID()
	}
	prop.PropertyID = id

	if prop.CreatedAt.IsZero() {
		prop.CreatedAt = time.Now()
	}

	// Use a transaction for atomicity (per prd-properties-interface R4.3)
	// If backfill fails, the property is not created.
	tx, err := t.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(
		`INSERT INTO properties (property_id, name, description, value_type, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(property_id) DO UPDATE SET
		 name = excluded.name,
		 description = excluded.description,
		 value_type = excluded.value_type`,
		prop.PropertyID,
		prop.Name,
		prop.Description,
		prop.ValueType,
		prop.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}

	// Backfill existing crumbs when creating a new property (per prd-properties-interface R4.2-R4.5)
	var backfillData []propertyInit
	if isNewProperty {
		backfillData, err = t.backfillExistingCrumbs(tx, prop)
		if err != nil {
			return "", fmt.Errorf("backfill existing crumbs: %w", err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	// Persist property to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := t.backend.savePropertyToJSONL(prop); err != nil {
			return "", fmt.Errorf("persist property to JSONL: %w", err)
		}
		// Persist backfilled crumb properties to JSONL (after transaction commits)
		for _, data := range backfillData {
			if err := t.backend.saveCrumbPropertyToJSONL(data.crumbID, prop.PropertyID, prop.ValueType, data.value); err != nil {
				return "", fmt.Errorf("persist backfilled property %s to JSONL: %w", data.crumbID, err)
			}
		}
	} else {
		// Capture property state for deferred write
		propCopy := *prop
		t.backend.queueWrite(types.PropertiesTable, "save", func() error {
			return t.backend.savePropertyToJSONL(&propCopy)
		})
		// Queue backfilled crumb properties
		for _, data := range backfillData {
			crumbID := data.crumbID
			propertyID := prop.PropertyID
			valueType := prop.ValueType
			value := data.value
			t.backend.queueWrite("crumb_properties", "save", func() error {
				return t.backend.saveCrumbPropertyToJSONL(crumbID, propertyID, valueType, value)
			})
		}
	}

	return id, nil
}

// backfillExistingCrumbs initializes the new property on all existing crumbs with the type-based default value.
// Called within a transaction when creating a new property.
// Per prd-properties-interface R4.2-R4.5.
func (t *Table) backfillExistingCrumbs(tx *sql.Tx, prop *types.Property) ([]propertyInit, error) {
	// Query all existing crumb IDs
	rows, err := tx.Query("SELECT crumb_id FROM crumbs")
	if err != nil {
		return nil, fmt.Errorf("query crumbs: %w", err)
	}

	var crumbIDs []string
	for rows.Next() {
		var crumbID string
		if err := rows.Scan(&crumbID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan crumb_id: %w", err)
		}
		crumbIDs = append(crumbIDs, crumbID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// No crumbs to backfill (no-op per requirements)
	if len(crumbIDs) == 0 {
		return nil, nil
	}

	// Get the default value for this property type
	defaultValue := getPropertyDefaultValue(prop.ValueType, t.backend, prop.PropertyID)

	// Marshal the default value for SQLite storage
	valueJSON, err := json.Marshal(defaultValue)
	if err != nil {
		return nil, fmt.Errorf("marshal default value: %w", err)
	}

	// Prepare the insert statement for efficiency
	stmt, err := tx.Prepare(
		`INSERT INTO crumb_properties (crumb_id, property_id, value_type, value)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(crumb_id, property_id) DO UPDATE SET
		 value_type = excluded.value_type,
		 value = excluded.value`,
	)
	if err != nil {
		return nil, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert property values for each crumb
	var backfillData []propertyInit
	for _, crumbID := range crumbIDs {
		_, err := stmt.Exec(crumbID, prop.PropertyID, prop.ValueType, string(valueJSON))
		if err != nil {
			return nil, fmt.Errorf("insert property for crumb %s: %w", crumbID, err)
		}
		backfillData = append(backfillData, propertyInit{
			crumbID: crumbID,
			value:   defaultValue,
		})
	}

	return backfillData, nil
}

func (t *Table) fetchProperties(filter map[string]any) ([]any, error) {
	query, args := buildPropertyQuery(filter)
	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		prop, err := hydratePropertyRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, prop)
	}
	return results, rows.Err()
}

func buildPropertyQuery(filter map[string]any) (string, []any) {
	query := "SELECT property_id, name, description, value_type, created_at FROM properties"
	var conditions []string
	var args []any

	for field, value := range filter {
		col := propertyFieldToColumn(field)
		if col != "" {
			conditions = append(conditions, col+" = ?")
			args = append(args, value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func propertyFieldToColumn(field string) string {
	mapping := map[string]string{
		"PropertyID":  "property_id",
		"Name":        "name",
		"Description": "description",
		"ValueType":   "value_type",
		"CreatedAt":   "created_at",
	}
	return mapping[field]
}

func hydrateProperty(row *sql.Row) (*types.Property, error) {
	var prop types.Property
	var description sql.NullString
	var createdAt string
	err := row.Scan(&prop.PropertyID, &prop.Name, &description, &prop.ValueType, &createdAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	prop.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if description.Valid {
		prop.Description = description.String
	}
	return &prop, nil
}

func hydratePropertyRow(rows *sql.Rows) (*types.Property, error) {
	var prop types.Property
	var description sql.NullString
	var createdAt string
	err := rows.Scan(&prop.PropertyID, &prop.Name, &description, &prop.ValueType, &createdAt)
	if err != nil {
		return nil, err
	}
	prop.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if description.Valid {
		prop.Description = description.String
	}
	return &prop, nil
}

// Metadata operations

func (t *Table) getMetadata(id string) (*types.Metadata, error) {
	row := t.backend.db.QueryRow(
		"SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata WHERE metadata_id = ?",
		id,
	)
	return hydrateMetadata(row)
}

func (t *Table) setMetadata(id string, meta *types.Metadata) (string, error) {
	if id == "" {
		id = generateUUID()
	}
	meta.MetadataID = id

	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}

	var propertyID interface{}
	if meta.PropertyID != nil {
		propertyID = *meta.PropertyID
	}

	_, err := t.backend.db.Exec(
		`INSERT INTO metadata (metadata_id, table_name, crumb_id, property_id, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(metadata_id) DO UPDATE SET
		 table_name = excluded.table_name,
		 crumb_id = excluded.crumb_id,
		 property_id = excluded.property_id,
		 content = excluded.content`,
		meta.MetadataID,
		meta.TableName,
		meta.CrumbID,
		propertyID,
		meta.Content,
		meta.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}

	// Persist to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := t.backend.saveMetadataToJSONL(meta); err != nil {
			return "", fmt.Errorf("persist metadata to JSONL: %w", err)
		}
	} else {
		// Capture metadata state for deferred write
		metaCopy := *meta
		t.backend.queueWrite(types.MetadataTable, "save", func() error {
			return t.backend.saveMetadataToJSONL(&metaCopy)
		})
	}

	return id, nil
}

func (t *Table) fetchMetadata(filter map[string]any) ([]any, error) {
	query, args := buildMetadataQuery(filter)
	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		meta, err := hydrateMetadataRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, meta)
	}
	return results, rows.Err()
}

func buildMetadataQuery(filter map[string]any) (string, []any) {
	query := "SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata"
	var conditions []string
	var args []any

	for field, value := range filter {
		col := metadataFieldToColumn(field)
		if col != "" {
			conditions = append(conditions, col+" = ?")
			args = append(args, value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func metadataFieldToColumn(field string) string {
	mapping := map[string]string{
		"MetadataID": "metadata_id",
		"TableName":  "table_name",
		"CrumbID":    "crumb_id",
		"PropertyID": "property_id",
		"Content":    "content",
		"CreatedAt":  "created_at",
	}
	return mapping[field]
}

func hydrateMetadata(row *sql.Row) (*types.Metadata, error) {
	var meta types.Metadata
	var propertyID sql.NullString
	var createdAt string
	err := row.Scan(&meta.MetadataID, &meta.TableName, &meta.CrumbID, &propertyID, &meta.Content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if propertyID.Valid {
		meta.PropertyID = &propertyID.String
	}
	return &meta, nil
}

func hydrateMetadataRow(rows *sql.Rows) (*types.Metadata, error) {
	var meta types.Metadata
	var propertyID sql.NullString
	var createdAt string
	err := rows.Scan(&meta.MetadataID, &meta.TableName, &meta.CrumbID, &propertyID, &meta.Content, &createdAt)
	if err != nil {
		return nil, err
	}
	meta.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if propertyID.Valid {
		meta.PropertyID = &propertyID.String
	}
	return &meta, nil
}

// Link operations

func (t *Table) getLink(id string) (*types.Link, error) {
	row := t.backend.db.QueryRow(
		"SELECT link_id, link_type, from_id, to_id, created_at FROM links WHERE link_id = ?",
		id,
	)
	return hydrateLink(row)
}

func (t *Table) setLink(id string, link *types.Link) (string, error) {
	if id == "" {
		id = generateUUID()
	}
	link.LinkID = id

	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now()
	}

	_, err := t.backend.db.Exec(
		`INSERT INTO links (link_id, link_type, from_id, to_id, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(link_id) DO UPDATE SET
		 link_type = excluded.link_type,
		 from_id = excluded.from_id,
		 to_id = excluded.to_id`,
		link.LinkID,
		link.LinkType,
		link.FromID,
		link.ToID,
		link.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}

	// Persist to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := t.backend.saveLinkToJSONL(link); err != nil {
			return "", fmt.Errorf("persist link to JSONL: %w", err)
		}
	} else {
		// Capture link state for deferred write
		linkCopy := *link
		t.backend.queueWrite(types.LinksTable, "save", func() error {
			return t.backend.saveLinkToJSONL(&linkCopy)
		})
	}

	return id, nil
}

func (t *Table) fetchLinks(filter map[string]any) ([]any, error) {
	query, args := buildLinkQuery(filter)
	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		link, err := hydrateLinkRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, link)
	}
	return results, rows.Err()
}

func buildLinkQuery(filter map[string]any) (string, []any) {
	query := "SELECT link_id, link_type, from_id, to_id, created_at FROM links"
	var conditions []string
	var args []any

	for field, value := range filter {
		col := linkFieldToColumn(field)
		if col != "" {
			conditions = append(conditions, col+" = ?")
			args = append(args, value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func linkFieldToColumn(field string) string {
	mapping := map[string]string{
		"LinkID":    "link_id",
		"LinkType":  "link_type",
		"FromID":    "from_id",
		"ToID":      "to_id",
		"CreatedAt": "created_at",
	}
	return mapping[field]
}

func hydrateLink(row *sql.Row) (*types.Link, error) {
	var link types.Link
	var createdAt string
	err := row.Scan(&link.LinkID, &link.LinkType, &link.FromID, &link.ToID, &createdAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	link.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &link, nil
}

func hydrateLinkRow(rows *sql.Rows) (*types.Link, error) {
	var link types.Link
	var createdAt string
	err := rows.Scan(&link.LinkID, &link.LinkType, &link.FromID, &link.ToID, &createdAt)
	if err != nil {
		return nil, err
	}
	link.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &link, nil
}

// Stash operations

func (t *Table) getStash(id string) (*types.Stash, error) {
	row := t.backend.db.QueryRow(
		"SELECT stash_id, name, stash_type, value, version, created_at, updated_at FROM stashes WHERE stash_id = ?",
		id,
	)
	return hydrateStash(row)
}

func (t *Table) setStash(id string, stash *types.Stash) (string, error) {
	if id == "" {
		id = generateUUID()
	}
	stash.StashID = id

	now := time.Now()
	if stash.CreatedAt.IsZero() {
		stash.CreatedAt = now
	}

	// JSON encode the value
	valueJSON, err := json.Marshal(stash.Value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal stash value: %w", err)
	}

	_, err = t.backend.db.Exec(
		`INSERT INTO stashes (stash_id, name, stash_type, value, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		 name = excluded.name,
		 stash_type = excluded.stash_type,
		 value = excluded.value,
		 version = excluded.version,
		 updated_at = excluded.updated_at`,
		stash.StashID,
		stash.Name,
		stash.StashType,
		string(valueJSON),
		stash.Version,
		stash.CreatedAt.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}

	// Persist to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		if err := t.backend.saveStashToJSONL(stash, now); err != nil {
			return "", fmt.Errorf("persist stash to JSONL: %w", err)
		}
	} else {
		// Capture stash state for deferred write
		stashCopy := *stash
		updatedAt := now
		t.backend.queueWrite(types.StashesTable, "save", func() error {
			return t.backend.saveStashToJSONL(&stashCopy, updatedAt)
		})
	}

	return id, nil
}

func (t *Table) fetchStashes(filter map[string]any) ([]any, error) {
	query, args := buildStashQuery(filter)
	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		stash, err := hydrateStashRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, stash)
	}
	return results, rows.Err()
}

func buildStashQuery(filter map[string]any) (string, []any) {
	query := "SELECT stash_id, name, stash_type, value, version, created_at, updated_at FROM stashes"
	var conditions []string
	var args []any

	for field, value := range filter {
		col := stashFieldToColumn(field)
		if col != "" {
			conditions = append(conditions, col+" = ?")
			args = append(args, value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func stashFieldToColumn(field string) string {
	mapping := map[string]string{
		"StashID":   "stash_id",
		"Name":      "name",
		"StashType": "stash_type",
		"Value":     "value",
		"Version":   "version",
		"CreatedAt": "created_at",
		"UpdatedAt": "updated_at",
	}
	return mapping[field]
}

func hydrateStash(row *sql.Row) (*types.Stash, error) {
	var stash types.Stash
	var valueJSON, createdAt, updatedAt string
	err := row.Scan(&stash.StashID, &stash.Name, &stash.StashType, &valueJSON, &stash.Version, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	stash.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	// Unmarshal JSON value
	if valueJSON != "" {
		_ = json.Unmarshal([]byte(valueJSON), &stash.Value)
	}
	return &stash, nil
}

func hydrateStashRow(rows *sql.Rows) (*types.Stash, error) {
	var stash types.Stash
	var valueJSON, createdAt, updatedAt string
	err := rows.Scan(&stash.StashID, &stash.Name, &stash.StashType, &valueJSON, &stash.Version, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	stash.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	// Unmarshal JSON value
	if valueJSON != "" {
		_ = json.Unmarshal([]byte(valueJSON), &stash.Value)
	}
	return &stash, nil
}

// Property initialization helpers
// Implements: prd-crumbs-interface R3.7; prd-properties-interface R3.5, R4.2-R4.5

// propertyInit holds data for a property to be initialized on a crumb.
// Used both for crumb creation (initializeCrumbProperties) and for property backfill (backfillExistingCrumbs).
type propertyInit struct {
	crumbID    string // The crumb ID (used for backfill)
	propertyID string // The property ID
	valueType  string // The property value type
	value      any    // The property value
}

// initializeCrumbProperties initializes all defined properties on a crumb with type-based defaults.
// Called when creating a new crumb (empty ID passed to Set).
func (t *Table) initializeCrumbProperties(crumb *types.Crumb) error {
	// Query all defined properties
	rows, err := t.backend.db.Query(
		"SELECT property_id, value_type FROM properties ORDER BY created_at ASC",
	)
	if err != nil {
		return err
	}

	// Collect all property definitions first (close rows before doing more DB operations)
	var props []propertyInit
	for rows.Next() {
		var propertyID, valueType string
		if err := rows.Scan(&propertyID, &valueType); err != nil {
			rows.Close()
			return err
		}
		props = append(props, propertyInit{propertyID: propertyID, valueType: valueType})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	// Initialize Properties map if nil
	if crumb.Properties == nil {
		crumb.Properties = make(map[string]any)
	}

	// For each property, get default value and set on crumb
	for i := range props {
		props[i].value = getPropertyDefaultValue(props[i].valueType, t.backend, props[i].propertyID)
		crumb.Properties[props[i].propertyID] = props[i].value
	}

	// Persist all properties to SQLite
	for _, prop := range props {
		valueJSON, err := json.Marshal(prop.value)
		if err != nil {
			return fmt.Errorf("marshal default value for property %s: %w", prop.propertyID, err)
		}
		_, err = t.backend.db.Exec(
			`INSERT INTO crumb_properties (crumb_id, property_id, value_type, value)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(crumb_id, property_id) DO UPDATE SET
			 value_type = excluded.value_type,
			 value = excluded.value`,
			crumb.CrumbID, prop.propertyID, prop.valueType, string(valueJSON),
		)
		if err != nil {
			return fmt.Errorf("persist crumb property %s: %w", prop.propertyID, err)
		}
	}

	// Persist all properties to JSONL based on sync strategy (prd-sqlite-backend R16)
	if t.backend.shouldPersistImmediately() {
		for _, prop := range props {
			if err := t.backend.saveCrumbPropertyToJSONL(crumb.CrumbID, prop.propertyID, prop.valueType, prop.value); err != nil {
				return fmt.Errorf("persist crumb property %s to JSONL: %w", prop.propertyID, err)
			}
		}
	} else {
		for _, prop := range props {
			crumbID := crumb.CrumbID
			propertyID := prop.propertyID
			valueType := prop.valueType
			value := prop.value
			t.backend.queueWrite("crumb_properties", "save", func() error {
				return t.backend.saveCrumbPropertyToJSONL(crumbID, propertyID, valueType, value)
			})
		}
	}

	return nil
}

// getPropertyDefaultValue returns the default value for a property type.
// Per prd-properties-interface R3.5:
//   - categorical: first category by ordinal, or empty string if no categories
//   - text: empty string
//   - integer: 0
//   - boolean: false
//   - timestamp: null (zero time)
//   - list: empty list
func getPropertyDefaultValue(valueType string, backend *Backend, propertyID string) any {
	switch valueType {
	case types.ValueTypeCategorical:
		// Get first category by ordinal for this property
		var categoryID sql.NullString
		_ = backend.db.QueryRow(
			`SELECT category_id FROM categories
			 WHERE property_id = ?
			 ORDER BY ordinal ASC, name ASC
			 LIMIT 1`,
			propertyID,
		).Scan(&categoryID)
		if categoryID.Valid {
			return categoryID.String
		}
		return "" // No categories defined yet
	case types.ValueTypeText:
		return ""
	case types.ValueTypeInteger:
		return int64(0)
	case types.ValueTypeBoolean:
		return false
	case types.ValueTypeTimestamp:
		return nil // null timestamp per R3.5
	case types.ValueTypeList:
		return []string{}
	default:
		return nil
	}
}
