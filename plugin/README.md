# rolecall plugin

The skills and scripts a coding harness loads to work a rolecall Workspace:
creating the Profile, capturing jobs, assessing Fit, preparing documents, drafting follow-ups,
sweeping Sources and keeping the Profile's project facts current. The `help` skill answers
questions about the other skills and the app, from `skills/*/SKILL.md` and
`skills/help/app.md`; keep `app.md` in step when screens, keys or settings change.

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

The skills name their scripts as `${CLAUDE_PLUGIN_ROOT}/scripts/…`, because the Harness
runs from inside the Workspace, where a relative `plugin/scripts/` does not exist. Claude
Code fills the variable in; another harness should set it to this folder's absolute path.

## Search rules

`scripts/rules.py` applies the Workspace's `search_rules` and Source registry
(SCHEMA.md) to saved Applications. `check` is a dry run that lists the Leads a rule would
dismiss, the borderline ones it keeps and the Drafts it would remove; `check --apply`
dismisses the Leads in place with a `filter_reason` (`--include-drafts` for Drafts too).
`label "1mo ago"` shows how a posting-age label reads against the limit. The Sweep and
`meta.py create` apply the same rules to new roles.

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
Dismissed folders, then moves the five dashboard-era files to `jobs/_archive/`,
makes them read-only and writes a `README.md` there saying the folder is a read-only
archive the app never reads. Nothing is deleted. Running it on an already-migrated
Workspace adds the notice if it is missing. A folder whose `meta.yml` already has `schema` is skipped,
so a second run changes nothing. Fit values that cannot be mapped are reported
and left unset for the Owner to set by hand.

## Building documents

`build.py` turns an Application's `resume.md` and `cover-letter.md` into the
PDFs in `out/`. It needs:

- `pandoc` — Markdown to Typst markup.
- `typst` — Typst to PDF. The templates and the eight Hanken Grotesk fonts
  (plus their OFL licence) are shipped in `plugin/templates/`; nothing needs
  installing for fonts.
- `pdftotext` (poppler), optional — used for the ATS/contact check when present.

On macOS: `brew install pandoc typst poppler`. The build gates every document
before it compiles either one, and publishes PDFs into `out/` only when every
gate, build and ATS check has passed.

## Drift check

`python3 plugin/scripts/drift.py --data <workspace>` runs the build's fact rules against the
current Profile for every unsent Application (Lead or Draft) with a `resume.md` or
`cover-letter.md`, and lists the stale ones with the reasons. Exit 1 if any are stale, 0 if none;
`--json` for machine output. `--urls` also checks that every http(s) URL in those documents
resolves: 404 and 410 are dead, 403/429/999 are reported as unverified, and no network at all is a
warning. It is read-only. `build.py` runs the same URL check before building and fails only on a
dead link; `--skip-url-check` turns it off. Re-checking fit verdicts when evidence leaves the
Profile is not covered yet.

The style rules the letter gate reads are in `plugin/styles/au.md`.

## Tone layering

Every document is written by layering three files, in this order:

1. `plugin/writing-rules.md` — the general rules, shipped with the plugin.
2. `plugin/styles/au.md` — the Australian style guide, also shipped here.
3. `<workspace>/profile/writing-preferences.md` — the Owner's own preferences and facts.

**Later files win**: the style guide overrides the general rules, and the Owner's preferences
override both in tone and convention. The facts-only rule in `plugin/writing-rules.md` is
absolute and none of the three may override it.

## Workspace setup

Before the first document run, the Owner's `writing-preferences.md` must sit at
`<workspace>/profile/writing-preferences.md`. Move it there from wherever it lives today; do not
copy it into this repository. The code repo never contains this file — it is entirely personal
data.

## Tests

```sh
python3 -m pytest plugin/tests
ruff check plugin
```

The Python tests stub `pandoc`, `typst` and `pdftotext`, so they pass on a
bare Python install.

`internal/store/tests/build_integration_test.go` runs `build.py` for real. It skips with a
clear reason when `python3`, `pandoc` or `typst` is missing; the CI Go job
installs only Go and Python, so it skips there. Install the tools locally to
exercise it.

## Manual acceptance

The automated tests cannot see a real Workspace, so this check is manual. Run it
on a **copy** of the Workspace; only the migration's final apply touches the real
one.

### Documents

1. Pick two Applications that already have documents built by the dashboard
   era, ideally one resume-only and one with a cover letter.
2. Copy the Workspace and snapshot today's sources and PDFs:

   ```sh
   cp -a <workspace> <copy>
   mkdir -p /tmp/rolecall-before/<slug>
   cp <workspace>/jobs/<track>/<slug>/{resume.md,cover-letter.md} /tmp/rolecall-before/<slug>/
   cp -r <workspace>/jobs/<track>/<slug>/out /tmp/rolecall-before/<slug>/out
   ```

3. Regenerate the documents with the `prepare-documents` skill in the copy. This
   is the step that exercises the new tone layering; `build.py` alone does not
   rewrite Markdown.
4. Build and diff:

   ```sh
   python3 plugin/scripts/build.py <copy>/jobs/<track>/<slug> --data <copy>
   diff -u /tmp/rolecall-before/<slug>/resume.md <copy>/jobs/<track>/<slug>/resume.md
   diff -u /tmp/rolecall-before/<slug>/cover-letter.md <copy>/jobs/<track>/<slug>/cover-letter.md
   pdftotext <copy>/jobs/<track>/<slug>/out/*.pdf - | less
   ```

5. Expected: `build.py` passes every gate, every fact is unchanged, and the PDF
   text reads in the same order with the same content. Wording differences must
   be explainable by the tone layering; a gate failure or a changed fact is a
   defect. If a Profile entry (for example an editorial note in a `Dates` field)
   blocks a gate, fix the Profile, not the gate.

Record the two slugs and the result in the ticket.

### Migration

1. Make sure `<workspace>/workspace.json` lists the tracks, e.g.
   `{"tracks": ["SWE", "Analyst", "AI"]}`. Without it, SHORTLIST and DISMISSED
   rows are reported and skipped because they have no track column.
2. Back up the Workspace (`git commit` or `cp -a`), then rehearse on a copy:

   ```sh
   python3 plugin/scripts/migrate.py --data <copy> --dry-run
   python3 plugin/scripts/migrate.py --data <copy>
   go run ./cmd/rolecall --dir <copy>
   ```

3. Review the plan, then run the real dry run, apply, and confirm a second run
   reports `No changes.` A `meta.yml` with no `schema` that holds only
   `status_log` is treated as an incomplete pre-migration stub and rebuilt from
   `jd.md` and `TRACKER.md`; a malformed file with real fields still stops the
   plan before any write.
4. Check `jobs/_archive/` holds the five old tables, and record the result in the
   ticket.
