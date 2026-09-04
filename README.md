# sqvue

A terminal-based database viewer built with Go. sqvue gives you a lightweight TUI to browse schemas, tables, and row data directly from your terminal.

## Features

- Navigate tables and page through row data with keyboard shortcuts
- Pluggable database driver abstraction with a registry pattern
- Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Cobra](https://github.com/spf13/cobra)

## Status

Work in progress. PostgreSQL support is functional: sqvue can list tables and views in the `public` schema, browse paginated row data, and display column metadata. The driver also supports arbitrary SQL queries through its internal API.

Schema selection, SQL mode in the TUI, alternate connection flags, and additional database drivers are still planned. See [roadmap.md](roadmap.md) for the current plan.

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
| `s`              | Switch schema         |
| `/`              | Filter table list     |
| `r`              | Refresh tables        |
| `q` / `Esc` / `Ctrl+C` | Quit              |

## Project layout

```
cmd/            Cobra command wiring
internal/
  config/       Connection/profile configuration and validation
  db/           Driver interface, metadata types, and driver registry
    postgres/   Postgres implementation (pgx)
  logging/      Log setup helpers
  theme/        UI theme (placeholder)
  tui/          Bubble Tea model and rendering
    components/ Reusable UI components (key bindings)
```

## License

See the repository for license details.
