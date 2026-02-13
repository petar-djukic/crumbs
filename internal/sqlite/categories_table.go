// This file implements the categories table accessor for the SQLite backend.
// Implements: prd002-sqlite-backend R12-R15 (table routing, interface, hydration, persistence);
//             prd004-properties-interface R2, R7, R8, R10 (Category entity, DefineCategory,
//             GetCategories, errors).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

var _ types.Table = (*categoriesTable)(nil)

type categoriesTable struct {
	backend *Backend
}

// Get retrieves a category by ID (prd004-properties-interface R7, R10).
func (ct *categoriesTable) Get(id string) (any, error) {
	if id == "" {
		return nil, types.ErrInvalidID
	}

	row := ct.backend.db.QueryRow(
		"SELECT category_id, property_id, name, ordinal FROM categories WHERE category_id = ?",
		id,
	)
	cat, err := hydrateCategory(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.ErrNotFound
		}
		return nil, fmt.Errorf("getting category %s: %w", id, err)
	}
	return cat, nil
}

// Set persists a category. If id is empty, generates a UUID v7 and creates the
// category with validation (property exists, name unique within property, property
// is categorical). If id is provided, updates the existing category
// (prd004-properties-interface R7).
func (ct *categoriesTable) Set(id string, data any) (string, error) {
	cat, ok := data.(*types.Category)
	if !ok {
		return "", types.ErrInvalidData
	}
	if cat.Name == "" {
		return "", types.ErrInvalidName
	}
	if cat.PropertyID == "" {
		return "", types.ErrInvalidID
	}

	isCreate := id == ""

	// Validate that the property exists and is categorical.
	var propValueType string
	err := ct.backend.db.QueryRow(
		"SELECT value_type FROM properties WHERE property_id = ?",
		cat.PropertyID,
	).Scan(&propValueType)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", types.ErrNotFound
		}
		return "", fmt.Errorf("checking property existence: %w", err)
	}
	if propValueType != types.ValueTypeCategorical {
		return "", types.ErrInvalidValueType
	}

	if isCreate {
		newID, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generating UUID v7: %w", err)
		}
		cat.CategoryID = newID.String()
		id = cat.CategoryID
	}

	// Check name uniqueness within property (prd004-properties-interface R2.4).
	var dupID string
	err = ct.backend.db.QueryRow(
		"SELECT category_id FROM categories WHERE property_id = ? AND name = ? AND category_id != ?",
		cat.PropertyID, cat.Name, id,
	).Scan(&dupID)
	if err == nil {
		return "", types.ErrDuplicateName
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("checking category name uniqueness: %w", err)
	}

	var exists bool
	err = ct.backend.db.QueryRow(
		"SELECT 1 FROM categories WHERE category_id = ?", id,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("checking category existence: %w", err)
	}

	tx, err := ct.backend.db.Begin()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if exists {
		_, err = tx.Exec(
			"UPDATE categories SET property_id = ?, name = ?, ordinal = ? WHERE category_id = ?",
			cat.PropertyID, cat.Name, cat.Ordinal, id,
		)
	} else {
		_, err = tx.Exec(
			"INSERT INTO categories (category_id, property_id, name, ordinal) VALUES (?, ?, ?, ?)",
			id, cat.PropertyID, cat.Name, cat.Ordinal,
		)
	}
	if err != nil {
		return "", fmt.Errorf("persisting category: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing category: %w", err)
	}

	// Persist to categories.jsonl.
	if err := persistTableJSONL(ct.backend, "categories", "categories.jsonl"); err != nil {
		return "", fmt.Errorf("persisting categories.jsonl: %w", err)
	}

	return id, nil
}

// Delete removes a category by ID (prd004-properties-interface R10).
func (ct *categoriesTable) Delete(id string) error {
	if id == "" {
		return types.ErrInvalidID
	}

	var exists bool
	err := ct.backend.db.QueryRow(
		"SELECT 1 FROM categories WHERE category_id = ?", id,
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		return fmt.Errorf("checking category existence: %w", err)
	}

	tx, err := ct.backend.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM categories WHERE category_id = ?", id); err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing category deletion: %w", err)
	}

	if err := persistTableJSONL(ct.backend, "categories", "categories.jsonl"); err != nil {
		return fmt.Errorf("persisting categories.jsonl: %w", err)
	}

	return nil
}

// Fetch queries categories matching the filter, ordered by ordinal ASC, name ASC
// (prd004-properties-interface R8.3, R8.4).
func (ct *categoriesTable) Fetch(filter types.Filter) ([]any, error) {
	query := "SELECT category_id, property_id, name, ordinal FROM categories"
	var conditions []string
	var args []any

	if filter != nil {
		if v, ok := filter["property_id"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, types.ErrInvalidFilter
			}
			conditions = append(conditions, "property_id = ?")
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

	// Order by ordinal ASC, name ASC (prd004-properties-interface R8.4).
	query += " ORDER BY ordinal ASC, name ASC"

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
		return nil, fmt.Errorf("fetching categories: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		cat, err := hydrateCategoryFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("hydrating category: %w", err)
		}
		results = append(results, cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating categories: %w", err)
	}

	if results == nil {
		results = []any{}
	}
	return results, nil
}

// hydrateCategory converts a single SQLite row into a *types.Category
// (prd002-sqlite-backend R14.4).
func hydrateCategory(row *sql.Row) (*types.Category, error) {
	var c types.Category
	if err := row.Scan(&c.CategoryID, &c.PropertyID, &c.Name, &c.Ordinal); err != nil {
		return nil, err
	}
	return &c, nil
}

// hydrateCategoryFromRows converts a row from sql.Rows into a *types.Category.
func hydrateCategoryFromRows(rows *sql.Rows) (*types.Category, error) {
	var c types.Category
	if err := rows.Scan(&c.CategoryID, &c.PropertyID, &c.Name, &c.Ordinal); err != nil {
		return nil, err
	}
	return &c, nil
}

// persistCategoryJSONL reads all categories from SQLite and writes them to
// categories.jsonl using the atomic write pattern.
func (ct *categoriesTable) persistCategoryJSONL() error {
	rows, err := ct.backend.db.Query(
		"SELECT category_id, property_id, name, ordinal FROM categories ORDER BY property_id, ordinal ASC, name ASC",
	)
	if err != nil {
		return fmt.Errorf("querying categories for JSONL: %w", err)
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		var id, propID, name string
		var ordinal int
		if err := rows.Scan(&id, &propID, &name, &ordinal); err != nil {
			return fmt.Errorf("scanning category for JSONL: %w", err)
		}
		rec := categoryJSONLRecord{
			CategoryID: id,
			PropertyID: propID,
			Name:       name,
			Ordinal:    ordinal,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling category for JSONL: %w", err)
		}
		records = append(records, data)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating categories for JSONL: %w", err)
	}

	return writeJSONL(
		fmt.Sprintf("%s/%s", ct.backend.config.DataDir, "categories.jsonl"),
		records,
	)
}

// categoryJSONLRecord matches the JSONL format for categories
// (prd002-sqlite-backend R2.4).
type categoryJSONLRecord struct {
	CategoryID string `json:"category_id"`
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	Ordinal    int    `json:"ordinal"`
}
