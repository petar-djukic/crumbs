package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/magefile/mage/mg"
)

// Cobbler groups the measure and stitch targets.
type Cobbler mg.Namespace

// Generator groups the code-generation trail lifecycle targets.
type Generator mg.Namespace

// targetArgs holds command-line arguments that follow the mage target name.
// Mage only supports positional parameters, not named flags. The init()
// function below intercepts os.Args before mage's parser runs, extracting
// everything after the target name into targetArgs so that individual
// targets can parse named flags with flag.NewFlagSet.
//
// Example: "mage measure --limit 5 --silence" sets targetArgs to
// ["--limit", "5", "--silence"] and leaves os.Args as ["mage", "measure"].
var targetArgs []string

func init() {
	// Mage's os.Args layout: [binary] [mage-flags...] [target] [target-args...]
	// Mage flags start with "-" (e.g., -v, -d, -h). The target is the first
	// non-dash argument after the binary name.
	//
	// We find the target, take everything after it as targetArgs, then trim
	// os.Args so mage only sees [binary, mage-flags, target].

	if len(os.Args) < 2 {
		return
	}

	targetIdx := -1
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--" {
			break
		}
		if len(os.Args[i]) > 0 && os.Args[i][0] != '-' {
			targetIdx = i
			break
		}
	}

	if targetIdx < 0 || targetIdx+1 >= len(os.Args) {
		return
	}

	targetArgs = os.Args[targetIdx+1:]
	os.Args = os.Args[:targetIdx+1]
}

// parseTargetFlags parses targetArgs into fs. On --help it prints usage and
// exits cleanly. On other parse errors it prints the error and exits with 1.
func parseTargetFlags(fs *flag.FlagSet) {
	err := fs.Parse(targetArgs)
	if err == nil {
		return
	}
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
