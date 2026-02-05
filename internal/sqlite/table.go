// Table implements the Table interface for a specific entity type.
// Implements: prd-sqlite-backend R13, R14, R15;
//             prd-cupboard-core R3;
//             docs/ARCHITECTURE § Table Interfaces.
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/petardjukic/crumbs/pkg/types"
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
		deleteFromJSON = t.backend.deleteCrumbFromJSON
	case types.TrailsTable:
		query = "DELETE FROM trails WHERE trail_id = ?"
		deleteFromJSON = t.backend.deleteTrailFromJSON
	case types.PropertiesTable:
		query = "DELETE FROM properties WHERE property_id = ?"
		deleteFromJSON = t.backend.deletePropertyFromJSON
	case types.MetadataTable:
		query = "DELETE FROM metadata WHERE metadata_id = ?"
		deleteFromJSON = t.backend.deleteMetadataFromJSON
	case types.LinksTable:
		query = "DELETE FROM links WHERE link_id = ?"
		deleteFromJSON = t.backend.deleteLinkFromJSON
	case types.StashesTable:
		query = "DELETE FROM stashes WHERE stash_id = ?"
		deleteFromJSON = t.backend.deleteStashFromJSON
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

	// Persist deletion to JSON (per R5)
	if err := deleteFromJSON(id); err != nil {
		return fmt.Errorf("persist deletion to JSON: %w", err)
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
	return hydrateCrumb(row)
}

func (t *Table) setCrumb(id string, crumb *types.Crumb) (string, error) {
	if id == "" {
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

	// Persist to JSON (per R5)
	if err := t.backend.saveCrumbToJSON(crumb); err != nil {
		return "", fmt.Errorf("persist crumb to JSON: %w", err)
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

// Trail operations

func (t *Table) getTrail(id string) (*types.Trail, error) {
	row := t.backend.db.QueryRow(
		"SELECT trail_id, parent_crumb_id, state, created_at, completed_at FROM trails WHERE trail_id = ?",
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

	var parentCrumbID, completedAt interface{}
	if trail.ParentCrumbID != nil {
		parentCrumbID = *trail.ParentCrumbID
	}
	if trail.CompletedAt != nil {
		completedAt = trail.CompletedAt.Format(time.RFC3339)
	}

	_, err := t.backend.db.Exec(
		`INSERT INTO trails (trail_id, parent_crumb_id, state, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(trail_id) DO UPDATE SET
		 parent_crumb_id = excluded.parent_crumb_id,
		 state = excluded.state,
		 completed_at = excluded.completed_at`,
		trail.TrailID,
		parentCrumbID,
		trail.State,
		trail.CreatedAt.Format(time.RFC3339),
		completedAt,
	)
	if err != nil {
		return "", err
	}

	// Persist to JSON (per R5)
	if err := t.backend.saveTrailToJSON(trail); err != nil {
		return "", fmt.Errorf("persist trail to JSON: %w", err)
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
	query := "SELECT trail_id, parent_crumb_id, state, created_at, completed_at FROM trails"
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
		"TrailID":       "trail_id",
		"ParentCrumbID": "parent_crumb_id",
		"State":         "state",
		"CreatedAt":     "created_at",
		"CompletedAt":   "completed_at",
	}
	return mapping[field]
}

func hydrateTrail(row *sql.Row) (*types.Trail, error) {
	var trail types.Trail
	var parentCrumbID, completedAt sql.NullString
	var createdAt string
	err := row.Scan(&trail.TrailID, &parentCrumbID, &trail.State, &createdAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	trail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if parentCrumbID.Valid {
		trail.ParentCrumbID = &parentCrumbID.String
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		trail.CompletedAt = &t
	}
	return &trail, nil
}

func hydrateTrailRow(rows *sql.Rows) (*types.Trail, error) {
	var trail types.Trail
	var parentCrumbID, completedAt sql.NullString
	var createdAt string
	err := rows.Scan(&trail.TrailID, &parentCrumbID, &trail.State, &createdAt, &completedAt)
	if err != nil {
		return nil, err
	}
	trail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if parentCrumbID.Valid {
		trail.ParentCrumbID = &parentCrumbID.String
	}
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
	if id == "" {
		id = generateUUID()
	}
	prop.PropertyID = id

	if prop.CreatedAt.IsZero() {
		prop.CreatedAt = time.Now()
	}

	_, err := t.backend.db.Exec(
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

	// Persist to JSON (per R5)
	if err := t.backend.savePropertyToJSON(prop); err != nil {
		return "", fmt.Errorf("persist property to JSON: %w", err)
	}

	return id, nil
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

	// Persist to JSON (per R5)
	if err := t.backend.saveMetadataToJSON(meta); err != nil {
		return "", fmt.Errorf("persist metadata to JSON: %w", err)
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

	// Persist to JSON (per R5)
	if err := t.backend.saveLinkToJSON(link); err != nil {
		return "", fmt.Errorf("persist link to JSON: %w", err)
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
		"SELECT stash_id, trail_id, name, stash_type, value, version, created_at, updated_at FROM stashes WHERE stash_id = ?",
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

	var trailID interface{}
	if stash.TrailID != nil {
		trailID = *stash.TrailID
	}

	// JSON encode the value
	valueJSON, err := json.Marshal(stash.Value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal stash value: %w", err)
	}

	_, err = t.backend.db.Exec(
		`INSERT INTO stashes (stash_id, trail_id, name, stash_type, value, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(stash_id) DO UPDATE SET
		 trail_id = excluded.trail_id,
		 name = excluded.name,
		 stash_type = excluded.stash_type,
		 value = excluded.value,
		 version = excluded.version,
		 updated_at = excluded.updated_at`,
		stash.StashID,
		trailID,
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

	// Persist to JSON (per R5)
	if err := t.backend.saveStashToJSON(stash, now); err != nil {
		return "", fmt.Errorf("persist stash to JSON: %w", err)
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
	query := "SELECT stash_id, trail_id, name, stash_type, value, version, created_at, updated_at FROM stashes"
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
		"TrailID":   "trail_id",
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
	var trailID sql.NullString
	var valueJSON, createdAt, updatedAt string
	err := row.Scan(&stash.StashID, &trailID, &stash.Name, &stash.StashType, &valueJSON, &stash.Version, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	stash.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if trailID.Valid {
		stash.TrailID = &trailID.String
	}
	// Unmarshal JSON value
	if valueJSON != "" {
		_ = json.Unmarshal([]byte(valueJSON), &stash.Value)
	}
	return &stash, nil
}

func hydrateStashRow(rows *sql.Rows) (*types.Stash, error) {
	var stash types.Stash
	var trailID sql.NullString
	var valueJSON, createdAt, updatedAt string
	err := rows.Scan(&stash.StashID, &trailID, &stash.Name, &stash.StashType, &valueJSON, &stash.Version, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	stash.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if trailID.Valid {
		stash.TrailID = &trailID.String
	}
	// Unmarshal JSON value
	if valueJSON != "" {
		_ = json.Unmarshal([]byte(valueJSON), &stash.Value)
	}
	return &stash, nil
}
