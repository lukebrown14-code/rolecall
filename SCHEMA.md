# Workspace schema

The contract between every writer of a rolecall workspace: the TUI (Go), the plugin scripts
(`build.py`, `sweep.py`, Python) and the harness agent following the plugin skills. If a
writer disagrees with this file, the writer is wrong. Both codebases test against the shared
fixtures in `fixtures/`.

Schema version: **1**. Terms (Application, Status, Lead, Fit…) are defined in `GLOSSARY.md`.

## Workspace layout

```
<workspace>/
├─ workspace.json              # tracks, gates, sweep sources
├─ brand.typ                   # optional document Template override (see below)
├─ profile/                    # Owner facts: read by the TUI and agent, written by the agent and Owner
│  ├─ master-profile.md
│  ├─ phrasing-bank.md
│  ├─ projects.md
│  └─ writing-preferences.md
├─ cv/                         # the Owner's general CV (optional)
└─ jobs/
   └─ <track>/
      └─ <slug>/               # one folder per Application
```

- A folder is an Application **if and only if** it contains `meta.yml`.
- Anything else under `jobs/` (stray files, `TRACKER.md` remnants) is ignored.
- `<track>` is where the folder was created. After that, the `track` key in `meta.yml` wins.
  Folders are never moved when the track changes.

### Folder name

`<slug>` = `<company>-<title>-<YYYY-MM-DD>`: lowercase, every run of characters outside
`[a-z0-9]` becomes `-`, trimmed, with the creation date. If the name is taken, append `-2`,
`-3`, and so on. The name is an identifier only. Never parse meaning out of it.

## Application folder

| File | Required | Written by | Meaning |
|---|---|---|---|
| `meta.yml` | ✅ | TUI, agent, `build.py` | the record (below) |
| `jd.md` | ✅ (may be empty) | TUI (Add), agent | job description, plain Markdown |
| `notes.md` | — | TUI (`$EDITOR`), Owner | free-text notes |
| `fit.md` | — | agent | the reasoning behind `fit` |
| `gaps.md` | — | agent | gaps found while writing |
| `resume.md` | — | agent | resume source (writing-rules Markdown format) |
| `cover-letter.md` | — | agent | cover letter source |
| `out/*.pdf` | — | `build.py` | built documents, upload-ready names |
| `sent/` | — | TUI (on → Applied) | frozen copy of what was sent. Never overwritten |

`jd.md` has no front matter. Older folders may still have it, and readers ignore it.

### Documents state

Worked out from files, never stored:

| State | Rule |
|---|---|
| **Ready** | `out/` holds a resume PDF, plus a cover letter PDF unless `resume_only: true` |
| **Written** | not Ready, and `resume.md` has non-whitespace text |
| *(none)* | otherwise |

PDF names come from `build.py`: `<Owner name> Resume <label>.pdf` and
`<Owner name> <label> Cover Letter.pdf`. To classify a file, readers match `Resume` /
`Cover Letter` in the name.

## `meta.yml`

### Example

```yaml
schema: 1
title: Software Engineer
company: UpGuard
track: SWE
url: https://jobs.ashbyhq.com/upguard/78833788
closes: 2026-12-01
fit: Strong
resume_only: false
status_log:
  - {status: Lead, at: 2026-09-24T01:00:00Z}
  - {status: Draft, at: 2026-09-24T03:12:00Z}
  - {status: Applied, at: 2026-09-25T00:30:00Z}
```

### Keys

| Key | Type | Required | Notes |
|---|---|---|---|
| `schema` | int | ✅ | `1`. A missing key means 1 (pre-migration files) |
| `title` | string | ✅ | role title as advertised |
| `company` | string | ✅ | |
| `track` | string | ✅ | one of `workspace.json` → `tracks`. Unknown values are shown, not rejected |
| `url` | string | — | ad URL. The Sweep's dedupe key |
| `closes` | date `YYYY-MM-DD` | — | application close date |
| `fit` | enum | — | `Strong` · `Good` · `Marginal` · `Poor`. Empty or missing = not assessed |
| `resume_only` | bool | — | `true` / `false`. Missing = `false` |
| `status_log` | list | ✅ | append-only history (below). The current Status is the **last** entry |

**Unknown keys must be preserved** by every writer, byte for byte. Legacy keys from the
dashboard era (`applied`, `applied_approx`, `followup`, `contact`, `salary`, `outcome`,
`outcome_date`, `rejection_reason`) fall under this rule. No writer adds them any more.

### `status_log`

Each entry is a flow map on one line:

```yaml
  - {status: <Status>, at: <RFC 3339 UTC, Z suffix>}
  - {status: <Status>, at: <date or timestamp>, approx: true}   # migration only
```

- **Status values:** `Lead` · `Draft` · `Applied` · `Interviewing` · `Offer` · `Rejected` ·
  `Withdrawn` · `Dismissed`. Case-sensitive.
- **Append only.** Entries are never edited or removed. Undo appends the earlier Status again.
- An entry is never appended if it repeats the current Status.
- An empty log (`status_log: []` or no entries) is read as `Lead`.
- **Applied date** = `at` of the **first** `Applied` entry. It's never stored separately.
- **Last change** = `at` of the last entry. Follow-up due is measured from it.
- `approx: true` marks a date rebuilt by migration. Readers show it with a `~` prefix.

### Who may change what

| Writer | May write | Must not |
|---|---|---|
| **TUI** | creates folders. Writes any key through Add / Edit. Appends any Status | touch docs, `fit.md`, `out/` |
| **Agent** (skills) | creates folders (capture, find). Writes `title` `company` `track` `url` `closes` `fit` `resume_only`, plus docs and `fit.md` | append to `status_log`, except a new folder's first `Lead` entry, or when the Owner explicitly asks in that session |
| **`build.py`** | `out/`. Appends `Draft` **only** when the current Status is `Lead` and the build produced PDFs | any other key or Status |

### Format rules (both languages)

The format is a strict subset of YAML, so Python can read it with no YAML library and Go can
patch it without reformatting.

1. Top-level lines: `key: value`. Key matches `[A-Za-z_][A-Za-z0-9_-]*`. No nesting except `status_log`.
2. Scalars may be plain or double-quoted. Inline `# comments` on scalar lines are allowed and kept.
3. `status_log:` is followed by entries indented two spaces, each `- {…}` on its own line.
4. UTF-8, LF line endings, trailing newline.
5. **Writers patch, they don't rewrite.** Re-read the file right before writing, change only the
   lines for the keys being set (append missing keys at the end, before `status_log`), and
   write atomically (temp file in the same folder, then rename).
6. A file that breaks these rules is **malformed**. Readers show the Application with a warning
   and its path. Writers refuse to touch it.

## Profile

`profile/` belongs to the Owner and the agent. The TUI reads it for the Profile screen and never
writes it. The Profile files are free Markdown, apart from the Contact table below.

### Contact table

The Contact is the `## Contact` section of `profile/master-profile.md`: a two-column table of
`| Field | Value |` rows. The section ends at the next `## ` heading or the end of the file.

```markdown
## Contact

| Field | Value |
| :-- | :-- |
| Name | Alex Quinn |
| Email | alex.quinn@example.test |
| Phone | +61 400 000 000 |
| Location | Brisbane, QLD |
| LinkedIn | linkedin.com/in/alexquinn |
| GitHub | github.com/alexquinn |
| Work rights | Full working rights in Australia *(subclass never printed)* |
```

- **Required:** `Name` and `Email`. A Profile missing either fails the build, and the Profile
  screen marks Contact with ✗ and says why.
- **Known fields:** `Phone`, `Location`, `LinkedIn`, `GitHub`, `Work rights`. Other fields are
  kept and shown, and the build ignores them.
- **Header and divider rows** (`Field`, and rows of only `:`, `-` and spaces) are skipped.
- **Cell values:** a trailing italic note `*(…)*` is a note to the human and is dropped.
  `**bold**`, `*italic*` and `` `code` `` wrappers are removed. Whitespace is trimmed.
- **Missing section or file:** no fields, so both required fields are missing.

## `workspace.json` (read by the TUI, which writes `tracks`, `search_area` and the gates)

The TUI reads `tracks` (array of strings, display order) and shows `search_area`, the gates
(`letterWordCap`, `letterWordMin`, `keySkillsMin`, `keySkillsMax`) and the number of `sources` and
`blockers` on the Settings screen. It writes `tracks` (add, reorder, remove, rename), `search_area` (the Search area
pop-up) and the four gates (edited on the Documents group; cap above 0 and the minimum, Key Skills minimum at
most the maximum): every other key keeps its value and its place, the file is indented with 2 spaces, the
write is atomic, and an invalid file is never overwritten. Missing file or key = no track
filter and the plugin's defaults.
Renaming a Track (`r` on the Settings screen, after a confirm) is the one Workspace-wide write: it
rewrites `track` in every readable `meta.yml` under `jobs/` (including `_archive`, using the
`meta.yml` write rules), then `tracks`, then the role `track` values in `jobs/roles.json`. Folders
never move, malformed `meta.yml` files are skipped and named afterwards, and `u` does not undo it. An invalid file is shown, with its parse error, on the Settings
screen. `sources`, `blockers` and the gates are used by the plugin.

`search_area` (read by the Sweep and `find-jobs`) is where roles must be:

| Field | Type | Meaning |
|---|---|---|
| `places` | array of strings | countries, regions, states or cities, e.g. `Australia`, `APAC`, `Sydney`. Matched ignoring case |
| `remote` | `in-places` \| `anywhere` \| `no` | whether remote roles count. Default `in-places`: a remote role is kept when it names a Place or no known place |

A missing `search_area` means no filter: the Sweep keeps every location and reports
`no Search area set: all locations kept`. A built-in gazetteer (`plugin/gazetteer.json`) maps
countries to their states and major cities, so `Australia` covers `Sydney, NSW`. A role is kept
if it names a Place, dropped if it names only known places outside the Places, and kept if its
location is empty or unknown. `--all-locations` bypasses the filter.

## `brand.typ` (optional document Template override)

The Plugin's default document Template is `plugin/templates/brand.typ`. To customize the
resume and cover-letter look for a Workspace, copy that file to `<workspace>/brand.typ` and
edit the copy. The next Build uses the Workspace file; if it is absent, the Plugin default is
used. Keep the `#let` names imported by `resume.typ` and `cover-letter.typ` when editing, or
Typst may fail to compile the documents.

## Fixtures

`fixtures/workspace/` holds a fictional workspace that covers: every Status; Ready, Written and
no docs; `resume_only`; unknown keys; `approx` entries; an empty log; one malformed
`meta.yml`; and a Profile with a Contact table. The Go store tests and the Python `build.py` tests
both load it. Any change to this
file updates the fixtures in the same commit.

`fixtures/dashboard-era/` is the dashboard-era migration fixture: tracker, shortlist and dismissed
tables, jd front matter, legacy keys, bulk-stamped logs and an unmappable Fit. It is used by
`plugin/tests/test_migrate.py` and `store/migrate_integration_test.go`. It is input to the
one-time migration only and is not scanned by the store.
