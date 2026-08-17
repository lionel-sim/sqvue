# Roadmap

Current status: early scaffolding. The CLI, driver registry, and a basic Bubble Tea TUI are in place. The Postgres driver exposes the `Driver` interface but most data/metadata operations are stubbed with `TODO`s.

## Phase 1 — Make PostgreSQL fully functional

Finish the Postgres driver so the TUI is actually usable end-to-end.

- [ ] Implement `ListTables` — query `information_schema.tables` (tables, views, materialized views)
- [ ] Implement `DescribeTable` — column metadata: name, type, nullability, default, primary key
- [ ] Implement `Rows` — paginated `SELECT ... LIMIT/OFFSET` with type-safe rendering
- [ ] Implement `Query` — arbitrary SQL with args, returning columns + rows + affected count
- [ ] Parse the connection string properly in `NewClient` (currently ignored) and support a `--host/--port/--user/--password/--db` flag group in addition to `--conn`
- [ ] Replace the hardcoded `"public"` schema list in the TUI with real schema selection
- [ ] TUI polish:
  - [ ] Column metadata view for the selected table (via `DescribeTable`)
  - [ ] Schema switcher / table filter
  - [ ] Cell truncation, wrapping, and row count in status bar
  - [ ] Error handling for per-query failures without killing the session
- [ ] Reuse `theme` package for a cohesive (and later themable) look

## Phase 2 — Broaden relational DB support

Keep the `Driver` interface as the seam; add one driver at a time behind feature flags.

- [ ] SQLite (file-based, great for local dev and testing)
- [ ] MySQL / MariaDB
- [ ] SQL Server

For each driver:

- [ ] New package under `internal/db/` registering itself via `db.Register`
- [ ] Support driver-specific DSNs and dialect quirks in metadata queries
- [ ] Extend the driver registry with a `--db-type` flag (currently hardcoded to `postgres` in `NewClient`)

## Phase 3 — General TUI capabilities

- [ ] SQL query mode — run ad-hoc queries and page through results
- [ ] Export results (CSV, JSON)
- [ ] Streaming / large-result handling (cursor or fetch-more pagination instead of LIMIT/OFFSET)
- [ ] Saved connections / connection profiles
- [ ] Vim-style and discoverable help overlay (reuse `components/keys` map)
- [ ] Test coverage for drivers (integration tests against a real Postgres via testcontainers or a local instance)
- [ ] CI: lint, build, and test matrix

## Phase 4 — Multi-DB / future

Position for non-relational databases when the time comes.

- [ ] Formalize a metadata model that covers NoSQL concepts (documents, keyspaces, collections)
- [ ] Investigate per-driver TUI panels vs. a generic table abstraction
- [ ] Pluggable authentication (IAM, SSH tunnels, client certificates)
