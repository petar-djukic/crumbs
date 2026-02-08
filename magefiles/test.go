package main

import (
	"fmt"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Test runs all tests (unit and integration).
func Test() error {
	return sh.RunV("go", "test", "./...")
}

// TestUnit runs only unit tests, excluding the tests/ directory.
func TestUnit() error {
	pkgs, err := sh.Output("go", "list", "./...")
	if err != nil {
		return err
	}
	var unitPkgs []string
	for pkg := range strings.SplitSeq(pkgs, "\n") {
		if pkg != "" && !strings.Contains(pkg, "/tests/") && !strings.HasSuffix(pkg, "/tests") {
			unitPkgs = append(unitPkgs, pkg)
		}
	}
	if len(unitPkgs) == 0 {
		fmt.Println("No unit test packages found.")
		return nil
	}
	args := append([]string{"test"}, unitPkgs...)
	return sh.RunV("go", args...)
}

// TestIntegration builds first, then runs only integration tests.
func TestIntegration() error {
	mg.Deps(Build)
	return sh.RunV("go", "test", "./tests/...")
}
