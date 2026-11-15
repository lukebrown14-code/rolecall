# Claude Code

This guide connects rolecall to Claude Code. The install script and the Homebrew formula
both install the Plugin next to the rolecall command; only its location differs.

## Find the Plugin

| Installed with | Plugin folder |
|---|---|
| Install script | `~/.local/share/rolecall/plugin` (or `$ROLECALL_PREFIX/share/rolecall/plugin`) |
| Homebrew | `$(brew --prefix rolecall)/libexec/plugin` |
| `go install` | the `plugin` folder of a clone of this repository |

`rolecall doctor` prints the Plugin path it found, on the `Plugin path` line.

## Install the Plugin

Add the Plugin folder as a local marketplace and install the `rolecall` Plugin. For the install
script:

```sh
claude plugin marketplace add ~/.local/share/rolecall/plugin
claude plugin install rolecall@rolecall
```

For Homebrew, use `"$(brew --prefix rolecall)/libexec/plugin"` as the folder instead.

The marketplace name and Plugin name are both `rolecall`. If Claude Code is already open,
restart it after installing the Plugin.

If `install` says the Plugin is not found in marketplace `rolecall`, an older marketplace with
that name points somewhere else. Remove it with `claude plugin marketplace remove rolecall` and
add the folder again.

## Update the Plugin

After updating rolecall, refresh the marketplace so Claude Code picks up the new skills:

```sh
claude plugin marketplace update rolecall
```

## Configure rolecall

Open rolecall's Settings screen (`6`) and set **Harness** to:

```text
claude "/rolecall:{skill} {dir}"
```

Or set the same value in `~/.config/rolecall/config.toml`:

```toml
harness = 'claude "/rolecall:{skill} {dir}"'
```

Keep the single quotes around the TOML value. rolecall replaces `{skill}` with `assess-fit`
or `prepare-documents`, and `{dir}` with the current Application folder. rolecall starts the
Harness in your Workspace, wherever you launched rolecall, so Claude Code opens with the
Workspace as its project and can edit the Application folder. The Plugin makes
these skills available as `/rolecall:assess-fit` and `/rolecall:prepare-documents` in
Claude Code.

## Work in the Workspace yourself

To use the skills outside a handoff, for example to build your Profile or capture a role,
open Claude Code in the Workspace:

```sh
cd /path/to/your/workspace && claude
```

Keep these sessions out of code repositories: the Workspace holds your personal data.

## Check the handoff

Start rolecall with your Workspace, open a Lead or Draft Application, then press `f` to run
Assess fit or `p` to run Prepare documents. Claude Code should open with the selected skill
and Application folder. Review its proposed file changes; the rolecall Plugin writes results
to the Application folder for the TUI to display.

`prepare-documents` is available on Leads and Drafts. `assess-fit` is available for any
Application. If the keys are missing, confirm that Claude Code is installed and that Harness
is set in rolecall's Settings.
