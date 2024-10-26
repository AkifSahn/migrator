# Migrator

Migrator is an automatic migration script creation tool for go.

Only works with MySQL, for now...

---

- Migrator uses GORM struct tags to parse Go struct fields into MySQL columns.
- You can check the [GORM Official Docs](https://gorm.io/docs/) to learn more about these GORM tags.
- Migrator uses versioned migration database approach. You can use [golang-migrate](https://github.com/golang-migrate/migrate) to apply migrations to database


---

### What migrator can do?

Migrator can handle given scenarios and create related migration scripts for you:

> [!WARNING]
> Migrator **DELETES** unused tables, columns and constraints from database 

- Creating and deleting `table`
- Adding, deleting and modifying `columns`
- Passing `default values` for colums. [GORM docs](https://gorm.io/docs/create.html#Default-Values)
- Creating, deleting and modifying `one-to-one` or `one-to-many` `foreign key` references [One-to-one](https://gorm.io/docs/has_one.html), [One-to-many](https://gorm.io/docs/has_many.html)
- Creating, deleting and modifying `unique keys`.
- Creating, deleting `composite unique keys`. 
- Creating `composite primary keys`. [GORM docs](https://gorm.io/docs/composite_primary_key.html)
- Renaming `primary key`

TODO:
- [ ] Creating indexes
- [ ] Renaming columns
- [ ] Type checking for default values
- [ ] General error checking
 
---

### Example

- You can add this into your `main` and run it by `go run . -migrate` to create migration scripts in the given destination.
If `-dry-run ` argument is passed along with the `-migrate`, migrator prints the migration script and exits.
- To be able to create `foreign key` relations, you **must** pass _referenced_ model into `migrator.MigrateAndSave` method before the model that has foreign key
 
`main.go`
```
var migrateFlag = flag.Bool("migrate", false, "creates migration scripts")
var dryRun = flag.Bool("dry-run", false, "Print the content of migration instead of creating the file")

func main() {
	flag.Parse()

	// If migrate flag is passed, create migration scripts without starting the server
	if *migrateFlag {
		dsn := fmt.Sprintf("name:password@tcp(host:port)/DBName")
		migrator := migrator.NewMigrator(dsn, "DBName")
		defer migrator.DB.Close()

		migrator.MigrateAndSave(*dryRun, "./database/migrations", // You can pass any destination to save migration scripts
			User{},
			Company{},
			// You can add more models
			...
		)

		// Close the program after migration script created
		os.Exit(0)
	}

	// Other main codes
	...

}

```
