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

// table implements types.Table for a single entity type.
// Each table knows its entity type, SQLite table name, primary JSONL file,
// and the backend it belongs to (for cross-table cascades and JSONL writes).
// Implements: prd001-cupboard-core R3, prd002-sqlite-backend R12.
type table struct {
	name    string         // Table name (e.g. "crumbs").
	backend *SQLiteBackend // Parent backend for DB access and JSONL writes.
}

// newUUID generates a UUID v7 string.
func newUUID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// Get retrieves an entity by ID.
// Returns ErrInvalidID if id is empty, ErrNotFound if not found.
// Implements: prd001-cupboard-core R3, prd002-sqlite-backend R14.
func (t *table) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}
	t.backend.mu.RLock()
	defer t.backend.mu.RUnlock()

	switch t.name {
	case types.TableCrumbs:
		return t.getCrumb(id)
	case types.TableTrails:
		return t.getTrail(id)
	case types.TableProperties:
		return t.getProperty(id)
	case types.TableMetadata:
		return t.getMetadata(id)
	case types.TableLinks:
		return t.getLink(id)
	case types.TableStashes:
		return t.getStash(id)
	default:
		return nil, types.ErrTableNotFound
	}
}

// Set creates or updates an entity. If id is empty, generates a UUID v7.
// Returns the entity ID and any error.
// Implements: prd001-cupboard-core R3, prd002-sqlite-backend R5, R15.
func (t *table) Set(id string, data any) (string, error) {
	t.backend.mu.Lock()
	defer t.backend.mu.Unlock()

	switch t.name {
	case types.TableCrumbs:
		return t.setCrumb(id, data)
	case types.TableTrails:
		return t.setTrail(id, data)
	case types.TableProperties:
		return t.setProperty(id, data)
	case types.TableMetadata:
		return t.setMetadata(id, data)
	case types.TableLinks:
		return t.setLink(id, data)
	case types.TableStashes:
		return t.setStash(id, data)
	default:
		return "", types.ErrTableNotFound
	}
}

// Delete removes an entity by ID with cascading deletes where appropriate.
// Returns ErrInvalidID if id is empty, ErrNotFound if not found.
// Implements: prd001-cupboard-core R3, prd002-sqlite-backend R5.5.
func (t *table) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}
	t.backend.mu.Lock()
	defer t.backend.mu.Unlock()

	switch t.name {
	case types.TableCrumbs:
		return t.deleteCrumb(id)
	case types.TableTrails:
		return t.deleteTrail(id)
	case types.TableProperties:
		return t.deleteProperty(id)
	case types.TableMetadata:
		return t.deleteMetadata(id)
	case types.TableLinks:
		return t.deleteLink(id)
	case types.TableStashes:
		return t.deleteStash(id)
	default:
		return types.ErrTableNotFound
	}
}

// Fetch returns entities matching the filter. Empty filter matches all.
// Implements: prd001-cupboard-core R3, prd003-crumbs-interface R9.
func (t *table) Fetch(filter map[string]any) ([]any, error) {
	t.backend.mu.RLock()
	defer t.backend.mu.RUnlock()

	switch t.name {
	case types.TableCrumbs:
		return t.fetchCrumbs(filter)
	case types.TableTrails:
		return t.fetchTrails(filter)
	case types.TableProperties:
		return t.fetchProperties(filter)
	case types.TableMetadata:
		return t.fetchMetadata(filter)
	case types.TableLinks:
		return t.fetchLinks(filter)
	case types.TableStashes:
		return t.fetchStashes(filter)
	default:
		return nil, types.ErrTableNotFound
	}
}

// Crumb CRUD operations.

func (t *table) getCrumb(id string) (any, error) {
	row := t.backend.db.QueryRow(
		"SELECT crumb_id, name, state, created_at, updated_at FROM crumbs WHERE crumb_id = ?", id)
	c, err := scanCrumb(row)
	if err != nil {
		return nil, err
	}
	if err := t.loadCrumbProperties(c); err != nil {
		return nil, err
	}
	return c, nil
}

func scanCrumb(row *sql.Row) (*types.Crumb, error) {
	var c types.Crumb
	var createdAt, updatedAt string
	err := row.Scan(&c.CrumbID, &c.Name, &c.State, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning crumb: %w", err)
	}
	c.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing crumb created_at: %w", err)
	}
	c.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing crumb updated_at: %w", err)
	}
	c.Properties = make(map[string]any)
	return &c, nil
}

func (t *table) loadCrumbProperties(c *types.Crumb) error {
	rows, err := t.backend.db.Query(
		"SELECT property_id, value FROM crumb_properties WHERE crumb_id = ?", c.CrumbID)
	if err != nil {
		return fmt.Errorf("loading crumb properties: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var propID, valueJSON string
		if err := rows.Scan(&propID, &valueJSON); err != nil {
			return fmt.Errorf("scanning crumb property: %w", err)
		}
		var val any
		if err := json.Unmarshal([]byte(valueJSON), &val); err != nil {
			return fmt.Errorf("parsing crumb property value: %w", err)
		}
		c.Properties[propID] = val
	}
	return rows.Err()
}

func (t *table) setCrumb(id string, data any) (string, error) {
	c, ok := data.(*types.Crumb)
	if !ok {
		return "", types.ErrInvalidData
	}
	if c.Name == "" {
		return "", types.ErrInvalidName
	}

	now := time.Now()
	isCreate := id == "" && c.CrumbID == ""

	if isCreate {
		c.CrumbID = newUUID()
		c.CreatedAt = now
		c.UpdatedAt = now
		if c.State == "" {
			c.State = types.CrumbStateDraft
		}
		// Initialize properties with defaults (prd003 R3.2).
		if err := t.initCrumbProperties(c); err != nil {
			return "", fmt.Errorf("initializing crumb properties: %w", err)
		}
	} else {
		if id != "" {
			c.CrumbID = id
		}
		c.UpdatedAt = now
	}

	// Upsert crumb.
	_, err := t.backend.db.Exec(`
		INSERT INTO crumbs (crumb_id, name, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(crumb_id) DO UPDATE SET
			name = excluded.name,
			state = excluded.state,
			updated_at = excluded.updated_at`,
		c.CrumbID, c.Name, c.State,
		c.CreatedAt.Format(time.RFC3339),
		c.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("upserting crumb: %w", err)
	}

	// Persist crumb properties.
	if err := t.persistCrumbProperties(c); err != nil {
		return "", err
	}

	// Persist JSONL (immediate sync).
	if err := t.persistCrumbsJSONL(); err != nil {
		return "", err
	}
	if err := t.persistCrumbPropertiesJSONL(); err != nil {
		return "", err
	}

	return c.CrumbID, nil
}

func (t *table) initCrumbProperties(c *types.Crumb) error {
	if c.Properties == nil {
		c.Properties = make(map[string]any)
	}
	rows, err := t.backend.db.Query("SELECT property_id, value_type FROM properties")
	if err != nil {
		return fmt.Errorf("loading properties: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var propID, valueType string
		if err := rows.Scan(&propID, &valueType); err != nil {
			return fmt.Errorf("scanning property: %w", err)
		}
		if _, exists := c.Properties[propID]; !exists {
			defVal, err := types.DefaultValue(valueType)
			if err != nil {
				return fmt.Errorf("getting default for %s: %w", valueType, err)
			}
			c.Properties[propID] = defVal
		}
	}
	return rows.Err()
}

func (t *table) persistCrumbProperties(c *types.Crumb) error {
	// Delete existing, re-insert all.
	if _, err := t.backend.db.Exec(
		"DELETE FROM crumb_properties WHERE crumb_id = ?", c.CrumbID); err != nil {
		return fmt.Errorf("clearing crumb properties: %w", err)
	}
	for propID, val := range c.Properties {
		valJSON, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshaling property %s: %w", propID, err)
		}
		if _, err := t.backend.db.Exec(
			"INSERT INTO crumb_properties (crumb_id, property_id, value) VALUES (?, ?, ?)",
			c.CrumbID, propID, string(valJSON)); err != nil {
			return fmt.Errorf("inserting crumb property: %w", err)
		}
	}
	return nil
}

func (t *table) deleteCrumb(id string) error {
	// Check existence.
	var exists int
	if err := t.backend.db.QueryRow(
		"SELECT 1 FROM crumbs WHERE crumb_id = ?", id).Scan(&exists); err == sql.ErrNoRows {
		return types.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("checking crumb: %w", err)
	}

	// Cascade delete: crumb_properties, metadata, links (prd002 R5.5).
	if _, err := t.backend.db.Exec(
		"DELETE FROM crumb_properties WHERE crumb_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb properties: %w", err)
	}
	if _, err := t.backend.db.Exec(
		"DELETE FROM metadata WHERE crumb_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb metadata: %w", err)
	}
	if _, err := t.backend.db.Exec(
		"DELETE FROM links WHERE from_id = ? OR to_id = ?", id, id); err != nil {
		return fmt.Errorf("deleting crumb links: %w", err)
	}
	if _, err := t.backend.db.Exec(
		"DELETE FROM crumbs WHERE crumb_id = ?", id); err != nil {
		return fmt.Errorf("deleting crumb: %w", err)
	}

	// Persist affected JSONL files.
	if err := t.persistCrumbsJSONL(); err != nil {
		return err
	}
	if err := t.persistCrumbPropertiesJSONL(); err != nil {
		return err
	}
	if err := t.persistMetadataJSONL(); err != nil {
		return err
	}
	if err := t.persistLinksJSONL(); err != nil {
		return err
	}
	return nil
}

func (t *table) fetchCrumbs(filter map[string]any) ([]any, error) {
	query := "SELECT crumb_id, name, state, created_at, updated_at FROM crumbs"
	var conditions []string
	var args []any

	if states, ok := filter["states"]; ok {
		ss, ok := states.([]string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		if len(ss) > 0 {
			placeholders := make([]string, len(ss))
			for i, s := range ss {
				placeholders[i] = "?"
				args = append(args, s)
			}
			conditions = append(conditions, "state IN ("+strings.Join(placeholders, ",")+")")
		}
	}

	if trailID, ok := filter["trail_id"]; ok {
		tid, ok := trailID.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions,
			"crumb_id IN (SELECT from_id FROM links WHERE link_type = 'belongs_to' AND to_id = ?)")
		args = append(args, tid)
	}

	if parentID, ok := filter["parent_id"]; ok {
		pid, ok := parentID.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions,
			"crumb_id IN (SELECT from_id FROM links WHERE link_type = 'child_of' AND to_id = ?)")
		args = append(args, pid)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if limit, ok := filter["limit"]; ok {
		l, ok := toInt(limit)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		if l > 0 {
			query += fmt.Sprintf(" LIMIT %d", l)
		}
	}
	if offset, ok := filter["offset"]; ok {
		o, ok := toInt(offset)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		if o > 0 {
			query += fmt.Sprintf(" OFFSET %d", o)
		}
	}

	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching crumbs: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var c types.Crumb
		var createdAt, updatedAt string
		if err := rows.Scan(&c.CrumbID, &c.Name, &c.State, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning crumb: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		c.Properties = make(map[string]any)
		if err := t.loadCrumbProperties(&c); err != nil {
			return nil, err
		}
		results = append(results, &c)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

// Trail CRUD operations.

func (t *table) getTrail(id string) (any, error) {
	var tr types.Trail
	var createdAt string
	var completedAt sql.NullString
	err := t.backend.db.QueryRow(
		"SELECT trail_id, state, created_at, completed_at FROM trails WHERE trail_id = ?", id).
		Scan(&tr.TrailID, &tr.State, &createdAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning trail: %w", err)
	}
	tr.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing trail created_at: %w", err)
	}
	if completedAt.Valid {
		ct, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parsing trail completed_at: %w", err)
		}
		tr.CompletedAt = &ct
	}
	return &tr, nil
}

func (t *table) setTrail(id string, data any) (string, error) {
	tr, ok := data.(*types.Trail)
	if !ok {
		return "", types.ErrInvalidData
	}

	now := time.Now()
	isCreate := id == "" && tr.TrailID == ""

	// Detect state change for cascade (prd002 R5.6, R5.7).
	var oldState string
	if !isCreate {
		effectiveID := id
		if effectiveID == "" {
			effectiveID = tr.TrailID
		}
		_ = t.backend.db.QueryRow(
			"SELECT state FROM trails WHERE trail_id = ?", effectiveID).Scan(&oldState)
	}

	if isCreate {
		tr.TrailID = newUUID()
		tr.CreatedAt = now
		if tr.State == "" {
			tr.State = types.TrailStateDraft
		}
	} else {
		if id != "" {
			tr.TrailID = id
		}
	}

	var completedAt sql.NullString
	if tr.CompletedAt != nil {
		completedAt = sql.NullString{String: tr.CompletedAt.Format(time.RFC3339), Valid: true}
	}

	_, err := t.backend.db.Exec(`
		INSERT INTO trails (trail_id, state, created_at, completed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(trail_id) DO UPDATE SET
			state = excluded.state,
			completed_at = excluded.completed_at`,
		tr.TrailID, tr.State,
		tr.CreatedAt.Format(time.RFC3339),
		completedAt)
	if err != nil {
		return "", fmt.Errorf("upserting trail: %w", err)
	}

	// Trail cascade (prd002 R5.6).
	if oldState == types.TrailStateActive {
		if tr.State == types.TrailStateCompleted {
			// Remove belongs_to links so crumbs become permanent.
			if _, err := t.backend.db.Exec(
				"DELETE FROM links WHERE link_type = 'belongs_to' AND to_id = ?", tr.TrailID); err != nil {
				return "", fmt.Errorf("completing trail cascade: %w", err)
			}
			if err := t.persistLinksJSONL(); err != nil {
				return "", err
			}
		} else if tr.State == types.TrailStateAbandoned {
			// Delete crumbs belonging to this trail, plus their properties, metadata, links.
			if err := t.abandonTrailCascade(tr.TrailID); err != nil {
				return "", err
			}
		}
	}

	if err := t.persistTrailsJSONL(); err != nil {
		return "", err
	}
	return tr.TrailID, nil
}

func (t *table) abandonTrailCascade(trailID string) error {
	// Find crumbs belonging to this trail.
	rows, err := t.backend.db.Query(
		"SELECT from_id FROM links WHERE link_type = 'belongs_to' AND to_id = ?", trailID)
	if err != nil {
		return fmt.Errorf("finding trail crumbs: %w", err)
	}
	defer rows.Close()

	var crumbIDs []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return fmt.Errorf("scanning trail crumb: %w", err)
		}
		crumbIDs = append(crumbIDs, cid)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, cid := range crumbIDs {
		if _, err := t.backend.db.Exec("DELETE FROM crumb_properties WHERE crumb_id = ?", cid); err != nil {
			return fmt.Errorf("deleting crumb properties in cascade: %w", err)
		}
		if _, err := t.backend.db.Exec("DELETE FROM metadata WHERE crumb_id = ?", cid); err != nil {
			return fmt.Errorf("deleting metadata in cascade: %w", err)
		}
		if _, err := t.backend.db.Exec("DELETE FROM links WHERE from_id = ? OR to_id = ?", cid, cid); err != nil {
			return fmt.Errorf("deleting links in cascade: %w", err)
		}
		if _, err := t.backend.db.Exec("DELETE FROM crumbs WHERE crumb_id = ?", cid); err != nil {
			return fmt.Errorf("deleting crumb in cascade: %w", err)
		}
	}

	// Also remove the trail's own links (branches_from, etc.)
	if _, err := t.backend.db.Exec(
		"DELETE FROM links WHERE from_id = ? OR to_id = ?", trailID, trailID); err != nil {
		return fmt.Errorf("deleting trail links: %w", err)
	}

	if err := t.persistCrumbsJSONL(); err != nil {
		return err
	}
	if err := t.persistCrumbPropertiesJSONL(); err != nil {
		return err
	}
	if err := t.persistMetadataJSONL(); err != nil {
		return err
	}
	if err := t.persistLinksJSONL(); err != nil {
		return err
	}
	return nil
}

func (t *table) deleteTrail(id string) error {
	var exists int
	if err := t.backend.db.QueryRow(
		"SELECT 1 FROM trails WHERE trail_id = ?", id).Scan(&exists); err == sql.ErrNoRows {
		return types.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("checking trail: %w", err)
	}
	if _, err := t.backend.db.Exec("DELETE FROM trails WHERE trail_id = ?", id); err != nil {
		return fmt.Errorf("deleting trail: %w", err)
	}
	return t.persistTrailsJSONL()
}

func (t *table) fetchTrails(filter map[string]any) ([]any, error) {
	query := "SELECT trail_id, state, created_at, completed_at FROM trails"
	var conditions []string
	var args []any

	if states, ok := filter["states"]; ok {
		ss, ok := states.([]string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		if len(ss) > 0 {
			placeholders := make([]string, len(ss))
			for i, s := range ss {
				placeholders[i] = "?"
				args = append(args, s)
			}
			conditions = append(conditions, "state IN ("+strings.Join(placeholders, ",")+")")
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if limit, ok := filter["limit"]; ok {
		l, ok := toInt(limit)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		if l > 0 {
			query += fmt.Sprintf(" LIMIT %d", l)
		}
	}
	if offset, ok := filter["offset"]; ok {
		o, ok := toInt(offset)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		if o > 0 {
			query += fmt.Sprintf(" OFFSET %d", o)
		}
	}

	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching trails: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var tr types.Trail
		var createdAt string
		var completedAt sql.NullString
		if err := rows.Scan(&tr.TrailID, &tr.State, &createdAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning trail: %w", err)
		}
		tr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if completedAt.Valid {
			ct, _ := time.Parse(time.RFC3339, completedAt.String)
			tr.CompletedAt = &ct
		}
		results = append(results, &tr)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

// Property CRUD operations.
// The properties table also handles Category entities via filter dispatch.

func (t *table) getProperty(id string) (any, error) {
	// Try property first.
	var p types.Property
	var createdAt string
	var desc sql.NullString
	err := t.backend.db.QueryRow(
		"SELECT property_id, name, description, value_type, created_at FROM properties WHERE property_id = ?", id).
		Scan(&p.PropertyID, &p.Name, &desc, &p.ValueType, &createdAt)
	if err == nil {
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if desc.Valid {
			p.Description = desc.String
		}
		return &p, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("scanning property: %w", err)
	}

	// Try category.
	var cat types.Category
	err = t.backend.db.QueryRow(
		"SELECT category_id, property_id, name, ordinal FROM categories WHERE category_id = ?", id).
		Scan(&cat.CategoryID, &cat.PropertyID, &cat.Name, &cat.Ordinal)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning category: %w", err)
	}
	return &cat, nil
}

func (t *table) setProperty(id string, data any) (string, error) {
	switch v := data.(type) {
	case *types.Property:
		return t.setPropertyEntity(id, v)
	case *types.Category:
		return t.setCategoryEntity(id, v)
	default:
		return "", types.ErrInvalidData
	}
}

func (t *table) setPropertyEntity(id string, p *types.Property) (string, error) {
	if p.Name == "" {
		return "", types.ErrInvalidName
	}
	if !types.IsValidValueType(p.ValueType) {
		return "", types.ErrInvalidValueType
	}

	now := time.Now()
	isCreate := id == "" && p.PropertyID == ""
	if isCreate {
		p.PropertyID = newUUID()
		p.CreatedAt = now
	} else if id != "" {
		p.PropertyID = id
	}

	// Check for duplicate name on create.
	if isCreate {
		var existing int
		err := t.backend.db.QueryRow(
			"SELECT 1 FROM properties WHERE name = ?", p.Name).Scan(&existing)
		if err == nil {
			return "", types.ErrDuplicateName
		}
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("checking property name: %w", err)
		}
	}

	var desc sql.NullString
	if p.Description != "" {
		desc = sql.NullString{String: p.Description, Valid: true}
	}

	_, err := t.backend.db.Exec(`
		INSERT INTO properties (property_id, name, description, value_type, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(property_id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			value_type = excluded.value_type`,
		p.PropertyID, p.Name, desc, p.ValueType,
		p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("upserting property: %w", err)
	}

	// Backfill existing crumbs with default value (prd004 R4.2).
	if isCreate {
		if err := t.backfillProperty(p); err != nil {
			return "", err
		}
	}

	if err := t.persistPropertiesJSONL(); err != nil {
		return "", err
	}
	if isCreate {
		if err := t.persistCrumbPropertiesJSONL(); err != nil {
			return "", err
		}
	}
	return p.PropertyID, nil
}

func (t *table) backfillProperty(p *types.Property) error {
	defVal, err := types.DefaultValue(p.ValueType)
	if err != nil {
		return fmt.Errorf("getting default for backfill: %w", err)
	}
	valJSON, err := json.Marshal(defVal)
	if err != nil {
		return fmt.Errorf("marshaling default for backfill: %w", err)
	}

	rows, err := t.backend.db.Query("SELECT crumb_id FROM crumbs")
	if err != nil {
		return fmt.Errorf("loading crumbs for backfill: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var crumbID string
		if err := rows.Scan(&crumbID); err != nil {
			return fmt.Errorf("scanning crumb for backfill: %w", err)
		}
		if _, err := t.backend.db.Exec(
			"INSERT OR IGNORE INTO crumb_properties (crumb_id, property_id, value) VALUES (?, ?, ?)",
			crumbID, p.PropertyID, string(valJSON)); err != nil {
			return fmt.Errorf("backfilling property: %w", err)
		}
	}
	return rows.Err()
}

func (t *table) setCategoryEntity(id string, cat *types.Category) (string, error) {
	if cat.Name == "" {
		return "", types.ErrInvalidName
	}
	if cat.PropertyID == "" {
		return "", types.ErrInvalidData
	}

	isCreate := id == "" && cat.CategoryID == ""
	if isCreate {
		cat.CategoryID = newUUID()
	} else if id != "" {
		cat.CategoryID = id
	}

	// Check duplicate name within property.
	if isCreate {
		var existing int
		err := t.backend.db.QueryRow(
			"SELECT 1 FROM categories WHERE property_id = ? AND name = ?",
			cat.PropertyID, cat.Name).Scan(&existing)
		if err == nil {
			return "", types.ErrDuplicateName
		}
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("checking category name: %w", err)
		}
	}

	_, err := t.backend.db.Exec(`
		INSERT INTO categories (category_id, property_id, name, ordinal)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(category_id) DO UPDATE SET
			name = excluded.name,
			ordinal = excluded.ordinal`,
		cat.CategoryID, cat.PropertyID, cat.Name, cat.Ordinal)
	if err != nil {
		return "", fmt.Errorf("upserting category: %w", err)
	}

	if err := t.persistCategoriesJSONL(); err != nil {
		return "", err
	}
	return cat.CategoryID, nil
}

func (t *table) deleteProperty(id string) error {
	// Try property first.
	var exists int
	err := t.backend.db.QueryRow(
		"SELECT 1 FROM properties WHERE property_id = ?", id).Scan(&exists)
	if err == nil {
		if _, err := t.backend.db.Exec("DELETE FROM properties WHERE property_id = ?", id); err != nil {
			return fmt.Errorf("deleting property: %w", err)
		}
		return t.persistPropertiesJSONL()
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("checking property: %w", err)
	}

	// Try category.
	err = t.backend.db.QueryRow(
		"SELECT 1 FROM categories WHERE category_id = ?", id).Scan(&exists)
	if err == sql.ErrNoRows {
		return types.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("checking category: %w", err)
	}
	if _, err := t.backend.db.Exec("DELETE FROM categories WHERE category_id = ?", id); err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}
	return t.persistCategoriesJSONL()
}

func (t *table) fetchProperties(filter map[string]any) ([]any, error) {
	// If filter has property_id, return categories for that property.
	if propID, ok := filter["property_id"]; ok {
		pid, ok := propID.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		return t.fetchCategories(pid)
	}

	query := "SELECT property_id, name, description, value_type, created_at FROM properties ORDER BY name"
	rows, err := t.backend.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("fetching properties: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var p types.Property
		var createdAt string
		var desc sql.NullString
		if err := rows.Scan(&p.PropertyID, &p.Name, &desc, &p.ValueType, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning property: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if desc.Valid {
			p.Description = desc.String
		}
		results = append(results, &p)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

func (t *table) fetchCategories(propertyID string) ([]any, error) {
	rows, err := t.backend.db.Query(
		"SELECT category_id, property_id, name, ordinal FROM categories WHERE property_id = ? ORDER BY ordinal, name",
		propertyID)
	if err != nil {
		return nil, fmt.Errorf("fetching categories: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var cat types.Category
		if err := rows.Scan(&cat.CategoryID, &cat.PropertyID, &cat.Name, &cat.Ordinal); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		results = append(results, &cat)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

// Metadata CRUD operations.

func (t *table) getMetadata(id string) (any, error) {
	var m types.Metadata
	var createdAt string
	var propID sql.NullString
	err := t.backend.db.QueryRow(
		"SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata WHERE metadata_id = ?", id).
		Scan(&m.MetadataID, &m.TableName, &m.CrumbID, &propID, &m.Content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning metadata: %w", err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if propID.Valid {
		m.PropertyID = &propID.String
	}
	return &m, nil
}

func (t *table) setMetadata(id string, data any) (string, error) {
	m, ok := data.(*types.Metadata)
	if !ok {
		return "", types.ErrInvalidData
	}
	if m.Content == "" {
		return "", types.ErrInvalidContent
	}
	if m.CrumbID == "" {
		return "", types.ErrInvalidData
	}

	now := time.Now()
	isCreate := id == "" && m.MetadataID == ""
	if isCreate {
		m.MetadataID = newUUID()
		m.CreatedAt = now
	} else if id != "" {
		m.MetadataID = id
	}

	var propID sql.NullString
	if m.PropertyID != nil {
		propID = sql.NullString{String: *m.PropertyID, Valid: true}
	}

	_, err := t.backend.db.Exec(`
		INSERT INTO metadata (metadata_id, table_name, crumb_id, property_id, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(metadata_id) DO UPDATE SET
			table_name = excluded.table_name,
			crumb_id = excluded.crumb_id,
			property_id = excluded.property_id,
			content = excluded.content`,
		m.MetadataID, m.TableName, m.CrumbID, propID, m.Content,
		m.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("upserting metadata: %w", err)
	}

	if err := t.persistMetadataJSONL(); err != nil {
		return "", err
	}
	return m.MetadataID, nil
}

func (t *table) deleteMetadata(id string) error {
	var exists int
	if err := t.backend.db.QueryRow(
		"SELECT 1 FROM metadata WHERE metadata_id = ?", id).Scan(&exists); err == sql.ErrNoRows {
		return types.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("checking metadata: %w", err)
	}
	if _, err := t.backend.db.Exec("DELETE FROM metadata WHERE metadata_id = ?", id); err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}
	return t.persistMetadataJSONL()
}

func (t *table) fetchMetadata(filter map[string]any) ([]any, error) {
	query := "SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata"
	var conditions []string
	var args []any

	if crumbID, ok := filter["crumb_id"]; ok {
		cid, ok := crumbID.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "crumb_id = ?")
		args = append(args, cid)
	}
	if tableName, ok := filter["table_name"]; ok {
		tn, ok := tableName.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "table_name = ?")
		args = append(args, tn)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var m types.Metadata
		var createdAt string
		var propID sql.NullString
		if err := rows.Scan(&m.MetadataID, &m.TableName, &m.CrumbID, &propID, &m.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning metadata: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if propID.Valid {
			m.PropertyID = &propID.String
		}
		results = append(results, &m)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

// Link CRUD operations.

func (t *table) getLink(id string) (any, error) {
	var l types.Link
	var createdAt string
	err := t.backend.db.QueryRow(
		"SELECT link_id, link_type, from_id, to_id, created_at FROM links WHERE link_id = ?", id).
		Scan(&l.LinkID, &l.LinkType, &l.FromID, &l.ToID, &createdAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning link: %w", err)
	}
	l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &l, nil
}

func (t *table) setLink(id string, data any) (string, error) {
	l, ok := data.(*types.Link)
	if !ok {
		return "", types.ErrInvalidData
	}
	if l.FromID == "" || l.ToID == "" {
		return "", types.ErrInvalidData
	}

	now := time.Now()
	isCreate := id == "" && l.LinkID == ""
	if isCreate {
		l.LinkID = newUUID()
		l.CreatedAt = now
	} else if id != "" {
		l.LinkID = id
	}

	_, err := t.backend.db.Exec(`
		INSERT INTO links (link_id, link_type, from_id, to_id, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(link_id) DO UPDATE SET
			link_type = excluded.link_type,
			from_id = excluded.from_id,
			to_id = excluded.to_id`,
		l.LinkID, l.LinkType, l.FromID, l.ToID,
		l.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("upserting link: %w", err)
	}

	if err := t.persistLinksJSONL(); err != nil {
		return "", err
	}
	return l.LinkID, nil
}

func (t *table) deleteLink(id string) error {
	var exists int
	if err := t.backend.db.QueryRow(
		"SELECT 1 FROM links WHERE link_id = ?", id).Scan(&exists); err == sql.ErrNoRows {
		return types.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("checking link: %w", err)
	}
	if _, err := t.backend.db.Exec("DELETE FROM links WHERE link_id = ?", id); err != nil {
		return fmt.Errorf("deleting link: %w", err)
	}
	return t.persistLinksJSONL()
}

func (t *table) fetchLinks(filter map[string]any) ([]any, error) {
	query := "SELECT link_id, link_type, from_id, to_id, created_at FROM links"
	var conditions []string
	var args []any

	if linkType, ok := filter["link_type"]; ok {
		lt, ok := linkType.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "link_type = ?")
		args = append(args, lt)
	}
	if fromID, ok := filter["from_id"]; ok {
		fid, ok := fromID.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "from_id = ?")
		args = append(args, fid)
	}
	if toID, ok := filter["to_id"]; ok {
		tid, ok := toID.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "to_id = ?")
		args = append(args, tid)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching links: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var l types.Link
		var createdAt string
		if err := rows.Scan(&l.LinkID, &l.LinkType, &l.FromID, &l.ToID, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning link: %w", err)
		}
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, &l)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

// Stash CRUD operations.

func (t *table) getStash(id string) (any, error) {
	var s types.Stash
	var createdAt, updatedAt string
	var valueJSON string
	var changedBy sql.NullString
	err := t.backend.db.QueryRow(
		"SELECT stash_id, name, stash_type, value, version, created_at, updated_at, last_operation, changed_by FROM stashes WHERE stash_id = ?", id).
		Scan(&s.StashID, &s.Name, &s.StashType, &valueJSON, &s.Version, &createdAt, &updatedAt, &s.LastOperation, &changedBy)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning stash: %w", err)
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if changedBy.Valid {
		s.ChangedBy = &changedBy.String
	}
	if valueJSON != "" && valueJSON != "null" {
		if err := json.Unmarshal([]byte(valueJSON), &s.Value); err != nil {
			return nil, fmt.Errorf("parsing stash value: %w", err)
		}
	}
	return &s, nil
}

func (t *table) setStash(id string, data any) (string, error) {
	s, ok := data.(*types.Stash)
	if !ok {
		return "", types.ErrInvalidData
	}
	if s.Name == "" {
		return "", types.ErrInvalidName
	}
	if !types.IsValidStashType(s.StashType) {
		return "", types.ErrInvalidStashType
	}

	now := time.Now()
	isCreate := id == "" && s.StashID == ""
	if isCreate {
		s.StashID = newUUID()
		s.CreatedAt = now
		if s.LastOperation == "" {
			s.LastOperation = types.StashOpCreate
		}
	} else if id != "" {
		s.StashID = id
	}

	valueJSON, err := json.Marshal(s.Value)
	if err != nil {
		return "", fmt.Errorf("marshaling stash value: %w", err)
	}

	var changedBy sql.NullString
	if s.ChangedBy != nil {
		changedBy = sql.NullString{String: *s.ChangedBy, Valid: true}
	}

	_, err = t.backend.db.Exec(`
		INSERT INTO stashes (stash_id, name, stash_type, value, version, created_at, updated_at, last_operation, changed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(stash_id) DO UPDATE SET
			name = excluded.name,
			value = excluded.value,
			version = excluded.version,
			updated_at = excluded.updated_at,
			last_operation = excluded.last_operation,
			changed_by = excluded.changed_by`,
		s.StashID, s.Name, s.StashType, string(valueJSON), s.Version,
		s.CreatedAt.Format(time.RFC3339), now.Format(time.RFC3339),
		s.LastOperation, changedBy)
	if err != nil {
		return "", fmt.Errorf("upserting stash: %w", err)
	}

	// Create history entry.
	entry := &types.StashHistoryEntry{
		HistoryID: newUUID(),
		StashID:   s.StashID,
		Version:   s.Version,
		Value:     s.Value,
		Operation: s.LastOperation,
		ChangedBy: s.ChangedBy,
		CreatedAt: now,
	}
	histValueJSON, err := json.Marshal(entry.Value)
	if err != nil {
		return "", fmt.Errorf("marshaling history value: %w", err)
	}
	var histChangedBy sql.NullString
	if entry.ChangedBy != nil {
		histChangedBy = sql.NullString{String: *entry.ChangedBy, Valid: true}
	}
	_, err = t.backend.db.Exec(`
		INSERT INTO stash_history (history_id, stash_id, version, value, operation, changed_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.HistoryID, entry.StashID, entry.Version, string(histValueJSON),
		entry.Operation, histChangedBy, entry.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("inserting stash history: %w", err)
	}

	if err := t.persistStashesJSONL(); err != nil {
		return "", err
	}
	if err := t.persistStashHistoryJSONL(); err != nil {
		return "", err
	}
	return s.StashID, nil
}

func (t *table) deleteStash(id string) error {
	var exists int
	if err := t.backend.db.QueryRow(
		"SELECT 1 FROM stashes WHERE stash_id = ?", id).Scan(&exists); err == sql.ErrNoRows {
		return types.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("checking stash: %w", err)
	}
	if _, err := t.backend.db.Exec("DELETE FROM stash_history WHERE stash_id = ?", id); err != nil {
		return fmt.Errorf("deleting stash history: %w", err)
	}
	if _, err := t.backend.db.Exec("DELETE FROM stashes WHERE stash_id = ?", id); err != nil {
		return fmt.Errorf("deleting stash: %w", err)
	}
	if err := t.persistStashesJSONL(); err != nil {
		return err
	}
	return t.persistStashHistoryJSONL()
}

func (t *table) fetchStashes(filter map[string]any) ([]any, error) {
	query := "SELECT stash_id, name, stash_type, value, version, created_at, updated_at, last_operation, changed_by FROM stashes"
	var conditions []string
	var args []any

	if name, ok := filter["name"]; ok {
		n, ok := name.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "name = ?")
		args = append(args, n)
	}
	if stashType, ok := filter["stash_type"]; ok {
		st, ok := stashType.(string)
		if !ok {
			return nil, types.ErrInvalidFilter
		}
		conditions = append(conditions, "stash_type = ?")
		args = append(args, st)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := t.backend.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching stashes: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var s types.Stash
		var createdAt, updatedAt, valueJSON string
		var changedBy sql.NullString
		if err := rows.Scan(&s.StashID, &s.Name, &s.StashType, &valueJSON, &s.Version,
			&createdAt, &updatedAt, &s.LastOperation, &changedBy); err != nil {
			return nil, fmt.Errorf("scanning stash: %w", err)
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if changedBy.Valid {
			s.ChangedBy = &changedBy.String
		}
		if valueJSON != "" && valueJSON != "null" {
			_ = json.Unmarshal([]byte(valueJSON), &s.Value)
		}
		results = append(results, &s)
	}
	if results == nil {
		results = []any{}
	}
	return results, rows.Err()
}

// JSONL persistence helpers. Each method reads all rows from the SQLite table
// and writes them to the corresponding JSONL file using the atomic pattern.
// Implements: prd002-sqlite-backend R5.2, R5.5.

func (t *table) persistCrumbsJSONL() error {
	rows, err := t.backend.db.Query("SELECT crumb_id, name, state, created_at, updated_at FROM crumbs ORDER BY created_at")
	if err != nil {
		return fmt.Errorf("reading crumbs for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var c types.Crumb
		var createdAt, updatedAt string
		if err := rows.Scan(&c.CrumbID, &c.Name, &c.State, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scanning crumb for JSONL: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		rec, err := dehydrateCrumb(&c)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "crumbs.jsonl"), records)
}

func (t *table) persistCrumbPropertiesJSONL() error {
	rows, err := t.backend.db.Query("SELECT crumb_id, property_id, value FROM crumb_properties ORDER BY crumb_id, property_id")
	if err != nil {
		return fmt.Errorf("reading crumb_properties for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var cp crumbProperty
		if err := rows.Scan(&cp.CrumbID, &cp.PropertyID, &cp.Value); err != nil {
			return fmt.Errorf("scanning crumb_property for JSONL: %w", err)
		}
		rec, err := dehydrateCrumbProperty(&cp)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "crumb_properties.jsonl"), records)
}

func (t *table) persistTrailsJSONL() error {
	rows, err := t.backend.db.Query("SELECT trail_id, state, created_at, completed_at FROM trails ORDER BY created_at")
	if err != nil {
		return fmt.Errorf("reading trails for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var tr types.Trail
		var createdAt string
		var completedAt sql.NullString
		if err := rows.Scan(&tr.TrailID, &tr.State, &createdAt, &completedAt); err != nil {
			return fmt.Errorf("scanning trail for JSONL: %w", err)
		}
		tr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if completedAt.Valid {
			ct, _ := time.Parse(time.RFC3339, completedAt.String)
			tr.CompletedAt = &ct
		}
		rec, err := dehydrateTrail(&tr)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "trails.jsonl"), records)
}

func (t *table) persistPropertiesJSONL() error {
	rows, err := t.backend.db.Query("SELECT property_id, name, description, value_type, created_at FROM properties ORDER BY name")
	if err != nil {
		return fmt.Errorf("reading properties for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var p types.Property
		var createdAt string
		var desc sql.NullString
		if err := rows.Scan(&p.PropertyID, &p.Name, &desc, &p.ValueType, &createdAt); err != nil {
			return fmt.Errorf("scanning property for JSONL: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if desc.Valid {
			p.Description = desc.String
		}
		rec, err := dehydrateProperty(&p)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "properties.jsonl"), records)
}

func (t *table) persistCategoriesJSONL() error {
	rows, err := t.backend.db.Query("SELECT category_id, property_id, name, ordinal FROM categories ORDER BY property_id, ordinal, name")
	if err != nil {
		return fmt.Errorf("reading categories for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var cat types.Category
		if err := rows.Scan(&cat.CategoryID, &cat.PropertyID, &cat.Name, &cat.Ordinal); err != nil {
			return fmt.Errorf("scanning category for JSONL: %w", err)
		}
		rec, err := dehydrateCategory(&cat)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "categories.jsonl"), records)
}

func (t *table) persistMetadataJSONL() error {
	rows, err := t.backend.db.Query("SELECT metadata_id, table_name, crumb_id, property_id, content, created_at FROM metadata ORDER BY created_at")
	if err != nil {
		return fmt.Errorf("reading metadata for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var m types.Metadata
		var createdAt string
		var propID sql.NullString
		if err := rows.Scan(&m.MetadataID, &m.TableName, &m.CrumbID, &propID, &m.Content, &createdAt); err != nil {
			return fmt.Errorf("scanning metadata for JSONL: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if propID.Valid {
			m.PropertyID = &propID.String
		}
		rec, err := dehydrateMetadata(&m)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "metadata.jsonl"), records)
}

func (t *table) persistLinksJSONL() error {
	rows, err := t.backend.db.Query("SELECT link_id, link_type, from_id, to_id, created_at FROM links ORDER BY created_at")
	if err != nil {
		return fmt.Errorf("reading links for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var l types.Link
		var createdAt string
		if err := rows.Scan(&l.LinkID, &l.LinkType, &l.FromID, &l.ToID, &createdAt); err != nil {
			return fmt.Errorf("scanning link for JSONL: %w", err)
		}
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rec, err := dehydrateLink(&l)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "links.jsonl"), records)
}

func (t *table) persistStashesJSONL() error {
	rows, err := t.backend.db.Query("SELECT stash_id, name, stash_type, value, version, created_at, updated_at, last_operation, changed_by FROM stashes ORDER BY created_at")
	if err != nil {
		return fmt.Errorf("reading stashes for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var s types.Stash
		var createdAt, updatedAt, valueJSON string
		var changedBy sql.NullString
		if err := rows.Scan(&s.StashID, &s.Name, &s.StashType, &valueJSON, &s.Version,
			&createdAt, &updatedAt, &s.LastOperation, &changedBy); err != nil {
			return fmt.Errorf("scanning stash for JSONL: %w", err)
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if changedBy.Valid {
			s.ChangedBy = &changedBy.String
		}
		if valueJSON != "" && valueJSON != "null" {
			_ = json.Unmarshal([]byte(valueJSON), &s.Value)
		}
		rec, err := dehydrateStash(&s)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "stashes.jsonl"), records)
}

func (t *table) persistStashHistoryJSONL() error {
	rows, err := t.backend.db.Query("SELECT history_id, stash_id, version, value, operation, changed_by, created_at FROM stash_history ORDER BY created_at")
	if err != nil {
		return fmt.Errorf("reading stash_history for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var h types.StashHistoryEntry
		var createdAt, valueJSON string
		var changedBy sql.NullString
		if err := rows.Scan(&h.HistoryID, &h.StashID, &h.Version, &valueJSON, &h.Operation, &changedBy, &createdAt); err != nil {
			return fmt.Errorf("scanning stash_history for JSONL: %w", err)
		}
		h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if changedBy.Valid {
			h.ChangedBy = &changedBy.String
		}
		if valueJSON != "" && valueJSON != "null" {
			_ = json.Unmarshal([]byte(valueJSON), &h.Value)
		}
		rec, err := dehydrateStashHistory(&h)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSONLAtomic(filepath.Join(t.backend.dataDir, "stash_history.jsonl"), records)
}

// toInt converts various numeric types to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
