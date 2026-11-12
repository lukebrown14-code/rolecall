# rolecall

## Agent skills

### Issue tracker

GitHub Issues and the public v1 repository are under `lukebrown14-code` (`rolecall`); the
Go module is `github.com/lukebrown14-code/rolecall`. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: root `GLOSSARY.md` + `docs/adr/`. See `docs/agents/domain.md`.

## Where things are

- Spec and roadmap: `docs/spec.md`. File contract: `SCHEMA.md`. Test Workspace: `fixtures/workspace`.
- Run the TUI: `go run ./cmd/rolecall --dir ./fixtures/workspace`.
- Checks: `scripts/check-go.sh` (also CI and the pre-commit hook; enable hooks with `git config core.hooksPath .githooks`).
- Review standards: `CODING_STANDARDS.md`.
