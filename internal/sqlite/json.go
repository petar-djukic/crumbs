// JSON persistence for the SQLite backend.
// Implements: prd-sqlite-backend R1, R2, R4, R5 (directory layout, JSON format, startup, writes);
//
//	docs/ARCHITECTURE § SQLite Backend.
package sqlite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dukaforge/crumbs/pkg/types"
)

// JSON file names per R1.2.
const (
	crumbsFile       = "crumbs.json"
	trailsFile       = "trails.json"
	linksFile        = "links.json"
	propertiesFile   = "properties.json"
	categoriesFile   = "categories.json"
	crumbPropsFile   = "crumb_properties.json"
	metadataFile     = "metadata.json"
	stashesFile      = "stashes.json"
	stashHistoryFile = "stash_history.json"
)

// JSON record structures that mirror the JSON file format per R2.

// crumbJSON represents a crumb in crumbs.json.
type crumbJSON struct {
	CrumbID   string `json:"crumb_id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// trailJSON represents a trail in trails.json.
type trailJSON struct {
	TrailID       string  `json:"trail_id"`
	ParentCrumbID *string `json:"parent_crumb_id"`
	State         string  `json:"state"`
	CreatedAt     string  `json:"created_at"`
	CompletedAt   *string `json:"completed_at"`
}

// linkJSON represents a link in links.json.
type linkJSON struct {
	LinkID    string `json:"link_id"`
	LinkType  string `json:"link_type"`
	FromID    string `json:"from_id"`
	ToID      string `json:"to_id"`
	CreatedAt string `json:"created_at"`
}

// propertyJSON represents a property in properties.json.
type propertyJSON struct {
	PropertyID  string `json:"property_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ValueType   string `json:"value_type"`
	CreatedAt   string `json:"created_at"`
}

// categoryJSON represents a category in categories.json.
type categoryJSON struct {
	CategoryID string `json:"category_id"`
	PropertyID string `json:"property_id"`
	Name       string `json:"name"`
	Ordinal    int    `json:"ordinal"`
}

// crumbPropertyJSON represents a crumb property value in crumb_properties.json.
type crumbPropertyJSON struct {
	CrumbID    string `json:"crumb_id"`
	PropertyID string `json:"property_id"`
	ValueType  string `json:"value_type"`
	Value      any    `json:"value"`
}

// metadataJSON represents metadata in metadata.json.
type metadataJSON struct {
	MetadataID string  `json:"metadata_id"`
	TableName  string  `json:"table_name"`
	CrumbID    string  `json:"crumb_id"`
	PropertyID *string `json:"property_id"`
	Content    string  `json:"content"`
	CreatedAt  string  `json:"created_at"`
}

// stashJSON represents a stash in stashes.json.
type stashJSON struct {
	StashID   string  `json:"stash_id"`
	TrailID   *string `json:"trail_id"`
	Name      string  `json:"name"`
	StashType string  `json:"stash_type"`
	Value     any     `json:"value"`
	Version   int64   `json:"version"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// stashHistoryJSON represents a stash history entry in stash_history.json.
type stashHistoryJSON struct {
	HistoryID string  `json:"history_id"`
	StashID   string  `json:"stash_id"`
	Version   int64   `json:"version"`
	Value     any     `json:"value"`
	Operation string  `json:"operation"`
	ChangedBy *string `json:"changed_by"`
	CreatedAt string  `json:"created_at"`
}

// initJSONFiles creates empty JSON files if they do not exist (per R1.4).
func (b *Backend) initJSONFiles() error {
	files := []string{
		crumbsFile, trailsFile, linksFile, propertiesFile, categoriesFile,
		crumbPropsFile, metadataFile, stashesFile, stashHistoryFile,
	}
	for _, name := range files {
		path := filepath.Join(b.config.DataDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := writeJSONAtomic(path, []any{}); err != nil {
				return fmt.Errorf("init %s: %w", name, err)
			}
		}
	}
	return nil
}

// loadAllJSON loads all JSON files into SQLite (per R4.1, R4.4).
func (b *Backend) loadAllJSON() error {
	dataDir := b.config.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	// Load crumbs
	if err := b.loadCrumbsJSON(filepath.Join(dataDir, crumbsFile)); err != nil {
		return fmt.Errorf("load crumbs.json: %w", err)
	}

	// Load trails
	if err := b.loadTrailsJSON(filepath.Join(dataDir, trailsFile)); err != nil {
		return fmt.Errorf("load trails.json: %w", err)
	}

	// Load links
	if err := b.loadLinksJSON(filepath.Join(dataDir, linksFile)); err != nil {
		return fmt.Errorf("load links.json: %w", err)
	}

	// Load properties
	if err := b.loadPropertiesJSON(filepath.Join(dataDir, propertiesFile)); err != nil {
		return fmt.Errorf("load properties.json: %w", err)
	}

	// Load categories
	if err := b.loadCategoriesJSON(filepath.Join(dataDir, categoriesFile)); err != nil {
		return fmt.Errorf("load categories.json: %w", err)
	}

	// Load crumb properties
	if err := b.loadCrumbPropertiesJSON(filepath.Join(dataDir, crumbPropsFile)); err != nil {
		return fmt.Errorf("load crumb_properties.json: %w", err)
	}

	// Load metadata
	if err := b.loadMetadataJSON(filepath.Join(dataDir, metadataFile)); err != nil {
		return fmt.Errorf("load metadata.json: %w", err)
	}

	// Load stashes
	if err := b.loadStashesJSON(filepath.Join(dataDir, stashesFile)); err != nil {
		return fmt.Errorf("load stashes.json: %w", err)
	}

	// Load stash history
	if err := b.loadStashHistoryJSON(filepath.Join(dataDir, stashHistoryFile)); err != nil {
		return fmt.Errorf("load stash_history.json: %w", err)
	}

	return nil
}

// loadCrumbsJSON loads crumbs from JSON file into SQLite.
func (b *Backend) loadCrumbsJSON(path string) error {
	var crumbs []crumbJSON
	if err := readJSON(path, &crumbs); err != nil {
		return err
	}

	for _, c := range crumbs {
		_, err := b.db.Exec(
			`INSERT INTO crumbs (crumb_id, name, state, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			c.CrumbID, c.Name, c.State, c.CreatedAt, c.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert crumb %s: %w", c.CrumbID, err)
		}
	}
	return nil
}

// loadTrailsJSON loads trails from JSON file into SQLite.
func (b *Backend) loadTrailsJSON(path string) error {
	var trails []trailJSON
	if err := readJSON(path, &trails); err != nil {
		return err
	}

	for _, t := range trails {
		_, err := b.db.Exec(
			`INSERT INTO trails (trail_id, parent_crumb_id, state, created_at, completed_at)
			 VALUES (?, ?, ?, ?, ?)`,
			t.TrailID, t.ParentCrumbID, t.State, t.CreatedAt, t.CompletedAt,
		)
		if err != nil {
			return fmt.Errorf("insert trail %s: %w", t.TrailID, err)
		}
	}
	return nil
}

// loadLinksJSON loads links from JSON file into SQLite.
func (b *Backend) loadLinksJSON(path string) error {
	var links []linkJSON
	if err := readJSON(path, &links); err != nil {
		return err
	}

	for _, l := range links {
		_, err := b.db.Exec(
			`INSERT INTO links (link_id, link_type, from_id, to_id, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			l.LinkID, l.LinkType, l.FromID, l.ToID, l.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert link %s: %w", l.LinkID, err)
		}
	}
	return nil
}

// loadPropertiesJSON loads properties from JSON file into SQLite.
func (b *Backend) loadPropertiesJSON(path string) error {
	var props []propertyJSON
	if err := readJSON(path, &props); err != nil {
		return err
	}

	for _, p := range props {
		_, err := b.db.Exec(
			`INSERT INTO properties (property_id, name, description, value_type, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			p.PropertyID, p.Name, p.Description, p.ValueType, p.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert property %s: %w", p.PropertyID, err)
		}
	}
	return nil
}

// loadCategoriesJSON loads categories from JSON file into SQLite.
func (b *Backend) loadCategoriesJSON(path string) error {
	var cats []categoryJSON
	if err := readJSON(path, &cats); err != nil {
		return err
	}

	for _, c := range cats {
		_, err := b.db.Exec(
			`INSERT INTO categories (category_id, property_id, name, ordinal)
			 VALUES (?, ?, ?, ?)`,
			c.CategoryID, c.PropertyID, c.Name, c.Ordinal,
		)
		if err != nil {
			return fmt.Errorf("insert category %s: %w", c.CategoryID, err)
		}
	}
	return nil
}

// loadCrumbPropertiesJSON loads crumb property values from JSON file into SQLite.
func (b *Backend) loadCrumbPropertiesJSON(path string) error {
	var props []crumbPropertyJSON
	if err := readJSON(path, &props); err != nil {
		return err
	}

	for _, p := range props {
		valueJSON, err := json.Marshal(p.Value)
		if err != nil {
			return fmt.Errorf("marshal crumb property value: %w", err)
		}
		_, err = b.db.Exec(
			`INSERT INTO crumb_properties (crumb_id, property_id, value_type, value)
			 VALUES (?, ?, ?, ?)`,
			p.CrumbID, p.PropertyID, p.ValueType, string(valueJSON),
		)
		if err != nil {
			return fmt.Errorf("insert crumb property %s/%s: %w", p.CrumbID, p.PropertyID, err)
		}
	}
	return nil
}

// loadMetadataJSON loads metadata from JSON file into SQLite.
func (b *Backend) loadMetadataJSON(path string) error {
	var metas []metadataJSON
	if err := readJSON(path, &metas); err != nil {
		return err
	}

	for _, m := range metas {
		_, err := b.db.Exec(
			`INSERT INTO metadata (metadata_id, table_name, crumb_id, property_id, content, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			m.MetadataID, m.TableName, m.CrumbID, m.PropertyID, m.Content, m.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert metadata %s: %w", m.MetadataID, err)
		}
	}
	return nil
}

// loadStashesJSON loads stashes from JSON file into SQLite.
func (b *Backend) loadStashesJSON(path string) error {
	var stashes []stashJSON
	if err := readJSON(path, &stashes); err != nil {
		return err
	}

	for _, s := range stashes {
		valueJSON, err := json.Marshal(s.Value)
		if err != nil {
			return fmt.Errorf("marshal stash value: %w", err)
		}
		_, err = b.db.Exec(
			`INSERT INTO stashes (stash_id, trail_id, name, stash_type, value, version, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			s.StashID, s.TrailID, s.Name, s.StashType, string(valueJSON), s.Version, s.CreatedAt, s.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert stash %s: %w", s.StashID, err)
		}
	}
	return nil
}

// loadStashHistoryJSON loads stash history from JSON file into SQLite.
func (b *Backend) loadStashHistoryJSON(path string) error {
	var history []stashHistoryJSON
	if err := readJSON(path, &history); err != nil {
		return err
	}

	for _, h := range history {
		valueJSON, err := json.Marshal(h.Value)
		if err != nil {
			return fmt.Errorf("marshal stash history value: %w", err)
		}
		_, err = b.db.Exec(
			`INSERT INTO stash_history (history_id, stash_id, version, value, operation, changed_by, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			h.HistoryID, h.StashID, h.Version, string(valueJSON), h.Operation, h.ChangedBy, h.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert stash history %s: %w", h.HistoryID, err)
		}
	}
	return nil
}

// saveCrumbToJSON persists crumb to JSON file after SQLite write (per R5).
func (b *Backend) saveCrumbToJSON(crumb *types.Crumb) error {
	path := filepath.Join(b.config.DataDir, crumbsFile)

	var crumbs []crumbJSON
	if err := readJSON(path, &crumbs); err != nil {
		return err
	}

	// Convert entity to JSON struct
	cj := crumbJSON{
		CrumbID:   crumb.CrumbID,
		Name:      crumb.Name,
		State:     crumb.State,
		CreatedAt: crumb.CreatedAt.Format(time.RFC3339),
		UpdatedAt: crumb.UpdatedAt.Format(time.RFC3339),
	}

	// Find and update or append
	found := false
	for i, c := range crumbs {
		if c.CrumbID == crumb.CrumbID {
			crumbs[i] = cj
			found = true
			break
		}
	}
	if !found {
		crumbs = append(crumbs, cj)
	}

	return writeJSONAtomic(path, crumbs)
}

// deleteCrumbFromJSON removes crumb from JSON file after SQLite delete (per R5).
func (b *Backend) deleteCrumbFromJSON(id string) error {
	path := filepath.Join(b.config.DataDir, crumbsFile)

	var crumbs []crumbJSON
	if err := readJSON(path, &crumbs); err != nil {
		return err
	}

	filtered := make([]crumbJSON, 0, len(crumbs))
	for _, c := range crumbs {
		if c.CrumbID != id {
			filtered = append(filtered, c)
		}
	}

	return writeJSONAtomic(path, filtered)
}

// saveTrailToJSON persists trail to JSON file after SQLite write.
func (b *Backend) saveTrailToJSON(trail *types.Trail) error {
	path := filepath.Join(b.config.DataDir, trailsFile)

	var trails []trailJSON
	if err := readJSON(path, &trails); err != nil {
		return err
	}

	var completedAt *string
	if trail.CompletedAt != nil {
		s := trail.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	tj := trailJSON{
		TrailID:       trail.TrailID,
		ParentCrumbID: trail.ParentCrumbID,
		State:         trail.State,
		CreatedAt:     trail.CreatedAt.Format(time.RFC3339),
		CompletedAt:   completedAt,
	}

	found := false
	for i, t := range trails {
		if t.TrailID == trail.TrailID {
			trails[i] = tj
			found = true
			break
		}
	}
	if !found {
		trails = append(trails, tj)
	}

	return writeJSONAtomic(path, trails)
}

// deleteTrailFromJSON removes trail from JSON file after SQLite delete.
func (b *Backend) deleteTrailFromJSON(id string) error {
	path := filepath.Join(b.config.DataDir, trailsFile)

	var trails []trailJSON
	if err := readJSON(path, &trails); err != nil {
		return err
	}

	filtered := make([]trailJSON, 0, len(trails))
	for _, t := range trails {
		if t.TrailID != id {
			filtered = append(filtered, t)
		}
	}

	return writeJSONAtomic(path, filtered)
}

// saveLinkToJSON persists link to JSON file after SQLite write.
func (b *Backend) saveLinkToJSON(link *types.Link) error {
	path := filepath.Join(b.config.DataDir, linksFile)

	var links []linkJSON
	if err := readJSON(path, &links); err != nil {
		return err
	}

	lj := linkJSON{
		LinkID:    link.LinkID,
		LinkType:  link.LinkType,
		FromID:    link.FromID,
		ToID:      link.ToID,
		CreatedAt: link.CreatedAt.Format(time.RFC3339),
	}

	found := false
	for i, l := range links {
		if l.LinkID == link.LinkID {
			links[i] = lj
			found = true
			break
		}
	}
	if !found {
		links = append(links, lj)
	}

	return writeJSONAtomic(path, links)
}

// deleteLinkFromJSON removes link from JSON file after SQLite delete.
func (b *Backend) deleteLinkFromJSON(id string) error {
	path := filepath.Join(b.config.DataDir, linksFile)

	var links []linkJSON
	if err := readJSON(path, &links); err != nil {
		return err
	}

	filtered := make([]linkJSON, 0, len(links))
	for _, l := range links {
		if l.LinkID != id {
			filtered = append(filtered, l)
		}
	}

	return writeJSONAtomic(path, filtered)
}

// savePropertyToJSON persists property to JSON file after SQLite write.
func (b *Backend) savePropertyToJSON(prop *types.Property) error {
	path := filepath.Join(b.config.DataDir, propertiesFile)

	var props []propertyJSON
	if err := readJSON(path, &props); err != nil {
		return err
	}

	pj := propertyJSON{
		PropertyID:  prop.PropertyID,
		Name:        prop.Name,
		Description: prop.Description,
		ValueType:   prop.ValueType,
		CreatedAt:   prop.CreatedAt.Format(time.RFC3339),
	}

	found := false
	for i, p := range props {
		if p.PropertyID == prop.PropertyID {
			props[i] = pj
			found = true
			break
		}
	}
	if !found {
		props = append(props, pj)
	}

	return writeJSONAtomic(path, props)
}

// deletePropertyFromJSON removes property from JSON file after SQLite delete.
func (b *Backend) deletePropertyFromJSON(id string) error {
	path := filepath.Join(b.config.DataDir, propertiesFile)

	var props []propertyJSON
	if err := readJSON(path, &props); err != nil {
		return err
	}

	filtered := make([]propertyJSON, 0, len(props))
	for _, p := range props {
		if p.PropertyID != id {
			filtered = append(filtered, p)
		}
	}

	return writeJSONAtomic(path, filtered)
}

// saveMetadataToJSON persists metadata to JSON file after SQLite write.
func (b *Backend) saveMetadataToJSON(meta *types.Metadata) error {
	path := filepath.Join(b.config.DataDir, metadataFile)

	var metas []metadataJSON
	if err := readJSON(path, &metas); err != nil {
		return err
	}

	mj := metadataJSON{
		MetadataID: meta.MetadataID,
		TableName:  meta.TableName,
		CrumbID:    meta.CrumbID,
		PropertyID: meta.PropertyID,
		Content:    meta.Content,
		CreatedAt:  meta.CreatedAt.Format(time.RFC3339),
	}

	found := false
	for i, m := range metas {
		if m.MetadataID == meta.MetadataID {
			metas[i] = mj
			found = true
			break
		}
	}
	if !found {
		metas = append(metas, mj)
	}

	return writeJSONAtomic(path, metas)
}

// deleteMetadataFromJSON removes metadata from JSON file after SQLite delete.
func (b *Backend) deleteMetadataFromJSON(id string) error {
	path := filepath.Join(b.config.DataDir, metadataFile)

	var metas []metadataJSON
	if err := readJSON(path, &metas); err != nil {
		return err
	}

	filtered := make([]metadataJSON, 0, len(metas))
	for _, m := range metas {
		if m.MetadataID != id {
			filtered = append(filtered, m)
		}
	}

	return writeJSONAtomic(path, filtered)
}

// saveStashToJSON persists stash to JSON file after SQLite write.
func (b *Backend) saveStashToJSON(stash *types.Stash, updatedAt time.Time) error {
	path := filepath.Join(b.config.DataDir, stashesFile)

	var stashes []stashJSON
	if err := readJSON(path, &stashes); err != nil {
		return err
	}

	sj := stashJSON{
		StashID:   stash.StashID,
		TrailID:   stash.TrailID,
		Name:      stash.Name,
		StashType: stash.StashType,
		Value:     stash.Value,
		Version:   stash.Version,
		CreatedAt: stash.CreatedAt.Format(time.RFC3339),
		UpdatedAt: updatedAt.Format(time.RFC3339),
	}

	found := false
	for i, s := range stashes {
		if s.StashID == stash.StashID {
			stashes[i] = sj
			found = true
			break
		}
	}
	if !found {
		stashes = append(stashes, sj)
	}

	return writeJSONAtomic(path, stashes)
}

// deleteStashFromJSON removes stash from JSON file after SQLite delete.
func (b *Backend) deleteStashFromJSON(id string) error {
	path := filepath.Join(b.config.DataDir, stashesFile)

	var stashes []stashJSON
	if err := readJSON(path, &stashes); err != nil {
		return err
	}

	filtered := make([]stashJSON, 0, len(stashes))
	for _, s := range stashes {
		if s.StashID != id {
			filtered = append(filtered, s)
		}
	}

	return writeJSONAtomic(path, filtered)
}

// readJSON reads a JSON file into the target slice.
func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Empty by default
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

// writeJSONAtomic writes data to a JSON file atomically (temp file + rename per R5.2).
func writeJSONAtomic(path string, data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file in the same directory
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(jsonData); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
