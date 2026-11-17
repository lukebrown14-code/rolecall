# Terminal UI

The root `ui` package coordinates the Bubble Tea model, screen transitions, commands,
and rendering. Its subpackages do not import it.

| Location | Responsibility |
| --- | --- |
| `settings/` | Settings row descriptions and provenance, workspace snapshot, cursor and focus navigation, and input validation. |
| `forms/` | Form field editing, validation, focus, submission, and discard confirmation. |
| `theme/` | Colour roles, dark and light palettes, the glyph table (`glyphs.go`: each glyph's meaning, ASCII stand-in and `?` legend row), status styling, Markdown styles, and terminal appearance fallbacks (NO_COLOR, inherit, ASCII, `ROLECALL_PLAIN`). |
| `watch/` | Filesystem subscriptions, debounce windows, changed folders, document-write classification, and shutdown. |
| `tests/` | Behaviour and rendering tests through the UI's public interface, with rendering goldens in `testdata/`. |
| `../reporting/` | Pure funnel, response, weekly, and track calculations, independent of terminal UI. |

The root model adapts those modules to the application. Files are grouped by name, not
listed here one by one (`ls internal/ui` is the inventory):

- `model.go`, `refresh.go`, `mapping.go`: state, updates and the one `reload` helper.
- `keymap.go` (row actions) and `help.go` (the registry): every key is declared once and read
  by the key line, `?` and the command palette (`palette.go`).
- `view.go`, `layout.go`, `columns.go`, `styles.go`: shared presentation; `plain.go` reshapes
  the finished frame for `ROLECALL_PLAIN`.
- `settings*.go`, `profile.go`, `stats.go`, `detail.go`, `form.go`: screens and their forms.
- `external.go`, `handoff.go`, `build.go`, `fetch.go`, `download.go`: external actions and their
  completion messages; every process start goes through `internal/process`.

Tests for extracted modules live in their own `tests/` directories. The watcher's
`internal/ui/watch/internal/debounce/` package lets watcher tests control timing without
exposing debounce to other application packages.

Run all UI checks with `go test ./internal/ui/...`, report tests with
`go test ./internal/reporting/...`, and the full suite with `go test ./...`.
Regenerate UI goldens with `go test ./internal/ui/tests -update` only for intended changes.
