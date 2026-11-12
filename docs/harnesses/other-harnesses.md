# Codex, Gemini CLI and opencode

These Harnesses can load Agent Skills directly from disk. This guide links the skills bundled
with a Homebrew installation of rolecall into the shared `~/.agents/skills` directory.

## Install rolecall and link its skills

```sh
brew install lukebrown14-code/homebrew-tap/rolecall

plugin="$(brew --prefix rolecall)/libexec/plugin"
mkdir -p "$HOME/.agents/skills"
for skill in "$plugin"/skills/*; do
  ln -s "$skill" "$HOME/.agents/skills/$(basename "$skill")"
done
```

This links each rolecall skill folder, including its `SKILL.md` and supporting files. The
Harness will discover the skills for new sessions. If a link already exists, inspect it
before replacing it.

## Configure a Harness

In rolecall's Settings screen (`6`), set **Harness** to the example for your CLI. Or put the
same value under `harness` in `~/.config/rolecall/config.toml`.

### Codex

```toml
harness = 'codex exec --cd {dir} "Use the {skill} skill for this rolecall Application."'
```

### Gemini CLI

```toml
harness = 'cd {dir} && gemini -p "Use the {skill} skill for this rolecall Application."'
```

### opencode

```toml
harness = 'cd {dir} && opencode run "Use the {skill} skill for this rolecall Application."'
```

rolecall replaces `{skill}` with `assess-fit` or `prepare-documents`, and `{dir}` with the
selected Application folder. Press `f` on an Application to run Assess fit, or `p` on a Lead
or Draft to run Prepare documents. Start a new Harness session if it was already open when
you linked the skills.

## Check the handoff

Open a Lead or Draft Application in rolecall and press `f`, then `p`. The Harness should
load the named skill and use that Application folder. Review its proposed changes; the
rolecall Plugin writes results to the Application folder for the TUI to display.

If `f` or `p` is missing, confirm that the Harness command is installed and that **Harness**
is set in rolecall's Settings. If the Harness does not list the skills, check that each
`~/.agents/skills/<skill>` entry points to the matching directory under the installed
Plugin's `skills/` folder, then start a new session.
