// JSONL persistence for the SQLite backend.
// Implements: prd-configuration-directories R3, R4, R5, R6;
//
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/dukaforge/crumbs/pkg/types"
)

// JSONL file names per prd-configuration-directories R4.
const (
	crumbsJSONL       = "crumbs.jsonl"
	trailsJSONL       = "trails.jsonl"
	linksJSONL        = "links.jsonl"
	propertiesJSONL   = "properties.jsonl"
	categoriesJSONL   = "categories.jsonl"
	crumbPropsJSONL   = "crumb_properties.jsonl"
	metadataJSONL     = "metadata.jsonl"
	stashesJSONL      = "stashes.jsonl"
	stashHistoryJSONL = "stash_history.jsonl"
)

// initJSONLFiles creates empty JSONL files if they do not exist (per R4.3).
func (b *Backend) initJSONLFiles() error {
	files := []string{
		crumbsJSONL, trailsJSONL, linksJSONL, propertiesJSONL, categoriesJSONL,
		crumbPropsJSONL, metadataJSONL, stashesJSONL, stashHistoryJSONL,
	}
	for _, name := range files {
		path := filepath.Join(b.config.DataDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Create empty file (zero bytes, not empty array per R4.3)
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("init %s: %w", name, err)
			}
			f.Close()
		}
	}
	return nil
}

// loadAllJSONL loads all JSONL files into SQLite (per R5.1).
func (b *Backend) loadAllJSONL() error {
	dataDir := b.config.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	// Load crumbs
	if err := b.loadCrumbsJSONL(filepath.Join(dataDir, crumbsJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", crumbsJSONL, err)
	}

	// Load trails
	if err := b.loadTrailsJSONL(filepath.Join(dataDir, trailsJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", trailsJSONL, err)
	}

	// Load links
	if err := b.loadLinksJSONL(filepath.Join(dataDir, linksJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", linksJSONL, err)
	}

	// Load properties
	if err := b.loadPropertiesJSONL(filepath.Join(dataDir, propertiesJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", propertiesJSONL, err)
	}

	// Load categories
	if err := b.loadCategoriesJSONL(filepath.Join(dataDir, categoriesJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", categoriesJSONL, err)
	}

	// Load crumb properties
	if err := b.loadCrumbPropertiesJSONL(filepath.Join(dataDir, crumbPropsJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", crumbPropsJSONL, err)
	}

	// Load metadata
	if err := b.loadMetadataJSONL(filepath.Join(dataDir, metadataJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", metadataJSONL, err)
	}

	// Load stashes
	if err := b.loadStashesJSONL(filepath.Join(dataDir, stashesJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", stashesJSONL, err)
	}

	// Load stash history
	if err := b.loadStashHistoryJSONL(filepath.Join(dataDir, stashHistoryJSONL)); err != nil {
		return fmt.Errorf("load %s: %w", stashHistoryJSONL, err)
	}

	return nil
}

// loadCrumbsJSONL loads crumbs from JSONL file into SQLite.
func (b *Backend) loadCrumbsJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var c crumbJSON
		if err := json.Unmarshal(data, &c); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil // skip malformed line per R5.2
		}
		_, err := b.db.Exec(
			`INSERT INTO crumbs (crumb_id, name, state, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			c.CrumbID, c.Name, c.State, c.CreatedAt, c.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert crumb %s: %w", c.CrumbID, err)
		}
		return nil
	})
}

// loadTrailsJSONL loads trails from JSONL file into SQLite.
func (b *Backend) loadTrailsJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var t trailJSON
		if err := json.Unmarshal(data, &t); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		_, err := b.db.Exec(
			`INSERT INTO trails (trail_id, parent_crumb_id, state, created_at, completed_at)
			 VALUES (?, ?, ?, ?, ?)`,
			t.TrailID, t.ParentCrumbID, t.State, t.CreatedAt, t.CompletedAt,
		)
		if err != nil {
			return fmt.Errorf("insert trail %s: %w", t.TrailID, err)
		}
		return nil
	})
}

// loadLinksJSONL loads links from JSONL file into SQLite.
func (b *Backend) loadLinksJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var l linkJSON
		if err := json.Unmarshal(data, &l); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		_, err := b.db.Exec(
			`INSERT INTO links (link_id, link_type, from_id, to_id, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			l.LinkID, l.LinkType, l.FromID, l.ToID, l.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert link %s: %w", l.LinkID, err)
		}
		return nil
	})
}

// loadPropertiesJSONL loads properties from JSONL file into SQLite.
func (b *Backend) loadPropertiesJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var p propertyJSON
		if err := json.Unmarshal(data, &p); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		_, err := b.db.Exec(
			`INSERT INTO properties (property_id, name, description, value_type, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			p.PropertyID, p.Name, p.Description, p.ValueType, p.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert property %s: %w", p.PropertyID, err)
		}
		return nil
	})
}

// loadCategoriesJSONL loads categories from JSONL file into SQLite.
func (b *Backend) loadCategoriesJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var c categoryJSON
		if err := json.Unmarshal(data, &c); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		_, err := b.db.Exec(
			`INSERT INTO categories (category_id, property_id, name, ordinal)
			 VALUES (?, ?, ?, ?)`,
			c.CategoryID, c.PropertyID, c.Name, c.Ordinal,
		)
		if err != nil {
			return fmt.Errorf("insert category %s: %w", c.CategoryID, err)
		}
		return nil
	})
}

// loadCrumbPropertiesJSONL loads crumb property values from JSONL file into SQLite.
func (b *Backend) loadCrumbPropertiesJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var p crumbPropertyJSON
		if err := json.Unmarshal(data, &p); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		valueJSON, err := json.Marshal(p.Value)
		if err != nil {
			log.Printf("warning: %s line %d: marshal value: %v", path, lineNum, err)
			return nil
		}
		_, err = b.db.Exec(
			`INSERT INTO crumb_properties (crumb_id, property_id, value_type, value)
			 VALUES (?, ?, ?, ?)`,
			p.CrumbID, p.PropertyID, p.ValueType, string(valueJSON),
		)
		if err != nil {
			return fmt.Errorf("insert crumb property %s/%s: %w", p.CrumbID, p.PropertyID, err)
		}
		return nil
	})
}

// loadMetadataJSONL loads metadata from JSONL file into SQLite.
func (b *Backend) loadMetadataJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var m metadataJSON
		if err := json.Unmarshal(data, &m); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		_, err := b.db.Exec(
			`INSERT INTO metadata (metadata_id, table_name, crumb_id, property_id, content, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			m.MetadataID, m.TableName, m.CrumbID, m.PropertyID, m.Content, m.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert metadata %s: %w", m.MetadataID, err)
		}
		return nil
	})
}

// loadStashesJSONL loads stashes from JSONL file into SQLite.
func (b *Backend) loadStashesJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var s stashJSON
		if err := json.Unmarshal(data, &s); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		valueJSON, err := json.Marshal(s.Value)
		if err != nil {
			log.Printf("warning: %s line %d: marshal value: %v", path, lineNum, err)
			return nil
		}
		_, err = b.db.Exec(
			`INSERT INTO stashes (stash_id, trail_id, name, stash_type, value, version, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			s.StashID, s.TrailID, s.Name, s.StashType, string(valueJSON), s.Version, s.CreatedAt, s.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert stash %s: %w", s.StashID, err)
		}
		return nil
	})
}

// loadStashHistoryJSONL loads stash history from JSONL file into SQLite.
func (b *Backend) loadStashHistoryJSONL(path string) error {
	return readJSONLFile(path, func(lineNum int, data []byte) error {
		var h stashHistoryJSON
		if err := json.Unmarshal(data, &h); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			return nil
		}
		valueJSON, err := json.Marshal(h.Value)
		if err != nil {
			log.Printf("warning: %s line %d: marshal value: %v", path, lineNum, err)
			return nil
		}
		_, err = b.db.Exec(
			`INSERT INTO stash_history (history_id, stash_id, version, value, operation, changed_by, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			h.HistoryID, h.StashID, h.Version, string(valueJSON), h.Operation, h.ChangedBy, h.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert stash history %s: %w", h.HistoryID, err)
		}
		return nil
	})
}

// readJSONLFile reads a JSONL file line by line, calling handler for each valid line.
// Empty lines are skipped per R3.2. Malformed lines are logged and skipped per R5.2.
func readJSONLFile(path string, handler func(lineNum int, data []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Empty by default
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		// Skip empty lines per R3.2
		if len(line) == 0 {
			continue
		}
		if err := handler(lineNum, line); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// saveCrumbToJSONL persists crumb to JSONL file after SQLite write (per R6).
func (b *Backend) saveCrumbToJSONL(crumb *types.Crumb) error {
	path := filepath.Join(b.config.DataDir, crumbsJSONL)
	return updateJSONLFile(path, crumb.CrumbID, "crumb_id", func() ([]byte, error) {
		return json.Marshal(crumbJSON{
			CrumbID:   crumb.CrumbID,
			Name:      crumb.Name,
			State:     crumb.State,
			CreatedAt: crumb.CreatedAt.Format(time.RFC3339),
			UpdatedAt: crumb.UpdatedAt.Format(time.RFC3339),
		})
	})
}

// deleteCrumbFromJSONL removes crumb from JSONL file after SQLite delete (per R6).
func (b *Backend) deleteCrumbFromJSONL(id string) error {
	path := filepath.Join(b.config.DataDir, crumbsJSONL)
	return deleteFromJSONLFile(path, id, "crumb_id")
}

// saveTrailToJSONL persists trail to JSONL file after SQLite write.
func (b *Backend) saveTrailToJSONL(trail *types.Trail) error {
	path := filepath.Join(b.config.DataDir, trailsJSONL)
	return updateJSONLFile(path, trail.TrailID, "trail_id", func() ([]byte, error) {
		var completedAt *string
		if trail.CompletedAt != nil {
			s := trail.CompletedAt.Format(time.RFC3339)
			completedAt = &s
		}
		return json.Marshal(trailJSON{
			TrailID:       trail.TrailID,
			ParentCrumbID: trail.ParentCrumbID,
			State:         trail.State,
			CreatedAt:     trail.CreatedAt.Format(time.RFC3339),
			CompletedAt:   completedAt,
		})
	})
}

// deleteTrailFromJSONL removes trail from JSONL file after SQLite delete.
func (b *Backend) deleteTrailFromJSONL(id string) error {
	path := filepath.Join(b.config.DataDir, trailsJSONL)
	return deleteFromJSONLFile(path, id, "trail_id")
}

// saveLinkToJSONL persists link to JSONL file after SQLite write.
func (b *Backend) saveLinkToJSONL(link *types.Link) error {
	path := filepath.Join(b.config.DataDir, linksJSONL)
	return updateJSONLFile(path, link.LinkID, "link_id", func() ([]byte, error) {
		return json.Marshal(linkJSON{
			LinkID:    link.LinkID,
			LinkType:  link.LinkType,
			FromID:    link.FromID,
			ToID:      link.ToID,
			CreatedAt: link.CreatedAt.Format(time.RFC3339),
		})
	})
}

// deleteLinkFromJSONL removes link from JSONL file after SQLite delete.
func (b *Backend) deleteLinkFromJSONL(id string) error {
	path := filepath.Join(b.config.DataDir, linksJSONL)
	return deleteFromJSONLFile(path, id, "link_id")
}

// savePropertyToJSONL persists property to JSONL file after SQLite write.
func (b *Backend) savePropertyToJSONL(prop *types.Property) error {
	path := filepath.Join(b.config.DataDir, propertiesJSONL)
	return updateJSONLFile(path, prop.PropertyID, "property_id", func() ([]byte, error) {
		return json.Marshal(propertyJSON{
			PropertyID:  prop.PropertyID,
			Name:        prop.Name,
			Description: prop.Description,
			ValueType:   prop.ValueType,
			CreatedAt:   prop.CreatedAt.Format(time.RFC3339),
		})
	})
}

// deletePropertyFromJSONL removes property from JSONL file after SQLite delete.
func (b *Backend) deletePropertyFromJSONL(id string) error {
	path := filepath.Join(b.config.DataDir, propertiesJSONL)
	return deleteFromJSONLFile(path, id, "property_id")
}

// saveMetadataToJSONL persists metadata to JSONL file after SQLite write.
func (b *Backend) saveMetadataToJSONL(meta *types.Metadata) error {
	path := filepath.Join(b.config.DataDir, metadataJSONL)
	return updateJSONLFile(path, meta.MetadataID, "metadata_id", func() ([]byte, error) {
		return json.Marshal(metadataJSON{
			MetadataID: meta.MetadataID,
			TableName:  meta.TableName,
			CrumbID:    meta.CrumbID,
			PropertyID: meta.PropertyID,
			Content:    meta.Content,
			CreatedAt:  meta.CreatedAt.Format(time.RFC3339),
		})
	})
}

// deleteMetadataFromJSONL removes metadata from JSONL file after SQLite delete.
func (b *Backend) deleteMetadataFromJSONL(id string) error {
	path := filepath.Join(b.config.DataDir, metadataJSONL)
	return deleteFromJSONLFile(path, id, "metadata_id")
}

// saveStashToJSONL persists stash to JSONL file after SQLite write.
func (b *Backend) saveStashToJSONL(stash *types.Stash, updatedAt time.Time) error {
	path := filepath.Join(b.config.DataDir, stashesJSONL)
	return updateJSONLFile(path, stash.StashID, "stash_id", func() ([]byte, error) {
		return json.Marshal(stashJSON{
			StashID:   stash.StashID,
			TrailID:   stash.TrailID,
			Name:      stash.Name,
			StashType: stash.StashType,
			Value:     stash.Value,
			Version:   stash.Version,
			CreatedAt: stash.CreatedAt.Format(time.RFC3339),
			UpdatedAt: updatedAt.Format(time.RFC3339),
		})
	})
}

// deleteStashFromJSONL removes stash from JSONL file after SQLite delete.
func (b *Backend) deleteStashFromJSONL(id string) error {
	path := filepath.Join(b.config.DataDir, stashesJSONL)
	return deleteFromJSONLFile(path, id, "stash_id")
}

// appendStashHistoryJSONL appends a stash history entry (append-only per R6.3).
func (b *Backend) appendStashHistoryJSONL(h *stashHistoryJSON) error {
	path := filepath.Join(b.config.DataDir, stashHistoryJSONL)
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return appendToJSONLFile(path, data)
}

// saveCrumbPropertyToJSONL persists a crumb property value to JSONL file.
// Uses composite key (crumb_id + property_id) for updates.
func (b *Backend) saveCrumbPropertyToJSONL(crumbID, propertyID, valueType string, value any) error {
	path := filepath.Join(b.config.DataDir, crumbPropsJSONL)
	return updateCrumbPropertyJSONLFile(path, crumbID, propertyID, func() ([]byte, error) {
		return json.Marshal(crumbPropertyJSON{
			CrumbID:    crumbID,
			PropertyID: propertyID,
			ValueType:  valueType,
			Value:      value,
		})
	})
}

// updateCrumbPropertyJSONLFile updates or appends a crumb property in the JSONL file.
// Uses composite key (crumb_id + property_id) for matching.
func updateCrumbPropertyJSONLFile(path, crumbID, propertyID string, marshal func() ([]byte, error)) error {
	var lines [][]byte
	found := false

	f, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var record map[string]any
			if err := json.Unmarshal(line, &record); err != nil {
				lineCopy := make([]byte, len(line))
				copy(lineCopy, line)
				lines = append(lines, lineCopy)
				continue
			}
			// Check composite key
			if record["crumb_id"] == crumbID && record["property_id"] == propertyID {
				newLine, err := marshal()
				if err != nil {
					f.Close()
					return err
				}
				lines = append(lines, newLine)
				found = true
			} else {
				lineCopy := make([]byte, len(line))
				copy(lineCopy, line)
				lines = append(lines, lineCopy)
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	if !found {
		newLine, err := marshal()
		if err != nil {
			return err
		}
		lines = append(lines, newLine)
	}

	return writeJSONLAtomic(path, lines)
}

// updateJSONLFile updates or appends a record in a JSONL file (read-modify-write per R6.2).
func updateJSONLFile(path, id, idField string, marshal func() ([]byte, error)) error {
	// Read existing records
	var lines [][]byte
	found := false

	f, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			// Parse to check ID
			var record map[string]any
			if err := json.Unmarshal(line, &record); err != nil {
				// Keep malformed lines as-is
				lineCopy := make([]byte, len(line))
				copy(lineCopy, line)
				lines = append(lines, lineCopy)
				continue
			}
			if record[idField] == id {
				// Replace this record
				newLine, err := marshal()
				if err != nil {
					f.Close()
					return err
				}
				lines = append(lines, newLine)
				found = true
			} else {
				lineCopy := make([]byte, len(line))
				copy(lineCopy, line)
				lines = append(lines, lineCopy)
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	// Append if not found
	if !found {
		newLine, err := marshal()
		if err != nil {
			return err
		}
		lines = append(lines, newLine)
	}

	// Write atomically
	return writeJSONLAtomic(path, lines)
}

// deleteFromJSONLFile removes a record from a JSONL file by ID.
func deleteFromJSONLFile(path, id, idField string) error {
	var lines [][]byte

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			// Keep malformed lines
			lineCopy := make([]byte, len(line))
			copy(lineCopy, line)
			lines = append(lines, lineCopy)
			continue
		}
		if record[idField] != id {
			lineCopy := make([]byte, len(line))
			copy(lineCopy, line)
			lines = append(lines, lineCopy)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return writeJSONLAtomic(path, lines)
}

// appendToJSONLFile appends a single record to a JSONL file (for append-only tables per R6.3).
func appendToJSONLFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

// writeJSONLAtomic writes lines to a JSONL file atomically (temp file, fsync, rename per R6.4).
func writeJSONLAtomic(path string, lines [][]byte) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "*.jsonl.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	writer := bufio.NewWriter(tmpFile)
	for _, line := range lines {
		if _, err := writer.Write(line); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}

	// fsync per R6.4
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename per R6.4
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// readJSONL reads all records from a JSONL file into a slice.
// Used for reading entire files when needed.
func readJSONL[T any](path string) ([]T, error) {
	var results []T

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record T
		if err := json.Unmarshal(line, &record); err != nil {
			log.Printf("warning: %s line %d: %v", path, lineNum, err)
			continue
		}
		results = append(results, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// writeJSONL writes all records to a JSONL file atomically.
func writeJSONL[T any](path string, records []T) error {
	var lines [][]byte
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	return writeJSONLAtomic(path, lines)
}

// StreamJSONL provides streaming read capability for large JSONL files.
type StreamJSONL[T any] struct {
	file    *os.File
	scanner *bufio.Scanner
	path    string
	lineNum int
}

// NewStreamJSONL opens a JSONL file for streaming reads.
func NewStreamJSONL[T any](path string) (*StreamJSONL[T], error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &StreamJSONL[T]{
		file:    f,
		scanner: bufio.NewScanner(f),
		path:    path,
	}, nil
}

// Next returns the next record from the stream, or io.EOF when done.
func (s *StreamJSONL[T]) Next() (T, error) {
	var zero T
	for s.scanner.Scan() {
		s.lineNum++
		line := s.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record T
		if err := json.Unmarshal(line, &record); err != nil {
			log.Printf("warning: %s line %d: %v", s.path, s.lineNum, err)
			continue
		}
		return record, nil
	}
	if err := s.scanner.Err(); err != nil {
		return zero, err
	}
	return zero, io.EOF
}

// Close closes the underlying file.
func (s *StreamJSONL[T]) Close() error {
	return s.file.Close()
}
