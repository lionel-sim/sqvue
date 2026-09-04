# Roadmap

Current status: core Postgres driver paths are working. The CLI, driver registry, and a basic Bubble Tea TUI are in place; `ListTables`, `Rows`, and conn-string support are implemented and smoke-tested against a local Postgres.

## Phase 1 — Make PostgreSQL fully functional

Finish the Postgres driver so the TUI is actually usable end-to-end.

- [x] Implement `ListTables` — query `information_schema.tables` (tables, views)
- [x] Implement `DescribeTable` — column metadata: name, type, nullability, default, primary key
- [x] Implement `Rows` — paginated `SELECT ... LIMIT/OFFSET` with type-aware rendering
- [x] Implement `Query` — arbitrary SQL with args, returning columns + rows + affected count
- [x] Parse the connection string properly in `NewClient` (DSN pass-through via `ConnectConfig`)
- [x] Add a `--host/--port/--user/--password/--db` flag group in addition to `--conn`
- [ ] Replace the hardcoded `"public"` schema list in the TUI with real schema selection
- [ ] TUI polish:
  - [x] Table-aware cell rendering — truncate/wrap values to terminal width (or use `bubbles/table`)
  - [ ] `?` help overlay using the keymap, plus a `bubbles/help` footer key bar
  - [ ] Schema switcher — select schemas via `ListSchemas` instead of hardcoded `public`
  - [ ] Table filter — type-ahead search over the table list
  - [ ] Column metadata view for the selected table (via `DescribeTable`)
  - [ ] Ad-hoc SQL mode — input prompt to run raw SQL through `Query` and page through results
  - [ ] Inline error handling for per-query failures without killing the session
  - [ ] QoL: total row count in the status bar, table type marker (view/table), footer key bar
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

- [ ] Export results (CSV, JSON)
- [ ] Streaming / large-result handling (cursor or fetch-more pagination instead of LIMIT/OFFSET)
- [ ] Saved connections / connection profiles
- [ ] Test coverage for drivers (integration tests against a real Postgres via testcontainers or a local instance)
- [ ] CI: lint, build, and test matrix

## Phase 4 — Multi-DB / future

Position for non-relational databases when the time comes.

- [ ] Formalize a metadata model that covers NoSQL concepts (documents, keyspaces, collections)
- [ ] Investigate per-driver TUI panels vs. a generic table abstraction
- [ ] Pluggable authentication (IAM, SSH tunnels, client certificates)
