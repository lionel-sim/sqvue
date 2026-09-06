# sqvue agent guidance

- Run `gofmt` on every changed Go file.
- Target Go 1.25; run `go test ./...` before completing code changes. If the environment blocks the test run, report the limitation and run the narrowest relevant tests that work.
- For TUI state, asynchronous loading, or driver changes, also run the relevant race tests and `go vet ./...` when practical.
- Keep database-specific SQL, placeholders, identifier quoting, and type handling under `internal/db/<driver>/`; keep `internal/db` interfaces and data types driver-neutral.
- Use `db.BrowseRequest` and parameterized `db.RowFilter` values for normal table browsing and row filters. Do not construct raw SQL from filter values in the TUI.
- Prefer unit tests for TUI state and rendering behavior; use SQLite integration tests for portable driver coverage and PostgreSQL integration tests when a real instance is available.
- Update `README.md` and `roadmap.md` when user-visible behavior or project status changes.
- Keep TUI state in `model.go`; keep `ui.go` limited to Bubble Tea dispatch and generic overlays. Place grid, browse-filter, browser, and browse-loading behavior in their corresponding focused files as the refactor proceeds.
- Keep rendering free of input handling and database loads. Register key bindings in `internal/tui/components/keys`, and ensure context-sensitive behavior is reflected in the help modal and README key-binding table.
- Preserve terminal safety: render database text through the existing sanitization helpers and retain width-aware truncation/wrapping.
- Use Conventional Commits for commit messages (for example, `feat(tui): add schema selection`). Commit independently testable sub-features separately.
- Preserve user-authored, unrelated working-tree changes.

## Working conventions

- Before changing code, run `git status --short`; preserve unrelated user changes.
- Keep each task narrowly scoped. Do not refactor adjacent code unless it is needed for correctness or the user asks.
- For new user-facing behavior, update the README, roadmap, keyboard-help text, and tests in the same change.
- Add focused tests for new behavior and run formatting, relevant narrow tests, then `go test ./...`; use race tests and `go vet ./...` for asynchronous or stateful TUI work.
- After implementation and verification, perform a code review of the complete diff before handing off. Check correctness, error and cancellation paths, state/lifecycle regressions, SQL safety, terminal rendering safety, test coverage, and documentation alignment; fix any findings before completion.
- Before handing off or committing, run `git diff --check` and review `git diff --stat`.
- Commit completed, independently testable work after verification unless the user explicitly asks not to commit.
- Treat `.codex/` as local machine configuration; do not add it to commits.
