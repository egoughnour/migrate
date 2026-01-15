// Package migrate provides a public API for database schema migration and transformation.
//
// The migrate package supports analyzing database schemas, comparing versions,
// and transforming schemas between different SQL dialects.
//
// Basic usage:
//
//	schema, err := migrate.Analyze("postgres://localhost/mydb")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(schema)
//
// For CLI usage, install the migrate command:
//
//	go install github.com/egoughnour/migrate/cmd/migrate@latest
package migrate

import (
	"github.com/egoughnour/migrate/internal/db"
	"github.com/egoughnour/migrate/internal/dialect"
	"github.com/egoughnour/migrate/internal/diff"
	"github.com/egoughnour/migrate/internal/schema"
)

// Schema represents a database schema with tables, indexes, and constraints.
type Schema = schema.Schema

// Table represents a database table.
type Table = schema.Table

// Column represents a table column.
type Column = schema.Column

// Index represents a database index.
type Index = schema.Index

// ForeignKey represents a foreign key constraint.
type ForeignKey = schema.ForeignKey

// Changes represents the differences between two schemas.
type Changes = diff.Changes

// TableChanges represents changes to a specific table.
type TableChanges = diff.TableChanges

// ColumnChanges represents changes to a specific column.
type ColumnChanges = diff.ColumnChanges

// Analyze connects to a database and extracts its schema.
//
// The source can be a connection string (e.g., "postgres://user:pass@host/db")
// or a file path to a SQL schema file.
//
// Example:
//
//	schema, err := migrate.Analyze("postgres://localhost/mydb")
//	if err != nil {
//	    return err
//	}
//	for _, table := range schema.Tables {
//	    fmt.Printf("Table: %s (%d columns)\n", table.Name, len(table.Columns))
//	}
func Analyze(source string) (*Schema, error) {
	// Check if source is a file or connection string
	if isConnectionString(source) {
		return analyzeDatabase(source)
	}
	return analyzeFile(source, "postgres") // Default dialect
}

// AnalyzeFile parses a SQL file and returns its schema.
//
// The dialect parameter specifies the SQL dialect: "postgres", "mysql", or "sqlserver".
func AnalyzeFile(path string, dialect string) (*Schema, error) {
	return analyzeFile(path, dialect)
}

// AnalyzeDatabase connects to a database and extracts its schema.
func AnalyzeDatabase(connStr string) (*Schema, error) {
	return analyzeDatabase(connStr)
}

// Diff compares two schemas and returns the differences.
//
// The returned Changes struct contains added, removed, and modified
// tables, columns, indexes, and constraints.
func Diff(source, target *Schema) *Changes {
	differ := diff.NewDiffer(source, target)
	return differ.Compare()
}

// Transform converts a schema from one SQL dialect to another.
//
// Supported dialects: "postgres", "mysql", "sqlserver".
//
// Returns the transformed schema and any warnings about lossy conversions.
//
// Example:
//
//	transformed, warnings := migrate.Transform(schema, "postgres", "mysql")
//	for _, w := range warnings {
//	    log.Printf("Warning: %s", w)
//	}
func Transform(s *Schema, fromDialect, toDialect string) (*Schema, []string) {
	transformer := dialect.NewTransformer(fromDialect, toDialect)
	return transformer.Transform(s)
}

// ParseSQL parses SQL content into a Schema.
//
// The dialect parameter specifies the SQL dialect: "postgres", "mysql", or "sqlserver".
func ParseSQL(sql string, dialect string) (*Schema, error) {
	return schema.Parse(sql, dialect)
}

// GenerateSQL generates SQL DDL statements from a Schema.
//
// The dialect parameter specifies the target SQL dialect.
func GenerateSQL(s *Schema, dialect string) string {
	gen := schema.NewGenerator(dialect)
	return gen.Generate(s)
}

// SupportedDialects returns the list of supported SQL dialects.
func SupportedDialects() []string {
	return dialect.SupportedDialects()
}

// Helper functions

func isConnectionString(s string) bool {
	// Check for common database URL schemes
	prefixes := []string{
		"postgres://",
		"postgresql://",
		"mysql://",
		"sqlserver://",
		"mssql://",
	}
	for _, prefix := range prefixes {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func analyzeFile(path string, sqlDialect string) (*Schema, error) {
	return schema.ParseFile(path, sqlDialect)
}

func analyzeDatabase(connStr string) (*Schema, error) {
	introspector, err := db.NewIntrospector(connStr)
	if err != nil {
		return nil, err
	}
	defer introspector.Close()

	return introspector.Introspect()
}
