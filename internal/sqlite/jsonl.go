// Package sqlite implements the SQLite backend for the Crumbs storage system.
// This file provides JSONL read/write helpers with atomic persistence.
// Implements: prd002-sqlite-backend R2 (JSONL format), R5.2 (atomic write).
package sqlite

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// readJSONL reads a JSONL file and returns each non-empty, parseable line as
// a json.RawMessage. Malformed lines are skipped per prd002-sqlite-backend
// R4.2 and R7.1.
func readJSONL(path string) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	var records []json.RawMessage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			// Skip malformed lines per R4.2.
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		records = append(records, json.RawMessage(cp))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning %s: %w", path, err)
	}
	return records, nil
}

// writeJSONL atomically writes records to a JSONL file using the temp-file,
// fsync, rename pattern (prd002-sqlite-backend R5.2).
func writeJSONL(path string, records []json.RawMessage) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".jsonl-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	w := bufio.NewWriter(tmp)
	for _, rec := range records {
		if _, err := w.Write(rec); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			return fmt.Errorf("writing record: %w", err)
		}
		if err := w.WriteByte('\n'); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			return fmt.Errorf("writing newline: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("flushing buffer: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// loadCrumbsJSONL reads crumbs.jsonl and returns the raw JSON records.
func loadCrumbsJSONL(dataDir string) ([]json.RawMessage, error) {
	return readJSONL(filepath.Join(dataDir, "crumbs.jsonl"))
}

// persistCrumbsJSONL writes all crumb records to crumbs.jsonl atomically.
func persistCrumbsJSONL(dataDir string, records []json.RawMessage) error {
	return writeJSONL(filepath.Join(dataDir, "crumbs.jsonl"), records)
}
