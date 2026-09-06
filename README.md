# sqvue

[![CI](https://github.com/lionel-sim/sqvue/actions/workflows/ci.yml/badge.svg)](https://github.com/lionel-sim/sqvue/actions/workflows/ci.yml)

A terminal-based database viewer built with Go. sqvue gives you a lightweight TUI to browse schemas, tables, and row data directly from your terminal.

## Features

- Browse schemas, tables, and views with keyboard navigation; views are marked in the table list
- Filter the table list and focused rows with parameterized comparisons; inspect column and foreign-key metadata; and choose visible columns
- Page through row data with total-row counts, type-aware value rendering, and incremental loading for large results
- Focus and navigate row data while preserving the selected row across pages and column visibility changes
- Sort table and query results by the active column, with the selected order retained for filtering, paging, streams, and exports
- Insert one row at a time through a guided table form, using database defaults, explicit `NULL`, literal values, or a database-side current timestamp
- Delete one selected primary-key table row through an explicit confirmation
- Run ad-hoc SQL queries, recall profile-specific history, save named queries, and page through results
- Switch among configured connection profiles without exposing credentials
- Export visible result columns to CSV or JSON
- Back up PostgreSQL, SQLite, and MySQL databases or the currently selected table
- Use a discoverable keyboard-help modal and compact footer controls
- Pluggable database driver abstraction with a registry pattern
- Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Cobra](https://github.com/spf13/cobra)

## Status

Work in progress. PostgreSQL, SQLite, and MySQL are supported: sqvue can connect with a DSN or individual connection flags; browse schemas, tables, and views; inspect column and foreign-key metadata and paginated row data; run and save ad-hoc SQL in the TUI; and create database backups.

Large table and query results stream incrementally, including complete exports
without retaining every row in TUI memory. See [roadmap.md](roadmap.md) for the
current plan.

## Requirements

- Go 1.25+
- A running Postgres or MySQL database, or a SQLite database file

## Installation

```sh
make build
```

This produces a binary at `bin/sqvue`. To clean up build artifacts:

```sh
make clean
```

## Usage

Provide a connection string via the `--conn` flag or the `DATABASE_URL` environment variable:

```sh
DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable" ./bin/sqvue

# or
./bin/sqvue --conn "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
```

You can also run directly from source with `make run` or `go run ./...`.

Open a SQLite database directly with its path or URI:

```sh
./bin/sqvue --db-type sqlite --conn ./sqvue_demo.db
```

Connect to MySQL with its standard DSN format:

```sh
./bin/sqvue --db-type mysql --conn "user:password@tcp(localhost:3306)/my_database?parseTime=true"
```

### Connection profiles

On its first run, sqvue creates a commented configuration template. The file is
located at `$XDG_CONFIG_HOME/sqvue/config.toml` (or `~/.config/sqvue/config.toml`)
on Linux, and `~/Library/Application Support/sqvue/config.toml` on macOS.

Define named connections under `connections` and set `settings.default_profile`
to select one automatically:

```toml
[settings]
default_profile = "local"
export_directory = "./exports"
theme = "default"

[connections.local]
db_type = "postgres"
host = "localhost"
port = 5432
user = "postgres"
database = "my_database"
sslmode = "disable"

[connections.local_sqlite]
db_type = "sqlite"
conn = "./sqvue_demo.db"

[connections.local_mysql]
db_type = "mysql"
conn = "user:password@tcp(localhost:3306)/my_database?parseTime=true"
```

Select another profile for one launch with `--profile work`. A Postgres profile
may use `conn = "postgres://..."` or the individual connection fields; SQLite
uses its database path or URI in `conn`; and MySQL uses its standard DSN in
`conn`. `--conn` wins over every other source. Without an explicit `--profile`,
`DATABASE_URL` takes precedence over the configured default Postgres profile.
Explicitly supplied connection flags override matching profile fields. Avoid
storing passwords in the config file—use a connection URL, environment variable,
or Postgres password file instead.

### Query history and saved queries

sqvue stores SQL history and named queries in `queries.toml` beside `config.toml`.
History and saved queries are isolated by connection profile (or the `default`
profile when no name is selected). SQL is preserved verbatim, including multiple
lines. In the SQL editor, use `Ctrl+P`/`Ctrl+N` to browse history; use plain
Up/Down to move between editor lines. Use `Ctrl+S` to save the current query and
`Ctrl+O` to open saved queries. The saved-query picker
uses Enter to run, `r` to rename, and `d` to open a deletion confirmation
(then Enter confirms it).

The SQL editor accepts multiple lines: Enter adds a line, while `Ctrl+R` runs the
query. `Ctrl+F` reformats the editor contents without running them, and `Ctrl+E`
opens an `EXPLAIN` result for one `SELECT` statement only; it refuses statements
that could mutate data. Database errors that report a character position are shown
with their line and column, and the editor stays open with the query intact.

`settings.export_directory` sets the directory prefilled when exporting CSV, JSON, or
creating a backup.
It defaults to sqvue's current working directory; relative paths are resolved
from that directory. Prompts let you change the final path, and sqvue will not
overwrite an existing file. Table exports include every row matching the active
filters. `SELECT` results stream incrementally; their exports rerun the active
query from the beginning so they include rows beyond the currently displayed
page. Other SQL results remain materialized to avoid replaying mutations.
Press `e` for CSV or `E` for JSON. JSON exports are arrays of objects whose
keys are the visible column names; duplicate labels receive stable numeric
suffixes.

### Themes

Set `settings.theme` to one of `default`, `light`, or `high-contrast` in the
config file. The default preserves sqvue's original dark ANSI-256 appearance.
Themes apply when sqvue starts; an unknown theme name is reported as a
configuration error.

### Backups

Press `B` to choose either the entire database or the currently selected table,
then confirm the suggested path or enter another one. Backups run in the
background and sqvue reports progress or an error in the footer. A destination
that already exists is never overwritten.

PostgreSQL backups are plain `.sql` dumps produced by `pg_dump`; install the
PostgreSQL client tools and ensure `pg_dump` is available on `PATH`. Restore a
full dump with `psql`. MySQL backups are plain `.sql` dumps produced by
`mysqldump`; install MySQL client tools, ensure `mysqldump` is available on
`PATH`, and restore a full dump with `mysql`. SQLite backups are standalone
`.db` files created with SQLite-native operations. A SQLite table backup
preserves that table's rows, schema, indexes, triggers, and constraints, but
does not include other tables referenced by foreign keys.

### Flags

| Flag         | Default     | Description                                |
| ------------ | ----------- | ------------------------------------------ |
| `--db-type`  | `postgres`  | Database type (`postgres`, `sqlite`, `mysql`) |
| `--conn`     |             | Connection string or SQLite database path   |
| `--profile`  |             | Named connection profile from the config file |
| `--host`     | `localhost` | Postgres host (used when `--conn` is unset) |
| `--port`     | `5432`      | Postgres port                               |
| `--user`     | `postgres`  | Postgres user                               |
| `--password` |             | Postgres password                           |
| `--db`       |             | Postgres database                           |
| `--sslmode`  | `require`   | Postgres TLS mode for individual flags      |
| `--timeout`  | `5s`        | Connection and query timeout                |

### Key bindings

| Key              | Action                |
| ---------------- | --------------------- |
| `j` / `↓`        | Next table or active row |
| `k` / `↑`        | Previous table or active row |
| `Ctrl+D` / `Ctrl+U` | Half page down / up in focused rows |
| `g` / `G`        | First / last focused row |
| `h` / `←` and `l` / `→` | Previous / next cell in focused rows |
| `count` + `j` / `k` / `h` / `l` | Repeat focused-grid movement (for example, `23j`) |
| `y` / `Y`        | Copy focused cell / visible row (focused grid) |
| `o`              | Open the matching row in the focused cell's foreign-key table as a clearable equality filter |
| `/`              | Add a focused-row filter for the active column (contains is literal and case-insensitive; choose LIKE, `>`, `<`, `>=`, `<=`, or null checks; PostgreSQL also offers ILIKE; null checks apply immediately; filters use AND) |
| `x`              | Clear focused-row filters |
| `S`              | Sort the focused active column (ascending, descending, then clear) |
| `f` / `PgDn`     | Next page             |
| `b` / `PgUp`     | Previous page         |
| `d`              | Show column descriptions (table picker) |
| `y`              | Show rows (table picker) |
| `c`              | Choose visible columns |
| `e`              | Edit the active cell in a focused table grid (primary-key tables only); otherwise export visible columns as CSV |
| `E`              | Export visible columns as JSON |
| `B`              | Back up the database or current table |
| `s`              | Switch schema         |
| `P`              | Switch configured connection profile |
| `/`              | Filter table list     |
| `:`              | Run an SQL query      |
| `Enter` (in SQL editor) | Insert a new line |
| `Ctrl+R` (in SQL editor) | Run the editor contents |
| `Ctrl+F` (in SQL editor) | Format the editor contents without running it |
| `Ctrl+E` (in SQL editor) | Show an `EXPLAIN` plan for one `SELECT` query |
| `Ctrl+S` (in SQL prompt) | Save the current named query |
| `Ctrl+O` (in SQL prompt) | List saved queries (Enter runs; `r` renames; `d` opens deletion confirmation) |
| `Ctrl+N` / `Ctrl+P` (in SQL editor) | Next / previous query history entry |
| `↓` / `↑` (in SQL editor) | Move cursor to the next / previous editor line |
| `Enter`          | Focus displayed rows, or show selected-row details (focused grid) |
| `a` (focused table grid) | Add a row |
| `d` (focused table grid) | Delete the active row |
| `?`              | Show keyboard help    |
| `r`              | Refresh the current table view, or rerun active SQL results |
| `Esc`            | Return from rows to table picker, or close an overlay |
| `q` / `Ctrl+C`   | Quit                  |

Press `c` in a row or SQL-result view to open the column picker. Use `j`/`k`
to choose a column, Space to show or hide it, and Enter or Esc to return.
Table column choices are retained when switching away and back during the
current connection.

Press `e` in a focused table grid to edit the active cell. Editing is available
only for tables with declared primary keys (including composite keys), and SQL
result grids stay read-only. Review the old and new values before confirming;
enter `NULL` to set a database `NULL` value. sqvue refreshes the current page
after a successful update.

Press `d` in a focused base-table grid to delete the active row. Deletion is
available only for tables with declared primary keys (including composite keys)
and never for views or SQL results. Review the table and primary-key values in
the confirmation screen; Enter permanently deletes the row, while Esc cancels.
Errors return to that confirmation without discarding the selected row. sqvue
refreshes the current view and row count after success, moving to the previous
page if the deletion leaves the current page empty.

Press `a` in a focused base-table grid to add one row. The form starts each
editable field at **use database default**; generated columns are not written.
Use `j`/`k` to select a field, Enter to enter a literal value, `d` to restore
the database default, `n` to set a database `NULL`, or `t` to set `NOW` for a
date/time field. A typed `NULL` remains the literal text `NULL`. `Ctrl+R`
reviews the complete row, and Enter on the confirmation screen inserts it.
Database validation errors return to the form without discarding values. After
success, sqvue refreshes the current view and warns if active filters may hide
the new row.

Press `S` in a focused row or SQL-result grid to sort the active column. Press
it again for descending order, and a third time to restore the driver's normal
table order or the original query order. The footer status shows the active
sort; table filters, incremental streams, and CSV/JSON exports retain it.

Press `P` from the table picker to choose a configured connection profile. sqvue
only displays profile names, keeps the current connection when reconnection
fails, and cancels outstanding loads and streams before loading the new schema.

## Testing

Most tests run without external services. The PostgreSQL and MySQL integration
tests are skipped unless their DSNs are set; each creates and removes isolated
test data in the target database:

```sh
SQVUE_TEST_POSTGRES_DSN="postgres://sqvue:sqvue@localhost:5432/sqvue?sslmode=disable" \
  go test ./internal/db/postgres -run '^TestPostgresDriverIntegration$'

SQVUE_TEST_MYSQL_DSN="sqvue:sqvue@tcp(127.0.0.1:3306)/sqvue?parseTime=true" \
  go test ./internal/db/mysql -run '^TestMySQLDriverIntegration$'
```

## Project layout

```
cmd/            Cobra command wiring
internal/
  config/       Connection/profile configuration and validation
  db/           Driver interface, metadata types, and driver registry
    postgres/   Postgres implementation (pgx)
    sqlite/     SQLite implementation (pure Go)
    mysql/      MySQL implementation
  logging/      Log setup helpers
  theme/        Built-in Lip Gloss theme presets
  tui/          Bubble Tea model, key bindings, and rendering
```

## License

See the repository for license details.
