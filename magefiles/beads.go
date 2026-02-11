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
	if beadsInitialized() {
		fmt.Println("Beads already initialized.")
		return nil
	}
	branch, err := gitCurrentBranch()
	if err != nil {
		branch = "main"
	}
	return beadsInit(branch)
}

// Reset destroys and reinitializes the beads database.
// Uses the current branch as the new prefix.
func (Beads) Reset() error {
	branch, err := gitCurrentBranch()
	if err != nil {
		branch = "main"
	}
	if err := beadsReset(); err != nil {
		return err
	}
	return beadsInit(branch)
}

// beadsInitialized returns true if the .beads/ directory exists.
func beadsInitialized() bool {
	_, err := os.Stat(beadsDir)
	return err == nil
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

// beadsInit initializes the beads database with the given prefix.
func beadsInit(prefix string) error {
	fmt.Printf("Initializing beads with prefix %s...\n", prefix)
	if err := bdInit(prefix); err != nil {
		return fmt.Errorf("bd init: %w", err)
	}
	fmt.Println("Beads initialized.")
	return nil
}

// beadsReset syncs state, stops the daemon, destroys the database,
// and commits empty JSONL files so bd init does not reimport from
// git history. Returns nil if no database exists.
func beadsReset() error {
	if !beadsInitialized() {
		return nil
	}
	fmt.Println("Resetting beads database...")
	_ = bdSync()
	if err := bdAdminReset(); err != nil {
		return err
	}
	// bd init scans git history for issues.jsonl. Commit empty JSONL
	// files so the next init starts with a clean slate.
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		return fmt.Errorf("recreating %s: %w", beadsDir, err)
	}
	for _, name := range []string{"issues.jsonl", "interactions.jsonl"} {
		_ = os.WriteFile(beadsDir+name, nil, 0o644)
	}
	_ = gitStageDir(beadsDir)
	_ = gitCommit("Reset beads: clear issue history")
	// Remove the directory again so bd init creates it fresh.
	_ = os.RemoveAll(beadsDir)
	return nil
}
