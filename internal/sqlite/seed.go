// Package sqlite implements the SQLite backend for the Crumbs storage system.
// This file implements built-in property seeding on backend attach.
// Implements: prd002-sqlite-backend R9 (built-in properties seeding);
//             prd004-properties-interface R9 (built-in properties and categories).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mesh-intelligence/crumbs/pkg/types"
)

// builtInProperty describes a property to seed on first startup.
type builtInProperty struct {
	name        string
	valueType   string
	description string
	categories  []builtInCategory
}

// builtInCategory describes a category to seed with a built-in property.
type builtInCategory struct {
	name    string
	ordinal int
}

// builtInProperties defines the properties seeded on first startup
// (prd002-sqlite-backend R9.1, prd004-properties-interface R9.1).
var builtInProperties = []builtInProperty{
	{
		name:        types.PropertyPriority,
		valueType:   types.ValueTypeCategorical,
		description: "Task priority (0=highest, 4=lowest)",
		categories: []builtInCategory{
			{"highest", 0},
			{"high", 1},
			{"medium", 2},
			{"low", 3},
			{"lowest", 4},
		},
	},
	{
		name:        types.PropertyType,
		valueType:   types.ValueTypeCategorical,
		description: "Crumb type (task, epic, bug, etc.)",
		categories: []builtInCategory{
			{"task", 0},
			{"epic", 1},
			{"bug", 2},
			{"chore", 3},
		},
	},
	{
		name:        types.PropertyDescription,
		valueType:   types.ValueTypeText,
		description: "Detailed description",
	},
	{
		name:        types.PropertyOwner,
		valueType:   types.ValueTypeText,
		description: "Assigned worker/user ID",
	},
	{
		name:        types.PropertyLabels,
		valueType:   types.ValueTypeList,
		description: "Capability tags",
	},
}

// seedBuiltInProperties creates the built-in properties and categories if the
// properties table is empty (first run). Seeding is idempotent: it only runs
// when properties.jsonl was empty on startup (prd002-sqlite-backend R9.4,
// prd004-properties-interface R9.4).
func seedBuiltInProperties(db *sql.DB, dataDir string) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count); err != nil {
		return fmt.Errorf("counting properties: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning seed transaction: %w", err)
	}
	defer tx.Rollback()

	for _, bp := range builtInProperties {
		propID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generating property UUID: %w", err)
		}

		_, err = tx.Exec(
			"INSERT INTO properties (property_id, name, description, value_type, created_at) VALUES (?, ?, ?, ?, ?)",
			propID.String(), bp.name, bp.description, bp.valueType, nowStr,
		)
		if err != nil {
			return fmt.Errorf("seeding property %s: %w", bp.name, err)
		}

		for _, cat := range bp.categories {
			catID, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generating category UUID: %w", err)
			}
			_, err = tx.Exec(
				"INSERT INTO categories (category_id, property_id, name, ordinal) VALUES (?, ?, ?, ?)",
				catID.String(), propID.String(), cat.name, cat.ordinal,
			)
			if err != nil {
				return fmt.Errorf("seeding category %s for %s: %w", cat.name, bp.name, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing seed transaction: %w", err)
	}

	if err := persistSeededJSONL(db, dataDir); err != nil {
		return fmt.Errorf("persisting seeded data: %w", err)
	}

	return nil
}

// persistSeededJSONL writes the seeded properties and categories to their
// JSONL files after first-run seeding.
func persistSeededJSONL(db *sql.DB, dataDir string) error {
	rows, err := db.Query(
		"SELECT property_id, name, description, value_type, created_at FROM properties ORDER BY created_at ASC",
	)
	if err != nil {
		return fmt.Errorf("querying properties for seed JSONL: %w", err)
	}
	defer rows.Close()

	var propRecords []json.RawMessage
	for rows.Next() {
		var id, name, valueType, createdAt string
		var desc sql.NullString
		if err := rows.Scan(&id, &name, &desc, &valueType, &createdAt); err != nil {
			return fmt.Errorf("scanning property for seed JSONL: %w", err)
		}
		rec := map[string]any{
			"property_id": id,
			"name":        name,
			"description": desc.String,
			"value_type":  valueType,
			"created_at":  createdAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling property for seed JSONL: %w", err)
		}
		propRecords = append(propRecords, data)
	}
	rows.Close()
	if err := writeJSONL(filepath.Join(dataDir, "properties.jsonl"), propRecords); err != nil {
		return fmt.Errorf("writing properties.jsonl: %w", err)
	}

	catRows, err := db.Query(
		"SELECT category_id, property_id, name, ordinal FROM categories ORDER BY property_id, ordinal",
	)
	if err != nil {
		return fmt.Errorf("querying categories for seed JSONL: %w", err)
	}
	defer catRows.Close()

	var catRecords []json.RawMessage
	for catRows.Next() {
		var id, propID, name string
		var ordinal int
		if err := catRows.Scan(&id, &propID, &name, &ordinal); err != nil {
			return fmt.Errorf("scanning category for seed JSONL: %w", err)
		}
		rec := map[string]any{
			"category_id": id,
			"property_id": propID,
			"name":        name,
			"ordinal":     ordinal,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshaling category for seed JSONL: %w", err)
		}
		catRecords = append(catRecords, data)
	}
	catRows.Close()
	return writeJSONL(filepath.Join(dataDir, "categories.jsonl"), catRecords)
}
