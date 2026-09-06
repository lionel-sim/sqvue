# Roadmap

Current status: Phase 1 is complete and Phase 2 is underway. sqvue supports PostgreSQL, SQLite, and MySQL, including schema and table browsing, paginated row and column views, row counts, and ad-hoc SQL queries in the Bubble Tea TUI.

## Phase 1 — Make PostgreSQL fully functional

Finish the Postgres driver so the TUI is actually usable end-to-end.

- [x] Implement `ListTables` — query `information_schema.tables` (tables, views)
- [x] Implement `DescribeTable` — column metadata: name, type, nullability, default, primary key, foreign-key target
- [x] Implement `Rows` — paginated `SELECT ... LIMIT/OFFSET` with type-aware rendering
- [x] Implement `Query` — arbitrary SQL with args, returning columns + rows + affected count
- [x] Parse the connection string properly in `NewClient` (DSN pass-through via `ConnectConfig`)
- [x] Add a `--host/--port/--user/--password/--db` flag group in addition to `--conn`
- [x] Use secure TLS defaults and the configured timeout for flagged connections
- [x] Replace the hardcoded `"public"` schema list in the TUI with real schema selection
- [x] TUI polish:
  - [x] Table-aware cell rendering — truncate/wrap values to terminal width (or use `bubbles/table`)
  - [x] `?` help overlay using the keymap, plus a `bubbles/help` footer key bar
  - [x] Schema switcher — select schemas via `ListSchemas` instead of hardcoded `public`
  - [x] Compact table selector — show the selected schema once in the header and mark views distinctly
  - [x] Table filter — type-ahead search over the table list
  - [x] Column metadata view for the selected table (via `DescribeTable`)
  - [x] Foreign-key indicators in table headers and referenced targets in the metadata view
  - [x] Ad-hoc SQL mode — input prompt to run raw SQL through `Query` and page through results
  - [x] Inline error handling for per-query failures without killing the session
  - [x] QoL: total row count in the status bar, table type marker (view/table), footer key bar
- [x] Reuse `theme` package for a cohesive (and later themable) look

## Phase 2 — Broaden relational DB support

Keep the `Driver` interface as the seam; add one driver at a time behind feature flags.

- [x] SQLite (file-based, great for local dev and testing)
- [x] MySQL

For each driver:

- [x] New package under `internal/db/` registering itself via `db.Register`
- [x] Support driver-specific DSNs and dialect quirks in metadata queries
- [x] Extend the driver registry with a `--db-type` flag

## Phase 3 — General TUI capabilities

- [x] Column visibility picker for row and SQL-result views
- [x] Data-grid focus mode
  - [x] Press Enter to move focus between the table picker and data grid; Esc returns to the picker
  - [x] Highlight the active row and show the current focus state in the footer
  - [x] Navigate rows with `j`/`k`, arrow keys, `Ctrl-d`/`Ctrl-u`, and `g`/`G`
  - [x] Keep the selected row visible while scrolling and when visible columns change
  - [x] Navigate cells left and right, highlighting the active cell
  - [x] Vim-style count prefixes for `j`/`k`/`h`/`l` movement
  - [x] Open a row-detail popup with full, untruncated values
  - [x] Copy the active cell (`y`) or whole row (`Y`)
  - [x] Follow a foreign-key cell to its matching referenced row when its table is available in the current schema
- [x] Structured row filtering (parameterized browse filters, separate from SQL mode)
  - [x] Define driver-neutral filter and browse-request types for table rows
  - [x] Replace the specialised foreign-key lookup with the shared filtered-row path
  - [x] Add a grid filter prompt for the active column, prefilled from its selected cell
  - [x] Support equality filtering first, including filtered row counts and pagination
  - [x] Show active filters in the status bar and add a clear-filter action
  - [x] Add `contains`, comparison (`>`, `<`, `>=`, `<=`), `is null`, and `is not null` operators
  - [x] Support multiple filters combined with `AND`
  - [x] Test TUI filter state and each driver's parameterized query generation
- [x] TUI maintainability refactor
  - [x] Extract `Model`, state groups, construction, and initialization into `model.go`
  - [x] Keep `ui.go` as the small Bubble Tea dispatcher and generic overlay handlers
  - [x] Move focused-grid input, count prefixes, cell/row navigation, copying, and FK traversal into `update_grid.go`
  - [x] Move row-filter prompts, operator selection, filter application, and clearing into `update_filters.go`
  - [x] Move schema/table selection, table-list filtering, and mode switching into `update_browser.go`
  - [x] Move browse-request construction, pagination, row/count result handling, and status formatting into `browse.go`
  - [x] Move shared view-state helpers (`currentTable`, visible-column bookkeeping, request IDs, and failures) to the closest owning file
  - [x] Keep rendering files independent of key handling and database loading
  - [x] Run the full test suite after every extraction; preserve behavior and public key bindings
- [x] Export results as CSV
  - [x] Export the current table or SQL-result view, respecting visible columns, active filters, and result ordering
  - [x] Offer a save-path prompt, write RFC 4180-compatible CSV, and surface success or write errors in the TUI
- [ ] Export results as JSON
- [ ] Streaming / large-result handling (cursor or fetch-more pagination instead of LIMIT/OFFSET)
- [x] Saved connections / connection profiles (XDG config file, named profiles, and `--profile`)
- [ ] Configure themes through the config file
- [x] Test coverage for drivers (integration tests against real Postgres and MySQL instances, plus SQLite temporary databases)
- [x] CI: lint, build, and test matrix (GitHub Actions on Linux, macOS, and Windows)
- [ ] Generate database backups
  - [x] Define a driver-specific backup interface so backups preserve database schema, data, indexes, and constraints without putting database-specific behavior in `internal/db`
  - [x] Add a `B` key binding and keyboard-help entry that opens a backup-scope picker
  - [x] Offer `Entire database` (the default) and `Current table`; add `Current schema` only for drivers that support schemas
  - [x] Follow the scope picker with a save-path prompt, provide timestamped driver-appropriate filenames, refuse to overwrite existing files, and report asynchronous progress, success, and errors in the TUI
  - [x] Implement PostgreSQL full-database and table backups with `pg_dump`, including a clear preflight error when the command is unavailable
  - [x] Implement SQLite full-database and table backups using SQLite-native mechanisms; document any intentional limits of table-level backups
  - [ ] Add focused unit tests for picker state, path validation, command construction, and completion/error handling
  - [ ] Document backup behavior, required external tools, output formats, and key bindings in the README
  - [ ] Add MySQL full-database and table backups with `mysqldump` after the PostgreSQL and SQLite workflow is established

## Phase 4 — Multi-DB / future

Position for non-relational databases when the time comes.

- [ ] Formalize a metadata model that covers NoSQL concepts (documents, keyspaces, collections)
- [ ] Investigate per-driver TUI panels vs. a generic table abstraction
- [ ] Pluggable authentication (IAM, SSH tunnels, client certificates)
