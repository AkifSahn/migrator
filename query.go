package migrator

import (
	"fmt"
	"github.com/AkifSahn/migrator/schema"
	"log"
	"slices"
	"strings"
)

func (m *Migrator) DropTableQuery(t *schema.Table) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", t.Name)
}

func (m *Migrator) CreateTableQuery(t *schema.Table) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", t.Name))
	for i, c := range t.Columns {
		sb.WriteString(fmt.Sprintf("\t%s %s", c.Name, c.ColumnType))

		if c.Null == "NO" {
			sb.WriteString(" NOT NULL")
		}
		if c.Extra != "" {
			sb.WriteRune(' ')
			sb.WriteString(c.Extra)
		}
		if c.DefaultValue.Valid {
			sb.WriteRune(' ')
			sb.WriteString("DEFAULT ")
			sb.WriteString(c.DefaultValue.String)
		}

		if i < len(t.Columns)-1 {
			sb.WriteString(",\n")
		}
	}

	sb.WriteString(",")
	sb.WriteString(fmt.Sprintf("\n\tPRIMARY KEY ("))
	for i, pk := range t.PrimaryCols {
		sb.WriteString(fmt.Sprintf("%s", pk))
		if i < len(t.PrimaryCols)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	for iName, cols := range t.IndexToUniqueCols {
		sb.WriteString(",")
		sb.WriteString(fmt.Sprintf("\n\tCONSTRAINT `uc.%s` UNIQUE (", iName))
		for i, col := range cols {
			sb.WriteString(fmt.Sprintf("%s", col))
			if i < len(cols)-1 {
				sb.WriteString(", ")
			}
		}
		sb.WriteRune(')')
	}

	if len(t.References) > 0 {
		sb.WriteString(",")
		for i, reference := range t.References {
			sb.WriteString(fmt.Sprintf("\n\tCONSTRAINT `fk.%s.%s` FOREIGN KEY (%s)",
				reference.TableName, reference.ColumnName, reference.ColumnName))

			sb.WriteString(fmt.Sprintf(" REFERENCES %s(%s) ON DELETE %s ON UPDATE %s",
				reference.ReferencedTableName, reference.ReferencedColumnName, reference.DeleteOption, reference.UpdateOption))

			if i < len(t.References)-1 {
				sb.WriteRune(',')
			}
		}

	}
	sb.WriteString("\n);\n")
	return sb.String()
}

func (m *Migrator) AddColumnQuery(t schema.Table, c schema.Column) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", t.Name))
	sb.WriteString(fmt.Sprintf("ADD COLUMN %s %s", c.Name, strings.ToUpper(c.ColumnType)))

	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	} else if c.Null == "NO" {
		sb.WriteString(" NOT NULL")
	}
	// Check if column has any extras. auto_increment etc.
	if c.Extra != "" {
		sb.WriteRune(' ')
		sb.WriteString(c.Extra)
	}

	if c.DefaultValue.Valid {
		sb.WriteRune(' ')
		sb.WriteString("DEFAULT ")
		sb.WriteString(c.DefaultValue.String)
	}

	if !c.PrimaryKey && c.UniqueIndex {
		sb.WriteString(fmt.Sprintf(",\n\tADD CONSTRAINT `uc.%s.%s` UNIQUE (%s)", t.Name, c.Name, c.Name))
	}

	sb.WriteString(";\n")
	return sb.String()
}

func (m *Migrator) DropColumnQuery(t schema.Table, c schema.Column) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", t.Name))

	if c.PrimaryKey {
		log.Fatalf("Cannot drop a primary key. %s.%s", t.Name, c.Name)
	}

	sb.WriteString(fmt.Sprintf("DROP COLUMN %s", c.Name))

	sb.WriteString(";\n")
	return sb.String()
}

func (m *Migrator) ModifyColumnQuery(t schema.Table, c schema.Column) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", t.Name))
	sb.WriteString(fmt.Sprintf("MODIFY COLUMN %s %s", c.Name, strings.ToUpper(c.ColumnType)))

	if c.Null == "NO" && !c.PrimaryKey {
		sb.WriteString(" NOT NULL")
	}

	if c.Extra != "" {
		sb.WriteRune(' ')
		sb.WriteString(c.Extra)
	}

	if c.DefaultValue.Valid {
		sb.WriteRune(' ')
		sb.WriteString("DEFAULT ")
		sb.WriteString(c.DefaultValue.String)
	}

	sb.WriteString(";\n")

	return sb.String()
}

func (m *Migrator) RenameColumnQuery(t schema.Table, newCol schema.Column, oldColumn schema.Column) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", t.Name))
	sb.WriteString(fmt.Sprintf("RENAME COLUMN %s TO %s;\n", oldColumn.Name, newCol.Name))
	return sb.String()
}

func (m *Migrator) AddReferenceQuery(reference schema.Reference) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", reference.TableName))
	sb.WriteString(fmt.Sprintf("ADD CONSTRAINT `fk.%s.%s` FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s ON UPDATE %s;\n",
		reference.TableName, reference.ColumnName,
		reference.ColumnName, reference.ReferencedTableName,
		reference.ReferencedColumnName, reference.DeleteOption, reference.UpdateOption))

	return sb.String()
}

func (m *Migrator) DropReferenceQuery(reference schema.Reference) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", reference.TableName))
	sb.WriteString(fmt.Sprintf("DROP CONSTRAINT `fk.%s.%s`,\n\t",
		reference.TableName, reference.ColumnName))
	sb.WriteString(fmt.Sprintf("DROP INDEX `fk.%s.%s`;\n",
		reference.TableName, reference.ColumnName))

	return sb.String()
}

func (m *Migrator) AddUniqueIndexQuery(table schema.Table, indexName string, colNames []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", table.Name))
	sb.WriteString(fmt.Sprintf("ADD CONSTRAINT `uc.%s` UNIQUE (", indexName))
	for i, col := range colNames {
		sb.WriteString(fmt.Sprintf("%s", col))
		if i < len(colNames)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(");\n")

	return sb.String()
}

func (m *Migrator) DropUniqueIndexQuery(table schema.Table, indexName string, colNames []string) string {
	var sb strings.Builder

	// If unique index is required in a foreign key, drop the foreign key first.
	// Then drop the unique constraint, then add back foreign key

	if len(colNames) > 0 {
		sb.WriteString("\n-- Removing unique constraint from a foreign key requires dropping and then adding back the foreign key!\n")
	}
	for _, colName := range colNames {
		referenceIndex := slices.IndexFunc(table.References, func(r schema.Reference) bool { return r.ColumnName == colName })
		if referenceIndex != -1 {
			sb.WriteString(m.DropReferenceQuery(table.References[referenceIndex]))
		}
	}

	sb.WriteString(fmt.Sprintf("ALTER TABLE %s\n\t", table.Name))
	sb.WriteString(fmt.Sprintf("DROP CONSTRAINT `uc.%s`;\n",
		indexName))

	for _, colName := range colNames {
		referenceIndex := slices.IndexFunc(table.References, func(r schema.Reference) bool { return r.ColumnName == colName })
		if referenceIndex != -1 {
			sb.WriteString(m.AddReferenceQuery(table.References[referenceIndex]))
		}
	}
	return sb.String()
}
