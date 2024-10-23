package schema

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

type ColumnOperation int

const (
	DROP_FOREIGN_KEY ColumnOperation = iota
	DROP_UNIQUE_INDEX
	DROP_COLUMN
	RENAME_COLUMN
	MODIFY_COLUMN
	ADD_COLUMN
	UPDATE_FOREIGN_KEY
	ADD_FOREIGN_KEY
	ADD_UNIQUE_INDEX
)

type Key string

const (
	PRIMARY_KEY  Key = "PRI"
	FOREIGN_KEY  Key = "MUL"
	UNIQUE_INDEX Key = "UNI"
)

type ReferenceOption string

const (
	RESTRICT_OPTION    ReferenceOption = "RESTRICT"
	CASCADE_OPTION     ReferenceOption = "CASCADE"
	SET_NULL_OPTION    ReferenceOption = "SET NULL"
	NO_ACTION_OPTION   ReferenceOption = "NO ACTION"
	SET_DEFAULT_OPTION ReferenceOption = "SET DEFAULT"
)

type Reference struct {
	TableName            string
	ColumnName           string
	ReferencedTableName  string
	ReferencedColumnName string
	DeleteOption         ReferenceOption
	UpdateOption         ReferenceOption
	IsUnique             bool
}

func (r *Reference) PrettyPrint() {
	fmt.Printf("%s.%s -> %s.%s ON DELETE %s, ON UPDATE %s\n", r.TableName, r.ColumnName, r.ReferencedTableName, r.ReferencedColumnName, r.DeleteOption, r.UpdateOption)
}

func NewReference(tableName string, columnName string, referencedTableName string,
	referencedColumnName string, deleteOption ReferenceOption, updateOption ReferenceOption) *Reference {
	return &Reference{
		TableName:            tableName,
		ColumnName:           columnName,
		ReferencedTableName:  referencedTableName,
		ReferencedColumnName: referencedColumnName,
		DeleteOption:         deleteOption,
		UpdateOption:         updateOption,
	}
}

type Table struct {
	Name              string
	Columns           []*Column
	References        []Reference
	IndexToUniqueCols map[string][]string // index name maps to list of column names
	PrimaryCols       []string
}

type TablePair struct {
	First  *Table
	Second *Table
}

type Column struct {
	TableName    string
	Name         string
	ColumnType   string
	Null         string
	PrimaryKey   bool
	ForeignKey   bool
	UniqueIndex  bool
	DefaultValue sql.NullString
	Extra        string
}

type ColumnMigration struct {
	ApplyOn   interface{}
	Old       interface{}
	Operation ColumnOperation
}

func NewColumnMigration(applyOn interface{}, old interface{}, operation ColumnOperation) *ColumnMigration {
	return &ColumnMigration{
		ApplyOn:   applyOn,
		Operation: operation,
		Old:       old,
	}
}

func (t *Table) PrettyPrint() {
	fmt.Printf("\n--- %s ---\n\n", t.Name)
	for _, col := range t.Columns {
		col.PrettyPrint()
	}
	if len(t.References) > 0 {
		fmt.Println("\nREFERENCES:")
		for _, reference := range t.References {
			reference.PrettyPrint()
		}
	}
}

// maps the old name to the new name of the renamed column
var renamedColumns = make(map[string]string)

// Compares the caller table with the given dst table
// and creates migrations to change caller table into given 'dst' table
func (t *Table) CompareWith(dst *Table) []*ColumnMigration {
	var migrations []*ColumnMigration

	var droppedColumns []string
	clear(renamedColumns)
	// Check for dropped columns
	for _, col := range t.Columns {
		if contains, _ := dst.HasColumn(col); !contains {
			if col.PrimaryKey {
				migrations = append(migrations, NewColumnMigration(*dst.GetPrimaryKeyColumn(), *col, RENAME_COLUMN))
				renamedColumns[col.Name] = dst.GetPrimaryKeyColumn().Name
				continue
			} else {
				migrations = append(migrations, NewColumnMigration(*col, nil, DROP_COLUMN))
				droppedColumns = append(droppedColumns, col.Name)
				continue
			}
		}
	}

	// Check for new columns
	for _, col := range dst.Columns {
		if contains, i := t.HasColumn(col); !contains { // dst has this column but method caller not. So ADD_COLUMN
			if !col.PrimaryKey {
				migrations = append(migrations, NewColumnMigration(*col, nil, ADD_COLUMN))
				continue
			}
		} else { // Both have this column, check if column is modified by any means. We need to check each column property. If all is same, skip
			if !col.Equals(*t.Columns[i]) { // Columns are not equal
				migrations = append(migrations, NewColumnMigration(*col, *t.Columns[i], MODIFY_COLUMN))
				continue
			}
		}
	}
	var droppedIndexes, createdIndexes []string
	for uIndex, uCols := range dst.IndexToUniqueCols {
		if slices.Contains(createdIndexes, uIndex) {
			continue
		}
		if t.CompareUniqueIndex(uIndex, uCols) == 1 { // t is missing unique index. Add it
			migrations = append(migrations, NewColumnMigration(uIndex, uCols, ADD_UNIQUE_INDEX))
			createdIndexes = append(createdIndexes, uIndex)
		}
	}

	for uIndex, uCols := range t.IndexToUniqueCols {
		if slices.Contains(droppedIndexes, uIndex) {
			continue
		}
		if dst.CompareUniqueIndex(uIndex, uCols) == 1 { // Dst is missing unique index. Drop it
			migrations = append(migrations, NewColumnMigration(uIndex, uCols, DROP_UNIQUE_INDEX))
			droppedIndexes = append(droppedIndexes, uIndex)
		}
	}

	// Check dropped foreign key
	for _, r1 := range t.References {
		// If everything except reference options are same. We don't delete or add new constraint just update the constraint option
		if !slices.ContainsFunc(dst.References, func(r2 Reference) bool {
			return (r1.TableName == r2.TableName &&
				r1.ColumnName == r2.ColumnName &&
				r1.ReferencedTableName == r2.ReferencedTableName &&
				r1.ReferencedColumnName == r2.ReferencedColumnName)
		}) {
			migrations = append(migrations, NewColumnMigration(r1, nil, DROP_FOREIGN_KEY))
		}
	}

	// Check new foreign keys
	for _, r1 := range dst.References {
		// Check if reference from dst exists in t
		if index := slices.IndexFunc(t.References, func(r2 Reference) bool {
			return (r1.TableName == r2.TableName &&
				r1.ColumnName == r2.ColumnName &&
				r1.ReferencedTableName == r2.ReferencedTableName &&
				r1.ReferencedColumnName == r2.ReferencedColumnName)
		}); index == -1 {
			migrations = append(migrations, NewColumnMigration(r1, nil, ADD_FOREIGN_KEY))
		} else if t.References[index].UpdateOption != r1.UpdateOption || t.References[index].DeleteOption != r1.DeleteOption {
			migrations = append(migrations, NewColumnMigration(r1, t.References[index], UPDATE_FOREIGN_KEY))
		}
	}
	return migrations
}

// TODO: handle composite primary keys
// Returns primaryKey column. If not found returns nil
func (t *Table) GetPrimaryKeyColumn() *Column {
	for _, c := range t.Columns {
		if c.PrimaryKey {
			return c
		}
	}
	return nil
}

// Checks columns existance by name, returns its index if exists.
// Returns -1 if column not found
func (t *Table) HasColumn(col *Column) (contains bool, index int) {
	for i, c := range t.Columns {
		if ((c.Name == col.Name) || (renamedColumns[c.Name] == col.Name)) && (c.PrimaryKey == col.PrimaryKey) {
			return true, i
		}
	}
	return false, -1
}

func (c *Column) PrettyPrint() {
	if !c.PrimaryKey {
		if c.UniqueIndex {
			fmt.Printf("\nfield: %s\ntype: %s \nNull: %s\nKey: %s\n", c.Name, c.ColumnType, c.Null, UNIQUE_INDEX)
		} else if c.ForeignKey {
			fmt.Printf("\nfield: %s\ntype: %s \nNull:%s\nKey: %s\n", c.Name, c.ColumnType, c.Null, FOREIGN_KEY)
		} else {
			fmt.Printf("\nfield: %s\ntype: %s \nNull:%s\n", c.Name, c.ColumnType, c.Null)
		}

	} else {
		fmt.Printf("\nfield: %s\ntype: %s \nNull: %s\nKey: %s\n", c.Name, c.ColumnType, c.Null, PRIMARY_KEY)
	}

	if c.Extra != "" {
		fmt.Printf("%s\n", c.Extra)
	}
	if c.DefaultValue.Valid {
		fmt.Println("Default: ", c.DefaultValue)
	}
}

// returns true if columns are same, false if not
func (c *Column) Equals(col Column) bool {
	if strings.ToLower(c.Name) != strings.ToLower(col.Name) ||
		strings.ToLower(c.ColumnType) != strings.ToLower(col.ColumnType) ||
		strings.ToLower(c.Null) != strings.ToLower(col.Null) ||
		strings.ToLower(c.Extra) != strings.ToLower(col.Extra) ||
		(c.DefaultValue.Valid != col.DefaultValue.Valid && c.DefaultValue.String != col.DefaultValue.String) {
		return false
	}
	return true
}

// 0  -> same \r
// 1  -> t does not have the index or some columns are missing. Indexes are not same
func (t *Table) CompareUniqueIndex(indexName string, colsCompare []string) int {
	cols, exists := t.IndexToUniqueCols[indexName]
	if !exists || !slices.Equal(cols, colsCompare) {
		return 1
	}
	return 0
}

// 0  -> same primary.
// 1  -> c is not primary, col is primary.
// -1 -> c is primary, col is not primary.
func (c *Column) ComparePrimary(col Column) int {
	if c.PrimaryKey == col.PrimaryKey {
		return 0
	}
	if !c.PrimaryKey && col.PrimaryKey {
		return 1
	}
	return -1
}

// This function sorts the given column migrations list by operation priority.
// The priority of the operation is determined by it's value, smaller value means higher priority
func SortMigrationsByOperationPriority(migrations []*ColumnMigration) {
	slices.SortStableFunc(migrations, func(a *ColumnMigration, b *ColumnMigration) int {
		return int(a.Operation) - int(b.Operation)
	})
}
