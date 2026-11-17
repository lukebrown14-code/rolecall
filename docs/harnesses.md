# Connect a Harness

The Harness is your coding assistant. rolecall installs the Plugin next to its command; a
Harness reads the Plugin's skills, and rolecall's `f`, `p` and `F` keys hand off to it through
the `harness` command in `config.toml`. The guide for Claude Code comes first; Codex, Gemini
CLI and opencode follow.

## Find the Plugin

| Installed with | Plugin folder |
|---|---|
| Install script | `~/.local/share/rolecall/plugin` (or `$ROLECALL_PREFIX/share/rolecall/plugin`) |
| `go install` | the `plugin` folder of a clone of this repository |

`rolecall doctor` prints the Plugin path it found, on the `Plugin path` line.

## The `harness` command

Open rolecall's Settings screen (`6`) and set **Harness**, or set `harness` in
`~/.config/rolecall/config.toml`. rolecall replaces `{skill}` with `assess-fit` (`f`),
`prepare-documents` (`p`) or `draft-follow-up` (`F`), and `{dir}` with the selected Application
folder. It runs the command through `/bin/sh -c`, starting in your Workspace.

Keep the single quotes around the TOML value.

## Claude Code

### Install the Plugin

Add the Plugin folder as a local marketplace and install the `rolecall` Plugin. For the
install script:

```sh
claude plugin marketplace add ~/.local/share/rolecall/plugin
claude plugin install rolecall@rolecall
```

The marketplace name and Plugin name are both `rolecall`. If Claude Code is already open,
restart it after installing the Plugin.

If `install` says the Plugin is not found in marketplace `rolecall`, an older marketplace with
that name points somewhere else. Remove it with `claude plugin marketplace remove rolecall` and
add the folder again.

### Update the Plugin

After updating rolecall, refresh the marketplace so Claude Code picks up the new skills:

```sh
claude plugin marketplace update rolecall
```

### Configure rolecall

```toml
harness = 'claude "/rolecall:{skill} {dir}"'
```

Claude Code opens with the Workspace as its project, so it can edit the Application folder.
The Plugin's skills appear as `/rolecall:assess-fit`, `/rolecall:prepare-documents`,
`/rolecall:draft-follow-up` and the rest.

### Work in the Workspace yourself

To use the skills outside a handoff, for example to build your Profile or capture a role,
open Claude Code in the Workspace:

```sh
cd /path/to/your/workspace && claude
```

Keep these sessions out of code repositories: the Workspace holds your personal data.

To ask how rolecall or any of its skills works, run `/rolecall:help` (or just ask: "which
rolecall skill finds jobs?"). It reads the skills and the app guide and changes nothing.

## Codex, Gemini CLI and opencode

These Harnesses load Agent Skills directly from disk. Link the Plugin's skills into the shared
`~/.agents/skills` directory, with `plugin` set to the Plugin folder from the table above:

```sh
plugin="$HOME/.local/share/rolecall/plugin"
mkdir -p "$HOME/.agents/skills"
for skill in "$plugin"/skills/*; do
  ln -s "$skill" "$HOME/.agents/skills/$(basename "$skill")"
done
```

This links each skill folder, including its `SKILL.md` and supporting files, and the Harness
discovers them in new sessions. If a link already exists, inspect it before replacing it.
Updating rolecall keeps the Plugin at the same path, so the links keep working.

Then set `harness` for your CLI:

```toml
# Codex
harness = 'codex exec --cd {dir} "Use the {skill} skill for this rolecall Application."'
# Gemini CLI
harness = 'cd {dir} && gemini -p "Use the {skill} skill for this rolecall Application."'
# opencode
harness = 'cd {dir} && opencode run "Use the {skill} skill for this rolecall Application."'
```

The command starts in your Workspace; these examples move into the Application folder with
`--cd` or `cd`. Start a new Harness session if it was already open when you linked the skills.

## Check the handoff

Start rolecall with your Workspace and open an Application: press `f` to run Assess fit, `p`
to run Prepare documents (Leads and Drafts) or `F` to draft a follow-up (an Application with
Follow-up due). The Harness should open with the named skill and that Application folder.
Review its proposed changes; the Plugin writes results to the Application folder for the app
to display.

If a key says `No Harness set`, confirm that the Harness command is installed and that
**Harness** is set in Settings. If the Harness does not list the skills, check that each
`~/.agents/skills/<skill>` entry points to the matching folder under the installed Plugin's
`skills/`, then start a new session.
