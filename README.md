# sqvue

A terminal-based database viewer built with Go. sqvue gives you a lightweight TUI to browse schemas, tables, and row data directly from your terminal.

## Features

- Navigate tables and page through row data with keyboard shortcuts
- Pluggable database driver abstraction with a registry pattern
- Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Cobra](https://github.com/spf13/cobra)

## Status

Work in progress. The Postgres driver scaffolding is in place, but data/metadata operations (`ListTables`, `DescribeTable`, `Query`, `Rows`) are not yet implemented.

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

| Flag          | Default | Description                     |
| ------------- | ------- | ------------------------------- |
| `--conn`      |         | Postgres connection string      |
| `--timeout`   | `5s`    | Query timeout                   |

### Key bindings

| Key              | Action           |
| ---------------- | ---------------- |
| `j` / `↓`        | Next table       |
| `k` / `↑`        | Previous table   |
| `f` / `PgDn`     | Next page        |
| `b` / `PgUp`     | Previous page    |
| `r`              | Refresh tables   |
| `q` / `Esc` / `Ctrl+C` | Quit         |

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