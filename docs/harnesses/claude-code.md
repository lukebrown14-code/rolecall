# Claude Code

This guide connects a Homebrew installation of rolecall to Claude Code. The rolecall
command and Plugin are installed by the same Homebrew formula.

## Install rolecall

```sh
brew install lukebrown14-code/homebrew-tap/rolecall
```

The formula installs the Plugin at `$(brew --prefix rolecall)/libexec/plugin`. Add that
directory as a local marketplace and install the `rolecall` Plugin:

```sh
claude plugin marketplace add "$(brew --prefix rolecall)/libexec/plugin"
claude plugin install rolecall@rolecall
```

The marketplace name and Plugin name are both `rolecall`. If Claude Code is already open,
restart it after installing the Plugin.

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
or `prepare-documents`, and `{dir}` with the current Application folder. The Plugin makes
these skills available as `/rolecall:assess-fit` and `/rolecall:prepare-documents` in
Claude Code.

## Check the handoff

Start rolecall with your Workspace, open a Lead or Draft Application, then press `f` to run
Assess fit or `p` to run Prepare documents. Claude Code should open with the selected skill
and Application folder. Review its proposed file changes; the rolecall Plugin writes results
to the Application folder for the TUI to display.

`prepare-documents` is available on Leads and Drafts. `assess-fit` is available for any
Application. If the keys are missing, confirm that Claude Code is installed and that Harness
is set in rolecall's Settings.
