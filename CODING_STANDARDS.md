# Coding standards

Judgement calls for review. Mechanical rules live in `scripts/check-go.sh` and CI, not here.

- Colour always pairs with a word or glyph, so meaning never rests on colour alone. The
  frame stays readable with `NO_COLOR` set (no colour, bold and underline kept) and with
  `ROLECALL_ASCII=1` (every glyph has an ASCII stand-in).
- Rescan-and-restore-cursor goes through the one `reload` helper in `ui/model.go`.
- Only `store` reads or writes Workspace files; `ui` never touches them directly.
- New tests drive the whole program with `teatest` (see `docs/spec.md` Testing Decisions); assert on the final screen and on disk bytes, and poll the disk instead of waiting on streamed output.
- Regenerate golden files with `go test ./ui -update` only when the change is intended; review the diff.
