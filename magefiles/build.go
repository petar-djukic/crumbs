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

	if rt := containerRuntime(); rt != "" {
		return buildImage(rt)
	}
	fmt.Println("No container runtime found; skipping image build.")
	return nil
}

// Clean removes build artifacts and the container image.
func Clean() error {
	if err := os.RemoveAll(binaryDir); err != nil {
		return err
	}
	if err := sh.RunV(binGo, "clean"); err != nil {
		return err
	}

	if rt := containerRuntime(); rt != "" {
		removeImage(rt)
	}
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
