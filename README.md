# sqvue

[![CI](https://github.com/lionel-sim/sqvue/actions/workflows/ci.yml/badge.svg)](https://github.com/lionel-sim/sqvue/actions/workflows/ci.yml)

A terminal-based database viewer built with Go. sqvue gives you a lightweight TUI to browse schemas, tables, and row data directly from your terminal.

## Features

- Browse schemas, tables, and views with keyboard navigation; views are marked in the table list
- Filter the table list, inspect column and foreign-key metadata, and choose visible columns
- Page through row data with total-row counts and type-aware value rendering
- Focus and navigate row data while preserving the selected row across pages and column visibility changes
- Run ad-hoc SQL queries and page through their results
- Use a discoverable keyboard-help modal and compact footer controls
- Pluggable database driver abstraction with a registry pattern
- Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Cobra](https://github.com/spf13/cobra)

## Status

Work in progress. PostgreSQL, SQLite, and MySQL are supported: sqvue can connect with a DSN or individual connection flags; browse schemas, tables, and views; inspect column and foreign-key metadata and paginated row data; and run ad-hoc SQL in the TUI.

Additional database drivers, exporting, streaming large results, and CI remain planned. See [roadmap.md](roadmap.md) for the current plan.

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
| `y` / `Y`        | Copy focused cell / visible row |
| `o`              | Open the matching row in the focused cell's foreign-key table |
| `f` / `PgDn`     | Next page             |
| `b` / `PgUp`     | Previous page         |
| `d`              | Show column descriptions |
| `y`              | Show row values       |
| `c`              | Choose visible columns |
| `s`              | Switch schema         |
| `/`              | Filter table list     |
| `:`              | Run an SQL query      |
| `Enter`          | Focus displayed rows, or show selected-row details |
| `?`              | Show keyboard help    |
| `r`              | Refresh tables        |
| `Esc`            | Return from rows to table picker, or quit |
| `q` / `Ctrl+C`   | Quit                  |

Press `c` in a row or SQL-result view to open the column picker. Use `j`/`k`
to choose a column, Space to show or hide it, and Enter or Esc to return.

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
  theme/        Shared Lip Gloss UI styles
  tui/          Bubble Tea model and rendering
    components/ Reusable UI components (key bindings)
```

## License

See the repository for license details.
