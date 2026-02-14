// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binGo      = "go"
	binaryName = "cupboard"
	binaryDir  = "bin"
	cmdDir     = "./cmd/cupboard"
)

// Build compiles the cupboard binary to bin/.
func Build() error {
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		return err
	}
	return sh.RunV(binGo, "build", "-v", "-o", filepath.Join(binaryDir, binaryName), cmdDir)
}

// Clean removes build artifacts.
func Clean() error {
	if err := os.RemoveAll(binaryDir); err != nil {
		return err
	}
	return sh.RunV(binGo, "clean")
}

// Init initializes project state (beads, seeds).
func Init() error {
	return newOrch().Init()
}

// Reset runs a full reset (cobbler, generator, beads).
func Reset() error {
	return newOrch().FullReset()
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
