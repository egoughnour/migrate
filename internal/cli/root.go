// Package cli implements the command-line interface for migrate.
package cli

import (
	"github.com/spf13/cobra"
)

var (
	// Global flags
	outputFormat string
	verbose      bool
)

var rootCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Schema migration and transformation toolkit",
	Long: `migrate is a CLI tool for analyzing, comparing, and transforming
database schemas across different database engines.

Supported databases:
  - PostgreSQL
  - MySQL
  - SQL Server

Examples:
  # Analyze a database schema
  migrate analyze --source postgres://localhost/mydb

  # Compare two schemas
  migrate diff --source schema_v1.sql --target schema_v2.sql

  # Transform schema between dialects
  migrate transform --input schema.sql --from postgres --to mysql

  # Generate migration SQL
  migrate generate --from schema_v1.sql --to schema_v2.sql`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml, sql")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(transformCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
