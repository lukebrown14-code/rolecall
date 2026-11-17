# Contributing to rolecall

Contributions are welcome. Open a GitHub issue before a large change, describing the problem,
what you expect and what you have tried. Small fixes can go straight to a pull request.

## Run the checks

Install Go (the version is declared in `go.mod`) and Python 3, then run:

```sh
python3 scripts/check-personal-data.py .      # personal-data scan of tracked files
scripts/test-pre-commit.sh                    # the pre-commit hook's own tests
scripts/check-go.sh              # gofmt, architecture rules, go vet, golangci-lint if installed
go test -race ./...
go test -race -count=3 ./internal/ui/...      # the UI tests run three times to catch flakiness
ruff check plugin                # Python plugin lint (needs: python3 -m pip install ruff pytest)
python3 -m pytest plugin/tests
```

CI runs these on every push and pull request, plus a macOS job (`scripts/check-go.sh` and
`go test ./...`) and `govulncheck`.

### Enable the pre-commit hook

The hook runs `scripts/check-go.sh` before each commit. Enable it once per clone:

```sh
git config core.hooksPath .githooks
```

## Reproduce bugs with the example workspace

```sh
go run ./cmd/rolecall --dir ./fixtures/workspace
```

Report bugs as steps in this fictional workspace. If a bug needs your own data, describe its
shape (for example, a Status with no Application documents) and never paste the data.

## Never commit personal data

The repository holds no personal data
([ADR-0005](docs/adr/0005-owner-data-never-leaves-the-workspace.md)).

- Use only fictional people, companies and contact details in fixtures, tests and docs.
- Do not commit a Workspace, Profile, CV, Application or real job text.
- Check your diff for real names, emails, phone numbers and profile URLs before you push.
- Error messages show paths, never file contents.

## Working conventions

- [CODING_STANDARDS.md](CODING_STANDARDS.md) lists the review judgement calls and where tests live.
- [GLOSSARY.md](GLOSSARY.md) defines the terms to use in code, issues and docs.
- [docs/adr/](docs/adr/) holds the architecture decisions. Read the relevant ADR before
  changing a rule it sets; propose a new ADR to change one.
- Only `store` reads or writes Workspace files and only `internal/process/process.go` launches
  processes. Colours live in `internal/ui/theme/theme.go` and UI styling in
  `internal/ui/styles.go`; the check script enforces the last two.

## Releasing (maintainers)

Merge to `main`, then tag it. There is no release branch.

```sh
git switch main && git pull
git tag vX.Y.Z && git push origin vX.Y.Z
```

The tag runs `.github/workflows/release.yml`: GoReleaser builds macOS and Linux archives (with
the Plugin), publishes the GitHub Release with `checksums.txt`, and updates the formula in
`lukebrown14-code/homebrew-tap` with the `TAP_GITHUB_TOKEN` secret. Check it in a fresh shell:
`curl -fsSL https://raw.githubusercontent.com/lukebrown14-code/rolecall/main/scripts/install.sh | sh`.

## Licence

Contributions are released under the MIT licence ([LICENSE](LICENSE)).

## Source layout

| Path | Holds |
|---|---|
| `cmd/rolecall/` | the command-line entry point |
| `internal/` | `config`, `doctor`, `process`, `reporting`, `store`, `ui` ([UI guide](internal/ui/README.md)) |
| `plugin/` | the Harness plugin |
| `fixtures/` | shared fictional Workspaces |
| `scripts/` | the development checks |
