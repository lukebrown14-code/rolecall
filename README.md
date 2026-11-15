# rolecall

rolecall is a local, terminal job tracker. Move Applications through your job search, read their
details, and open their documents from one place. Your coding Harness can help capture roles,
assess fit, and prepare documents through the rolecall Plugin.

## Install

### Homebrew (macOS and Linux)

```sh
brew install lukebrown14-code/homebrew-tap/rolecall
```

### Install script (macOS and Linux, no Go needed)

```sh
curl -fsSL https://raw.githubusercontent.com/lukebrown14-code/rolecall/main/scripts/install.sh | sh
```

The script downloads the latest release for your platform, verifies its checksum, and installs
rolecall with the Plugin into `~/.local` (`~/.local/bin/rolecall`). Make sure `~/.local/bin` is
on your `PATH`.

Run the same command again to update; your Workspace and settings are kept. Set
`ROLECALL_VERSION=v0.2.2` to pick a release or `ROLECALL_PREFIX=/usr/local` to install elsewhere.

To install by hand instead, download `rolecall_<os>_<arch>.tar.gz` from
[Releases](https://github.com/lukebrown14-code/rolecall/releases), extract it, and keep the
`rolecall` executable next to its `plugin` folder.

### Go

With Go 1.21 or newer installed, run the command below. Go downloads the toolchain rolecall
needs automatically.

```sh
go install github.com/lukebrown14-code/rolecall/cmd/rolecall@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

This installs only the executable. To use the Harness Plugin, clone the repository and set
`plugin` in `config.toml` (see [Get started](#get-started) for where it lives) to the clone's
`plugin` folder. The Homebrew package
and the install script include the Plugin automatically.

## Get started

```sh
rolecall init     # creates a rolecall folder here; or pass one: rolecall init ~/jobs
rolecall          # opens the app
```

`rolecall init` asks nothing: it creates the Workspace (a private folder with its own local git
repository) and remembers where it is in `config.toml` (`$XDG_CONFIG_HOME/rolecall/config.toml`
when `XDG_CONFIG_HOME` is set, otherwise `~/.config/rolecall/config.toml`). Then:

1. Run `rolecall`. The first time it opens it asks for your Contact (Name and Email are
   required; the rest is optional). Press Esc to skip, and it asks again next time.
2. [Connect your Harness](#connect-your-harness).
3. In your Harness, run the `create-profile` skill to build your Profile.

Until you add a Lead, the app's welcome lists the setup steps still to do, each with the key
that goes there. Add Tracks and a Search area later under Settings (`6`). To check local
dependencies, run `rolecall doctor`; `rolecall --help` lists every command.

To try the interface with the fictional example Workspace from a source checkout, run:

```sh
go run ./cmd/rolecall --dir ./fixtures/workspace
```

## Editor

The `editor` setting opens files. If it is unset, rolecall uses `$VISUAL`, then `$EDITOR`, then
`vi`. GUI editors need a wait flag so rolecall resumes after editing finishes. For example:

```toml
editor = "code --wait"
```

Press `,` in the TUI to open Settings and see which Editor is in effect. Select **This machine**
and press `e` to edit `config.toml`; rolecall reloads it when your editor exits. Press `Esc` to
return.

## Screens and keys

The number keys switch screens. `←` and `→` (or `s`) move between a screen's views, such as Leads
and Dismissed; `j`/`k` move through rows. Press `space` for a menu of screens and commands, or `?`
in rolecall for the complete help for the current screen.

| Key | Action |
| --- | --- |
| `1` | Leads, including the Dismissed view |
| `2` | Drafts |
| `3` | Applied, Interviewing, Offer, and Closed Applications |
| `4` | Stats |
| `5` | Profile |
| `6` / `,` | Settings |
| `a` | Add an Application |
| `v` / `Enter` | Open the selected Application's details |
| `m` | Move the selected Application to another Status |
| `u` | Undo the last move |
| `x` | Dismiss a Lead or Draft |
| `e` | Edit a row, or open the current document in your editor |
| `n` | Open the Application's notes |
| `f` / `p` | Ask the Harness to assess fit or prepare documents |
| `F` | Ask the Harness to draft a follow-up for a Follow-up due Application |
| `b` | Build PDFs for the selected Application |
| `d` | Copy the Application's PDFs to your Downloads folder |
| `/` | Filter the current view by title or company |
| `t` | Change the Track filter |
| `o` | Sort Leads by close date or newest first |
| `r` | Rescan the Workspace |
| `:` | Command palette |
| `?` | Show screen and key help |
| `q` | Quit |

## Connect your Harness

The Harness is your coding assistant (for example, Claude Code, Codex, Gemini CLI, or
opencode). rolecall installs the Plugin with the command; configure your Harness to use the
Plugin's skills and set the `harness` command in your local `config.toml` to enable the `f`,
`p` and `F` handoffs.

- [Set up Claude Code](docs/harnesses/claude-code.md)
- [Set up Codex, Gemini CLI or opencode](docs/harnesses/other-harnesses.md)

## Make your documents yours

The default resume and cover-letter layouts use Typst. To customize the header, copy
`plugin/templates/brand.typ` into the root of your Workspace as `brand.typ` and edit it there.
The Workspace copy overrides the bundled default and stays with your data when rolecall is
updated. See the comments in the template for the available fields.

## Privacy

rolecall collects no data and has no telemetry, server, account, sync, or crash reporter.
Your Profile, Applications, and documents stay in your local Workspace. Keep that Workspace
private, including any git repository you create for it.

When you ask your Harness to work on your Profile, CV, or job text, the Harness sends that
content to the AI provider you configured. Review that provider's privacy practices. See
[ADR-0005](docs/adr/0005-owner-data-never-leaves-the-workspace.md) for the privacy decisions.

## Supported platforms

macOS and Linux are supported. On Windows, use WSL. Native Windows file opening is best-effort.

## Project links

- [Source code and releases](https://github.com/lukebrown14-code/rolecall)
- [Report a bug or request a feature](https://github.com/lukebrown14-code/rolecall/issues)
- [Workspace file contract](SCHEMA.md)
- [Contributing](CONTRIBUTING.md)
- [MIT licence](LICENSE)
