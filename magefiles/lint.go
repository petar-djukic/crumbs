// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import "github.com/magefile/mage/sh"

const binLint = "golangci-lint"

// Lint runs golangci-lint.
func Lint() error {
	return sh.RunV(binLint, "run", "./...")
}
