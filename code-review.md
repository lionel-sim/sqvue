# Code Review

Reviewed: current `master` after `bef409c` and `370a522`.

## Remediation status

The following findings were resolved after this review:

- Stale asynchronous result handling
- Terminal-control escaping and display-cell width handling
- Safe connection configuration, TLS defaults, and startup connectivity checks
- Configured startup timeout and direct CLI coverage for connection-string precedence
- Table ordering and numeric precision
- Bounded ad-hoc SQL results
- Schema picker cancellation and scrolling
- Per-table column visibility and filter cancellation
- Driver unit-test coverage for connection configuration, ordering, and numeric formatting

The requested exclusions remain open: write protection for SQL mode, exact row
count cost, `make run` argument forwarding, narrow-terminal help behavior, and
repository licensing.

## Verification

- `go vet ./...`
- `go test ./...`
- `go test -race ./internal/tui`

All checks passed. No code was changed as part of this review.

## P1 — High priority

### Stale asynchronous results can replace the active table or page

Row and description messages do not carry a request identity, table, page, or
view mode. When requests finish out of order, an older response can overwrite
the selected table's data. This is easy to trigger by quickly navigating,
resizing, filtering, switching schemas, or running SQL against a slow database.

Use a monotonically increasing load generation and include the target table,
page, and mode in each message. Ignore messages that do not match the current
generation and target.

Affected: [internal/tui/ui.go](internal/tui/ui.go#L431), [internal/tui/ui.go](internal/tui/ui.go#L550)

### Database content is rendered as raw terminal control content

Cell values, SQL-result values, and database object names are written directly
to the terminal. Newlines can break the pinned layout; escape and OSC sequences
can change terminal rendering or activate terminal features.

Escape control characters before applying width/layout logic, rendering newline
and escape bytes visibly (for example, `\n` and `\x1b`).

Affected: [internal/tui/render.go](internal/tui/render.go#L182), [internal/tui/ui.go](internal/tui/ui.go#L495)

### Flag-based connections force plaintext TLS and break special-character credentials

The Postgres DSN is assembled with string interpolation and hard-codes
`sslmode=disable`. A password, user, database, or host containing URL-reserved
characters can be parsed incorrectly, and remote connections are silently
unencrypted.

Construct the `pgx`/`pgxpool` configuration structurally, add explicit SSL
configuration, and choose a secure default.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L23)

### Table pagination is not reliably ordered

`ORDER BY 1` fails where the first column has no ordering operator, such as a
`json` column. It also does not produce stable pages where first-column values
tie, so offsets can skip or repeat rows as data changes.

Prefer primary-key columns for ordering, with a deterministic and documented
fallback for tables without a key.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L177)

### Raw SQL results are fully retained in memory

Arbitrary SQL results are scanned into a full in-memory slice before TUI paging.
A large `SELECT` can freeze or exhaust the client even though only one page is
shown at a time.

Add a row/byte limit or redesign the query result API to stream or page results.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L140)

### SQL mode permits destructive statements without an explicit write mode

The SQL prompt executes any statement immediately, including `DROP`, `UPDATE`,
and `DELETE`. That conflicts with the product positioning as a database viewer.

Make SQL mode read-only by default and require deliberate write enablement or a
confirmation flow for mutations.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L128)

## P2 — Important follow-ups

### Startup does not verify connectivity

Creating a `pgxpool` is lazy, so the application starts Bubble Tea without
proving that the server is reachable or credentials work. Users enter the TUI
and receive an asynchronous schema-load error instead of an actionable startup
failure.

Ping during connection setup with the startup context and close the new pool if
the ping fails.

Affected: [cmd/run.go](cmd/run.go#L55), [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L30)

### Footer row counts can be expensive

Each newly viewed table runs an exact `COUNT(*)`. On a large production table,
this may perform a costly scan merely to fill a status-bar detail.

Use a catalog estimate by default, make exact counts opt-in, or request them
only when the user asks.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L205)

### Cancelling schema selection changes the active schema without loading it

The picker edits the active schema while moving. Esc closes the picker without
reloading tables, so the header can name one schema while displaying another
schema's tables.

Use a pending schema cursor and commit it only on Enter, or restore the prior
schema on cancel.

Affected: [internal/tui/ui.go](internal/tui/ui.go#L275)

### Column visibility leaks between unrelated tables

Visibility is retained whenever the next result has the same number of columns.
Hiding a column in one table can therefore hide an unrelated column at the same
ordinal position in another table.

Store preferences by table and column identity, or reset them when changing
tables.

Affected: [internal/tui/ui.go](internal/tui/ui.go#L523)

### `make run` does not forward command-line flags

`make run -- --conn ...` is interpreted by Make as extra targets. This makes
the README's source-run recommendation impractical unless `DATABASE_URL` is
set.

Add a `RUN_ARGS`/`ARGS` variable and document it, or recommend `go run ./...`
for command-line flags.

Affected: [Makefile](Makefile#L6)

### CLI and driver behavior have no direct test coverage

The TUI has meaningful tests, but `cmd`, `internal/db`, and
`internal/db/postgres` have no test files. This misses connection failure
handling, DSN construction, metadata behavior, pagination ordering, and query
semantics.

Add unit coverage for configuration/DSN behavior and integration tests against
a real Postgres instance.

Affected: [cmd](cmd), [internal/db](internal/db)

### Numeric values lose precision in display

Postgres `numeric` values are converted to `float64`, which silently rounds
large or high-precision values and can mislead users inspecting data.

Render numeric values from their integer/exponent representation or their text
form instead.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L266)

### Reconnecting an existing driver leaks the previous pool

`Connect` overwrites the driver's pool without closing an existing pool. The
driver interface permits reconnecting, so this can leave old connections and
pool resources alive.

Close the old pool after a successful replacement, or reject repeated connects.

Affected: [internal/db/postgres/driver.go](internal/db/postgres/driver.go#L23)

## P3 — Usability and polish

### Schema picker does not scroll past the first four schemas

Navigation can select any schema, but only the first four are rendered. The
selection becomes invisible after moving past that list.

Add a schema-list scroll offset.

Affected: [internal/tui/render.go](internal/tui/render.go#L197)

### Width handling does not use terminal display cells consistently

Footer alignment, column sizing, and truncation use byte or rune lengths.
Emoji, CJK characters, combining marks, ANSI-styled content, and long labels
can misalign or wrap, breaking the pinned footer.

Use ANSI-safe display-width measurement and truncation throughout the renderer.

Affected: [internal/tui/render.go](internal/tui/render.go#L146), [internal/tui/render.go](internal/tui/render.go#L380), [internal/tui/render.go](internal/tui/render.go#L447)

### Filter cancellation clears the existing filter

Opening the filter resets it immediately, and Esc applies the now-empty filter.
Preserve the original filter value and restore it on cancellation, or present a
separate explicit clear action.

Affected: [internal/tui/ui.go](internal/tui/ui.go#L176), [internal/tui/ui.go](internal/tui/ui.go#L262)

### Help modal overflows very narrow terminals

The help dialog forces a minimum content width and adds border/padding, so it
can exceed a narrow terminal width.

Allow a compact or borderless fallback at small widths.

Affected: [internal/tui/render.go](internal/tui/render.go#L106)

### Repository license is not present

The README refers readers to repository licensing details, but no tracked
license file exists. Add an explicit license or remove the section.

Affected: [README.md](README.md#L98)
