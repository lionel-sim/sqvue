# sqvue

A terminal-based database viewer built with Go. sqvue gives you a lightweight TUI to browse schemas, tables, and row data directly from your terminal.

## Features

- Browse schemas, tables, and views with keyboard navigation
- Filter the table list, inspect column metadata, and choose visible columns
- Page through row data with total-row counts and type-aware value rendering
- Run ad-hoc SQL queries and page through their results
- Use a discoverable keyboard-help modal and compact footer controls
- Pluggable database driver abstraction with a registry pattern
- Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Cobra](https://github.com/spf13/cobra)

## Status

Work in progress. PostgreSQL support is functional: sqvue can connect with a DSN or individual connection flags; browse schemas, tables, and views; inspect column metadata and paginated row data; and run ad-hoc SQL in the TUI.

Additional database drivers, exporting, streaming large results, saved connections, and CI remain planned. See [roadmap.md](roadmap.md) for the current plan.

## Requirements

- Go 1.24+
- A running Postgres database

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

### Flags

| Flag         | Default     | Description                                |
| ------------ | ----------- | ------------------------------------------ |
| `--conn`     |             | Postgres connection string                 |
| `--host`     | `localhost` | Postgres host (used when `--conn` is unset) |
| `--port`     | `5432`      | Postgres port                               |
| `--user`     | `postgres`  | Postgres user                               |
| `--password` |             | Postgres password                           |
| `--db`       |             | Postgres database                           |
| `--timeout`  | `5s`        | Query timeout                               |

### Key bindings

| Key              | Action                |
| ---------------- | --------------------- |
| `j` / `↓`        | Next table            |
| `k` / `↑`        | Previous table        |
| `f` / `PgDn`     | Next page             |
| `b` / `PgUp`     | Previous page         |
| `d`              | Show column descriptions |
| `y`              | Show row values       |
| `c`              | Choose visible columns |
| `s`              | Switch schema         |
| `/`              | Filter table list     |
| `:`              | Run an SQL query      |
| `?`              | Show keyboard help    |
| `r`              | Refresh tables        |
| `q` / `Esc` / `Ctrl+C` | Quit              |

Press `c` in a row or SQL-result view to open the column picker. Use `j`/`k`
to choose a column, Space to show or hide it, and Enter or Esc to return.

## Project layout

```
cmd/            Cobra command wiring
internal/
  config/       Connection/profile configuration and validation
  db/           Driver interface, metadata types, and driver registry
    postgres/   Postgres implementation (pgx)
  logging/      Log setup helpers
  theme/        Shared Lip Gloss UI styles
  tui/          Bubble Tea model and rendering
    components/ Reusable UI components (key bindings)
```

## License

See the repository for license details.
