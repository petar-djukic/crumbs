// Package main provides build targets for the crumbs project using Mage.
//
// Usage:
//
//	mage build          Compile cupboard binary to bin/
//	mage test           Run all tests (unit + integration)
//	mage testUnit       Run only unit tests (exclude integration)
//	mage testIntegration Run only integration tests (builds first)
//	mage lint           Run golangci-lint
//	mage clean          Remove build artifacts
//	mage install        Install cupboard to GOPATH/bin
//	mage stats          Print Go LOC and documentation word counts
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binaryName = "cupboard"
	binaryDir  = "bin"
	cmdDir     = "./cmd/cupboard"
)

// Build compiles the cupboard binary to bin/.
func Build() error {
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		return err
	}
	return sh.RunV("go", "build", "-v", "-o", filepath.Join(binaryDir, binaryName), cmdDir)
}

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
	for _, pkg := range strings.Split(pkgs, "\n") {
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

// Lint runs golangci-lint.
func Lint() error {
	return sh.RunV("golangci-lint", "run", "./...")
}

// Clean removes build artifacts.
func Clean() error {
	if err := os.RemoveAll(binaryDir); err != nil {
		return err
	}
	return sh.RunV("go", "clean")
}

// Install builds and copies the binary to GOPATH/bin.
func Install() error {
	mg.Deps(Build)
	gopath, err := sh.Output("go", "env", "GOPATH")
	if err != nil {
		return err
	}
	src := filepath.Join(binaryDir, binaryName)
	dst := filepath.Join(gopath, "bin", binaryName)
	return sh.Copy(dst, src)
}

// Stats prints Go lines of code and documentation word counts.
func Stats() error {
	var prodLines, testLines int

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path == "vendor" || path == ".git" || path == binaryDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip magefiles — they are build tooling, not project code.
		if strings.HasPrefix(path, "magefiles") {
			return nil
		}
		count, countErr := countLines(path)
		if countErr != nil {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			testLines += count
		} else {
			prodLines += count
		}
		return nil
	})
	if err != nil {
		return err
	}

	docWords, err := countDocWords()
	if err != nil {
		return err
	}

	fmt.Printf("Lines of code (Go, production): %d\n", prodLines)
	fmt.Printf("Lines of code (Go, tests):      %d\n", testLines)
	fmt.Printf("Lines of code (Go, total):      %d\n", prodLines+testLines)
	fmt.Printf("Words (documentation):          %d\n", docWords)
	return nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func countDocWords() (int, error) {
	total := 0

	// Match the same files as stats.sh: README.md, docs/*.md, docs/**/*.md
	patterns := []string{"README.md", "docs/*.md", "docs/**/*.md"}
	seen := map[string]bool{}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if seen[path] {
				continue
			}
			seen[path] = true
			words, err := countWordsInFile(path)
			if err != nil {
				continue
			}
			total += words
		}
	}
	return total, nil
}

func countWordsInFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	count := 0
	inWord := false
	for _, r := range string(data) {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count, nil
}
