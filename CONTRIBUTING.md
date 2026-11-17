# Contributing to rolecall

Thanks for looking at rolecall. It is a terminal job tracker: you move Applications between
Statuses and the coding harness does the AI work through the plugin. Contributions are
welcome, and the rules below keep the checks and the data safe.

## Propose a change

Open a GitHub issue before a large change. Describe the problem, what you expect, and
what you have tried. Small fixes can go straight to a pull request.

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

CI runs all of these on every push and pull request, plus a macOS job (`scripts/check-go.sh`
and `go test ./...`) and `govulncheck` on the dependencies. A change is ready when the checks
pass locally.

### Enable the pre-commit hook

The hook runs `scripts/check-go.sh` before each commit. Enable it once per clone:

```sh
git config core.hooksPath .githooks
```

## Reproduce bugs with the example workspace

Use the fictional workspace in `fixtures/workspace` rather than your own data:

```sh
go run ./cmd/rolecall --dir ./fixtures/workspace
```

When you report a bug, say which steps reproduce it in this workspace. If the bug only shows
with your own data, describe the shape of the data (for example, a Status with no Application
documents) and never paste the data itself.

## Never commit personal data

rolecall holds no personal data in the repository, and it must stay that way. See
[ADR-0005](docs/adr/0005-owner-data-never-leaves-the-workspace.md).

- Use only fictional people, companies and contact details in fixtures, tests and docs.
- Do not commit a Workspace, a Profile, a CV, an Application, or any real job text.
- Do not commit emails, phone numbers, or profile URLs, even in examples.
- Check your diff before you push. Look for real names, emails, phone numbers and profile
  URLs in what you are about to commit.

Error messages show paths but never file contents, so please keep that true in new code.

## Working conventions

- [CODING_STANDARDS.md](CODING_STANDARDS.md) lists the review judgement calls.
- [GLOSSARY.md](GLOSSARY.md) defines the project's terms. Use them in code, issues and docs.
- Architecture decisions live in [docs/adr/](docs/adr/). Read the relevant ADR before
  changing a rule it sets, and propose a new ADR for a decision that changes one.
- Only `store` reads or writes Workspace files. Only `internal/process/process.go` launches processes.
  Colours live in `internal/ui/theme/theme.go`; UI-specific styling lives in `internal/ui/styles.go`. The check script enforces the last two.

## Releasing (maintainers)

Merge to `main`, then tag it. There is no release branch.

```sh
git switch main && git pull
git tag vX.Y.Z && git push origin vX.Y.Z
```

The tag runs `.github/workflows/release.yml`: GoReleaser builds macOS and Linux archives
(with the Plugin) and publishes the GitHub Release with `checksums.txt`. The install script
picks it up from there. Check with
`curl -fsSL https://raw.githubusercontent.com/lukebrown14-code/rolecall/main/scripts/install.sh | sh`
in a fresh shell.

## Licence

rolecall is released under the MIT licence (see [LICENSE](LICENSE)). By contributing, you agree
that your contribution is released under the same licence.

## Source layout

`cmd/rolecall/` is the command-line entry point; application packages live under `internal/`
(`config`, `doctor`, `process`, `reporting`, `store`, `ui`); `plugin/` is the Harness plugin,
`fixtures/` holds shared fictional Workspaces and `scripts/` the development checks. The UI
packages are described in [the UI guide](internal/ui/README.md), and where tests live in
[CODING_STANDARDS.md](CODING_STANDARDS.md).
