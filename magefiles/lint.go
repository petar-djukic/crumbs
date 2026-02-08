package main

import "github.com/magefile/mage/sh"

// Lint runs golangci-lint.
func Lint() error {
	return sh.RunV("golangci-lint", "run", "./...")
}
