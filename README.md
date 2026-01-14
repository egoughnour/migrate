# migrate

[![Go Reference](https://pkg.go.dev/badge/github.com/egoughnour/migrate.svg)](https://pkg.go.dev/github.com/egoughnour/migrate)
[![Go Report Card](https://goreportcard.com/badge/github.com/egoughnour/migrate)](https://goreportcard.com/report/github.com/egoughnour/migrate)
[![CI](https://github.com/egoughnour/migrate/workflows/CI/badge.svg)](https://github.com/egoughnour/migrate/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A CLI tool for analyzing, comparing, and transforming database schemas across different database engines.

## Features

- **Schema Analysis**: Extract and visualize database schema structure from live databases or SQL files
- **Schema Diffing**: Compare two schemas and generate migration SQL
- **Dialect Transformation**: Convert schemas between PostgreSQL, MySQL, and SQL Server
- **Multiple Output Formats**: Text, JSON, YAML, SQL

## Installation

### Using Go

```bash
go install github.com/egoughnour/migrate/cmd/migrate@latest
```

### From Source

```bash
git clone https://github.com/egoughnour/migrate.git
cd migrate
go build -o migrate ./cmd/migrate
```

### Pre-built Binaries

Download pre-built binaries from [Releases](https://github.com/egoughnour/migrate/releases).

## Quick Start

```bash
# Analyze a database schema
migrate analyze --source postgres://localhost/mydb

# Analyze a SQL file
migrate analyze --source schema.sql --dialect postgres

# Compare two schemas
migrate diff --source schema_v1.sql --target schema_v2.sql

# Generate migration SQL
migrate diff --source old.sql --target new.sql --output sql

# Transform schema between dialects
migrate transform --input schema.sql --from postgres --to mysql
```

## Commands

### analyze

Extract and display database schema information.

```bash
# From a live database
migrate analyze --source postgres://user:pass@host:5432/dbname

# From a SQL file
migrate analyze --source schema.sql --dialect postgres

# Output as JSON
migrate analyze --source schema.sql --dialect postgres --output json

# Verbose output with additional metadata
migrate analyze --source schema.sql --dialect postgres --verbose
```

**Flags:**
- `--source, -s` - Database connection string or SQL file path (required)
- `--dialect, -d` - SQL dialect when analyzing files: postgres, mysql, sqlserver
- `--output, -o` - Output format: text, json, yaml, sql (default: text)
- `--verbose, -v` - Show additional information

### diff

Compare two schemas and show differences.

```bash
# Compare two SQL files
migrate diff --source v1.sql --target v2.sql

# Compare database to file
migrate diff --source postgres://localhost/mydb --target schema.sql

# Generate migration SQL
migrate diff --source old.sql --target new.sql --output sql --dialect postgres

# Output as JSON for programmatic use
migrate diff --source old.sql --target new.sql --output json
```

**Flags:**
- `--source` - Source schema (connection string or file path)
- `--target` - Target schema (connection string or file path)
- `--dialect` - SQL dialect for file parsing and SQL output
- `--output` - Output format: text, json, yaml, sql

### transform

Convert a schema from one SQL dialect to another.

```bash
# PostgreSQL to MySQL
migrate transform --input schema.sql --from postgres --to mysql

# MySQL to SQL Server
migrate transform --input schema.sql --from mysql --to sqlserver

# Show transformation warnings
migrate transform --input schema.sql --from postgres --to mysql --verbose
```

**Flags:**
- `--input, -i` - Input SQL file path (required)
- `--from` - Source dialect: postgres, mysql, sqlserver (required)
- `--to` - Target dialect: postgres, mysql, sqlserver (required)
- `--verbose` - Show transformation warnings

## Supported Transformations

| From | To | Notes |
|------|------|-------|
| PostgreSQL | MySQL | SERIAL → AUTO_INCREMENT, BOOLEAN → TINYINT(1), etc. |
| PostgreSQL | SQL Server | SERIAL → IDENTITY, TEXT → NVARCHAR(MAX), etc. |
| MySQL | PostgreSQL | AUTO_INCREMENT → SERIAL, DATETIME → TIMESTAMP, etc. |
| MySQL | SQL Server | AUTO_INCREMENT → IDENTITY, etc. |
| SQL Server | PostgreSQL | IDENTITY → SERIAL, DATETIME2 → TIMESTAMP, etc. |
| SQL Server | MySQL | IDENTITY → AUTO_INCREMENT, etc. |

### Type Mappings

| Concept | PostgreSQL | MySQL | SQL Server |
|---------|-----------|-------|------------|
| Auto-increment | SERIAL | INT AUTO_INCREMENT | INT IDENTITY(1,1) |
| Boolean | BOOLEAN | TINYINT(1) | BIT |
| Long text | TEXT | LONGTEXT | NVARCHAR(MAX) |
| Timestamp | TIMESTAMP | DATETIME | DATETIME2 |
| Binary | BYTEA | LONGBLOB | VARBINARY(MAX) |
| UUID | UUID | CHAR(36) | UNIQUEIDENTIFIER |

## Library Usage

The migrate package can also be used as a Go library:

```go
package main

import (
    "fmt"
    "log"

    "github.com/egoughnour/migrate/pkg/migrate"
)

func main() {
    // Analyze a schema file
    schema, err := migrate.AnalyzeFile("schema.sql", "postgres")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d tables\n", len(schema.Tables))

    // Transform to MySQL
    mysql, warnings := migrate.Transform(schema, "postgres", "mysql")
    for _, w := range warnings {
        log.Printf("Warning: %s", w)
    }

    // Generate MySQL DDL
    sql := migrate.GenerateSQL(mysql, "mysql")
    fmt.Println(sql)
}
```

## Examples

### Comparing Schema Versions

```bash
# Compare development to production
migrate diff \
  --source postgres://localhost/myapp_dev \
  --target postgres://prod-server/myapp \
  --output sql
```

### CI/CD Integration

```bash
# In CI pipeline: ensure schema changes are captured
migrate diff --source main.sql --target feature.sql --output json | jq '.added_tables | length'
```

### Database Migration Planning

```bash
# 1. Export current schema
pg_dump --schema-only mydb > current.sql

# 2. Compare with desired schema
migrate diff --source current.sql --target desired.sql --output sql > migration.sql

# 3. Review and apply
psql mydb < migration.sql
```

## Project Structure

```
migrate/
├── cmd/migrate/          # CLI entry point
├── internal/
│   ├── cli/              # Command implementations
│   ├── schema/           # Schema types and parsing
│   ├── dialect/          # Dialect transformation
│   ├── diff/             # Schema comparison
│   └── db/               # Database introspection
├── pkg/migrate/          # Public API
└── testdata/             # Test fixtures
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [lib/pq](https://github.com/lib/pq) - PostgreSQL driver
- [tablewriter](https://github.com/olekukonko/tablewriter) - ASCII table output
- [fatih/color](https://github.com/fatih/color) - Terminal colors
