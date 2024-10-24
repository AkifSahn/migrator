# Migrator

Migrator is an automatic migration script creation tool for go.

Only works with MySQL, for now...

---

- Migrator uses GORM struct tags to parse Go struct fields into MySQL columns.
- You can check the [GORM Official Docs](https://gorm.io/docs/) to learn more about these GORM tags.

---

### What migrator can do?

Migrator can handle given scenarios and create related migration scripts for you:

- Creating and deleting `table`
- Adding, deleting and modifying `columns`
- Passing `default values` for colums
- Creating, deleting and modifying `one-to-one` or `one-to-many` `foreign key` references
- Creating, deleting and modifying `unique keys`.
- Creating, deleting `composite unique keys`.
- Creating `composite primary keys`
- Renaming `primary key`

---

TODO:
- [ ] Creating indexes
- [ ] Renaming columns
- [ ] Type checking for default values
- [ ] General error checking
