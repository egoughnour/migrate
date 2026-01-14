package schema

import (
	"fmt"
	"strings"
)

// Generator generates SQL from a Schema.
type Generator struct {
	dialect string
}

// NewGenerator creates a new SQL generator for the given dialect.
func NewGenerator(dialect string) *Generator {
	return &Generator{dialect: dialect}
}

// Generate produces SQL DDL statements from a Schema.
func (g *Generator) Generate(s *Schema) string {
	var sb strings.Builder

	// Generate CREATE TABLE statements
	for i, table := range s.Tables {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(g.generateCreateTable(&table))
		sb.WriteString("\n")
	}

	// Generate standalone indexes
	for _, idx := range s.Indexes {
		sb.WriteString("\n")
		sb.WriteString(g.generateCreateIndex(&idx))
		sb.WriteString("\n")
	}

	// Generate views
	for _, view := range s.Views {
		sb.WriteString("\n")
		sb.WriteString(g.generateCreateView(&view))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (g *Generator) generateCreateTable(t *Table) string {
	var sb strings.Builder

	tableName := g.quoteName(t.Name)
	if t.Schema != "" {
		tableName = g.quoteName(t.Schema) + "." + tableName
	}

	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", tableName))

	// Columns
	for i, col := range t.Columns {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("    ")
		sb.WriteString(g.generateColumnDef(&col))
	}

	// Primary key constraint (if not inline)
	if t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 1 {
		sb.WriteString(",\n    ")
		sb.WriteString(g.generatePrimaryKey(t.PrimaryKey))
	}

	// Foreign key constraints
	for _, fk := range t.ForeignKeys {
		sb.WriteString(",\n    ")
		sb.WriteString(g.generateForeignKey(&fk))
	}

	// Other constraints
	for _, c := range t.Constraints {
		sb.WriteString(",\n    ")
		sb.WriteString(g.generateConstraint(&c))
	}

	sb.WriteString("\n);")

	// Inline indexes for this table
	for _, idx := range t.Indexes {
		if !idx.IsPrimary {
			sb.WriteString("\n\n")
			sb.WriteString(g.generateCreateIndex(&idx))
		}
	}

	return sb.String()
}

func (g *Generator) generateColumnDef(c *Column) string {
	var parts []string

	parts = append(parts, g.quoteName(c.Name))
	parts = append(parts, g.mapType(c.Type, c.IsIdentity))

	if !c.Nullable {
		parts = append(parts, "NOT NULL")
	}

	if c.Default != nil {
		parts = append(parts, "DEFAULT", *c.Default)
	}

	if c.IsPrimaryKey && !c.IsIdentity {
		parts = append(parts, "PRIMARY KEY")
	}

	if c.IsUnique && !c.IsPrimaryKey {
		parts = append(parts, "UNIQUE")
	}

	return strings.Join(parts, " ")
}

func (g *Generator) generatePrimaryKey(pk *PrimaryKey) string {
	cols := make([]string, len(pk.Columns))
	for i, c := range pk.Columns {
		cols[i] = g.quoteName(c)
	}

	if pk.Name != "" {
		return fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)",
			g.quoteName(pk.Name), strings.Join(cols, ", "))
	}
	return fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(cols, ", "))
}

func (g *Generator) generateForeignKey(fk *ForeignKey) string {
	localCols := make([]string, len(fk.Columns))
	for i, c := range fk.Columns {
		localCols[i] = g.quoteName(c)
	}

	refCols := make([]string, len(fk.ReferencedCols))
	for i, c := range fk.ReferencedCols {
		refCols[i] = g.quoteName(c)
	}

	refTable := g.quoteName(fk.ReferencedTable)
	if fk.ReferencedSchema != "" {
		refTable = g.quoteName(fk.ReferencedSchema) + "." + refTable
	}

	var sb strings.Builder

	if fk.Name != "" {
		sb.WriteString(fmt.Sprintf("CONSTRAINT %s ", g.quoteName(fk.Name)))
	}

	sb.WriteString(fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
		strings.Join(localCols, ", "), refTable, strings.Join(refCols, ", ")))

	if fk.OnDelete != "" {
		sb.WriteString(" ON DELETE " + fk.OnDelete)
	}
	if fk.OnUpdate != "" {
		sb.WriteString(" ON UPDATE " + fk.OnUpdate)
	}

	return sb.String()
}

func (g *Generator) generateConstraint(c *Constraint) string {
	var sb strings.Builder

	if c.Name != "" {
		sb.WriteString(fmt.Sprintf("CONSTRAINT %s ", g.quoteName(c.Name)))
	}

	switch c.Type {
	case "UNIQUE":
		cols := make([]string, len(c.Columns))
		for i, col := range c.Columns {
			cols[i] = g.quoteName(col)
		}
		sb.WriteString(fmt.Sprintf("UNIQUE (%s)", strings.Join(cols, ", ")))

	case "CHECK":
		sb.WriteString(fmt.Sprintf("CHECK (%s)", c.Expression))
	}

	return sb.String()
}

func (g *Generator) generateCreateIndex(idx *Index) string {
	cols := make([]string, len(idx.Columns))
	for i, c := range idx.Columns {
		cols[i] = g.quoteName(c)
	}

	tableName := g.quoteName(idx.Table)
	if idx.Schema != "" {
		tableName = g.quoteName(idx.Schema) + "." + tableName
	}

	unique := ""
	if idx.IsUnique {
		unique = "UNIQUE "
	}

	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);",
		unique, g.quoteName(idx.Name), tableName, strings.Join(cols, ", "))
}

func (g *Generator) generateCreateView(v *View) string {
	viewName := g.quoteName(v.Name)
	if v.Schema != "" {
		viewName = g.quoteName(v.Schema) + "." + viewName
	}

	return fmt.Sprintf("CREATE VIEW %s AS\n%s;", viewName, v.Definition)
}

func (g *Generator) quoteName(name string) string {
	switch g.dialect {
	case "mysql":
		return "`" + name + "`"
	case "sqlserver":
		return "[" + name + "]"
	default: // postgres
		return `"` + name + `"`
	}
}

func (g *Generator) mapType(t string, isIdentity bool) string {
	upper := strings.ToUpper(t)

	switch g.dialect {
	case "mysql":
		return g.toMySQL(upper, isIdentity)
	case "sqlserver":
		return g.toSQLServer(upper, isIdentity)
	default: // postgres
		return g.toPostgres(upper, isIdentity)
	}
}

func (g *Generator) toPostgres(t string, isIdentity bool) string {
	if isIdentity {
		if strings.Contains(t, "BIG") {
			return "BIGSERIAL"
		}
		return "SERIAL"
	}

	switch {
	case t == "INT" || t == "INTEGER":
		return "INTEGER"
	case t == "BIGINT":
		return "BIGINT"
	case t == "SMALLINT" || t == "TINYINT":
		return "SMALLINT"
	case strings.HasPrefix(t, "VARCHAR"):
		return t
	case t == "TEXT" || t == "LONGTEXT" || t == "MEDIUMTEXT":
		return "TEXT"
	case t == "DATETIME" || t == "DATETIME2":
		return "TIMESTAMP"
	case t == "BIT" || t == "TINYINT(1)":
		return "BOOLEAN"
	case strings.HasPrefix(t, "DECIMAL") || strings.HasPrefix(t, "NUMERIC"):
		return t
	case t == "FLOAT" || t == "REAL":
		return "REAL"
	case t == "DOUBLE" || t == "DOUBLE PRECISION":
		return "DOUBLE PRECISION"
	case t == "BLOB" || t == "VARBINARY(MAX)" || t == "IMAGE":
		return "BYTEA"
	case t == "NVARCHAR(MAX)" || t == "NTEXT":
		return "TEXT"
	default:
		return t
	}
}

func (g *Generator) toMySQL(t string, isIdentity bool) string {
	baseType := t
	if isIdentity {
		baseType = "INT"
		if strings.Contains(t, "BIG") || t == "BIGSERIAL" {
			baseType = "BIGINT"
		}
		return baseType + " AUTO_INCREMENT"
	}

	switch {
	case t == "SERIAL":
		return "INT AUTO_INCREMENT"
	case t == "BIGSERIAL":
		return "BIGINT AUTO_INCREMENT"
	case t == "INTEGER":
		return "INT"
	case t == "BOOLEAN" || t == "BOOL":
		return "TINYINT(1)"
	case t == "TIMESTAMP" || t == "TIMESTAMP WITHOUT TIME ZONE":
		return "DATETIME"
	case t == "TIMESTAMP WITH TIME ZONE" || t == "TIMESTAMPTZ":
		return "DATETIME"
	case t == "BYTEA":
		return "LONGBLOB"
	case strings.HasPrefix(t, "CHARACTER VARYING"):
		return strings.Replace(t, "CHARACTER VARYING", "VARCHAR", 1)
	default:
		return t
	}
}

func (g *Generator) toSQLServer(t string, isIdentity bool) string {
	baseType := t
	if isIdentity {
		baseType = "INT"
		if strings.Contains(t, "BIG") || t == "BIGSERIAL" {
			baseType = "BIGINT"
		}
		return baseType + " IDENTITY(1,1)"
	}

	switch {
	case t == "SERIAL":
		return "INT IDENTITY(1,1)"
	case t == "BIGSERIAL":
		return "BIGINT IDENTITY(1,1)"
	case t == "INTEGER":
		return "INT"
	case t == "BOOLEAN" || t == "BOOL":
		return "BIT"
	case t == "TIMESTAMP" || t == "TIMESTAMP WITHOUT TIME ZONE":
		return "DATETIME2"
	case t == "TEXT":
		return "NVARCHAR(MAX)"
	case t == "BYTEA":
		return "VARBINARY(MAX)"
	case strings.HasPrefix(t, "VARCHAR"):
		// Convert VARCHAR(n) to NVARCHAR(n)
		return strings.Replace(t, "VARCHAR", "NVARCHAR", 1)
	default:
		return t
	}
}
