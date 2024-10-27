package schema_test

import (
	"testing"

	"github.com/AkifSahn/migrator/schema"
	"github.com/stretchr/testify/assert"
)

func TestGetPrimaryKeyColumn(t *testing.T) {
	assert := assert.New(t)

	table := schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "name", ColumnType: "varchar(255)", PrimaryKey: false},
		},
	}

	expected := &schema.Column{
		Name:       "id",
		ColumnType: "int",
		PrimaryKey: true,
	}

	result := table.GetPrimaryKeyColumn()
	assert.Equal(expected, result, "name")
}

func TestCompareWith_AddColumn(t *testing.T) {
	assert := assert.New(t)

	table := &schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
		},
	}

	dstTable := &schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "name", ColumnType: "varchar(255)"},
		},
	}

	migrations := table.CompareWith(dstTable)

	assert.Len(migrations, 1, "Num of migrations")
	assert.Equal(schema.ADD_COLUMN, migrations[0].Operation, "Migration type")
	assert.Equal("name", migrations[0].ApplyOn.(schema.Column).Name, "New column")
}

func TestCompareWith_DropColumn(t *testing.T) {
	assert := assert.New(t)

	table := &schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "name", ColumnType: "varchar(255)"},
		},
	}

	dstTable := &schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
		},
	}

	migrations := table.CompareWith(dstTable)

	assert.Len(migrations, 1, "Num of migrations")
	assert.Equal(schema.DROP_COLUMN, migrations[0].Operation, "Migration type")
	assert.Equal("name", migrations[0].ApplyOn.(schema.Column).Name, "Dropped column")
}

func TestCompareWith_ModifyColumn(t *testing.T) {
	assert := assert.New(t)

	table := &schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "age", ColumnType: "varchar(255)"},
			{Name: "name", ColumnType: "varchar(255)", Null: "YES"},
			{Name: "weight", ColumnType: "int"},
		},
	}

	dstTable := &schema.Table{
		Name: "users",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "age", ColumnType: "int"},                            // Modify type
			{Name: "name", ColumnType: "varchar(255)", Null: "NO"},      // Modify NULL
			{Name: "weight", ColumnType: "int", Extra: "autoIncrement"}, // Modify Extra
		},
	}

	migrations := table.CompareWith(dstTable)

	assert.Len(migrations, 3, "Num of migrations")
	for i, migration := range migrations {
		assert.Equal(schema.MODIFY_COLUMN, migration.Operation, "Migration type")
		assert.Equal(*dstTable.Columns[i+1], migration.ApplyOn.(schema.Column), "Modified column")
	}
}

func TestCompareWith_AddForeignKey(t *testing.T) {
	// original table with no foreign keys
	table := &schema.Table{
		Name: "orders",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "user_id", ColumnType: "int"},
		},
	}

	// Arrange: destination table with a foreign key
	dstTable := &schema.Table{
		Name: "orders",
		Columns: []*schema.Column{
			{Name: "id", ColumnType: "int", PrimaryKey: true},
			{Name: "user_id", ColumnType: "int"},
		},
		References: []schema.Reference{
			{
				TableName:            "orders",
				ColumnName:           "user_id",
				ReferencedTableName:  "users",
				ReferencedColumnName: "id",
				DeleteOption:         schema.CASCADE_OPTION,
			},
		},
	}

	// Act
	migrations := table.CompareWith(dstTable)

	// Assert: Check that a migration to add foreign key is created
	assert.Len(t, migrations, 1)
	assert.Equal(t, schema.ADD_FOREIGN_KEY, migrations[0].Operation)
	assert.Equal(t, "user_id", migrations[0].ApplyOn.(schema.Reference).ColumnName)
}
