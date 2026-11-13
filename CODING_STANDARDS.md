# Coding standards

Judgement calls for review. Mechanical rules live in `scripts/check-go.sh` and CI, not here.

- Colour always pairs with a word or glyph, so meaning never rests on colour alone. Colours
  are roles with a dark and a light value; text keeps a 4.5:1 contrast floor on both themes
  and rules and dividers 3:1, even when a pop-up dims the frame behind it. The accent is the colour of keys
  (and the selection gutter). The frame stays readable with `NO_COLOR` set (no colour, bold
  and underline kept) and with `ROLECALL_ASCII=1` (every glyph has an ASCII stand-in); a
  locale that is not UTF-8, or `TERM=linux`, turns ASCII on by itself.
- Rescan-and-restore-cursor goes through the one `reload` helper in `internal/ui/model.go`.
- Only `store` reads or writes Workspace files; `ui` never touches them directly.
- Behaviour tests drive `Update` directly, not the whole program: a key in, model and disk
  state out, with external programs faked (`internal/ui/tests/drive_test.go`). No `teatest`, no `WaitFor`,
  no `time.Sleep` except where the program genuinely waits on an external process. A few
  `teatest` smoke tests remain where the program itself is the subject: live refresh, the
  captured build runner, and external programs.
- Rendering tests draw `View()` from fixed model states into ANSI-stripped goldens; one
  coloured golden (`TestFixtureTables/palette`) covers the palette, so a palette change
  touches one file. Regenerate with `go test ./internal/ui/tests -update` only when the change is
  intended; review the diff.

Public API tests live in each package’s `tests/` folder, with their `testdata/` fixtures.
Tests that need private implementation details stay beside the source. Run `go test ./...`
for the full suite, or `go test ./internal/ui/...` for all UI packages and their tests.
