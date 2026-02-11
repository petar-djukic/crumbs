package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/mg"
)

// Beads groups issue-tracker lifecycle targets.
type Beads mg.Namespace

// Init initializes the beads database using the current branch as prefix.
// Safe to call when beads is already initialized (no-op).
func (Beads) Init() error {
	logf("beads:init: checking if already initialized")
	if beadsInitialized() {
		logf("beads:init: already initialized, skipping")
		return nil
	}
	branch, err := gitCurrentBranch()
	if err != nil {
		logf("beads:init: gitCurrentBranch failed (%v), defaulting to main", err)
		branch = "main"
	}
	logf("beads:init: initializing with prefix=%s", branch)
	return beadsInit(branch)
}

// Reset destroys and reinitializes the beads database.
// Uses the current branch as the new prefix.
func (Beads) Reset() error {
	logf("beads:reset: starting")
	branch, err := gitCurrentBranch()
	if err != nil {
		logf("beads:reset: gitCurrentBranch failed (%v), defaulting to main", err)
		branch = "main"
	}
	logf("beads:reset: branch=%s", branch)

	if err := beadsReset(); err != nil {
		logf("beads:reset: beadsReset failed: %v", err)
		return err
	}
	logf("beads:reset: reinitializing with prefix=%s", branch)
	return beadsInit(branch)
}

// beadsInitialized returns true if the .beads/ directory exists.
func beadsInitialized() bool {
	_, err := os.Stat(beadsDir)
	exists := err == nil
	logf("beadsInitialized: %s exists=%v", beadsDir, exists)
	return exists
}

// requireBeads checks that beads is initialized and returns an error
// with fix instructions if not. Cobbler and stitch call this before
// doing any beads work.
func requireBeads() error {
	if beadsInitialized() {
		return nil
	}
	return fmt.Errorf("beads database not found\n\n  Run 'mage beads:init' to create one, or\n  Run 'mage generator:start' to begin a new generation (which initializes beads)")
}

// beadsInit initializes the beads database with the given prefix
// and commits the resulting .beads/ directory.
func beadsInit(prefix string) error {
	logf("beadsInit: prefix=%s", prefix)
	if err := bdInit(prefix); err != nil {
		logf("beadsInit: bdInit failed: %v", err)
		return fmt.Errorf("bd init: %w", err)
	}
	logf("beadsInit: bdInit succeeded, committing")
	beadsCommit("Initialize beads database")
	logf("beadsInit: done")
	return nil
}

// beadsReset syncs state, stops the daemon, destroys the database,
// and commits empty JSONL files so bd init does not reimport from
// git history. Returns nil if no database exists.
func beadsReset() error {
	if !beadsInitialized() {
		logf("beadsReset: no database found, nothing to reset")
		return nil
	}
	logf("beadsReset: syncing beads state")
	if err := bdSync(); err != nil {
		logf("beadsReset: bdSync warning: %v", err)
	}

	logf("beadsReset: running bd admin reset")
	if err := bdAdminReset(); err != nil {
		logf("beadsReset: bdAdminReset failed: %v", err)
		return err
	}
	logf("beadsReset: bd admin reset succeeded")

	// bd init scans git history for issues.jsonl. Commit empty JSONL
	// files so the next init starts with a clean slate.
	logf("beadsReset: creating empty JSONL files in %s", beadsDir)
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		logf("beadsReset: MkdirAll failed: %v", err)
		return fmt.Errorf("recreating %s: %w", beadsDir, err)
	}
	for _, name := range []string{"issues.jsonl", "interactions.jsonl"} {
		if err := os.WriteFile(beadsDir+name, nil, 0o644); err != nil {
			logf("beadsReset: WriteFile %s warning: %v", name, err)
		}
	}

	logf("beadsReset: staging and committing empty JSONL files")
	if err := gitStageDir(beadsDir); err != nil {
		logf("beadsReset: gitStageDir warning: %v", err)
	}
	if err := gitCommit("Reset beads: clear issue history"); err != nil {
		logf("beadsReset: gitCommit warning: %v", err)
	}

	// Remove the directory again so bd init creates it fresh.
	logf("beadsReset: removing %s so bd init creates it fresh", beadsDir)
	if err := os.RemoveAll(beadsDir); err != nil {
		logf("beadsReset: RemoveAll warning: %v", err)
	}

	logf("beadsReset: done")
	return nil
}
