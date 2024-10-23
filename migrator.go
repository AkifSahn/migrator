package migrator

import (
	"database/sql"
	"fmt"
	"github.com/AkifSahn/migrator/schema"
	"github.com/AkifSahn/migrator/utils"
	"log"
	"os"
	"reflect"
	"slices"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type Migrator struct {
	DB             *sql.DB
	SchemaName     string
	Relations      []schema.Reference
	CurrentVersion int
}

// Returns a new migrator instance that is connected to the database by given dsn
func NewMigrator(dsn, schemaName string) *Migrator {

	// Open a connection to the database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return nil
	}

	// Check if the connection is alive
	if err := db.Ping(); err != nil {
		fmt.Println("Error pinging database:", err)
		return nil
	}

	fmt.Println("Successfully connected to the database")

	m := &Migrator{
		DB:         db,
		SchemaName: schemaName,
		Relations:  make([]schema.Reference, 0),
	}
	version, err := m.getCurrentVersion()
	if err != nil {
		fmt.Println("Error getting current database version!: ", err.Error())
		return nil
	}
	m.CurrentVersion = version
	return m
}

// Creates and saves migration script based on the given target models and current state of the database.
// If dryRun flag is true Migrator prints the migration script and exits
func (m *Migrator) MigrateAndSave(dryRun bool, saveDirectory string, targetModels ...interface{}) {
	// Desired database state
	dst := m.ParseTablesFromStructs(targetModels...)

	upScript, downScript := m.CreateMigration(dst, true)

	// Check if any migration is necessary
	if upScript == "" {
		fmt.Println("No migration necessary!")
		return
	}

	if !dryRun {
		var migrationName string

		fmt.Println("Current database version is: ", m.CurrentVersion)
		fmt.Println("Setting migration version as: ", m.CurrentVersion+1)
		fmt.Print("Enter the migration name: ")
		_, err := fmt.Scan(&migrationName)
		if err != nil {
			fmt.Println("Cannot scan given input!")
			return
		}

		migrationName = fmt.Sprintf("%d_%s", m.CurrentVersion+1, migrationName)
		fmt.Printf("Migrations are saved as: %s.[up/down].sql\n", migrationName)

		os.WriteFile(fmt.Sprintf("%s/%s.up.sql", saveDirectory, migrationName), []byte(upScript), 0644)
		os.WriteFile(fmt.Sprintf("%s/%s.down.sql", saveDirectory, migrationName), []byte(downScript), 0644)
	} else {
		fmt.Println("*****UP SCRIPT*****")
		fmt.Println(upScript)
		fmt.Println("*****DOWN SCRIPT*****")
		fmt.Println(downScript)
		fmt.Println("--dry-run argument is passed, no migration file created!")
	}

}

func (m *Migrator) getCurrentVersion() (int, error) {
	row := m.DB.QueryRow("SELECT version FROM schema_migrations")

	var version int
	if err := row.Scan(&version); err != nil {
		fmt.Println("Setting version as 0, since there is no version being stored in database!")
		return 0, nil
	}

	return version, nil

}

// Parses the current database state into 'schema.Table' struct
func (m *Migrator) GetTables() []*schema.Table {
	rows, err := m.DB.Query("SHOW TABLES")
	if err != nil {
		fmt.Println("Error querying tables: ", err)
		return nil
	}
	defer rows.Close()

	var tables []*schema.Table
	for rows.Next() {
		var table schema.Table
		if err := rows.Scan(&table.Name); err != nil {
			fmt.Println("Error scanning row: ", err)
			return nil
		}

		if table.Name == "schema_migrations" {
			continue
		}
		table.Columns = m.DescribeTable(table.Name)
		table.References = m.GetReferences(table.Name)

		// Set primaryCols table
		for _, c := range table.Columns {
			if c.PrimaryKey {
				table.PrimaryCols = append(table.PrimaryCols, c.Name)
			}
		}

		// Fill the 'table.IndexToUniqueCols'
		table.IndexToUniqueCols = m.getUniqueIndexes(table.Name)

		// Set foreign keys for columns based on reference information
		for _, r := range table.References {
			col := table.Columns[slices.IndexFunc(table.Columns, func(c *schema.Column) bool { return c.Name == r.ColumnName })]
			col.ForeignKey = true
		}

		tables = append(tables, &table)
	}

	return tables
}

// Returns the 'map[indexName] -> []ColumnName' for the current database state
func (m *Migrator) getUniqueIndexes(tableName string) map[string][]string {
	rows, err := m.DB.Query(fmt.Sprintf("SHOW INDEX FROM %s WHERE Non_unique=0 AND Key_name != 'PRIMARY'", tableName))
	if err != nil {
		fmt.Println("Error querying the indexes: ", err)
		return nil
	}
	defer rows.Close()

	var table, keyName, columnName string
	var discard interface{}

	indexToCols := make(map[string][]string)

	for rows.Next() {
		// Only scan the necessary fields, discard others
		if err := rows.Scan(&table, &discard, &keyName, &discard,
			&columnName, &discard, &discard, &discard, &discard,
			&discard, &discard, &discard, &discard, &discard,
			&discard); err != nil {
			fmt.Println("Error scanning the row: ", err)
			return nil
		}
		keyName = strings.TrimPrefix(keyName, "uc.")
		if _, exists := indexToCols[keyName]; !exists {
			indexToCols[keyName] = []string{columnName}
		} else {
			indexToCols[keyName] = append(indexToCols[keyName], columnName)
		}
	}

	return indexToCols
}

// Parses given structs into `schema.Table` struct.
// Given structs must be in order so that referenced table comes before the foreignKey table
func (m *Migrator) ParseTablesFromStructs(dst ...interface{}) []*schema.Table {
	var tables []*schema.Table
	for _, item := range dst {
		tables = append(tables, m.parseTableFromStruct(item))
	}

	return tables
}

func (m *Migrator) parseTableFromStruct(dst interface{}) *schema.Table {
	table := schema.Table{}
	val := reflect.ValueOf(dst)
	typ := reflect.TypeOf(dst)

	if typ.Kind() != reflect.Struct {
		fmt.Println("Expected a struct, but got: ", typ.Kind())
		return nil
	}

	table.Name = typ.Name()
	table.IndexToUniqueCols = make(map[string][]string)

	// iterate each field in the struct and parse them into 'schema.Column' struct
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)

		col := m.parseStructField(&table, field)
		if col != nil {
			table.Columns = append(table.Columns, col)
		}
	}

	if table.GetPrimaryKeyColumn() == nil {
		log.Fatalf("A table must have a primary key!. Table: %s\n", table.Name)
	}

	table.Name = utils.Pluralize(utils.ToMysqlName(table.Name))

	return &table
}

// Parses the given 'reflect.StructField' into 'schema.Column'
func (m *Migrator) parseStructField(table *schema.Table, field reflect.StructField) *schema.Column {
	var col schema.Column

	col.TableName = table.Name
	col.Name = field.Name

	col.Null = "YES"
	col.PrimaryKey = false
	col.ForeignKey = false
	col.UniqueIndex = false
	col.DefaultValue = sql.NullString{String: "", Valid: false}
	col.Extra = ""

	if !(strings.ToLower(col.ColumnType) == "tinyint" ||
		strings.ToLower(col.ColumnType) == "smallint" ||
		strings.ToLower(col.ColumnType) == "mediumint" ||
		strings.ToLower(col.ColumnType) == "int" ||
		strings.ToLower(col.ColumnType) == "bigint") &&
		strings.Contains(col.Extra, "auto_increment") {

		log.Fatalf("Auto increment can only be applied to integer type columns! %s.%s\n", table.Name, col.Name)
	}

	// By default, reference options are cascade
	deleteOption := schema.CASCADE_OPTION
	updateOption := schema.CASCADE_OPTION

	var fkColumnName string
	var fkTableName string
	var isFkUnique bool
	var referencedTableName string
	var referencedColumnName string

	setRelation := false

	var indexName string

	// Set default foreign key properties
	if field.Type.Kind() == reflect.Struct {
		// is this good?
		if _, err := utils.ToMysqlDataType(field.Type.String()); err != nil {
			referencedTableName = table.Name
			referencedColumnName = table.GetPrimaryKeyColumn().Name

			fkTableName = field.Type.Name()
			fkColumnName = referencedTableName + "ID"
			setRelation = true
			isFkUnique = true
		}
	}

	if field.Type.Kind() == reflect.Slice {
		referencedTableName = table.Name
		referencedColumnName = table.GetPrimaryKeyColumn().Name

		fkTableName = field.Type.Elem().Name()
		fkColumnName = referencedTableName + "ID"
		setRelation = true
		isFkUnique = false
	}

	// Parsing tag fields accordingly
	if field.Tag.Get("gorm") != "" {
		for _, v := range strings.Split(field.Tag.Get("gorm"), ";") { // split gorm fields by ';'
			if strings.Contains(v, "type") {
				col.ColumnType = strings.Split(v, ":")[1]

			} else if strings.Contains(v, "constraint") {
				for _, s := range strings.Split(v, ",") {
					if strings.Contains(s, "OnUpdate") {
						updateOption = schema.ReferenceOption(strings.ToUpper(strings.Split(v, ":")[2]))
					}
					if strings.Contains(s, "OnDelete") {
						deleteOption = schema.ReferenceOption(strings.ToUpper(strings.Split(v, ":")[2]))
					}
				}

			} else if v == "primaryKey" {
				table.PrimaryCols = append(table.PrimaryCols, utils.ToMysqlName(col.Name))
				col.PrimaryKey = true
				col.Null = "NO"

			} else if strings.Contains(v, "uniqueIndex") {
				if !col.PrimaryKey {
					indexName = fmt.Sprintf("%s.%s", utils.Pluralize(utils.ToMysqlName(table.Name)), utils.ToMysqlName(col.Name))
					if len(strings.Split(v, ":")) > 1 { // Handle custom unique index name
						indexName = fmt.Sprintf("%s.%s", utils.Pluralize(utils.ToMysqlName(col.TableName)), strings.Split(v, ":")[1])
					}
					if _, exists := table.IndexToUniqueCols[indexName]; !exists {
						table.IndexToUniqueCols[indexName] = []string{utils.ToMysqlName(col.Name)}
					} else {
						table.IndexToUniqueCols[indexName] = append(table.IndexToUniqueCols[indexName], utils.ToMysqlName(col.Name))
					}
					col.UniqueIndex = true
				}
				col.UniqueIndex = true
			} else if v == "not null" {
				col.Null = "NO"
			} else if v == "auto_increment" {
				col.Extra = strings.TrimSpace(col.Extra + " " + "auto_increment")
			} else if strings.Contains(v, "foreignKey") {
				// Override the default foreign key column
				fkColumnName = strings.Split(v, ":")[1]
			} else if v == "references" {
				// Override the default referenced column
				referencedColumnName = strings.Split(v, ":")[1]
			} else if strings.Contains(v, "default") {
				// TODO: type checking for default option
				col.DefaultValue.String = strings.Split(v, ":")[1]
				col.DefaultValue.Valid = true
			}
		}
	}

	for _, rel := range m.Relations {
		if utils.Pluralize(utils.ToMysqlName(rel.TableName)) == utils.Pluralize(utils.ToMysqlName(table.Name)) && utils.ToMysqlName(rel.ColumnName) == utils.ToMysqlName(col.Name) {
			table.References = append(table.References, rel)
			col.ForeignKey = true
			if rel.IsUnique && !col.PrimaryKey {
				if indexName == "" {
					indexName = fmt.Sprintf("%s.%s", utils.Pluralize(utils.ToMysqlName(table.Name)), utils.ToMysqlName(col.Name))
					table.IndexToUniqueCols[indexName] = []string{utils.ToMysqlName(col.Name)}
				}
				col.UniqueIndex = true
			}
			break
		}
	}

	// Mysqlize table and column names
	referencedColumnName = utils.ToMysqlName(referencedColumnName)
	fkColumnName = utils.ToMysqlName(fkColumnName)
	col.Name = utils.ToMysqlName(col.Name)

	col.TableName = utils.Pluralize(utils.ToMysqlName(col.TableName))
	referencedTableName = utils.Pluralize(utils.ToMysqlName(referencedTableName))
	fkTableName = utils.Pluralize(utils.ToMysqlName(fkTableName))

	if setRelation {
		m.newRelation(fkTableName, fkColumnName, referencedTableName, referencedColumnName, deleteOption, updateOption, isFkUnique)
	} else if col.ColumnType == "" {
		var err error
		col.ColumnType, err = utils.ToMysqlDataType(field.Type.String())
		if err != nil {
			log.Fatalf("Cannot resolve the field type to a mysql type. Table: %s, Column: %s, Type: %s", table.Name, col.Name, field.Type.String())
		}
	}

	if !setRelation {
		return &col
	}

	return nil
}

// Creates a new relation and appends to the relations list of migrator object
func (m *Migrator) newRelation(tableName, columnName, referencedTableName, referencedColumnName string, onDelete, onUpdate schema.ReferenceOption, isUnique bool) {
	m.Relations = append(m.Relations, schema.Reference{
		TableName:            tableName,
		ColumnName:           columnName,
		ReferencedTableName:  referencedTableName,
		ReferencedColumnName: referencedColumnName,
		DeleteOption:         onDelete,
		UpdateOption:         onUpdate,
		IsUnique:             isUnique,
	})
}

func (m *Migrator) GetReferences(tableName string) []schema.Reference {
	query := fmt.Sprintf(
		`SELECT rc.UPDATE_RULE, rc.DELETE_RULE, rc.TABLE_NAME, kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME
        FROM 
        INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc
        JOIN 
        INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
        ON rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
        AND rc.TABLE_NAME = kcu.TABLE_NAME
        WHERE 
        rc.CONSTRAINT_SCHEMA = '%s' AND kcu.TABLE_SCHEMA = '%s' AND rc.TABLE_NAME = '%s' AND kcu.REFERENCED_TABLE_NAME IS NOT NULL; `,
		m.SchemaName, m.SchemaName, tableName)

	rows, err := m.DB.Query(query)
	if err != nil {
		log.Fatal("Cannot get references from database!: ", err)
		return nil
	}
	defer rows.Close()

	var references []schema.Reference
	for rows.Next() {
		var reference schema.Reference
		err := rows.Scan(&reference.UpdateOption, &reference.DeleteOption, &reference.TableName, &reference.ColumnName, &reference.ReferencedTableName, &reference.ReferencedColumnName)
		if err != nil {
			log.Fatal("Error scanning the row: ", err)
			return nil
		}

		references = append(references, reference)
	}

	return references
}

func (m *Migrator) DescribeTable(tableName string) []*schema.Column {
	query := fmt.Sprintf("DESCRIBE %s", tableName)
	rows, err := m.DB.Query(query)
	if err != nil {
		fmt.Println("Error describing table: ", err)
		return nil
	}
	defer rows.Close()

	var cols []*schema.Column

	var key string
	for rows.Next() {
		var col schema.Column
		col.TableName = tableName

		err := rows.Scan(&col.Name, &col.ColumnType, &col.Null, &key, &col.DefaultValue, &col.Extra)
		if err != nil {
			fmt.Println("Error scanning the row: ", err)
			return nil
		}

		switch schema.Key(key) {
		case schema.PRIMARY_KEY:
			col.PrimaryKey = true
		case schema.UNIQUE_INDEX:
			col.UniqueIndex = true
		}

		cols = append(cols, &col)
	}

	return cols
}

// Compares the current state of the database schema with the given 'dst' schema.
// Creates and returns the migration script that will bring database to the desired state
func (m *Migrator) CreateMigration(dst []*schema.Table, verbose bool) (string, string) {
	dbTables := m.GetTables()

	var sbUp strings.Builder
	var sbDown strings.Builder

	var newTables []*schema.Table                       // List of tables to create
	var deletedTables []*schema.Table                   // List of tables to delete
	alteredTables := make(map[string]*schema.TablePair) // List of tables that requires column by column compare
	var alteredTablesOrder []string                     // Order of alteredTables map

	// Figure out new tables
	for _, modelTable := range dst {
		if !slices.ContainsFunc(dbTables, func(n *schema.Table) bool { return n.Name == modelTable.Name }) {
			newTables = append(newTables, modelTable)
		} else {
			alteredTables[modelTable.Name] = &schema.TablePair{First: nil, Second: modelTable}
			alteredTablesOrder = append(alteredTablesOrder, modelTable.Name)
		}
	}

	// Figure out deleted tables
	for _, dbTable := range dbTables {
		if !slices.ContainsFunc(dst, func(n *schema.Table) bool { return n.Name == dbTable.Name }) {
			deletedTables = append(deletedTables, dbTable)
		} else {
			alteredTables[dbTable.Name].First = dbTable
		}
	}

	for _, t := range newTables {
		sbUp.WriteString(m.CreateTableQuery(t))
	}

	// Create deleted tables in down script
	for _, t := range deletedTables {
		sbDown.WriteString(m.CreateTableQuery(t))
	}

	// Compare the tables that are not new or deleted to figure out if they are same
	for _, i := range alteredTablesOrder {
		v := alteredTables[i]
		upMigrations := v.First.CompareWith(v.Second)
		downMigrations := v.Second.CompareWith(v.First)
		schema.SortMigrationsByOperationPriority(upMigrations)
		schema.SortMigrationsByOperationPriority(downMigrations)
		m.createColumnMigrations(upMigrations, *v.First, &sbUp)
		m.createColumnMigrations(downMigrations, *v.Second, &sbDown)
	}

	// Delete created tables in down script
	for i := len(newTables) - 1; i >= 0; i-- {
		sbDown.WriteString(m.DropTableQuery(newTables[i]))
	}

	for i := len(deletedTables) - 1; i >= 0; i-- {
		sbUp.WriteString(m.DropTableQuery(deletedTables[i]))
	}

	return sbUp.String(), sbDown.String()

}

func (m *Migrator) createColumnMigrations(migrations []*schema.ColumnMigration, table schema.Table, sb *strings.Builder) {
	for _, migration := range migrations {
		switch migration.Operation {
		case schema.ADD_COLUMN:
			sb.WriteString(m.AddColumnQuery(table, migration.ApplyOn.(schema.Column)))
			break
		case schema.DROP_COLUMN:
			sb.WriteString(m.DropColumnQuery(table, migration.ApplyOn.(schema.Column)))
			break
		case schema.MODIFY_COLUMN:
			sb.WriteString(m.ModifyColumnQuery(table, migration.ApplyOn.(schema.Column)))
			break
		case schema.RENAME_COLUMN:
			sb.WriteString(m.RenameColumnQuery(table, migration.ApplyOn.(schema.Column), migration.Old.(schema.Column)))
			break
		case schema.ADD_FOREIGN_KEY:
			sb.WriteString(m.AddReferenceQuery(migration.ApplyOn.(schema.Reference)))
			break
		case schema.DROP_FOREIGN_KEY:
			sb.WriteString(m.DropReferenceQuery(migration.ApplyOn.(schema.Reference)))
			break
		case schema.UPDATE_FOREIGN_KEY:
			sb.WriteString(m.DropReferenceQuery(migration.Old.(schema.Reference)))
			sb.WriteString(m.AddReferenceQuery(migration.ApplyOn.(schema.Reference)))
			break
		case schema.ADD_UNIQUE_INDEX:
			sb.WriteString(m.AddUniqueIndexQuery(table, migration.ApplyOn.(string), migration.Old.([]string)))
			break
		case schema.DROP_UNIQUE_INDEX:
			sb.WriteString(m.DropUniqueIndexQuery(table, migration.ApplyOn.(string), migration.Old.([]string)))
			break
		}
	}
}
