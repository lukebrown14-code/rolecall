# rolecall

A terminal job tracker. Move Applications between Statuses, read them, and grab their
documents. Your coding harness does the AI work (capture, fit, resumes, cover letters) through
the rolecall plugin, by writing files the TUI watches.

**Status:** Workspace store and interactive TUI (v0.1) implemented. Private until v1.0.

- [Spec](docs/spec.md): roadmap and v0.1 behaviour
- [SCHEMA.md](SCHEMA.md): the Workspace file contract
- [GLOSSARY.md](GLOSSARY.md): terms
- [docs/adr/](docs/adr/): decisions

## Run locally

Install Go on macOS with `brew install go`, or use the installer at
https://go.dev/dl/. The required Go version is declared in `go.mod`.

From an up-to-date checkout (with `go.mod` present):

```sh
go run ./cmd/rolecall --dir ./fixtures/workspace
```

This opens the interactive TUI. To use your own Workspace, replace
`./fixtures/workspace` with its path.

To use a default Workspace, create `~/.config/rolecall/config.toml`:

```toml
workspace = "~/path/to/workspace"
editor = "vi"
follow_up_days = 14
downloads = "~/Downloads"
```

Then run `go run ./cmd/rolecall`. `--dir` overrides the configured Workspace.

`editor` is the command that opens files. If empty, rolecall uses `$VISUAL`, then `$EDITOR`,
then `vi`. Example values:

- `nvim`
- `code --wait`
- `zed --wait`
- `hx`
- `subl --wait`

GUI editors need their wait flag so rolecall rescans only after editing finishes. Press `,` in
the TUI to open Settings, which shows the Editor in effect and where it came from
(`Esc` returns); `e` on This machine edits `config.toml` and reloads it when the Editor exits.

To install with Homebrew (macOS and Linux):

```sh
brew install lukebrown14-code/tap/rolecall
```

The formula installs the rolecall Plugin with the command. rolecall finds the
bundled Plugin automatically when neither `plugin` nor `ROLECALL_PLUGIN` is set.

To build and install a standalone command:

```sh
mkdir -p ~/.local/bin
go build -o ~/.local/bin/rolecall ./cmd/rolecall
```

Add `~/.local/bin` to your shell's PATH, then run `rolecall`.
If `go.mod` is missing, check `pwd` and update the intended checkout with
`git pull origin main`; this repository already has a module, so no initialization
is needed.

## Privacy

rolecall collects nothing and has no telemetry. Your Profile, CV and job ads stay in
your Workspace unless you ask the Harness to work with them. The Harness sends that
content to your own AI provider; review that provider's privacy practices.

Keep your Workspace git repository private. It contains your Profile, CV and
application data.

## Checks

```sh
python3 scripts/check-skills.py
go vet ./...
go test -race ./...
```

The skill check validates explicit Skill-tool dependencies in repository skills.
Run it after installing or updating skills; CI also runs it on every push.
