# sqvue agent guidance

- Run `gofmt` on every changed Go file.
- Run `go test ./...` before completing code changes. If the environment blocks the test run, report the limitation and run the narrowest relevant tests that work.
- Keep database-specific behavior under `internal/db/<driver>/`; keep the shared driver interface driver-neutral.
- Prefer unit tests for TUI state and rendering behavior; use integration tests for PostgreSQL behavior.
- Update `README.md` and `roadmap.md` when user-visible behavior or project status changes.
- Use Conventional Commits for commit messages (for example, `feat(tui): add schema selection`).
- Preserve user-authored, unrelated working-tree changes.
