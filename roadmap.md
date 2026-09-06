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

- [x] Column visibility picker for row and SQL-result views, retaining table choices while browsing
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
- [x] Export results as JSON
  - [x] Export visible columns as a JSON array, with the same table/SQL, filtered-row, save-path, and background-status flow as CSV
- [x] Streaming / large-result handling (cursor or fetch-more pagination instead of LIMIT/OFFSET)
  - [x] Define a driver-neutral row-stream contract with explicit close/cancellation semantics and focused unit tests
  - [x] Implement filtered and unfiltered table row streams in the PostgreSQL, SQLite, and MySQL drivers, preserving each driver's existing ordering and parameterized filters
  - [x] Page table browsing from an active stream, resetting and closing streams when the table, schema, filters, view mode, or window page size changes
  - [x] Stream ad-hoc SQL query results into fetch-more pages rather than retaining the current fixed 1,000-row result set
    - [x] Define query-stream metadata and implement it for PostgreSQL, SQLite, and MySQL without changing non-row command results
    - [x] Page SQL results from a live stream, including backward navigation, stale-load cleanup, and query status reporting
    - [x] Preserve complete CSV and JSON query exports without retaining every streamed row in TUI memory
  - [x] Cover stream lifecycle, error, pagination, and cancellation behavior; document fetch-more behavior and key bindings
- [x] Saved connections / connection profiles (XDG config file, named profiles, and `--profile`)
- [x] Configure themes through the config file
  - [x] Replace mutable global theme styles with an immutable semantic theme value while preserving the current default appearance
  - [x] Add named ANSI-256 presets (`default`, `light`, and `high-contrast`)
  - [x] Add a `settings.theme` configuration value with validation and startup resolution
  - [x] Pass the resolved theme through command setup, TUI options, and model state
  - [x] Apply themes consistently to renderers, inputs, help, and all dialogs
  - [x] Add configuration/rendering coverage and document available themes
- [x] Test coverage for drivers (integration tests against real Postgres and MySQL instances, plus SQLite temporary databases)
- [x] CI: lint, build, and test matrix (GitHub Actions on Linux, macOS, and Windows)
- [x] Generate database backups
  - [x] Define a driver-specific backup interface so backups preserve database schema, data, indexes, and constraints without putting database-specific behavior in `internal/db`
  - [x] Add a `B` key binding and keyboard-help entry that opens a backup-scope picker
  - [x] Offer `Entire database` (the default) and `Current table`; add `Current schema` only for drivers that support schemas
  - [x] Follow the scope picker with a save-path prompt, provide timestamped driver-appropriate filenames, refuse to overwrite existing files, and report asynchronous progress, success, and errors in the TUI
  - [x] Implement PostgreSQL full-database and table backups with `pg_dump`, including a clear preflight error when the command is unavailable
  - [x] Implement SQLite full-database and table backups using SQLite-native mechanisms; document any intentional limits of table-level backups
  - [x] Add focused unit tests for picker state, path validation, command construction, and completion/error handling
  - [x] Document backup behavior, required external tools, output formats, and key bindings in the README
  - [x] Add MySQL full-database and table backups with `mysqldump` after the PostgreSQL and SQLite workflow is established

## Phase 4 — Power-user TUI workflows

Build on the completed browsing foundation with faster recurring investigation,
safer query execution, and richer relational navigation.

- [x] Edit active table cells
  - [x] Bind `e` to edit the active grid cell while retaining CSV export outside grid focus; keep SQL-result grids read-only
  - [x] Enable editing only when the selected table has a declared primary key; show a clear status message otherwise
  - [x] Open a prefilled, terminal-safe cell editor and require an explicit confirmation before saving
  - [x] Add a driver-neutral single-cell update contract using primary-key values, with parameterized PostgreSQL, SQLite, and MySQL implementations
  - [x] Refresh the updated row and report asynchronous success or failure without losing grid focus, filters, visible columns, or pagination state
  - [x] Cover composite primary keys, null and typed values, cancellation, driver errors, and context-sensitive key bindings; document the workflow
- [x] Query history and saved queries
  - [x] Retain per-profile SQL history with Ctrl+P/Ctrl+N navigation in the SQL editor
  - [x] Save, list, run, rename, and delete named queries in the XDG config directory
  - [x] Preserve multi-line SQL and show saved-query failures inline without losing edits
- [x] Sort table and query results
  - [x] Add a driver-neutral sort specification to browse requests and implement dialect-safe identifier handling per driver
  - [x] Choose the active column and ascending/descending order from the data grid
  - [x] Show active sort order in the footer and preserve it through filtering, streaming, export, and pagination
- [ ] Safe SQL execution mode
  - [ ] Default ad-hoc SQL sessions to read-only where each driver supports it
  - [ ] Add an explicit, clearly labelled session-level write-mode confirmation
  - [ ] Surface transaction/read-only state in the SQL UI and test driver-specific enforcement
- [ ] Relationship explorer
  - [ ] Add a compact table relationship view using existing foreign-key metadata
  - [ ] Navigate inbound and outbound relationships and open related rows with parameterized filters
  - [ ] Preserve terminal-safe rendering and add focused navigation tests
- [x] Switch connection profiles inside the TUI
  - [x] Present configured profiles in a picker without exposing credentials
  - [x] Reconnect safely, cancel stale loads and streams, and retain a clear connection status
  - [x] Document reconnection behavior and error recovery
- [x] SQL editor quality of life
  - [x] Support multi-line editing, query history navigation, and readable SQL error locations
  - [x] Add optional formatting and `EXPLAIN`/query-plan views without silently executing mutations
  - [x] Keep editor key bindings discoverable in the help modal and README
  - [x] Refresh the active table or SQL result without discarding its browsing state

## Phase 5 - INSERT / DELETE capabilities

- [x] Insert a single row through the table grid
  - [x] Bind `a` in a focused base-table grid to open a terminal-safe new-row form; keep views and SQL-result grids read-only
  - [x] List columns with their data types, required/default/generated status, and foreign-key targets
  - [x] Give every editable field an explicit state: use database default (omit the column), set `NULL`, or enter a literal value; never infer `NULL` from typed text
  - [x] Offer a `NOW` field action only for date/time-compatible columns; represent it as a typed database-current-timestamp value, not user-supplied SQL, and render the appropriate driver expression
  - [x] Automatically omit generated/read-only columns and add the driver metadata needed to identify identity, auto-increment, and computed columns
  - [x] Require an insert confirmation that summarizes the table and every supplied value/state before writing
  - [x] Define a driver-neutral single-row insert contract, then implement parameterized and dialect-safe PostgreSQL, SQLite, and MySQL inserts, including rows that use only defaults
  - [x] Refresh the current browse view after success without losing focus, filters, sort, pagination, or visible-column choices; clearly report when filters hide the inserted row
  - [x] Surface database validation, constraint, cancellation, and driver errors in the form without discarding entered values
  - [x] Add focused driver and TUI tests, update keyboard help and README workflow documentation, and preserve terminal-safe rendering
- [ ] Delete the active table row
  - [ ] Bind `d` in a focused base-table grid; retain `d` for column descriptions outside grid focus, and keep views and SQL-result grids read-only
  - [ ] Enable deletion only for rows in tables with declared primary keys (including composite keys), with a clear status message otherwise
  - [ ] Open a terminal-safe confirmation that identifies the target table and primary-key values; require Enter to delete and let Esc cancel without changes
  - [ ] Define a driver-neutral single-row delete contract using primary-key values, with parameterized, dialect-safe PostgreSQL, SQLite, and MySQL implementations that require exactly one affected row
  - [ ] Run the delete asynchronously with normal timeout/cancellation handling; return constraint, permission, and driver errors to the confirmation without losing the selected row
  - [ ] Refresh the browse view and row counts after success, preserving focus, filters, sorting, and visible columns while safely stepping back when the deleted row empties the current page
  - [ ] Add focused driver and TUI tests for composite keys, confirmation/cancellation, row-count and pagination state, errors, and context-sensitive key bindings; update keyboard help and README documentation

## Phase 6 — Multi-DB / future

Position for non-relational databases when the time comes.

- [ ] Formalize a metadata model that covers NoSQL concepts (documents, keyspaces, collections)
- [ ] Investigate per-driver TUI panels vs. a generic table abstraction
- [ ] Pluggable authentication (IAM, SSH tunnels, client certificates)
