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
`ROLECALL_VERSION=v0.3.0` to pick a release or `ROLECALL_PREFIX=/usr/local` to install elsewhere.

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
when `XDG_CONFIG_HOME` is set, otherwise `~/.config/rolecall/config.toml`). If `config.toml`
already opens another Workspace, init leaves it as your default and prints how to open the new
one (`rolecall --dir DIR`); `rolecall init --switch DIR` makes DIR the default instead. If
`config.toml` has an error, `rolecall` and `rolecall doctor` name the line and the reason. Then:

1. Run `rolecall`. The first time it opens it asks for your Contact (Name and Email are
   required; the rest is optional). Press Esc to skip, and it asks again next time.
2. [Connect your Harness](#connect-your-harness).
3. In your Harness, run the `create-profile` skill to build your Profile. No Harness? See
   [Use rolecall without a Harness](#use-rolecall-without-a-harness).

Until you add a Lead, the app's welcome lists the setup steps still to do, each with the key
that goes there. Add Tracks and a Search area later under Settings (`6`). To check local
dependencies, run `rolecall doctor`; `rolecall --help` lists every command. To move an Application
from a script (or an agent you have asked to), run `rolecall move FOLDER STATUS`, for example
`rolecall move jobs/SWE/acme applied`: it appends the Status as the app does and, for Applied,
saves `sent/`.

To try the interface with the fictional example Workspace from a source checkout, run:

```sh
go run ./cmd/rolecall --dir ./fixtures/workspace
```

## Track what is next, and use it elsewhere

`e` on an Application also takes a next step (a date, `YYYY-MM-DD` or `YYYY-MM-DDTHH:MM`, and
what it is) and its first contact (name, role, email or link). The next step shows on
Interviewing and Offer rows, on the detail Overview and in Stats → Now. Moving an Application to
Rejected or Withdrawn asks why it ended (optional; `esc` skips), and Stats shows where Applications
dropped off.

Two read-only commands put the same facts in your terminal and calendar. Neither writes to the
Workspace; both take `--dir PATH`.

```sh
rolecall now                  # 2 follow-ups due | 1 next step within 7 days | 3 closing within 7 days
rolecall now --days 3 --list  # one tab-separated line per item: kind, folder, when, title, company
rolecall export --format csv --out applications.csv   # or --format json; stdout without --out
rolecall export --ics --out rolecall.ics              # next steps and Lead/Draft close dates
```

`rolecall now` prints one plain line with no colour and exits 0, printing nothing when nothing is
due, so it drops into a tmux status line, a prompt or cron:

```sh
# tmux: ~/.tmux.conf
set -g status-right '#(rolecall now) %H:%M'
# bash prompt: show it only when there is something to do
PS1='$(rolecall now | sed "s/.*/[&] /")\$ '
# cron: cron mails any output, so a quiet day sends nothing
0 9 * * 1-5  rolecall now --dir ~/rolecall
```

`export` writes only where `--out` says (the file is created readable by you alone), so nothing
leaves your Workspace unless you send it ([Privacy](#privacy)). A CSV cell that starts with `=`, `+`,
`-` or `@` gets a leading `'`, so a spreadsheet reads it as text.

## Editor

The `editor` setting opens files. If it is unset, rolecall uses `$VISUAL`, then `$EDITOR`, then
`vi`. GUI editors need a wait flag so rolecall resumes after editing finishes. For example:

```toml
editor = "code --wait"
```

Press `,` in the TUI to open Settings and see which Editor is in effect. Select This machine
and press `e` to edit `config.toml`; rolecall reloads it when your editor exits. Press `Esc` to
return.

## Screens and keys

The number keys switch screens. `←` and `→` (or `s`) move between a screen's views, such as Leads
and Dismissed; `j`/`k` move through rows. Press `space` for the command palette, a filterable list of
this screen's actions and every other screen (`:` and `Ctrl-K` open the same one), or `?` in
rolecall for the complete help for the current screen.

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
| `u` | Undo the last move or recorded follow-up |
| `x` | Dismiss a Lead or Draft |
| `*` | Mark or unmark the row, so `x` and `m` act on every marked row (`Esc` clears the marks) |
| `l` | Record that you followed up today, on an Applied or Interviewing Application |
| `e` | Edit a row, or open the current document in your editor |
| `n` | Open the Application's notes |
| `f` / `p` | Ask the Harness to assess fit or prepare documents |
| `F` | Ask the Harness to draft a follow-up for a Follow-up due Application |
| `b` | Build PDFs for the selected Application, or the selected CV on the Profile screen |
| `d` | Copy the Application's or CV's PDFs to your Downloads folder |
| `w` / `c` | Open the job ad's link in your browser / copy it to the clipboard |
| `/` | Filter the current view by title or company |
| `t` / `T` | Next / previous Track filter |
| `o` | Sort Leads by close date or newest first |
| `z` | Compact rows: one line per row at wide sizes (saved as `compact_rows`) |
| `r` | Rescan the Workspace |
| `h` | Connect a Harness, while none is set |
| `J` / `K` | In the detail view, the next / previous Application on the same tab |
| `space` / `:` | Command palette |
| `?` | Show screen and key help |
| `q` | Quit |

## Connect your Harness

The Harness is your coding assistant (for example, Claude Code, Codex, Gemini CLI, or
opencode). rolecall installs the Plugin with the command; configure your Harness to use the
Plugin's skills and set the `harness` command in your local `config.toml` to enable the `f`,
`p` and `F` handoffs.

See [Connect a Harness](docs/harnesses.md) for Claude Code, Codex, Gemini CLI and opencode.

## Use rolecall without a Harness

A Harness is optional. Without one, rolecall is a tracker you fill in yourself:

- Press `a` to add a Lead: Title and Company are required, the URL is
  optional, and the first Lead asks for a Track, a kind of role such as Nursing or Marketing.
  Pick an Editor first (`6`, then This machine, then `e` on the Editor row) so pasting the job
  ad into `jd.md` works; otherwise add a URL. Move an Application along with `m`.
- On Drafts (`2`), `→` switches to Deadlines: every Lead and Draft with a closing
  date still to come, soonest first.
- A sent Application with no news for the follow-up period (Settings, 14 days by default) is
  Follow-up due and shows `⚑`. Press `l` once you have followed up and the mark clears.
- Stats (`4`) shows your funnel, Response rate and applications per week from the moves you
  make, with nothing else to set up.
- To write your Profile by hand, start from `profile/master-profile.md`: `rolecall init` creates
  it with its Contact table and one heading per section, each with a one-line how-to under it.
  Press `5`, then `c` for your Contact and `e` to write the rest in your Editor.
- To build PDFs from a hand-written resume, install the Plugin and set `plugin` in
  `config.toml` (with `typst`, `pandoc` and `python3` on your PATH; `rolecall doctor` checks
  them). Write `resume.md` and `cover-letter.md` in the Application's folder, select it
  and press `b` to build the PDFs; `d` copies them to your Downloads folder. Documents are
  still checked against your Profile, so keep its facts and skills current.

## Your CV

A CV is a resume for no particular job, kept in your Workspace as `cv/general.md` (add others,
such as `cv/master.md`, beside it). Ask your Harness to write one from your Profile, then open
the Profile screen (`5`): each CV is listed under CVs, where `b` builds it and `d` copies the
PDF to your Downloads folder. Its headline is the `General` row of your Profile's Headlines
table. From a shell: `python3 <plugin>/scripts/build.py --cv general`.

## Make your documents yours

The default resume and cover-letter layouts use Typst. To customize them, copy any of
`plugin/templates/brand.typ` (colours, fonts and the header), `resume.typ` or
`cover-letter.typ` into a `templates/` folder in your Workspace and edit the copy there.
Each Workspace copy overrides the bundled file of the same name and stays with your data
when rolecall is updated, but it also stops receiving the Plugin's fixes to that file.
`rolecall doctor` lists the overrides in use and warns about files in `templates/` the build
never reads. See the comments in the templates for the available fields.

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
