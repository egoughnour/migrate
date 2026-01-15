// Package main provides the entry point for the migrate CLI tool.
package main

import (
	"os"

	"github.com/egoughnour/migrate/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
