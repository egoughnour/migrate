package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/egoughnour/migrate/internal/db"
	"github.com/egoughnour/migrate/internal/schema"
)

var (
	sourceURI     string
	sourceDialect string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze a database schema",
	Long: `Connect to a database or read a SQL file and extract its schema structure.

The source can be:
  - A database connection string (postgres://, mysql://, sqlserver://)
  - A path to a SQL schema file (.sql)

Output formats:
  - text: Human-readable table format (default)
  - json: JSON schema representation
  - yaml: YAML schema representation
  - sql:  SQL CREATE statements`,
	Example: `  # Analyze a PostgreSQL database
  migrate analyze --source postgres://user:pass@localhost/mydb

  # Analyze a SQL file
  migrate analyze --source ./schema.sql --dialect postgres

  # Output as JSON
  migrate analyze --source postgres://localhost/mydb -o json`,
	RunE: runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&sourceURI, "source", "", "Database connection string or SQL file path (required)")
	analyzeCmd.Flags().StringVar(&sourceDialect, "dialect", "", "SQL dialect for file parsing: postgres, mysql, sqlserver")
	analyzeCmd.MarkFlagRequired("source")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	var s *schema.Schema
	var err error

	// Determine if source is a file or connection string
	if isFile(sourceURI) {
		if sourceDialect == "" {
			return fmt.Errorf("--dialect is required when analyzing a SQL file")
		}
		s, err = schema.ParseFile(sourceURI, sourceDialect)
		if err != nil {
			return fmt.Errorf("failed to parse SQL file: %w", err)
		}
	} else {
		// Detect dialect from connection string
		detectedDialect := detectDialect(sourceURI)
		if verbose {
			fmt.Fprintf(os.Stderr, "Connecting to %s database...\n", detectedDialect)
		}

		s, err = db.Introspect(sourceURI)
		if err != nil {
			return fmt.Errorf("failed to introspect database: %w", err)
		}
	}

	// Output in requested format
	switch outputFormat {
	case "json":
		return schema.WriteJSON(os.Stdout, s)
	case "yaml":
		return schema.WriteYAML(os.Stdout, s)
	case "sql":
		dialect := sourceDialect
		if dialect == "" {
			dialect = detectDialect(sourceURI)
		}
		return schema.WriteSQL(os.Stdout, s, dialect)
	default:
		return schema.WriteText(os.Stdout, s)
	}
}

func isFile(path string) bool {
	// Check if it looks like a connection string
	if strings.HasPrefix(path, "postgres://") ||
		strings.HasPrefix(path, "postgresql://") ||
		strings.HasPrefix(path, "mysql://") ||
		strings.HasPrefix(path, "sqlserver://") {
		return false
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func detectDialect(connStr string) string {
	switch {
	case strings.HasPrefix(connStr, "postgres://"), strings.HasPrefix(connStr, "postgresql://"):
		return "postgres"
	case strings.HasPrefix(connStr, "mysql://"):
		return "mysql"
	case strings.HasPrefix(connStr, "sqlserver://"):
		return "sqlserver"
	default:
		return "unknown"
	}
}
