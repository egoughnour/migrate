// Package dialect provides SQL dialect transformation utilities.
package dialect

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/egoughnour/migrate/internal/schema"
)

// Transformer converts schemas between SQL dialects.
type Transformer struct {
	from string
	to   string
}

// NewTransformer creates a new dialect transformer.
func NewTransformer(from, to string) *Transformer {
	return &Transformer{from: from, to: to}
}

// Transform converts a schema from one dialect to another.
// Returns the transformed schema and any warnings about lossy conversions.
func (t *Transformer) Transform(s *schema.Schema) (*schema.Schema, []string) {
	var warnings []string

	result := &schema.Schema{
		Tables:  make([]schema.Table, len(s.Tables)),
		Indexes: make([]schema.Index, len(s.Indexes)),
		Views:   make([]schema.View, len(s.Views)),
	}

	// Transform tables
	for i, table := range s.Tables {
		transformed, tableWarnings := t.transformTable(&table)
		result.Tables[i] = *transformed
		warnings = append(warnings, tableWarnings...)
	}

	// Transform standalone indexes
	for i, idx := range s.Indexes {
		result.Indexes[i] = t.transformIndex(&idx)
	}

	// Transform views (with warnings about potential incompatibilities)
	for i, view := range s.Views {
		transformed, viewWarnings := t.transformView(&view)
		result.Views[i] = *transformed
		warnings = append(warnings, viewWarnings...)
	}

	return result, warnings
}

func (t *Transformer) transformTable(table *schema.Table) (*schema.Table, []string) {
	var warnings []string

	result := &schema.Table{
		Name:        table.Name,
		Schema:      table.Schema,
		Columns:     make([]schema.Column, len(table.Columns)),
		PrimaryKey:  table.PrimaryKey,
		ForeignKeys: make([]schema.ForeignKey, len(table.ForeignKeys)),
		Indexes:     make([]schema.Index, len(table.Indexes)),
		Constraints: make([]schema.Constraint, len(table.Constraints)),
	}

	// Transform columns
	for i, col := range table.Columns {
		transformed, colWarnings := t.transformColumn(&col, table.Name)
		result.Columns[i] = *transformed
		warnings = append(warnings, colWarnings...)
	}

	// Copy foreign keys (structure is dialect-agnostic)
	copy(result.ForeignKeys, table.ForeignKeys)

	// Transform indexes
	for i, idx := range table.Indexes {
		result.Indexes[i] = t.transformIndex(&idx)
	}

	// Copy constraints
	copy(result.Constraints, table.Constraints)

	return result, warnings
}

func (t *Transformer) transformColumn(col *schema.Column, tableName string) (*schema.Column, []string) {
	var warnings []string

	result := &schema.Column{
		Name:         col.Name,
		Nullable:     col.Nullable,
		Default:      col.Default,
		IsPrimaryKey: col.IsPrimaryKey,
		IsUnique:     col.IsUnique,
		IsIdentity:   col.IsIdentity,
		Comment:      col.Comment,
	}

	// Transform data type
	result.Type, warnings = t.transformType(col.Type, col.IsIdentity, tableName, col.Name)

	// Transform default value if needed
	if col.Default != nil {
		result.Default = t.transformDefault(*col.Default)
	}

	return result, warnings
}

func (t *Transformer) transformType(dataType string, isIdentity bool, tableName, colName string) (string, []string) {
	var warnings []string
	upper := strings.ToUpper(dataType)

	// Handle identity/auto-increment types
	if isIdentity {
		return t.mapIdentityType(upper), nil
	}

	// Map types based on source and target dialects
	mapped := t.mapDataType(upper)

	// Check for potential data loss
	if warning := t.checkDataLoss(upper, mapped, tableName, colName); warning != "" {
		warnings = append(warnings, warning)
	}

	return mapped, warnings
}

func (t *Transformer) mapIdentityType(dataType string) string {
	isBig := strings.Contains(dataType, "BIG")

	switch t.to {
	case "postgres":
		if isBig {
			return "BIGSERIAL"
		}
		return "SERIAL"
	case "mysql":
		if isBig {
			return "BIGINT AUTO_INCREMENT"
		}
		return "INT AUTO_INCREMENT"
	case "sqlserver":
		if isBig {
			return "BIGINT IDENTITY(1,1)"
		}
		return "INT IDENTITY(1,1)"
	default:
		return dataType
	}
}

func (t *Transformer) mapDataType(dataType string) string {
	// First normalize from source dialect
	normalized := t.normalizeType(dataType)

	// Then convert to target dialect
	return t.toTargetType(normalized)
}

func (t *Transformer) normalizeType(dataType string) string {
	// Normalize to a common intermediate representation

	switch {
	// Integer types
	case dataType == "INT" || dataType == "INTEGER":
		return "INTEGER"
	case dataType == "BIGINT":
		return "BIGINT"
	case dataType == "SMALLINT" || dataType == "TINYINT":
		return "SMALLINT"

	// Boolean types
	case dataType == "BOOLEAN" || dataType == "BOOL" || dataType == "BIT" || dataType == "TINYINT(1)":
		return "BOOLEAN"

	// String types
	case strings.HasPrefix(dataType, "VARCHAR") || strings.HasPrefix(dataType, "NVARCHAR") ||
		strings.HasPrefix(dataType, "CHARACTER VARYING"):
		// Extract length if present
		re := regexp.MustCompile(`\((\d+|MAX)\)`)
		if matches := re.FindStringSubmatch(dataType); len(matches) >= 2 {
			if matches[1] == "MAX" {
				return "TEXT"
			}
			return fmt.Sprintf("VARCHAR(%s)", matches[1])
		}
		return "VARCHAR(255)"

	case dataType == "TEXT" || dataType == "LONGTEXT" || dataType == "MEDIUMTEXT" ||
		dataType == "NVARCHAR(MAX)" || dataType == "NTEXT":
		return "TEXT"

	case dataType == "CHAR" || strings.HasPrefix(dataType, "CHAR(") ||
		strings.HasPrefix(dataType, "NCHAR"):
		return dataType

	// Date/Time types
	case dataType == "TIMESTAMP" || dataType == "TIMESTAMP WITHOUT TIME ZONE" ||
		dataType == "DATETIME" || dataType == "DATETIME2":
		return "TIMESTAMP"

	case dataType == "TIMESTAMP WITH TIME ZONE" || dataType == "TIMESTAMPTZ" ||
		dataType == "DATETIMEOFFSET":
		return "TIMESTAMP_TZ"

	case dataType == "DATE":
		return "DATE"

	case dataType == "TIME" || dataType == "TIME WITHOUT TIME ZONE":
		return "TIME"

	// Numeric types
	case strings.HasPrefix(dataType, "DECIMAL") || strings.HasPrefix(dataType, "NUMERIC"):
		return dataType
	case dataType == "FLOAT" || dataType == "REAL":
		return "REAL"
	case dataType == "DOUBLE" || dataType == "DOUBLE PRECISION":
		return "DOUBLE"

	// Binary types
	case dataType == "BYTEA" || dataType == "BLOB" || dataType == "LONGBLOB" ||
		dataType == "VARBINARY(MAX)" || dataType == "IMAGE":
		return "BINARY"

	// JSON type
	case dataType == "JSON" || dataType == "JSONB":
		return "JSON"

	// UUID type
	case dataType == "UUID" || dataType == "UNIQUEIDENTIFIER":
		return "UUID"

	default:
		return dataType
	}
}

func (t *Transformer) toTargetType(normalized string) string {
	switch t.to {
	case "postgres":
		return t.toPostgres(normalized)
	case "mysql":
		return t.toMySQL(normalized)
	case "sqlserver":
		return t.toSQLServer(normalized)
	default:
		return normalized
	}
}

func (t *Transformer) toPostgres(normalized string) string {
	switch normalized {
	case "BOOLEAN":
		return "BOOLEAN"
	case "TIMESTAMP":
		return "TIMESTAMP"
	case "TIMESTAMP_TZ":
		return "TIMESTAMP WITH TIME ZONE"
	case "BINARY":
		return "BYTEA"
	case "JSON":
		return "JSONB"
	case "UUID":
		return "UUID"
	case "DOUBLE":
		return "DOUBLE PRECISION"
	default:
		return normalized
	}
}

func (t *Transformer) toMySQL(normalized string) string {
	switch normalized {
	case "BOOLEAN":
		return "TINYINT(1)"
	case "TIMESTAMP":
		return "DATETIME"
	case "TIMESTAMP_TZ":
		return "DATETIME" // MySQL doesn't have native timezone support
	case "BINARY":
		return "LONGBLOB"
	case "JSON":
		return "JSON"
	case "UUID":
		return "CHAR(36)"
	case "DOUBLE":
		return "DOUBLE"
	case "TEXT":
		return "LONGTEXT"
	default:
		return normalized
	}
}

func (t *Transformer) toSQLServer(normalized string) string {
	switch normalized {
	case "BOOLEAN":
		return "BIT"
	case "TIMESTAMP":
		return "DATETIME2"
	case "TIMESTAMP_TZ":
		return "DATETIMEOFFSET"
	case "BINARY":
		return "VARBINARY(MAX)"
	case "JSON":
		return "NVARCHAR(MAX)" // SQL Server 2016+ supports JSON functions on NVARCHAR
	case "UUID":
		return "UNIQUEIDENTIFIER"
	case "DOUBLE":
		return "FLOAT"
	case "TEXT":
		return "NVARCHAR(MAX)"
	case "INTEGER":
		return "INT"
	default:
		if strings.HasPrefix(normalized, "VARCHAR") {
			return strings.Replace(normalized, "VARCHAR", "NVARCHAR", 1)
		}
		return normalized
	}
}

func (t *Transformer) transformDefault(defaultVal string) *string {
	upper := strings.ToUpper(defaultVal)

	// Handle common default value transformations
	switch {
	case upper == "NOW()" || upper == "CURRENT_TIMESTAMP" || upper == "GETDATE()" || upper == "GETUTCDATE()":
		var result string
		switch t.to {
		case "postgres":
			result = "NOW()"
		case "mysql":
			result = "CURRENT_TIMESTAMP"
		case "sqlserver":
			result = "GETDATE()"
		}
		return &result

	case upper == "TRUE" || upper == "FALSE":
		var result string
		switch t.to {
		case "mysql":
			if upper == "TRUE" {
				result = "1"
			} else {
				result = "0"
			}
		case "sqlserver":
			if upper == "TRUE" {
				result = "1"
			} else {
				result = "0"
			}
		default:
			result = defaultVal
		}
		return &result

	case upper == "GEN_RANDOM_UUID()" || upper == "UUID()" || upper == "NEWID()":
		var result string
		switch t.to {
		case "postgres":
			result = "gen_random_uuid()"
		case "mysql":
			result = "UUID()"
		case "sqlserver":
			result = "NEWID()"
		}
		return &result
	}

	return &defaultVal
}

func (t *Transformer) transformIndex(idx *schema.Index) schema.Index {
	return schema.Index{
		Name:      idx.Name,
		Table:     idx.Table,
		Schema:    idx.Schema,
		Columns:   idx.Columns,
		IsUnique:  idx.IsUnique,
		IsPrimary: idx.IsPrimary,
		Type:      t.mapIndexType(idx.Type),
	}
}

func (t *Transformer) mapIndexType(indexType string) string {
	if indexType == "" {
		return ""
	}

	upper := strings.ToUpper(indexType)

	switch t.to {
	case "mysql":
		// MySQL supports BTREE and HASH
		if upper == "GIN" || upper == "GIST" || upper == "BRIN" {
			return "BTREE" // Fallback
		}
		return upper
	case "sqlserver":
		// SQL Server uses CLUSTERED/NONCLUSTERED
		if upper == "GIN" || upper == "GIST" || upper == "BRIN" || upper == "HASH" {
			return "" // Let SQL Server choose default
		}
		return ""
	default:
		return indexType
	}
}

func (t *Transformer) transformView(view *schema.View) (*schema.View, []string) {
	var warnings []string

	result := &schema.View{
		Name:       view.Name,
		Schema:     view.Schema,
		Definition: view.Definition,
	}

	// Views often contain dialect-specific SQL that can't be automatically converted
	if t.from != t.to {
		warnings = append(warnings, fmt.Sprintf(
			"View '%s' may contain %s-specific SQL that requires manual review",
			view.Name, t.from))
	}

	return result, warnings
}

func (t *Transformer) checkDataLoss(original, mapped, tableName, colName string) string {
	// Check for potential precision loss or incompatibilities
	switch {
	case (original == "JSONB" || original == "JSON") && t.to == "sqlserver":
		return fmt.Sprintf("%s.%s: JSON stored as NVARCHAR(MAX) - JSON functions available but no native type", tableName, colName)

	case (original == "UUID" || original == "UNIQUEIDENTIFIER") && t.to == "mysql":
		return fmt.Sprintf("%s.%s: UUID stored as CHAR(36) - no native UUID type in MySQL", tableName, colName)

	case strings.HasPrefix(original, "TIMESTAMP") && strings.Contains(original, "TIME ZONE") && t.to == "mysql":
		return fmt.Sprintf("%s.%s: Timezone information will be lost - MySQL DATETIME has no timezone", tableName, colName)

	case original == "DOUBLE PRECISION" && t.to == "sqlserver":
		return fmt.Sprintf("%s.%s: DOUBLE PRECISION mapped to FLOAT - verify precision requirements", tableName, colName)
	}

	return ""
}

// SupportedDialects returns the list of supported SQL dialects.
func SupportedDialects() []string {
	return []string{"postgres", "mysql", "sqlserver"}
}

// IsSupported checks if a dialect is supported.
func IsSupported(dialect string) bool {
	for _, d := range SupportedDialects() {
		if d == dialect {
			return true
		}
	}
	return false
}
