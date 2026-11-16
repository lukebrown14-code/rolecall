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

The root model adapts those modules to the application:

- `model.go`, `refresh.go`, `mapping.go`, `next.go`: application state, updates, refresh, and navigation.
- `settings*.go`: Settings rendering and action adapters. `settings.State` owns the loaded values and navigation; the model handles popups, external editing, save completion, and undo callbacks.
- `form.go`: renders forms and connects submission to application actions.
- `styles.go`: application-specific composition using the theme palette.
- `profile.go`, `stats.go`: screen handlers and rendering.
- `view.go`, `layout.go`, `columns.go`, `help.go`: shared presentation; `plain.go` reshapes the finished frame for `ROLECALL_PLAIN`.
- `mode.go`, `overlay.go`, `palette.go`, `search.go`, `workspace_menu.go`: overlays and application input. `keymap.go` (row actions) and `help.go` (the registry) declare every key once for the key line, `?`, the palette and the `space` menu.
- `empty.go`: empty states and the first-run welcome checklist.
- `external.go`, `handoff.go`, `build.go`, `fetch.go`, `download.go`: external actions and completion messages.
- `writing.go`: transient writing indicators driven by `watch.Changed` messages.

Tests for extracted modules live in their own `tests/` directories. The watcher's
`internal/debounce/` package lets watcher tests control timing without exposing debounce
to other application packages.

Run all UI checks with `go test ./internal/ui/...`, report tests with
`go test ./internal/reporting/...`, and the full suite with `go test ./...`.
Regenerate UI goldens with `go test ./internal/ui/tests -update` only for intended changes.
