# Coding standards

Judgement calls for review. Mechanical rules live in `scripts/check-go.sh` and CI, not here.

- Every colour and style lives in `ui/styles.go`; colour always pairs with a word or glyph.
- Status names go through one type or list, never repeated string literals in switches.
- Rescan-and-restore-cursor goes through the one `reload` helper in `ui/model.go`.
- Only `store` reads or writes Workspace files; `ui` never touches them directly.
- New tests drive the whole program with `teatest` (see `docs/spec.md` Testing Decisions); assert on the final screen and on disk bytes, and poll the disk instead of waiting on streamed output.
- Regenerate golden files with `go test ./ui -update` only when the change is intended; review the diff.
