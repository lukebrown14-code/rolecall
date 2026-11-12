# Coding standards

Judgement calls for review. Mechanical rules live in `scripts/check-go.sh` and CI, not here.

- Colour always pairs with a word or glyph, so meaning never rests on colour alone. Text
  keeps a 4.5:1 contrast floor on the dark background and rules and dividers 3:1, even when
  a pop-up dims the frame behind it. The frame stays readable with `NO_COLOR` set (no colour,
  bold and underline kept) and with `ROLECALL_ASCII=1` (every glyph has an ASCII stand-in);
  a locale that is not UTF-8 turns ASCII on by itself.
- Rescan-and-restore-cursor goes through the one `reload` helper in `ui/model.go`.
- Only `store` reads or writes Workspace files; `ui` never touches them directly.
- Behaviour tests drive `Update` directly, not the whole program: a key in, model and disk
  state out, with external programs faked (`ui/drive_test.go`). No `teatest`, no `WaitFor`,
  no `time.Sleep` except where the program genuinely waits on an external process. A few
  `teatest` smoke tests remain where the program itself is the subject: live refresh, the
  captured build runner, and external programs.
- Rendering tests draw `View()` from fixed model states into ANSI-stripped goldens; one
  coloured golden (`TestFixtureTables/palette`) covers the palette, so a palette change
  touches one file. Regenerate with `go test ./ui -update` only when the change is
  intended; review the diff.
