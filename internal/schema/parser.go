package schema

import (
	"regexp"
	"strings"
)

// Parser parses SQL statements into a Schema.
type Parser struct {
	dialect string
}

// NewParser creates a new SQL parser for the given dialect.
func NewParser(dialect string) *Parser {
	return &Parser{dialect: dialect}
}

// Parse parses SQL content and returns a Schema.
func (p *Parser) Parse(sql string) (*Schema, error) {
	schema := &Schema{
		Tables:  []Table{},
		Indexes: []Index{},
		Views:   []View{},
	}

	// Normalize line endings and remove comments
	sql = normalizeSQL(sql)

	// Split into statements
	statements := splitStatements(sql)

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		upper := strings.ToUpper(stmt)

		switch {
		case strings.HasPrefix(upper, "CREATE TABLE"):
			table, err := p.parseCreateTable(stmt)
			if err != nil {
				continue // Skip unparseable statements
			}
			schema.Tables = append(schema.Tables, *table)

		case strings.HasPrefix(upper, "CREATE INDEX") || strings.HasPrefix(upper, "CREATE UNIQUE INDEX"):
			idx, err := p.parseCreateIndex(stmt)
			if err != nil {
				continue
			}
			schema.Indexes = append(schema.Indexes, *idx)

		case strings.HasPrefix(upper, "CREATE VIEW") || strings.HasPrefix(upper, "CREATE OR REPLACE VIEW"):
			view, err := p.parseCreateView(stmt)
			if err != nil {
				continue
			}
			schema.Views = append(schema.Views, *view)
		}
	}

	return schema, nil
}

func (p *Parser) parseCreateTable(stmt string) (*Table, error) {
	table := &Table{
		Columns:     []Column{},
		ForeignKeys: []ForeignKey{},
		Indexes:     []Index{},
		Constraints: []Constraint{},
	}

	// Extract table name
	nameRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:(\w+)\.)?["']?(\w+)["']?`)
	matches := nameRe.FindStringSubmatch(stmt)
	if len(matches) >= 3 {
		table.Schema = matches[1]
		table.Name = matches[2]
	}

	// Extract column definitions (between parentheses)
	parenStart := strings.Index(stmt, "(")
	parenEnd := strings.LastIndex(stmt, ")")
	if parenStart == -1 || parenEnd == -1 {
		return table, nil
	}

	body := stmt[parenStart+1 : parenEnd]
	definitions := splitColumnDefs(body)

	for _, def := range definitions {
		def = strings.TrimSpace(def)
		if def == "" {
			continue
		}

		upper := strings.ToUpper(def)

		switch {
		case strings.HasPrefix(upper, "PRIMARY KEY"):
			pk := p.parsePrimaryKeyConstraint(def)
			table.PrimaryKey = pk

		case strings.HasPrefix(upper, "FOREIGN KEY"):
			fk := p.parseForeignKeyConstraint(def)
			if fk != nil {
				table.ForeignKeys = append(table.ForeignKeys, *fk)
			}

		case strings.HasPrefix(upper, "UNIQUE"):
			// Handle UNIQUE constraint
			constraint := p.parseUniqueConstraint(def)
			if constraint != nil {
				table.Constraints = append(table.Constraints, *constraint)
			}

		case strings.HasPrefix(upper, "CHECK"):
			// Handle CHECK constraint
			constraint := p.parseCheckConstraint(def)
			if constraint != nil {
				table.Constraints = append(table.Constraints, *constraint)
			}

		case strings.HasPrefix(upper, "CONSTRAINT"):
			// Named constraint - could be PK, FK, UNIQUE, or CHECK
			p.parseNamedConstraint(def, table)

		default:
			// Column definition
			col := p.parseColumnDef(def)
			if col != nil {
				table.Columns = append(table.Columns, *col)
			}
		}
	}

	return table, nil
}

func (p *Parser) parseColumnDef(def string) *Column {
	parts := strings.Fields(def)
	if len(parts) < 2 {
		return nil
	}

	col := &Column{
		Name:     strings.Trim(parts[0], `"'`+"``"),
		Type:     parts[1],
		Nullable: true,
	}

	upper := strings.ToUpper(def)

	// Check for NOT NULL
	if strings.Contains(upper, "NOT NULL") {
		col.Nullable = false
	}

	// Check for PRIMARY KEY
	if strings.Contains(upper, "PRIMARY KEY") {
		col.IsPrimaryKey = true
		col.Nullable = false
	}

	// Check for UNIQUE
	if strings.Contains(upper, "UNIQUE") {
		col.IsUnique = true
	}

	// Check for identity/auto-increment
	if strings.Contains(upper, "SERIAL") ||
		strings.Contains(upper, "AUTO_INCREMENT") ||
		strings.Contains(upper, "IDENTITY") ||
		strings.Contains(upper, "AUTOINCREMENT") {
		col.IsIdentity = true
	}

	// Check for DEFAULT
	defaultRe := regexp.MustCompile(`(?i)DEFAULT\s+(.+?)(?:\s+(?:NOT\s+)?NULL|\s+PRIMARY|\s+UNIQUE|\s+CHECK|\s+REFERENCES|,|$)`)
	if matches := defaultRe.FindStringSubmatch(def); len(matches) >= 2 {
		defaultVal := strings.TrimSpace(matches[1])
		col.Default = &defaultVal
	}

	return col
}

func (p *Parser) parsePrimaryKeyConstraint(def string) *PrimaryKey {
	pk := &PrimaryKey{}

	// Extract columns
	colsRe := regexp.MustCompile(`\(([^)]+)\)`)
	if matches := colsRe.FindStringSubmatch(def); len(matches) >= 2 {
		cols := strings.Split(matches[1], ",")
		for _, c := range cols {
			pk.Columns = append(pk.Columns, strings.TrimSpace(strings.Trim(c, `"'`+"``")))
		}
	}

	return pk
}

func (p *Parser) parseForeignKeyConstraint(def string) *ForeignKey {
	fk := &ForeignKey{}

	// Extract local columns
	localRe := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(([^)]+)\)`)
	if matches := localRe.FindStringSubmatch(def); len(matches) >= 2 {
		cols := strings.Split(matches[1], ",")
		for _, c := range cols {
			fk.Columns = append(fk.Columns, strings.TrimSpace(strings.Trim(c, `"'`+"``")))
		}
	}

	// Extract referenced table and columns
	refRe := regexp.MustCompile(`(?i)REFERENCES\s+(?:(\w+)\.)?["']?(\w+)["']?\s*\(([^)]+)\)`)
	if matches := refRe.FindStringSubmatch(def); len(matches) >= 4 {
		fk.ReferencedSchema = matches[1]
		fk.ReferencedTable = matches[2]
		cols := strings.Split(matches[3], ",")
		for _, c := range cols {
			fk.ReferencedCols = append(fk.ReferencedCols, strings.TrimSpace(strings.Trim(c, `"'`+"``")))
		}
	}

	// ON DELETE
	if strings.Contains(strings.ToUpper(def), "ON DELETE CASCADE") {
		fk.OnDelete = "CASCADE"
	} else if strings.Contains(strings.ToUpper(def), "ON DELETE SET NULL") {
		fk.OnDelete = "SET NULL"
	}

	// ON UPDATE
	if strings.Contains(strings.ToUpper(def), "ON UPDATE CASCADE") {
		fk.OnUpdate = "CASCADE"
	}

	return fk
}

func (p *Parser) parseUniqueConstraint(def string) *Constraint {
	constraint := &Constraint{Type: "UNIQUE"}

	colsRe := regexp.MustCompile(`\(([^)]+)\)`)
	if matches := colsRe.FindStringSubmatch(def); len(matches) >= 2 {
		cols := strings.Split(matches[1], ",")
		for _, c := range cols {
			constraint.Columns = append(constraint.Columns, strings.TrimSpace(strings.Trim(c, `"'`+"``")))
		}
	}

	return constraint
}

func (p *Parser) parseCheckConstraint(def string) *Constraint {
	constraint := &Constraint{Type: "CHECK"}

	// Extract expression
	checkRe := regexp.MustCompile(`(?i)CHECK\s*\((.+)\)`)
	if matches := checkRe.FindStringSubmatch(def); len(matches) >= 2 {
		constraint.Expression = matches[1]
	}

	return constraint
}

func (p *Parser) parseNamedConstraint(def string, table *Table) {
	upper := strings.ToUpper(def)

	// Extract constraint name
	nameRe := regexp.MustCompile(`(?i)CONSTRAINT\s+["']?(\w+)["']?`)
	var name string
	if matches := nameRe.FindStringSubmatch(def); len(matches) >= 2 {
		name = matches[1]
	}

	switch {
	case strings.Contains(upper, "PRIMARY KEY"):
		pk := p.parsePrimaryKeyConstraint(def)
		pk.Name = name
		table.PrimaryKey = pk

	case strings.Contains(upper, "FOREIGN KEY"):
		fk := p.parseForeignKeyConstraint(def)
		if fk != nil {
			fk.Name = name
			table.ForeignKeys = append(table.ForeignKeys, *fk)
		}

	case strings.Contains(upper, "UNIQUE"):
		constraint := p.parseUniqueConstraint(def)
		if constraint != nil {
			constraint.Name = name
			table.Constraints = append(table.Constraints, *constraint)
		}

	case strings.Contains(upper, "CHECK"):
		constraint := p.parseCheckConstraint(def)
		if constraint != nil {
			constraint.Name = name
			table.Constraints = append(table.Constraints, *constraint)
		}
	}
}

func (p *Parser) parseCreateIndex(stmt string) (*Index, error) {
	idx := &Index{}

	upper := strings.ToUpper(stmt)
	idx.IsUnique = strings.Contains(upper, "UNIQUE")

	// Extract index name
	nameRe := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:CONCURRENTLY\s+)?["']?(\w+)["']?`)
	if matches := nameRe.FindStringSubmatch(stmt); len(matches) >= 2 {
		idx.Name = matches[1]
	}

	// Extract table name
	tableRe := regexp.MustCompile(`(?i)ON\s+(?:(\w+)\.)?["']?(\w+)["']?`)
	if matches := tableRe.FindStringSubmatch(stmt); len(matches) >= 3 {
		idx.Schema = matches[1]
		idx.Table = matches[2]
	}

	// Extract columns
	colsRe := regexp.MustCompile(`\(([^)]+)\)`)
	if matches := colsRe.FindStringSubmatch(stmt); len(matches) >= 2 {
		cols := strings.Split(matches[1], ",")
		for _, c := range cols {
			c = strings.TrimSpace(c)
			// Remove ASC/DESC
			c = regexp.MustCompile(`(?i)\s+(ASC|DESC)$`).ReplaceAllString(c, "")
			idx.Columns = append(idx.Columns, strings.Trim(c, `"'`+"``"))
		}
	}

	return idx, nil
}

func (p *Parser) parseCreateView(stmt string) (*View, error) {
	view := &View{}

	// Extract view name
	nameRe := regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?VIEW\s+(?:(\w+)\.)?["']?(\w+)["']?`)
	if matches := nameRe.FindStringSubmatch(stmt); len(matches) >= 3 {
		view.Schema = matches[1]
		view.Name = matches[2]
	}

	// Extract definition (everything after AS)
	asIdx := regexp.MustCompile(`(?i)\sAS\s`).FindStringIndex(stmt)
	if asIdx != nil {
		view.Definition = strings.TrimSpace(stmt[asIdx[1]:])
	}

	return view, nil
}

// Helper functions

func normalizeSQL(sql string) string {
	// Remove single-line comments
	sql = regexp.MustCompile(`--[^\n]*`).ReplaceAllString(sql, "")
	// Remove multi-line comments
	sql = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(sql, "")
	// Normalize whitespace
	sql = regexp.MustCompile(`\s+`).ReplaceAllString(sql, " ")
	return strings.TrimSpace(sql)
}

func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for _, ch := range sql {
		if !inString && (ch == '\'' || ch == '"') {
			inString = true
			stringChar = ch
		} else if inString && ch == stringChar {
			inString = false
		}

		if ch == ';' && !inString {
			statements = append(statements, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}

func splitColumnDefs(body string) []string {
	var defs []string
	var current strings.Builder
	parenDepth := 0

	for _, ch := range body {
		switch ch {
		case '(':
			parenDepth++
			current.WriteRune(ch)
		case ')':
			parenDepth--
			current.WriteRune(ch)
		case ',':
			if parenDepth == 0 {
				defs = append(defs, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		defs = append(defs, current.String())
	}

	return defs
}
