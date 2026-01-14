// Package diff provides schema comparison utilities.
package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/egoughnour/migrate/internal/schema"
	"gopkg.in/yaml.v3"
)

// Changes represents the differences between two schemas.
type Changes struct {
	AddedTables    []schema.Table  `json:"added_tables,omitempty" yaml:"added_tables,omitempty"`
	RemovedTables  []schema.Table  `json:"removed_tables,omitempty" yaml:"removed_tables,omitempty"`
	ModifiedTables []TableChanges  `json:"modified_tables,omitempty" yaml:"modified_tables,omitempty"`
	AddedIndexes   []schema.Index  `json:"added_indexes,omitempty" yaml:"added_indexes,omitempty"`
	RemovedIndexes []schema.Index  `json:"removed_indexes,omitempty" yaml:"removed_indexes,omitempty"`
	AddedViews     []schema.View   `json:"added_views,omitempty" yaml:"added_views,omitempty"`
	RemovedViews   []schema.View   `json:"removed_views,omitempty" yaml:"removed_views,omitempty"`
	ModifiedViews  []ViewChanges   `json:"modified_views,omitempty" yaml:"modified_views,omitempty"`
}

// TableChanges represents changes to a specific table.
type TableChanges struct {
	Name              string              `json:"name" yaml:"name"`
	AddedColumns      []schema.Column     `json:"added_columns,omitempty" yaml:"added_columns,omitempty"`
	RemovedColumns    []schema.Column     `json:"removed_columns,omitempty" yaml:"removed_columns,omitempty"`
	ModifiedColumns   []ColumnChanges     `json:"modified_columns,omitempty" yaml:"modified_columns,omitempty"`
	AddedIndexes      []schema.Index      `json:"added_indexes,omitempty" yaml:"added_indexes,omitempty"`
	RemovedIndexes    []schema.Index      `json:"removed_indexes,omitempty" yaml:"removed_indexes,omitempty"`
	AddedForeignKeys  []schema.ForeignKey `json:"added_foreign_keys,omitempty" yaml:"added_foreign_keys,omitempty"`
	RemovedForeignKeys []schema.ForeignKey `json:"removed_foreign_keys,omitempty" yaml:"removed_foreign_keys,omitempty"`
	AddedConstraints  []schema.Constraint `json:"added_constraints,omitempty" yaml:"added_constraints,omitempty"`
	RemovedConstraints []schema.Constraint `json:"removed_constraints,omitempty" yaml:"removed_constraints,omitempty"`
	PrimaryKeyChanged bool                `json:"primary_key_changed,omitempty" yaml:"primary_key_changed,omitempty"`
}

// ColumnChanges represents changes to a specific column.
type ColumnChanges struct {
	Name            string  `json:"name" yaml:"name"`
	OldType         string  `json:"old_type,omitempty" yaml:"old_type,omitempty"`
	NewType         string  `json:"new_type,omitempty" yaml:"new_type,omitempty"`
	NullableChanged bool    `json:"nullable_changed,omitempty" yaml:"nullable_changed,omitempty"`
	OldNullable     bool    `json:"old_nullable,omitempty" yaml:"old_nullable,omitempty"`
	NewNullable     bool    `json:"new_nullable,omitempty" yaml:"new_nullable,omitempty"`
	DefaultChanged  bool    `json:"default_changed,omitempty" yaml:"default_changed,omitempty"`
	OldDefault      *string `json:"old_default,omitempty" yaml:"old_default,omitempty"`
	NewDefault      *string `json:"new_default,omitempty" yaml:"new_default,omitempty"`
}

// ViewChanges represents changes to a specific view.
type ViewChanges struct {
	Name          string `json:"name" yaml:"name"`
	OldDefinition string `json:"old_definition,omitempty" yaml:"old_definition,omitempty"`
	NewDefinition string `json:"new_definition,omitempty" yaml:"new_definition,omitempty"`
}

// Differ compares two schemas.
type Differ struct {
	source *schema.Schema
	target *schema.Schema
}

// NewDiffer creates a new schema differ.
func NewDiffer(source, target *schema.Schema) *Differ {
	return &Differ{source: source, target: target}
}

// Compare computes the differences between source and target schemas.
func (d *Differ) Compare() *Changes {
	changes := &Changes{}

	// Compare tables
	d.compareTables(changes)

	// Compare standalone indexes
	d.compareStandaloneIndexes(changes)

	// Compare views
	d.compareViews(changes)

	return changes
}

func (d *Differ) compareTables(changes *Changes) {
	sourceMap := make(map[string]*schema.Table)
	for i := range d.source.Tables {
		t := &d.source.Tables[i]
		sourceMap[t.Name] = t
	}

	targetMap := make(map[string]*schema.Table)
	for i := range d.target.Tables {
		t := &d.target.Tables[i]
		targetMap[t.Name] = t
	}

	// Find added tables
	for name, table := range targetMap {
		if _, exists := sourceMap[name]; !exists {
			changes.AddedTables = append(changes.AddedTables, *table)
		}
	}

	// Find removed tables
	for name, table := range sourceMap {
		if _, exists := targetMap[name]; !exists {
			changes.RemovedTables = append(changes.RemovedTables, *table)
		}
	}

	// Find modified tables
	for name, sourceTable := range sourceMap {
		if targetTable, exists := targetMap[name]; exists {
			tableChanges := d.compareTable(sourceTable, targetTable)
			if tableChanges != nil {
				changes.ModifiedTables = append(changes.ModifiedTables, *tableChanges)
			}
		}
	}
}

func (d *Differ) compareTable(source, target *schema.Table) *TableChanges {
	changes := &TableChanges{Name: source.Name}
	hasChanges := false

	// Compare columns
	sourceColMap := make(map[string]*schema.Column)
	for i := range source.Columns {
		c := &source.Columns[i]
		sourceColMap[c.Name] = c
	}

	targetColMap := make(map[string]*schema.Column)
	for i := range target.Columns {
		c := &target.Columns[i]
		targetColMap[c.Name] = c
	}

	// Added columns
	for name, col := range targetColMap {
		if _, exists := sourceColMap[name]; !exists {
			changes.AddedColumns = append(changes.AddedColumns, *col)
			hasChanges = true
		}
	}

	// Removed columns
	for name, col := range sourceColMap {
		if _, exists := targetColMap[name]; !exists {
			changes.RemovedColumns = append(changes.RemovedColumns, *col)
			hasChanges = true
		}
	}

	// Modified columns
	for name, sourceCol := range sourceColMap {
		if targetCol, exists := targetColMap[name]; exists {
			colChanges := d.compareColumn(sourceCol, targetCol)
			if colChanges != nil {
				changes.ModifiedColumns = append(changes.ModifiedColumns, *colChanges)
				hasChanges = true
			}
		}
	}

	// Compare indexes within table
	sourceIdxMap := make(map[string]*schema.Index)
	for i := range source.Indexes {
		idx := &source.Indexes[i]
		sourceIdxMap[idx.Name] = idx
	}

	targetIdxMap := make(map[string]*schema.Index)
	for i := range target.Indexes {
		idx := &target.Indexes[i]
		targetIdxMap[idx.Name] = idx
	}

	for name, idx := range targetIdxMap {
		if _, exists := sourceIdxMap[name]; !exists {
			changes.AddedIndexes = append(changes.AddedIndexes, *idx)
			hasChanges = true
		}
	}

	for name, idx := range sourceIdxMap {
		if _, exists := targetIdxMap[name]; !exists {
			changes.RemovedIndexes = append(changes.RemovedIndexes, *idx)
			hasChanges = true
		}
	}

	// Compare foreign keys
	sourceFKMap := make(map[string]*schema.ForeignKey)
	for i := range source.ForeignKeys {
		fk := &source.ForeignKeys[i]
		key := fk.Name
		if key == "" {
			key = strings.Join(fk.Columns, "_") + "_fk"
		}
		sourceFKMap[key] = fk
	}

	targetFKMap := make(map[string]*schema.ForeignKey)
	for i := range target.ForeignKeys {
		fk := &target.ForeignKeys[i]
		key := fk.Name
		if key == "" {
			key = strings.Join(fk.Columns, "_") + "_fk"
		}
		targetFKMap[key] = fk
	}

	for key, fk := range targetFKMap {
		if _, exists := sourceFKMap[key]; !exists {
			changes.AddedForeignKeys = append(changes.AddedForeignKeys, *fk)
			hasChanges = true
		}
	}

	for key, fk := range sourceFKMap {
		if _, exists := targetFKMap[key]; !exists {
			changes.RemovedForeignKeys = append(changes.RemovedForeignKeys, *fk)
			hasChanges = true
		}
	}

	// Compare primary keys
	if !d.samePrimaryKey(source.PrimaryKey, target.PrimaryKey) {
		changes.PrimaryKeyChanged = true
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return changes
}

func (d *Differ) compareColumn(source, target *schema.Column) *ColumnChanges {
	changes := &ColumnChanges{Name: source.Name}
	hasChanges := false

	// Type change
	if !strings.EqualFold(source.Type, target.Type) {
		changes.OldType = source.Type
		changes.NewType = target.Type
		hasChanges = true
	}

	// Nullable change
	if source.Nullable != target.Nullable {
		changes.NullableChanged = true
		changes.OldNullable = source.Nullable
		changes.NewNullable = target.Nullable
		hasChanges = true
	}

	// Default change
	if !sameDefault(source.Default, target.Default) {
		changes.DefaultChanged = true
		changes.OldDefault = source.Default
		changes.NewDefault = target.Default
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return changes
}

func (d *Differ) samePrimaryKey(source, target *schema.PrimaryKey) bool {
	if source == nil && target == nil {
		return true
	}
	if source == nil || target == nil {
		return false
	}
	if len(source.Columns) != len(target.Columns) {
		return false
	}
	for i := range source.Columns {
		if source.Columns[i] != target.Columns[i] {
			return false
		}
	}
	return true
}

func sameDefault(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(*a, *b)
}

func (d *Differ) compareStandaloneIndexes(changes *Changes) {
	sourceMap := make(map[string]*schema.Index)
	for i := range d.source.Indexes {
		idx := &d.source.Indexes[i]
		sourceMap[idx.Name] = idx
	}

	targetMap := make(map[string]*schema.Index)
	for i := range d.target.Indexes {
		idx := &d.target.Indexes[i]
		targetMap[idx.Name] = idx
	}

	for name, idx := range targetMap {
		if _, exists := sourceMap[name]; !exists {
			changes.AddedIndexes = append(changes.AddedIndexes, *idx)
		}
	}

	for name, idx := range sourceMap {
		if _, exists := targetMap[name]; !exists {
			changes.RemovedIndexes = append(changes.RemovedIndexes, *idx)
		}
	}
}

func (d *Differ) compareViews(changes *Changes) {
	sourceMap := make(map[string]*schema.View)
	for i := range d.source.Views {
		v := &d.source.Views[i]
		sourceMap[v.Name] = v
	}

	targetMap := make(map[string]*schema.View)
	for i := range d.target.Views {
		v := &d.target.Views[i]
		targetMap[v.Name] = v
	}

	for name, view := range targetMap {
		if _, exists := sourceMap[name]; !exists {
			changes.AddedViews = append(changes.AddedViews, *view)
		}
	}

	for name, view := range sourceMap {
		if _, exists := targetMap[name]; !exists {
			changes.RemovedViews = append(changes.RemovedViews, *view)
		}
	}

	for name, sourceView := range sourceMap {
		if targetView, exists := targetMap[name]; exists {
			if normalizeSQL(sourceView.Definition) != normalizeSQL(targetView.Definition) {
				changes.ModifiedViews = append(changes.ModifiedViews, ViewChanges{
					Name:          name,
					OldDefinition: sourceView.Definition,
					NewDefinition: targetView.Definition,
				})
			}
		}
	}
}

func normalizeSQL(sql string) string {
	// Normalize whitespace for comparison
	sql = strings.TrimSpace(sql)
	sql = strings.Join(strings.Fields(sql), " ")
	return strings.ToUpper(sql)
}

// IsEmpty returns true if there are no changes.
func (c *Changes) IsEmpty() bool {
	return len(c.AddedTables) == 0 &&
		len(c.RemovedTables) == 0 &&
		len(c.ModifiedTables) == 0 &&
		len(c.AddedIndexes) == 0 &&
		len(c.RemovedIndexes) == 0 &&
		len(c.AddedViews) == 0 &&
		len(c.RemovedViews) == 0 &&
		len(c.ModifiedViews) == 0
}

// WriteText writes a human-readable diff output.
func WriteText(w io.Writer, c *Changes) error {
	var sb strings.Builder

	if c.IsEmpty() {
		sb.WriteString("No differences found.\n")
		_, err := w.Write([]byte(sb.String()))
		return err
	}

	// Added tables
	if len(c.AddedTables) > 0 {
		sb.WriteString("Added Tables:\n")
		for _, t := range c.AddedTables {
			sb.WriteString(fmt.Sprintf("  + %s (%d columns)\n", t.Name, len(t.Columns)))
		}
		sb.WriteString("\n")
	}

	// Removed tables
	if len(c.RemovedTables) > 0 {
		sb.WriteString("Removed Tables:\n")
		for _, t := range c.RemovedTables {
			sb.WriteString(fmt.Sprintf("  - %s\n", t.Name))
		}
		sb.WriteString("\n")
	}

	// Modified tables
	for _, tc := range c.ModifiedTables {
		sb.WriteString(fmt.Sprintf("Modified Table: %s\n", tc.Name))
		sb.WriteString(strings.Repeat("-", 40) + "\n")

		for _, col := range tc.AddedColumns {
			sb.WriteString(fmt.Sprintf("  + Column: %s %s\n", col.Name, col.Type))
		}
		for _, col := range tc.RemovedColumns {
			sb.WriteString(fmt.Sprintf("  - Column: %s\n", col.Name))
		}
		for _, col := range tc.ModifiedColumns {
			if col.OldType != "" {
				sb.WriteString(fmt.Sprintf("  ~ Column %s: type %s → %s\n", col.Name, col.OldType, col.NewType))
			}
			if col.NullableChanged {
				nullable := func(b bool) string {
					if b {
						return "NULL"
					}
					return "NOT NULL"
				}
				sb.WriteString(fmt.Sprintf("  ~ Column %s: %s → %s\n", col.Name, nullable(col.OldNullable), nullable(col.NewNullable)))
			}
		}

		for _, idx := range tc.AddedIndexes {
			sb.WriteString(fmt.Sprintf("  + Index: %s\n", idx.Name))
		}
		for _, idx := range tc.RemovedIndexes {
			sb.WriteString(fmt.Sprintf("  - Index: %s\n", idx.Name))
		}

		for _, fk := range tc.AddedForeignKeys {
			sb.WriteString(fmt.Sprintf("  + FK: %s → %s\n", strings.Join(fk.Columns, ", "), fk.ReferencedTable))
		}
		for _, fk := range tc.RemovedForeignKeys {
			sb.WriteString(fmt.Sprintf("  - FK: %s → %s\n", strings.Join(fk.Columns, ", "), fk.ReferencedTable))
		}

		if tc.PrimaryKeyChanged {
			sb.WriteString("  ~ Primary key changed\n")
		}

		sb.WriteString("\n")
	}

	// Added indexes
	if len(c.AddedIndexes) > 0 {
		sb.WriteString("Added Indexes:\n")
		for _, idx := range c.AddedIndexes {
			sb.WriteString(fmt.Sprintf("  + %s ON %s\n", idx.Name, idx.Table))
		}
		sb.WriteString("\n")
	}

	// Removed indexes
	if len(c.RemovedIndexes) > 0 {
		sb.WriteString("Removed Indexes:\n")
		for _, idx := range c.RemovedIndexes {
			sb.WriteString(fmt.Sprintf("  - %s\n", idx.Name))
		}
		sb.WriteString("\n")
	}

	// Views
	if len(c.AddedViews) > 0 {
		sb.WriteString("Added Views:\n")
		for _, v := range c.AddedViews {
			sb.WriteString(fmt.Sprintf("  + %s\n", v.Name))
		}
		sb.WriteString("\n")
	}

	if len(c.RemovedViews) > 0 {
		sb.WriteString("Removed Views:\n")
		for _, v := range c.RemovedViews {
			sb.WriteString(fmt.Sprintf("  - %s\n", v.Name))
		}
		sb.WriteString("\n")
	}

	if len(c.ModifiedViews) > 0 {
		sb.WriteString("Modified Views:\n")
		for _, v := range c.ModifiedViews {
			sb.WriteString(fmt.Sprintf("  ~ %s (definition changed)\n", v.Name))
		}
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}

// WriteJSON writes the changes as JSON.
func WriteJSON(w io.Writer, c *Changes) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

// WriteYAML writes the changes as YAML.
func WriteYAML(w io.Writer, c *Changes) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(c)
}
