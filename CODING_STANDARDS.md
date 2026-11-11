# Coding standards

Judgement calls for review. Mechanical rules live in `scripts/check-go.sh` and CI, not here.

- Colour always pairs with a word or glyph, so meaning never rests on colour alone. The
  frame stays readable with `NO_COLOR` set (no colour, bold and underline kept) and with
  `ROLECALL_ASCII=1` (every glyph has an ASCII stand-in).
- Rescan-and-restore-cursor goes through the one `reload` helper in `ui/model.go`.
- Only `store` reads or writes Workspace files; `ui` never touches them directly.
- Behaviour tests drive `Update` directly, not the whole program: a key in, model and disk
  state out, with external programs faked (`ui/drive_test.go`). No `teatest`, no `WaitFor`,
  no `time.Sleep` except where the program genuinely waits on an external process. A
  handful of `teatest` smoke tests remain: start, move, add, live refresh, quit.
- Rendering tests draw `View()` from fixed model states into ANSI-stripped goldens; one
  coloured golden (`TestFixtureTables/palette`) covers the palette, so a palette change
  touches one file. Regenerate with `go test ./ui -update` only when the change is
  intended; review the diff.
