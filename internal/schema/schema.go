// Package schema provides types and utilities for representing database schemas.
package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema represents a complete database schema.
type Schema struct {
	Tables  []Table  `json:"tables" yaml:"tables"`
	Indexes []Index  `json:"indexes,omitempty" yaml:"indexes,omitempty"`
	Views   []View   `json:"views,omitempty" yaml:"views,omitempty"`
}

// Table represents a database table.
type Table struct {
	Name        string       `json:"name" yaml:"name"`
	Schema      string       `json:"schema,omitempty" yaml:"schema,omitempty"`
	Columns     []Column     `json:"columns" yaml:"columns"`
	PrimaryKey  *PrimaryKey  `json:"primary_key,omitempty" yaml:"primary_key,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty" yaml:"indexes,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty" yaml:"constraints,omitempty"`
}

// Column represents a table column.
type Column struct {
	Name         string  `json:"name" yaml:"name"`
	Type         string  `json:"type" yaml:"type"`
	Nullable     bool    `json:"nullable" yaml:"nullable"`
	Default      *string `json:"default,omitempty" yaml:"default,omitempty"`
	IsPrimaryKey bool    `json:"is_primary_key,omitempty" yaml:"is_primary_key,omitempty"`
	IsUnique     bool    `json:"is_unique,omitempty" yaml:"is_unique,omitempty"`
	IsIdentity   bool    `json:"is_identity,omitempty" yaml:"is_identity,omitempty"`
	Comment      string  `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// PrimaryKey represents a primary key constraint.
type PrimaryKey struct {
	Name    string   `json:"name,omitempty" yaml:"name,omitempty"`
	Columns []string `json:"columns" yaml:"columns"`
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Name             string   `json:"name,omitempty" yaml:"name,omitempty"`
	Columns          []string `json:"columns" yaml:"columns"`
	ReferencedTable  string   `json:"referenced_table" yaml:"referenced_table"`
	ReferencedSchema string   `json:"referenced_schema,omitempty" yaml:"referenced_schema,omitempty"`
	ReferencedCols   []string `json:"referenced_columns" yaml:"referenced_columns"`
	OnDelete         string   `json:"on_delete,omitempty" yaml:"on_delete,omitempty"`
	OnUpdate         string   `json:"on_update,omitempty" yaml:"on_update,omitempty"`
}

// Index represents a database index.
type Index struct {
	Name      string   `json:"name" yaml:"name"`
	Table     string   `json:"table" yaml:"table"`
	Schema    string   `json:"schema,omitempty" yaml:"schema,omitempty"`
	Columns   []string `json:"columns" yaml:"columns"`
	IsUnique  bool     `json:"is_unique,omitempty" yaml:"is_unique,omitempty"`
	IsPrimary bool     `json:"is_primary,omitempty" yaml:"is_primary,omitempty"`
	Type      string   `json:"type,omitempty" yaml:"type,omitempty"` // btree, hash, gin, etc.
}

// Constraint represents a table constraint.
type Constraint struct {
	Name       string   `json:"name" yaml:"name"`
	Type       string   `json:"type" yaml:"type"` // CHECK, UNIQUE, etc.
	Columns    []string `json:"columns,omitempty" yaml:"columns,omitempty"`
	Expression string   `json:"expression,omitempty" yaml:"expression,omitempty"`
}

// View represents a database view.
type View struct {
	Name       string `json:"name" yaml:"name"`
	Schema     string `json:"schema,omitempty" yaml:"schema,omitempty"`
	Definition string `json:"definition" yaml:"definition"`
}

// ParseFile reads and parses a SQL schema file.
func ParseFile(path string, dialect string) (*Schema, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return Parse(string(content), dialect)
}

// Parse parses SQL content into a Schema.
func Parse(sql string, dialect string) (*Schema, error) {
	parser := NewParser(dialect)
	return parser.Parse(sql)
}

// WriteSQL writes the schema as SQL to the given writer.
func WriteSQL(w io.Writer, s *Schema, dialect string) error {
	gen := NewGenerator(dialect)
	sql := gen.Generate(s)
	_, err := w.Write([]byte(sql))
	return err
}

// WriteJSON writes the schema as JSON to the given writer.
func WriteJSON(w io.Writer, s *Schema) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// WriteYAML writes the schema as YAML to the given writer.
func WriteYAML(w io.Writer, s *Schema) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(s)
}

// WriteText writes a human-readable text representation of the schema.
func WriteText(w io.Writer, s *Schema) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Schema: %d tables, %d indexes, %d views\n\n",
		len(s.Tables), len(s.Indexes), len(s.Views)))

	for _, t := range s.Tables {
		sb.WriteString(fmt.Sprintf("Table: %s\n", t.Name))
		sb.WriteString(strings.Repeat("-", 40) + "\n")

		for _, c := range t.Columns {
			nullable := "NOT NULL"
			if c.Nullable {
				nullable = "NULL"
			}
			pk := ""
			if c.IsPrimaryKey {
				pk = " [PK]"
			}
			sb.WriteString(fmt.Sprintf("  %-20s %-15s %s%s\n", c.Name, c.Type, nullable, pk))
		}

		if len(t.ForeignKeys) > 0 {
			sb.WriteString("\n  Foreign Keys:\n")
			for _, fk := range t.ForeignKeys {
				sb.WriteString(fmt.Sprintf("    %s -> %s(%s)\n",
					strings.Join(fk.Columns, ", "),
					fk.ReferencedTable,
					strings.Join(fk.ReferencedCols, ", ")))
			}
		}

		if len(t.Indexes) > 0 {
			sb.WriteString("\n  Indexes:\n")
			for _, idx := range t.Indexes {
				unique := ""
				if idx.IsUnique {
					unique = " UNIQUE"
				}
				sb.WriteString(fmt.Sprintf("    %s%s (%s)\n",
					idx.Name, unique, strings.Join(idx.Columns, ", ")))
			}
		}

		sb.WriteString("\n")
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}
