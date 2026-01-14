// Package db provides database introspection capabilities.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/egoughnour/migrate/internal/schema"
)

// Introspector extracts schema information from a database.
type Introspector interface {
	Introspect() (*schema.Schema, error)
	Close() error
}

// NewIntrospector creates an introspector for the given connection string.
func NewIntrospector(connStr string) (Introspector, error) {
	dialect, err := detectDialect(connStr)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName(dialect), connStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	switch dialect {
	case "postgres":
		return &PostgresIntrospector{db: db}, nil
	case "mysql":
		return &MySQLIntrospector{db: db}, nil
	case "sqlserver":
		return &SQLServerIntrospector{db: db}, nil
	default:
		db.Close()
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

func detectDialect(connStr string) (string, error) {
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		return "postgres", nil
	}
	if strings.HasPrefix(connStr, "mysql://") {
		return "mysql", nil
	}
	if strings.HasPrefix(connStr, "sqlserver://") || strings.HasPrefix(connStr, "mssql://") {
		return "sqlserver", nil
	}

	// Try to parse as URL and check scheme
	u, err := url.Parse(connStr)
	if err == nil && u.Scheme != "" {
		switch u.Scheme {
		case "postgres", "postgresql":
			return "postgres", nil
		case "mysql":
			return "mysql", nil
		case "sqlserver", "mssql":
			return "sqlserver", nil
		}
	}

	return "", fmt.Errorf("unable to detect database dialect from connection string")
}

func driverName(dialect string) string {
	switch dialect {
	case "postgres":
		return "postgres"
	case "mysql":
		return "mysql"
	case "sqlserver":
		return "sqlserver"
	default:
		return dialect
	}
}

// PostgresIntrospector extracts schema from PostgreSQL databases.
type PostgresIntrospector struct {
	db *sql.DB
}

// Introspect extracts the schema from a PostgreSQL database.
func (p *PostgresIntrospector) Introspect() (*schema.Schema, error) {
	s := &schema.Schema{
		Tables:  []schema.Table{},
		Indexes: []schema.Index{},
		Views:   []schema.View{},
	}

	// Get tables
	tables, err := p.getTables()
	if err != nil {
		return nil, fmt.Errorf("getting tables: %w", err)
	}

	for _, tableName := range tables {
		table := schema.Table{Name: tableName}

		// Get columns
		columns, err := p.getColumns(tableName)
		if err != nil {
			return nil, fmt.Errorf("getting columns for %s: %w", tableName, err)
		}
		table.Columns = columns

		// Get primary key
		pk, err := p.getPrimaryKey(tableName)
		if err != nil {
			return nil, fmt.Errorf("getting primary key for %s: %w", tableName, err)
		}
		table.PrimaryKey = pk

		// Get foreign keys
		fks, err := p.getForeignKeys(tableName)
		if err != nil {
			return nil, fmt.Errorf("getting foreign keys for %s: %w", tableName, err)
		}
		table.ForeignKeys = fks

		// Get indexes
		indexes, err := p.getIndexes(tableName)
		if err != nil {
			return nil, fmt.Errorf("getting indexes for %s: %w", tableName, err)
		}
		table.Indexes = indexes

		s.Tables = append(s.Tables, table)
	}

	// Get views
	views, err := p.getViews()
	if err != nil {
		return nil, fmt.Errorf("getting views: %w", err)
	}
	s.Views = views

	return s, nil
}

func (p *PostgresIntrospector) getTables() ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (p *PostgresIntrospector) getColumns(tableName string) ([]schema.Column, error) {
	query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			CASE WHEN column_default LIKE 'nextval%' THEN true ELSE false END as is_identity
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []schema.Column
	for rows.Next() {
		var col schema.Column
		var nullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &col.IsIdentity); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "YES"
		if defaultVal.Valid {
			col.Default = &defaultVal.String
		}

		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (p *PostgresIntrospector) getPrimaryKey(tableName string) (*schema.PrimaryKey, error) {
	query := `
		SELECT kcu.column_name, tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		AND tc.table_schema = 'public'
		AND tc.table_name = $1
		ORDER BY kcu.ordinal_position`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pk *schema.PrimaryKey
	for rows.Next() {
		var colName, constraintName string
		if err := rows.Scan(&colName, &constraintName); err != nil {
			return nil, err
		}
		if pk == nil {
			pk = &schema.PrimaryKey{Name: constraintName}
		}
		pk.Columns = append(pk.Columns, colName)
	}
	return pk, rows.Err()
}

func (p *PostgresIntrospector) getForeignKeys(tableName string) ([]schema.ForeignKey, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS referenced_table,
			ccu.column_name AS referenced_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = 'public'
		AND tc.table_name = $1`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fkMap := make(map[string]*schema.ForeignKey)
	for rows.Next() {
		var constraintName, colName, refTable, refCol string
		if err := rows.Scan(&constraintName, &colName, &refTable, &refCol); err != nil {
			return nil, err
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, colName)
			fk.ReferencedCols = append(fk.ReferencedCols, refCol)
		} else {
			fkMap[constraintName] = &schema.ForeignKey{
				Name:            constraintName,
				Columns:         []string{colName},
				ReferencedTable: refTable,
				ReferencedCols:  []string{refCol},
			}
		}
	}

	var fks []schema.ForeignKey
	for _, fk := range fkMap {
		fks = append(fks, *fk)
	}
	return fks, rows.Err()
}

func (p *PostgresIntrospector) getIndexes(tableName string) ([]schema.Index, error) {
	query := `
		SELECT
			i.relname as index_name,
			a.attname as column_name,
			ix.indisunique as is_unique,
			ix.indisprimary as is_primary
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relkind = 'r'
		AND t.relname = $1
		AND t.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
		ORDER BY i.relname, a.attnum`

	rows, err := p.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idxMap := make(map[string]*schema.Index)
	for rows.Next() {
		var idxName, colName string
		var isUnique, isPrimary bool
		if err := rows.Scan(&idxName, &colName, &isUnique, &isPrimary); err != nil {
			return nil, err
		}

		if idx, exists := idxMap[idxName]; exists {
			idx.Columns = append(idx.Columns, colName)
		} else {
			idxMap[idxName] = &schema.Index{
				Name:      idxName,
				Table:     tableName,
				Columns:   []string{colName},
				IsUnique:  isUnique,
				IsPrimary: isPrimary,
			}
		}
	}

	var indexes []schema.Index
	for _, idx := range idxMap {
		indexes = append(indexes, *idx)
	}
	return indexes, rows.Err()
}

func (p *PostgresIntrospector) getViews() ([]schema.View, error) {
	query := `
		SELECT table_name, view_definition
		FROM information_schema.views
		WHERE table_schema = 'public'`

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []schema.View
	for rows.Next() {
		var v schema.View
		if err := rows.Scan(&v.Name, &v.Definition); err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

// Close closes the database connection.
func (p *PostgresIntrospector) Close() error {
	return p.db.Close()
}

// MySQLIntrospector extracts schema from MySQL databases.
type MySQLIntrospector struct {
	db *sql.DB
}

// Introspect extracts the schema from a MySQL database.
func (m *MySQLIntrospector) Introspect() (*schema.Schema, error) {
	// Similar implementation to PostgreSQL but with MySQL-specific queries
	return nil, fmt.Errorf("MySQL introspection not yet implemented")
}

// Close closes the database connection.
func (m *MySQLIntrospector) Close() error {
	return m.db.Close()
}

// SQLServerIntrospector extracts schema from SQL Server databases.
type SQLServerIntrospector struct {
	db *sql.DB
}

// Introspect extracts the schema from a SQL Server database.
func (s *SQLServerIntrospector) Introspect() (*schema.Schema, error) {
	// Similar implementation to PostgreSQL but with SQL Server-specific queries
	return nil, fmt.Errorf("SQL Server introspection not yet implemented")
}

// Close closes the database connection.
func (s *SQLServerIntrospector) Close() error {
	return s.db.Close()
}
