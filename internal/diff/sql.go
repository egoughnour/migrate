package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/egoughnour/migrate/internal/schema"
)

// SQLGenerator generates migration SQL from schema changes.
type SQLGenerator struct {
	dialect string
}

// NewSQLGenerator creates a new SQL generator for the given dialect.
func NewSQLGenerator(dialect string) *SQLGenerator {
	return &SQLGenerator{dialect: dialect}
}

// WriteSQL generates migration SQL for the given changes.
func (g *SQLGenerator) WriteSQL(w io.Writer, c *Changes) error {
	var sb strings.Builder

	sb.WriteString("-- Migration SQL\n")
	sb.WriteString(fmt.Sprintf("-- Dialect: %s\n\n", g.dialect))

	// Drop removed tables (at the end for FK dependencies)
	var dropTables []string
	for _, t := range c.RemovedTables {
		dropTables = append(dropTables, g.generateDropTable(&t))
	}

	// Create new tables
	for _, t := range c.AddedTables {
		sb.WriteString(g.generateCreateTable(&t))
		sb.WriteString("\n\n")
	}

	// Alter existing tables
	for _, tc := range c.ModifiedTables {
		alterSQL := g.generateAlterTable(&tc)
		if alterSQL != "" {
			sb.WriteString(alterSQL)
			sb.WriteString("\n")
		}
	}

	// Create new standalone indexes
	for _, idx := range c.AddedIndexes {
		sb.WriteString(g.generateCreateIndex(&idx))
		sb.WriteString("\n")
	}

	// Drop removed indexes
	for _, idx := range c.RemovedIndexes {
		sb.WriteString(g.generateDropIndex(&idx))
		sb.WriteString("\n")
	}

	// Create new views
	for _, v := range c.AddedViews {
		sb.WriteString(g.generateCreateView(&v))
		sb.WriteString("\n\n")
	}

	// Modify views (drop and recreate)
	for _, vc := range c.ModifiedViews {
		sb.WriteString(g.generateDropView(vc.Name))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("CREATE VIEW %s AS\n%s;\n\n", g.quoteName(vc.Name), vc.NewDefinition))
	}

	// Drop removed views
	for _, v := range c.RemovedViews {
		sb.WriteString(g.generateDropView(v.Name))
		sb.WriteString("\n")
	}

	// Drop tables at the end
	for _, sql := range dropTables {
		sb.WriteString(sql)
		sb.WriteString("\n")
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}

func (g *SQLGenerator) generateCreateTable(t *schema.Table) string {
	gen := schema.NewGenerator(g.dialect)
	return gen.Generate(&schema.Schema{Tables: []schema.Table{*t}})
}

func (g *SQLGenerator) generateDropTable(t *schema.Table) string {
	tableName := g.quoteName(t.Name)
	if t.Schema != "" {
		tableName = g.quoteName(t.Schema) + "." + tableName
	}
	return fmt.Sprintf("DROP TABLE %s;", tableName)
}

func (g *SQLGenerator) generateAlterTable(tc *TableChanges) string {
	var sb strings.Builder
	tableName := g.quoteName(tc.Name)

	// Drop removed foreign keys first (before dropping columns)
	for _, fk := range tc.RemovedForeignKeys {
		sb.WriteString(g.generateDropConstraint(tableName, fk.Name, "FOREIGN KEY"))
		sb.WriteString("\n")
	}

	// Drop removed indexes
	for _, idx := range tc.RemovedIndexes {
		sb.WriteString(g.generateDropIndex(&idx))
		sb.WriteString("\n")
	}

	// Drop removed columns
	for _, col := range tc.RemovedColumns {
		sb.WriteString(g.generateDropColumn(tableName, col.Name))
		sb.WriteString("\n")
	}

	// Add new columns
	for _, col := range tc.AddedColumns {
		sb.WriteString(g.generateAddColumn(tableName, &col))
		sb.WriteString("\n")
	}

	// Modify columns
	for _, col := range tc.ModifiedColumns {
		sb.WriteString(g.generateAlterColumn(tableName, &col))
		sb.WriteString("\n")
	}

	// Add new indexes
	for _, idx := range tc.AddedIndexes {
		sb.WriteString(g.generateCreateIndex(&idx))
		sb.WriteString("\n")
	}

	// Add new foreign keys
	for _, fk := range tc.AddedForeignKeys {
		sb.WriteString(g.generateAddForeignKey(tableName, &fk))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (g *SQLGenerator) generateDropColumn(tableName, colName string) string {
	switch g.dialect {
	case "sqlserver":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", tableName, g.quoteName(colName))
	default:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", tableName, g.quoteName(colName))
	}
}

func (g *SQLGenerator) generateAddColumn(tableName string, col *schema.Column) string {
	colDef := g.generateColumnDef(col)
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", tableName, colDef)
}

func (g *SQLGenerator) generateAlterColumn(tableName string, col *ColumnChanges) string {
	var sb strings.Builder

	switch g.dialect {
	case "postgres":
		if col.NewType != "" {
			sb.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;\n",
				tableName, g.quoteName(col.Name), col.NewType))
		}
		if col.NullableChanged {
			if col.NewNullable {
				sb.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;\n",
					tableName, g.quoteName(col.Name)))
			} else {
				sb.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;\n",
					tableName, g.quoteName(col.Name)))
			}
		}
		if col.DefaultChanged {
			if col.NewDefault != nil {
				sb.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;\n",
					tableName, g.quoteName(col.Name), *col.NewDefault))
			} else {
				sb.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;\n",
					tableName, g.quoteName(col.Name)))
			}
		}

	case "mysql":
		// MySQL uses MODIFY COLUMN
		var parts []string
		parts = append(parts, g.quoteName(col.Name))
		if col.NewType != "" {
			parts = append(parts, col.NewType)
		} else {
			parts = append(parts, col.OldType)
		}
		if !col.NewNullable {
			parts = append(parts, "NOT NULL")
		}
		if col.NewDefault != nil {
			parts = append(parts, "DEFAULT", *col.NewDefault)
		}
		sb.WriteString(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;\n",
			tableName, strings.Join(parts, " ")))

	case "sqlserver":
		if col.NewType != "" || col.NullableChanged {
			nullable := ""
			if !col.NewNullable {
				nullable = " NOT NULL"
			} else {
				nullable = " NULL"
			}
			colType := col.NewType
			if colType == "" {
				colType = col.OldType
			}
			sb.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s %s%s;\n",
				tableName, g.quoteName(col.Name), colType, nullable))
		}
		if col.DefaultChanged {
			// SQL Server requires dropping and adding default constraint
			sb.WriteString(fmt.Sprintf("-- Note: May need to drop existing default constraint first\n"))
			if col.NewDefault != nil {
				sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD DEFAULT %s FOR %s;\n",
					tableName, *col.NewDefault, g.quoteName(col.Name)))
			}
		}
	}

	return sb.String()
}

func (g *SQLGenerator) generateDropConstraint(tableName, constraintName, constraintType string) string {
	if constraintName == "" {
		return fmt.Sprintf("-- Warning: Cannot drop unnamed %s constraint on %s\n", constraintType, tableName)
	}

	switch g.dialect {
	case "mysql":
		if constraintType == "FOREIGN KEY" {
			return fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s;", tableName, g.quoteName(constraintName))
		}
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", tableName, g.quoteName(constraintName))
	default:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", tableName, g.quoteName(constraintName))
	}
}

func (g *SQLGenerator) generateAddForeignKey(tableName string, fk *schema.ForeignKey) string {
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
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD ", tableName))

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

	sb.WriteString(";")
	return sb.String()
}

func (g *SQLGenerator) generateCreateIndex(idx *schema.Index) string {
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

func (g *SQLGenerator) generateDropIndex(idx *schema.Index) string {
	switch g.dialect {
	case "mysql":
		tableName := g.quoteName(idx.Table)
		if idx.Schema != "" {
			tableName = g.quoteName(idx.Schema) + "." + tableName
		}
		return fmt.Sprintf("DROP INDEX %s ON %s;", g.quoteName(idx.Name), tableName)
	case "sqlserver":
		tableName := g.quoteName(idx.Table)
		if idx.Schema != "" {
			tableName = g.quoteName(idx.Schema) + "." + tableName
		}
		return fmt.Sprintf("DROP INDEX %s ON %s;", g.quoteName(idx.Name), tableName)
	default: // postgres
		return fmt.Sprintf("DROP INDEX %s;", g.quoteName(idx.Name))
	}
}

func (g *SQLGenerator) generateCreateView(v *schema.View) string {
	viewName := g.quoteName(v.Name)
	if v.Schema != "" {
		viewName = g.quoteName(v.Schema) + "." + viewName
	}
	return fmt.Sprintf("CREATE VIEW %s AS\n%s;", viewName, v.Definition)
}

func (g *SQLGenerator) generateDropView(name string) string {
	return fmt.Sprintf("DROP VIEW IF EXISTS %s;", g.quoteName(name))
}

func (g *SQLGenerator) generateColumnDef(col *schema.Column) string {
	var parts []string
	parts = append(parts, g.quoteName(col.Name), col.Type)

	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	}

	if col.Default != nil {
		parts = append(parts, "DEFAULT", *col.Default)
	}

	return strings.Join(parts, " ")
}

func (g *SQLGenerator) quoteName(name string) string {
	switch g.dialect {
	case "mysql":
		return "`" + name + "`"
	case "sqlserver":
		return "[" + name + "]"
	default:
		return `"` + name + `"`
	}
}
