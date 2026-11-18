# Connect a Harness

The Harness is your coding assistant. It reads the Plugin's skills, and rolecall's `f`, `p` and
`F` keys hand off to it through the `harness` command in `config.toml`.

## Find the Plugin

| Installed with | Plugin folder |
|---|---|
| Install script | `~/.local/share/rolecall/plugin` (or `$ROLECALL_PREFIX/share/rolecall/plugin`) |
| `go install` | the `plugin` folder of a clone of this repository |

`rolecall doctor` prints the path it found on the `Plugin path` line.

## The `harness` command

Set Harness on the Settings screen (`6`), or `harness` in `~/.config/rolecall/config.toml`
(keep the single quotes around the TOML value). rolecall replaces `{skill}` with `assess-fit`
(`f`), `prepare-documents` (`p`) or `draft-follow-up` (`F`), and `{dir}` with the selected
Application folder, then runs the command through `/bin/sh -c` from your Workspace.

The folder never goes into the command text. rolecall passes it to the shell as `$1` (and the
skill as `$2`), and writes each placeholder as a quoted reference to that value: `"${1}"`
when the placeholder is bare, `${1}` inside double quotes, and `'"${1}"'` inside single
quotes. A folder name with spaces, quotes, `$(...)` or backticks therefore reaches the
Harness as written, and nothing in it runs. You can put `{dir}` bare or inside either kind
of quotes; don't put it inside a `$(...)` or backtick substitution of your own.

## Claude Code

### Install the Plugin

Add the Plugin folder as a local marketplace and install the Plugin. For the install script:

```sh
claude plugin marketplace add ~/.local/share/rolecall/plugin
claude plugin install rolecall@rolecall
```

Restart Claude Code if it was open.

If `install` says the Plugin is not found in marketplace `rolecall`, an older marketplace of
that name points elsewhere. Run `claude plugin marketplace remove rolecall` and add the folder
again.

### Update the Plugin

After updating rolecall, refresh the marketplace:

```sh
claude plugin marketplace update rolecall
```

### Configure rolecall

```toml
harness = 'claude "/rolecall:{skill} {dir}"'
```

Claude Code opens with the Workspace as its project. The skills appear as
`/rolecall:assess-fit`, `/rolecall:prepare-documents`, `/rolecall:draft-follow-up` and so on.

### Work in the Workspace yourself

To use the skills outside a handoff, for example to build your Profile or capture a role, open
Claude Code in the Workspace. Keep these sessions out of code repositories, since the Workspace
holds your personal data.

```sh
cd /path/to/your/workspace && claude
```

Run `/rolecall:help` (or ask, "which rolecall skill finds jobs?") to learn how rolecall or a
skill works. It changes nothing.

## Codex, Gemini CLI and opencode

These Harnesses load Agent Skills from disk. Link each skill folder into `~/.agents/skills`,
with `plugin` set to the Plugin folder from the table above:

```sh
plugin="$HOME/.local/share/rolecall/plugin"
mkdir -p "$HOME/.agents/skills"
for skill in "$plugin"/skills/*; do
  ln -s "$skill" "$HOME/.agents/skills/$(basename "$skill")"
done
```

If a link already exists, inspect it before replacing it. Updates keep the Plugin at the same
path, so the links keep working. Then set `harness` for your CLI:

```toml
# Codex
harness = 'codex exec --cd {dir} "Use the {skill} skill for this rolecall Application."'
# Gemini CLI
harness = 'cd {dir} && gemini -p "Use the {skill} skill for this rolecall Application."'
# opencode
harness = 'cd {dir} && opencode run "Use the {skill} skill for this rolecall Application."'
```

Start a new Harness session if one was open when you linked the skills.

## Check the handoff

Open an Application in rolecall and press `f` (Assess fit), `p` (Prepare documents, for Leads
and Drafts) or `F` (draft a follow-up, when Follow-up is due). The Harness should open with that
skill and Application folder. Review its changes; results land in the Application folder for
the app to show.

- `No Harness set`: check the Harness command is installed and Harness is set in Settings.
- Skills not listed: check each `~/.agents/skills/<skill>` points to the matching folder under
  the installed Plugin's `skills/`, then start a new session.
