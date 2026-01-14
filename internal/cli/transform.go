package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/egoughnour/migrate/internal/dialect"
	"github.com/egoughnour/migrate/internal/schema"
)

var (
	inputFile   string
	fromDialect string
	toDialect   string
)

var transformCmd = &cobra.Command{
	Use:   "transform",
	Short: "Transform schema between SQL dialects",
	Long: `Convert a database schema from one SQL dialect to another.

Supported transformations:
  - PostgreSQL ↔ MySQL
  - PostgreSQL ↔ SQL Server
  - MySQL ↔ SQL Server

The transformer handles:
  - Data type mappings (e.g., SERIAL → AUTO_INCREMENT)
  - Syntax differences (e.g., LIMIT vs TOP)
  - Feature mappings where possible`,
	Example: `  # Convert PostgreSQL to MySQL
  migrate transform --input schema.sql --from postgres --to mysql

  # Convert MySQL to PostgreSQL
  migrate transform --input schema.sql --from mysql --to postgres`,
	RunE: runTransform,
}

func init() {
	transformCmd.Flags().StringVar(&inputFile, "input", "", "Input SQL file path (required)")
	transformCmd.Flags().StringVar(&fromDialect, "from", "", "Source dialect: postgres, mysql, sqlserver (required)")
	transformCmd.Flags().StringVar(&toDialect, "to", "", "Target dialect: postgres, mysql, sqlserver (required)")
	transformCmd.MarkFlagRequired("input")
	transformCmd.MarkFlagRequired("from")
	transformCmd.MarkFlagRequired("to")
}

func runTransform(cmd *cobra.Command, args []string) error {
	// Validate dialects
	if !isValidDialect(fromDialect) {
		return fmt.Errorf("invalid source dialect: %s (use: postgres, mysql, sqlserver)", fromDialect)
	}
	if !isValidDialect(toDialect) {
		return fmt.Errorf("invalid target dialect: %s (use: postgres, mysql, sqlserver)", toDialect)
	}

	// Parse the input schema
	s, err := schema.ParseFile(inputFile, fromDialect)
	if err != nil {
		return fmt.Errorf("failed to parse input file: %w", err)
	}

	// Transform to target dialect
	transformer := dialect.NewTransformer(fromDialect, toDialect)
	transformed, warnings := transformer.Transform(s)

	// Print warnings if any
	if verbose && len(warnings) > 0 {
		fmt.Fprintln(os.Stderr, "Transformation warnings:")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Output the transformed schema
	return schema.WriteSQL(os.Stdout, transformed, toDialect)
}

func isValidDialect(d string) bool {
	switch d {
	case "postgres", "mysql", "sqlserver":
		return true
	default:
		return false
	}
}
