# rolecall plugin

The skills and scripts a coding harness loads to work a rolecall Workspace. The `help` skill
answers questions about the other skills and the app from `skills/*/SKILL.md` and
`skills/help/app.md`; keep `app.md` in step when screens, keys or settings change.

## Registering the local marketplace (Claude Code)

Register this directory as a local marketplace; edits then take effect on the next skill run
with no reinstall:

```sh
claude plugin marketplace add /path/to/rolecall/plugin
claude plugin install rolecall@rolecall
```

The manifests, `.claude-plugin/marketplace.json` and `.claude-plugin/plugin.json`, are plain
JSON with no build step. In a worktree, register that worktree's `plugin/`; re-run
`marketplace add` after moving it.

## Other harnesses

Only the two manifests are Claude-specific. Any harness can read
`plugin/skills/<name>/SKILL.md` by path, and the scripts in `plugin/scripts/` take the
Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. The skills call scripts as
`${CLAUDE_PLUGIN_ROOT}/scripts/…` because the Harness runs inside the Workspace; Claude Code
sets the variable, and another harness should set it to this folder's absolute path. See
[docs/harnesses.md](../docs/harnesses.md).

## Skills

| Skill | Use it to | Writes |
| --- | --- | --- |
| `create-profile` | build or update the Profile from a resume or an interview | `profile/` |
| `capture-job` | save a job ad (a URL, a file or pasted text) as an Application | `meta.yml`, `jd.md` |
| `find-jobs` | sweep the Sources and save the roles worth a look as Leads | Lead folders |
| `assess-fit` | judge an Application against the Profile | `fit.md`, `fit` |
| `prepare-documents` | write the tailored resume, cover letter and gaps, and build them | `resume.md`, `cover-letter.md`, `gaps.md`, `out/` |
| `draft-follow-up` | draft a nudge for a sent Application | `follow-up.md` |
| `import-applications` | bring in applications the Owner already made, from a CSV | Application folders |
| `interview-prep` | prepare for an interview: questions, a STAR story per gap, questions to ask | `prep.md` |
| `answer-questions` | answer an employer's application-form questions | `answers.md` |
| `help` | answer questions about rolecall | nothing |

`prep.md` and `answers.md` are optional extra files (SCHEMA.md): the app does not read them and
they do not change an Application's Documents or Ready state.

## Importing an existing list

`scripts/import_csv.py` imports a CSV or TSV export. The `import-applications` skill drives
it, or run it by hand:

```sh
python3 plugin/scripts/import_csv.py --data <workspace> --file export.csv --inspect
python3 plugin/scripts/import_csv.py --data <workspace> --file export.csv --track SWE --dry-run
python3 plugin/scripts/import_csv.py --data <workspace> --file export.csv --track SWE
```

- `--inspect` shows the matched columns (title, company, status, date applied, URL, notes) and
  the status mapping. Fix them with `--map field=Column` and `--status-map "Value=Status"`.
- An unmapped status stops the import.
- Rows become folders through `meta.create` with date-only, `approx: true` status entries.
- A duplicate of a saved Application or an earlier row is skipped (`--allow-duplicate`
  overrides). `--dry-run` writes nothing.

## Search rules

`scripts/rules.py` applies `search_rules` and the Source registry (SCHEMA.md) to saved
Applications; the Sweep and `meta.py create` apply them to new roles.

- `check`: dry run listing the Leads a rule would dismiss, borderline ones it keeps and Drafts
  it would remove.
- `check --apply`: dismisses those Leads with a `filter_reason` (`--include-drafts` for Drafts).
- `label "1mo ago"`: shows how a posting-age label reads against the limit.

## GitHub sync

`sync-github.py` needs the GitHub account and its in-scope repositories. They are personal
data, so set `ROLECALL_GITHUB_OWNER` and `ROLECALL_GITHUB_REPOS` (comma-separated) instead of
committing them.

## Fetching an ad

`new-job.py --url` fetches the raw page through `httpfetch` (http and https only, size-capped).
It follows a known aggregator listing (Jora, Adzuna, Jooble ...) to the employer's posting,
which becomes `url`, and keeps the listing as `aggregator_url`. `--fetch-only` prints the ad as
JSON. On failure its last stderr line is `{"error": "...", "kind": "..."}`, where `kind` is
`login_wall`, `blocked`, `not_found`, `timeout`, `bad_url`, `non_html`, `unreachable`,
`thin_page` or `too_short`; the Add form shows a plain-language message for each.

## One-time migration from the dashboard era

`plugin/scripts/migrate.py` converts an older Workspace (job folders with `jd.md` front matter
plus `TRACKER.md`, `SHORTLIST.md` and `DISMISSED.md`) to SCHEMA v1. Close the TUI and print the
plan first; `--dry-run` writes nothing:

```sh
python3 plugin/scripts/migrate.py --data /path/to/workspace --dry-run
python3 plugin/scripts/migrate.py --data /path/to/workspace
```

It writes `meta.yml` through the shared validating writer, keeps the `jd.md` body bytes and
moves the dashboard-era files to a read-only `jobs/_archive/` with a `README.md` notice. Nothing
is deleted, and a rerun skips folders that have `schema` (it adds a missing archive notice).
Unmappable Fit values are reported for the Owner to set. The full mapping is in
[docs/history.md](../docs/history.md#migration-one-time-migratepy).

## Building documents

`build.py` turns an Application's `resume.md` and `cover-letter.md` into PDFs in `out/`. It
needs `pandoc` and `typst`, and uses `pdftotext` (poppler) for the ATS/contact check when
present. On macOS: `brew install pandoc typst poppler`. The templates and Hanken Grotesk fonts
(with their OFL licence) ship in `plugin/templates/`.

The build gates every document before compiling either one, and publishes to `out/` only when
every gate, build and ATS check has passed.

## Drift check

`python3 plugin/scripts/drift.py --data <workspace>` (read-only) runs the build's fact rules
against the current Profile for each Lead or Draft with a `resume.md` or `cover-letter.md`, and
lists the drifted ones with reasons. Exit 1 if any drifted; `--json` for machine output.

`--urls` also checks each http(s) URL: 404 and 410 are dead, 403/429/999 unverified, no network
a warning. `build.py` runs the same check, fails only on a dead link, and skips it with
`--skip-url-check`. Fit is not yet re-checked when evidence leaves the Profile.

## Tone layering

Every document layers three files; later files win on tone and convention:

1. `plugin/writing-rules.md`: the general rules.
2. `plugin/styles/au.md`: the Australian style guide, also read by the letter gate.
3. `<workspace>/profile/writing-preferences.md`: the Owner's preferences and facts. It is
   personal data and never lives in this repository.

No file may override the facts-only rule in `plugin/writing-rules.md`.

## Script modules

The skills run only the CLIs. Each one puts `plugin/scripts/` on `sys.path`, so it imports
the shared modules beside it from any working directory. A module name must be a valid
Python identifier, which is why shared code never lives in `new-job.py` or `set-fit.py`.

- `meta.py` is the meta.yml CLI and the one import other scripts use. It re-exports
  `metayaml` (the line grammar), `metaread` (reading Applications), `metawrite` (the
  validating writers) and `followup` (Follow-up due).
- `sweep.py` is the Sweep CLI. Its parts are `boards` (the collectors), `sourceregistry`,
  `rolecatalogue` (`jobs/roles.json`), `searcharea`, `captured` (what the Workspace holds)
  and `sweepreport`.
- `workspace`, `workspaceio`, `httpfetch`, `signals`, `adfacts`, `aggregator`, `contact`
  and `urlcheck` are shared by several scripts.

## Tests

See [CONTRIBUTING.md](../CONTRIBUTING.md#run-the-checks). The Python tests stub `pandoc`,
`typst` and `pdftotext`; `internal/store/tests/build_integration_test.go` runs `build.py` for
real and skips when `python3`, `pandoc` or `typst` is missing.
