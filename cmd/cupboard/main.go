// Entry point for the cupboard CLI.
// Implements: prd009-cupboard-cli R1.1, R1.5.
package main

import "os"

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitUserError)
	}
}
