package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/egoughnour/migrate/internal/db"
	"github.com/egoughnour/migrate/internal/diff"
	"github.com/egoughnour/migrate/internal/schema"
)

var (
	targetURI     string
	targetDialect string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare two database schemas",
	Long: `Compare two schemas and show the differences.

Both source and target can be:
  - A database connection string
  - A path to a SQL schema file

The diff shows:
  - Added tables, columns, indexes, constraints
  - Removed tables, columns, indexes, constraints
  - Modified columns (type changes, nullability, defaults)`,
	Example: `  # Compare two SQL files
  migrate diff --source schema_v1.sql --target schema_v2.sql --dialect postgres

  # Compare two databases
  migrate diff --source postgres://localhost/db_old --target postgres://localhost/db_new

  # Generate migration SQL
  migrate diff --source schema_v1.sql --target schema_v2.sql --dialect postgres -o sql`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVar(&sourceURI, "source", "", "Source schema (connection string or file path)")
	diffCmd.Flags().StringVar(&targetURI, "target", "", "Target schema (connection string or file path)")
	diffCmd.Flags().StringVar(&sourceDialect, "dialect", "", "SQL dialect for file parsing: postgres, mysql, sqlserver")
	diffCmd.MarkFlagRequired("source")
	diffCmd.MarkFlagRequired("target")
}

func runDiff(cmd *cobra.Command, args []string) error {
	source, err := loadSchema(sourceURI, sourceDialect)
	if err != nil {
		return fmt.Errorf("failed to load source schema: %w", err)
	}

	target, err := loadSchema(targetURI, sourceDialect)
	if err != nil {
		return fmt.Errorf("failed to load target schema: %w", err)
	}

	differ := diff.NewDiffer(source, target)
	changes := differ.Compare()

	// Output in requested format
	switch outputFormat {
	case "json":
		return diff.WriteJSON(os.Stdout, changes)
	case "yaml":
		return diff.WriteYAML(os.Stdout, changes)
	case "sql":
		dialect := sourceDialect
		if dialect == "" {
			dialect = detectDialect(sourceURI)
		}
		sqlGen := diff.NewSQLGenerator(dialect)
		return sqlGen.WriteSQL(os.Stdout, changes)
	default:
		return diff.WriteText(os.Stdout, changes)
	}
}

func loadSchema(uri, dialect string) (*schema.Schema, error) {
	if isFile(uri) {
		if dialect == "" {
			return nil, fmt.Errorf("--dialect is required when using SQL files")
		}
		return schema.ParseFile(uri, dialect)
	}

	introspector, err := db.NewIntrospector(uri)
	if err != nil {
		return nil, err
	}
	defer introspector.Close()

	return introspector.Introspect()
}
