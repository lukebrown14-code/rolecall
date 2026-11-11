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

## One-time migration from the dashboard era

`plugin/scripts/migrate.py` converts an older Workspace (job folders with `jd.md`
front matter plus `TRACKER.md`, `SHORTLIST.md` and `DISMISSED.md`) to SCHEMA v1.
It is a **one-time** step: after it runs, the TUI and the plugin skills own the
Workspace. Run it with the TUI closed.

Always print the plan first. `--dry-run` writes nothing:

```sh
python3 plugin/scripts/migrate.py --data /path/to/workspace --dry-run
python3 plugin/scripts/migrate.py --data /path/to/workspace
```

The plan lists every folder, key, Status, Fit conflict and archived file. The
apply step writes `meta.yml` through the same validating atomic writer as the
other scripts, removes `jd.md` front matter while keeping the body bytes, turns
`SHORTLIST.md` rows at `New` into Lead folders and `DISMISSED.md` rows into
Dismissed folders, then moves the five dashboard-era files to `jobs/_archive/`.
Nothing is deleted. A folder whose `meta.yml` already has `schema` is skipped,
so a second run changes nothing. Fit values that cannot be mapped are reported
and left unset for the Owner to set by hand.

## Tests

```sh
python3 -m pytest plugin/tests
ruff check plugin
```
