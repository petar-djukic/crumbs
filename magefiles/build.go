package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binaryName = "cupboard"
	binaryDir  = "bin"
	cmdDir     = "./cmd/cupboard"
)

// Build compiles the cupboard binary to bin/ and builds the container image.
func Build() error {
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		return err
	}
	if err := sh.RunV(binGo, "build", "-v", "-o", filepath.Join(binaryDir, binaryName), cmdDir); err != nil {
		return err
	}

	rt := containerRuntime()
	if rt == "" {
		fmt.Fprintln(os.Stderr, "WARNING: no container runtime found (tried podman, docker); skipping image build")
		return nil
	}
	return buildImage(rt)
}

// Clean removes build artifacts and the container image.
func Clean() error {
	if err := os.RemoveAll(binaryDir); err != nil {
		return err
	}
	if err := sh.RunV(binGo, "clean"); err != nil {
		return err
	}

	rt := containerRuntime()
	if rt == "" {
		fmt.Fprintln(os.Stderr, "WARNING: no container runtime found (tried podman, docker); skipping image removal")
	} else {
		removeImage(rt)
	}
	return nil
}

// Init initializes project state. Currently calls beads:init.
func Init() error {
	return (Beads{}).Init()
}

// Reset runs cobbler:reset, generator:reset, and beads:reset in order.
// Each tool only cleans its own artifacts; this target orchestrates them.
// Intermediate commits from sub-resets are squashed into a single commit
// so main stays clean when recovering from failed generations.
func Reset() error {
	logf("reset: full reset starting (cobbler, generator, beads)")

	startSHA, err := gitRevParseHEAD()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	if err := (Cobbler{}).Reset(); err != nil {
		return fmt.Errorf("cobbler reset: %w", err)
	}
	if err := (Generator{}).Reset(); err != nil {
		return fmt.Errorf("generator reset: %w", err)
	}
	if err := (Beads{}).Reset(); err != nil {
		return fmt.Errorf("beads reset: %w", err)
	}

	// Squash all intermediate commits into one.
	logf("reset: squashing intermediate commits")
	if err := gitResetSoft(startSHA); err != nil {
		return fmt.Errorf("squashing reset commits: %w", err)
	}
	_ = gitStageAll()
	_ = gitCommit("Reset to clean state")

	logf("reset: full reset complete")
	return nil
}

// Install builds and copies the binary to GOPATH/bin.
func Install() error {
	mg.Deps(Build)
	gopath, err := sh.Output(binGo, "env", "GOPATH")
	if err != nil {
		return err
	}
	src := filepath.Join(binaryDir, binaryName)
	dst := filepath.Join(gopath, "bin", binaryName)
	return sh.Copy(dst, src)
}
