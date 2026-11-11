# rolecall plugin

The skills and scripts a coding harness loads to work a rolecall Workspace:
capturing jobs, assessing Fit, preparing documents, sweeping sources and
keeping the Profile's project facts current.

## Registering the local marketplace (Claude Code)

Register this directory as a **local-path marketplace**, not an installed
package. Skill and script edits then take effect the next time the skill runs,
with no reinstall step:

```sh
claude plugin marketplace add /path/to/rolecall/plugin
claude plugin install rolecall@rolecall
```

The marketplace manifest is `.claude-plugin/marketplace.json`; the plugin
manifest is `.claude-plugin/plugin.json`. Both are plain JSON with no build
step. If rolecall lives in a worktree, register that worktree's `plugin/`
directory; re-run `marketplace add` after moving it.

## Other harnesses

Nothing here is Claude-specific except the two manifest files. Any harness can
read `plugin/skills/<name>/SKILL.md` by path, and the Python scripts under
`plugin/scripts/` take the Workspace from `--data <path>` or
`ROLECALL_WORKSPACE`. The code repo holds no Workspace data.

`sync-github.py` also needs the GitHub account and its in-scope repositories,
which are personal data: set `ROLECALL_GITHUB_OWNER` and
`ROLECALL_GITHUB_REPOS` (comma-separated) rather than committing them here.

## Building documents

`build.py` turns an Application's `resume.md` and `cover-letter.md` into the
PDFs in `out/`. It needs:

- `pandoc` — Markdown to Typst markup.
- `typst` — Typst to PDF. The templates and the eight Hanken Grotesk fonts
  (plus their OFL licence) are shipped in `plugin/templates/`; nothing needs
  installing for fonts.
- `pdftotext` (poppler), optional — used for the ATS/identity check when present.

On macOS: `brew install pandoc typst poppler`. The document gates need none of
these: `python3 plugin/scripts/build.py <job_dir> --check` reads Markdown,
`meta.yml` and the Profile only, compiles nothing and writes no `out/`.

The style rules the letter gate reads are in `plugin/styles/au.md`.

## Tests

```sh
python3 -m pytest plugin/tests
ruff check plugin
```

The Python tests stub `pandoc`, `typst` and `pdftotext` (or run `--check`,
which needs none of them), so they pass on a bare Python install.

`store/build_integration_test.go` runs `build.py` for real. It skips with a
clear reason when `python3`, `pandoc` or `typst` is missing; the CI Go job
installs only Go and Python, so it skips there. Install the tools locally to
exercise it.
