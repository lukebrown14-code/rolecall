# Workspace schema

The contract between every writer of a Rolecall workspace: the TUI (Go), the plugin scripts
(`build.py`, `sweep.py`, Python) and the harness agent following the plugin skills. If a
writer disagrees with this file, the writer is wrong. Both codebases test against the shared
fixtures in `fixtures/`.

Schema version: 1. A missing `schema` in `workspace.json` or `meta.yml` means 1 for
pre-schema files. Readers refuse a newer schema and tell the Owner to upgrade Rolecall;
they must not interpret files written for a format they do not know. A `meta.yml` with a
newer schema is shown as a warning row with the upgrade detail. Terms (Application, Status, Lead, Fit…) are
defined in `GLOSSARY.md`.

## Workspace layout

```
<workspace>/
├─ workspace.json              # tracks, gates, Search rules, Source registry
├─ templates/                  # optional document Template overrides (see below)
│  ├─ brand.typ
│  ├─ resume.typ
│  └─ cover-letter.typ
├─ brand.typ                   # older spot for the brand.typ override (see below)
├─ profile/                    # Owner facts: read by the TUI and agent, written by the agent and Owner
│  ├─ master-profile.md
│  ├─ phrasing-bank.md
│  ├─ projects.md
│  └─ writing-preferences.md
├─ cv/                         # CVs: general resumes for no Application (see below)
│  ├─ general.md
│  └─ out/<name>/              # built CV PDFs
└─ jobs/
   ├─ roles.json               # optional role catalogue (see below)
   ├─ sweep-candidates.md      # raw Sweep output; not an Application (see below)
   ├─ _archive/                # read-only dashboard-era files; never read (see below)
   └─ <track>/
      └─ <slug>/               # one folder per Application
```

- A folder is an Application if and only if it contains `meta.yml`.
- Any other file directly under `jobs/` that this schema does not name (stray files,
  `TRACKER.md` remnants) is ignored. `roles.json` and `sweep-candidates.md` are named and
  documented below.
- `jobs/_archive/` holds files a migration retired. No reader opens it (a Track rename
  still rewrites `meta.yml` files there). `migrate.py` makes the files read-only and writes
  `jobs/_archive/README.md`, a notice that the folder is a read-only archive.
- `<track>` is where the folder was created. After that, the `track` key in `meta.yml` wins.
  Folders are never moved when the track changes.

### Folder name

`<slug>` = `<company>-<title>-<YYYY-MM-DD>`: lowercase, every run of characters outside
`[a-z0-9]` becomes `-`, trimmed, cut to at most 60 characters at a `-`, with the creation
date in the Owner's time zone. Each character is lowercased on its own (`İ` is `i`, `K` (Kelvin) is `k`); when the
first 61 characters hold no `-`, the cut keeps the first 60. When nothing is left (a CJK or symbol-only name) it is `application`. If the name is taken, append `-2`,
`-3`, and so on. The name is only an identifier; never parse meaning out of it.

## Application folder

| File | Required | Written by | Meaning |
|---|---|---|---|
| `meta.yml` | ✅ | TUI, agent, `build.py` | the record (below) |
| `jd.md` | ✅ (may be empty) | TUI (Add), agent | job description, plain Markdown |
| `notes.md` | — | TUI (`$EDITOR`), Owner | free-text notes |
| `fit.md` | — | agent | the reasoning behind `fit`. Its `## Gaps` section (any `##` heading starting `Gaps`) lists one gap per `- ` bullet, name first, then ` — ` and the detail; Insights counts the names |
| `gaps.md` | — | agent | gaps found while writing |
| `follow-up.md` | — | agent | a drafted follow-up message for the Owner to send |
| `prep.md` | — | agent (`interview-prep`) | interview preparation: likely questions, a STAR story from the Profile per gap, questions to ask. Additive: not a Document, not read by the TUI, no Ready or Written state depends on it |
| `answers.md` | — | agent (`answer-questions`) | answers to the employer's application-form questions, from `jd.md` and the Profile. Additive: as `prep.md` |
| `resume.md` | — | agent | resume source (writing-rules Markdown format) |
| `cover-letter.md` | — | agent | cover letter source |
| `out/*.pdf` | — | `build.py` | built documents, upload-ready names |
| `sent/` | — | TUI and `rolecall move` (on → Applied) | frozen copy of what was sent: the regular files among the docs above and `out/*.pdf` (a symlink is not copied), private (`0700`). Built in a `sent.tmp-*` folder and renamed into place whole, so a failed copy leaves no `sent/` and moves nothing. Never overwritten |
| `sent.tmp-*/` | — | TUI and `rolecall move` (on → Applied) | `sent/` while it is being copied. Only a crash leaves one; never read, safe to delete |
| `sent.undone-<YYYY-MM-DD>/` | — | TUI (undo of → Applied) | a `sent/` renamed aside when the move that made it is undone, so the next real Applied freezes a fresh `sent/`. `-2`, `-3`… when taken. Not read |

`jd.md` has no front matter. Older folders may still have it, and readers ignore it.

### Documents state

Worked out from files, never stored:

| State | Rule |
|---|---|
| **Ready** | `out/` holds a resume PDF, plus a cover letter PDF unless `resume_only: true` |
| **Written** | not Ready, and `resume.md` has non-whitespace text |
| *(none)* | otherwise |

The Documents are the Resume and, unless `resume_only: true`, the Cover letter (GLOSSARY →
Document). Each has two steps the TUI counts, "○ 2/4 ready": Written (`resume.md` /
`cover-letter.md` has non-whitespace text) and Built (its PDF is in `out/`). Every needed
Document Built is Ready. `jd.md` and `gaps.md` are inputs and never counted.

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
next_step: {at: 2026-12-03T10:30, what: "Phone screen with the hiring manager"}
contacts:
  - {name: "Sam Lee", role: "Recruiter", email: sam.lee@example.test}
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
| `company` | string | ✅ | the real employer. For an agency ad, the client the recruiter is hiring for |
| `track` | string | ✅ | one of `workspace.json` → `tracks`. Unknown values are shown, not rejected |
| `url` | string | — | ad URL. The Sweep's dedupe key |
| `aggregator_url` | string | — | the aggregator listing the ad was found on, when `url` is the employer's own posting that the listing links to. Written by `new-job.py`; `url` stays the dedupe key |
| `closes` | date `YYYY-MM-DD` | — | application close date |
| `fit` | enum | — | `Strong` · `Good` · `Marginal` · `Poor`. Empty or missing = not assessed |
| `resume_only` | bool | — | `true` / `false`. Missing = `false` |
| `posted` | date `YYYY-MM-DD` | — | when the ad was posted, when known. Ages a Lead for the Search rules |
| `posted_label` | string | — | the age exactly as the Source showed it (`14d ago`, `Posted 30+ Days Ago`, `1mo ago`). Read when `posted` is absent |
| `applicants` | int ≥ 0 | — | the applicant count the Source showed, when it shows one |
| `source` | string | — | the Source registry `name` the Lead came from (`linkedin`, `greenhouse`) |
| `filter_reason` | enum | — | why a Search rule dismissed it: `expired` · `closed` (its `closes` date passed) · `pulled` (the ad is gone or says closed) · `duplicate` · `blocker` · `applicant_cap` · `location` · `contract_length`. Set with the `Dismissed` entry it explains |
| `filter_note` | string | — | the detail behind `filter_reason` ("Posted 30+ Days Ago; limit 30 days") |
| `filter_overruled` | bool | — | `true` once the Owner restored a rule-dismissed Application. No rule dismisses it again |
| `recruiter` | string | — | the agency that posted the ad, when it is not the employer. Shown as `AGL via Chandler Macleod` |
| `employment_type` | enum | — | `permanent` · `contract` · `fixed-term` · `casual` · `part-time` |
| `term_months` | int ≥ 1 | — | the contract or fixed term's length |
| `extension` | enum | — | `yes` · `no` · `possible` |
| `sponsorship` | enum | — | what the ad says about visa sponsorship: `offered` · `not-offered` · `unknown` |
| `pay_min` · `pay_max` | number ≥ 0 | — | the pay figures as the ad states them (`95`, `95.5`). `pay_max` absent means a single figure |
| `pay_unit` | enum | — | `hour` · `day` · `week` · `month` · `year` |
| `currency` | string | — | ISO 4217 code in capitals: `AUD`, `USD` |
| `closes_unclear` | bool | — | `true` when the ad has a closing date that could not be read. `closes` stays empty rather than guessed |
| `jd_completeness` | enum | — | what `jd.md` holds: `full` · `summary` (condensed) · `blurb` (a snippet; assess-fit asks for the full ad first) |
| `added_by` | enum | — | who saved it: `sweep` · `agent` · `owner` (the TUI's Add writes `owner`) |
| `search_query` | string | — | the search that surfaced it |
| `listing` | enum | — | whether the ad was still up when last checked: `live` · `gone` · `closed` |
| `listing_checked` | date `YYYY-MM-DD` | — | when `listing` was last checked (`rules.py recheck`) |
| `followed_up` | date `YYYY-MM-DD` | — | the day the Owner last followed up on a sent Application without its Status changing. Written by the TUI (`l`, the Owner's action; its undo removes it or puts back the day before). Follow-up due counts from the later of this and the last change. A value that is not a date reads as unset (the file is not malformed). Not the legacy `followup` key |
| `next_step` | flow map `{at, what}` | — | what the Owner does next and when. `at` is `YYYY-MM-DD` or `YYYY-MM-DDTHH:MM` (local time, no zone) and required; `what` is text, quoted when it holds a comma, brace or `#`. One line, as a `status_log` entry is. Empty or missing = none. A map without a valid `at` makes the file malformed. Shown on Applied, Interviewing and Offer rows, in Stats → Now and on the detail Overview; `rolecall now` and `rolecall export` read it |
| `contacts` | list of flow maps | — | the people to write to, one `  - {name, role, email, url}` line each, indented two spaces below `contacts:` (as `status_log` is). Each needs a `name`, an `email` or a `url`; `role` is optional; other inner keys are ignored. `contacts: []` or no key = none. The first is who a follow-up addresses. The old single `contact` still reads, as the one Contact, until the Owner saves a list |
| `close_reason` | string | — | why a Rejected or Withdrawn Application ended. Suggested values: `no-response` · `after-screen` · `after-interview` · `salary` · `role-closed` · `other`; free text is allowed. Set from the Move pop-up when the move is to Rejected or Withdrawn (optional, skippable), or later from Edit. Shown only while the Status is Rejected or Withdrawn; the TUI removes it when the Application leaves them (an undo included), and an undo of that move puts it back. Stats shows where Applications dropped off by it |
| `status_log` | list | ✅ | append-only history (below). The current Status is the last entry |

Readers estimate an annual AUD pay figure as pay × the unit's multiplier (hour 1976, day
230, week 52, month 12, year 1) × the currency's AUD rate (built in: AUD 1, USD 1.55, GBP 1.95,
EUR 1.65, NZD 0.92, CAD 1.12, SGD 1.15; `workspace.json` `pay_fx` overrides or adds codes, each a
3-letter capital code with a rate above 0). No `currency` reads as AUD; an unknown one has no
estimate. A figure that looks wrong for its unit is flagged as mislabelled, never changed: an
hourly `pay_min` of 300 or more looks like a day rate, a daily one of 5000 or more like an
annual figure, and an annual `pay_max` under 1000 like an hourly or daily rate. The flag is
set even when there is no estimate.

**Likely duplicate.** Two Applications are likely the same role when their titles match after
normalising (lower case, a trailing `(…)` dropped, punctuation to spaces, the words `level`,
`the`, `a` and `an` dropped) and they share an employer (`company` or `recruiter`, lower case
with `pty`, `ltd`, `limited`, `inc`, `group` and `australia` dropped), or when their `jd.md`
texts match (sha256 of the text lower-cased with whitespace collapsed, for texts of 200
characters or more). `meta.py create` and `new-job.py` refuse a likely duplicate unless given
`--allow-duplicate`; the TUI's Add asks before saving one; `rules.py check` dismisses the later
of two duplicate Leads; the Sweep skips a role whose employer and title are already saved.

A **Filtered** Application is one whose Status is `Dismissed` and that has a `filter_reason`.
The TUI lists it under Leads → Filtered with its reason. Restoring it (`m`, back to the
earlier Status) also writes `filter_overruled: true` and removes `filter_reason` and
`filter_note`. An invalid `posted`, `applicants`, `filter_reason` or `filter_overruled` makes the
file malformed, as an invalid `closes` does.

Every writer must preserve unknown keys, byte for byte. Legacy keys from the dashboard era
(`applied`, `applied_approx`, `followup`, `contact`, `salary`, `outcome`, `outcome_date`,
`rejection_reason`) fall under this rule, except that the TUI drops `contact` when the Owner
saves a `contacts` list (its value is read into the list first). No writer adds them any
more. The legacy `followup` is unrelated to `followed_up` and is never read.

`next_step`, `contacts` and `close_reason` are additive keys, and the schema stays 1. A file
without them reads exactly as before. An older Rolecall cannot read a file
that has `next_step` or `contacts` (its reader does not know a flow-map value) and shows it as
malformed, so upgrade Rolecall before using them. The schema version guard is unchanged.

### `status_log`

Each entry is a flow map on one line:

```yaml
  - {status: <Status>, at: <RFC 3339 UTC, Z suffix>}
  - {status: <Status>, at: <date or timestamp>, approx: true}   # migration only
```

- Status values: `Lead` · `Draft` · `Applied` · `Interviewing` · `Offer` · `Rejected` ·
  `Withdrawn` · `Dismissed`. Case-sensitive.
- The log is append only: entries are never edited or removed. Undo appends the earlier Status again.
- An entry is never appended if it repeats the current Status.
- An exact repeat (same Status and `at`) is never written: a write that replaces the whole
  log (migration) keeps only the last copy, and `rolecall doctor` names any repeat already
  in a file.
- An empty log (`status_log: []` or no entries) is read as `Lead`.
- **Applied date** = `at` of the first `Applied` entry. It is never stored separately.
- **Last change** = `at` of the last entry. Follow-up due is measured from it, or from
  `followed_up` when that day is later.
- Follow-up due, exactly: an undone move is left out first (an entry that restores the
  Status before the previous entry, both timestamped and no more than 15 minutes apart,
  cancels that entry and itself; date-only entries never do). The day it counts from is the
  later of the last remaining entry's day and `followed_up`, the entry winning a tie. A
  timestamp's day is its date in the Owner's time zone; a date-only entry and `followed_up`
  keep the day written. It is due `follow_up_days` after that day and on every day after,
  for Applied and Interviewing only, except while `next_step` is on or after today.
- `approx: true` marks a date rebuilt by migration. Readers show it with a `~` prefix.

### Who may change what

| Writer | May write | Must not |
|---|---|---|
| TUI | creates folders. Writes any key through Add / Edit (`next_step` and the first of `contacts` included), `followed_up` through `l`, and `close_reason` from the Move pop-up or Edit. Appends any Status | touch docs, `fit.md`, `out/` |
| `rolecall now`, `rolecall export` | nothing in the Workspace. Read-only. `export` writes only where the Owner's `--out` names | any file under the Workspace |
| `rolecall move` | appends any Status as the TUI does; on Applied also freezes `sent/`, and refuses an Application whose documents are not Ready unless `--force` is given (the TUI asks). Run by the Owner or a script, or by an agent only when the Owner explicitly asks in that session | any key but `status_log`; docs, `fit.md`, `out/` |
| Agent (skills) | creates folders (capture, find, and `import-applications` through `import_csv.py`) through the shared writer, never by hand. Writes `title` `company` `track` `url` `aggregator_url` `closes` `fit` `resume_only` `posted` `posted_label` `applicants` `source`, plus docs, `fit.md`, `follow-up.md`, `prep.md` and `answers.md` | append to `status_log`, except a new folder's first `Lead` entry (for `import-applications`, the entries of the Owner's own export, all `approx: true`), a rule dismissal the Owner confirmed after a `rules.py check` dry run, or when the Owner explicitly asks in that session; then it uses `rolecall move FOLDER STATUS` (which also freezes `sent/` on Applied) rather than `meta.py append-status` |
| `rules.py check --apply` | appends `Dismissed` with `filter_reason` and `filter_note` to a Lead (a Draft only with `--include-drafts`) a Search rule rules out | an Application with `filter_overruled: true`; any other key; moving a folder |
| `build.py` | `out/`. Appends `Draft` only when the current Status is `Lead` and the build produced PDFs | any other key or Status |

### Format rules (both languages)

The format is a strict subset of YAML, so Python can read it with no YAML library and Go can
patch it without reformatting.

1. Top-level lines: `key: value`. Key matches `[A-Za-z_][A-Za-z0-9_-]*`. No nesting except `status_log` and `contacts`. A value may be a one-line flow map (`next_step`, and each `status_log` and `contacts` entry): `{key: value, key: "value"}`, values plain (no comma, brace, bracket or ` #`) or double-quoted, text after the closing brace only an inline comment.
2. Scalars may be plain or double-quoted. Inline `# comments` on scalar lines are allowed and kept.
3. `status_log:` is followed by entries indented two spaces, each `- {…}` on its own line. `contacts:` is written the same way, above `status_log:`.
4. UTF-8, LF line endings, trailing newline.
5. Writers patch the file and never rewrite it whole. Re-read the file right before writing, change only the
   lines for the keys being set (append missing keys at the end, before `status_log`), and
   write atomically (temp file in the same folder, then rename).
6. A file that breaks these rules is **malformed**. Readers show the Application with a warning
   and its path. Writers refuse to touch it.
7. A line that is empty or holds only spaces and tabs is blank, and a line starting with `#`
   is a comment; both are skipped anywhere, inside `status_log` and `contacts` too. Any other
   line, an indented `# …` included, must follow the rules above. A whole number (`schema`,
   `applicants`, `term_months`) is ASCII digits only, with no sign, and fits a signed 64-bit
   integer. A timestamp's offset is at most `23:59`.

## Profile

`profile/` belongs to the Owner and the agent. Its files are free Markdown, apart from the
Contact table below. The TUI reads them for the Profile screen and writes only the Contact
table, from its first-run form; it never touches the rest.

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

- Required: `Name` and `Email`. A Profile missing either fails the build, and the Profile
  screen marks Contact with ○ (missing) and says why.
- Known fields: `Phone`, `Location`, `LinkedIn`, `GitHub`, `Work rights`. Other fields are
  kept and shown, and the build ignores them.
- Header and divider rows (`Field`, and rows of only `:`, `-` and spaces) are skipped.
- Cell values: a trailing italic note `*(…)*` is a note to the human and is dropped.
  `**bold**`, `*italic*` and `` `code` `` wrappers are removed. Whitespace is trimmed.
- Missing section or file: no fields, so both required fields are missing.

## `workspace.json` (read by the TUI, which writes `tracks`, `search_area`, `search_rules`, `sources` and the gates)

The TUI reads `schema` and `tracks` (array of strings, display order) and shows `search_area`,
`search_rules`, `sources`, the gates (`letterWordCap`, `letterWordMin`, `keySkillsMin`,
`keySkillsMax`) and the number of `blockers` on the Settings screen. `sources` (below), when
present, wholly replaces the plugin's shipped source plan. `blockers` is a list of signal
names. `sources`, `blockers` and the gates are used by the plugin.

The TUI writes `tracks` (add, reorder, remove, rename), `search_area` (the Search area
pop-up), each `search_rules` field (the Rules section), `sources` entries (Sources: add,
enable, disable) and the four gates (edited on the Documents group; cap above 0 and the
minimum, Key Skills minimum at most the maximum). Every other key keeps its value and its
place, the file is indented with 2 spaces, the write is atomic, and an invalid file is never
overwritten.

Renaming a Track (`r` on the Settings screen, after a confirm) is the one Workspace-wide write:
it rewrites `track` in every readable `meta.yml` under `jobs/` (including `_archive`, using the
`meta.yml` write rules), then `tracks`, then the role `track` values in `jobs/roles.json`.
Folders never move, malformed `meta.yml` files are skipped and named afterwards, and `u` does
not undo it.

An invalid file is shown, with its parse error, on the Settings screen. Missing schema means 1;
a newer schema stops startup with an instruction to upgrade Rolecall. A missing file, or a
missing key such as `tracks`, means no track filter and the plugin's defaults.

`sources` has two forms. The older one is an object mapping a Platform name to its
targets. The **Source registry** is a list with one entry per Source, of every kind the
Owner searches:

| Field | Type | Required | Meaning |
|---|---|---|---|
| `name` | string | ✅ | unique, ignoring case. What `meta.yml` `source` and the rules name |
| `type` | `api` · `browser` · `manual` | ✅ | `api`: the Sweep fetches it. `browser`: a site the agent reads in a browser. `manual`: searched by hand |
| `platform` | string | `api` | a registered Platform (`greenhouse`, `ashby`, `lever`, `smartrecruiters`, `michaelpage`) |
| `targets` | list of strings | `api` | that Platform's boards |
| `url` | string | — | the search page, for `browser` and `manual` Sources |
| `access` | `works` · `403` · `login wall` · `cloudflare` · `unknown` | — | what happens when it is fetched. Default `unknown` |
| `enabled` | bool | — | `false` keeps the entry but skips it. Default `true` |
| `age_format` | string | — | how the site shows posting age, e.g. `14d ago · 1mo ago · Posted 30+ Days Ago` |
| `applicants_format` | string | — | how it shows the applicant count, e.g. `Over 100 applicants` |
| `notes` | string | — | anything else the agent needs to search it |
| `rules` | object | — | this Source's own `max_applicants` and `max_posting_age_days` |

```json
"sources": [
  {"name": "greenhouse", "type": "api", "platform": "greenhouse", "targets": ["canva"]},
  {"name": "LinkedIn", "type": "browser", "url": "https://www.linkedin.com/jobs/search/",
   "access": "login wall", "age_format": "2 weeks ago", "applicants_format": "Over 100 applicants",
   "rules": {"max_applicants": 100}},
  {"name": "Jora", "type": "manual", "enabled": false}
]
```

The Sweep fetches the enabled `api` entries and lists every other one in its report for the
agent to search. The Settings screen's Sources section lists, adds, tests and disables
entries; a write converts the older object form to the list, keeping every Platform and
target. `sweep.py --test-source <name>` checks one entry and prints its access status.
The catalogue's `additional_sources` and `manual_search_sources` (below) still work when
`sources` is absent. Prefer the registry.

`search_rules` (read by the Sweep, `meta.py create`, `rules.py` and the TUI) holds the rules
that rule a role out, so none of them lives in prose:

| Field | Type | Meaning |
|---|---|---|
| `max_posting_age_days` | int ≥ 1 | a role this many days old or older is expired. A Source's own `rules.max_posting_age_days` replaces it for roles whose `source` names that Source (ignoring case), in the TUI and `rules.py` alike |
| `expiry_warning_days` | int ≥ 0 | the TUI warns "expires in N days" this close to the limit. Default 5. N is the limit minus the age in whole days (today minus `posted`), so the last day before expiry reads "expires in 1 day" |
| `ambiguous_age_labels` | object | label → `discard` or `keep`, for a label that may be either side of the limit (`"1mo ago"` is 28 to 61 days). Matched ignoring case and spacing. Unlisted, a borderline Lead is kept and flagged for the Owner |
| `max_applicants` | object | Source name → the most applicants a role may have there |
| `max_contract_months` | int ≥ 1 | a longer contract is flagged, never dismissed |

```json
"search_rules": {
  "max_posting_age_days": 30,
  "ambiguous_age_labels": {"1mo ago": "discard"},
  "max_applicants": {"linkedin": 100},
  "max_contract_months": 6
}
```

`search_area` (below) sets where a role must be, and `blockers` sets what rules a role out on
its ad text (citizenship, clearance, permanent residency); neither is repeated in
`search_rules`. The age is read from `posted`, else from `posted_label`. A Source's own `rules`
win over these for that Source. A value of the wrong shape stops the Sweep and `rules.py` with
the key named, and the Settings screen refuses it.

`visa` (read by the TUI and assess-fit) is the Owner's work-rights position:

| Field | Type | Meaning |
|---|---|---|
| `needs_sponsorship` | bool | the Owner needs visa sponsorship. A Lead whose `sponsorship` is `not-offered` is badged "no sponsorship", one whose `sponsorship` is `offered` "sponsors visa" |
| `max_contract_months` | int ≥ 1 | the longest term the Owner's visa allows. A Lead with `term_months` is badged "contract fits visa cap" or "12-mo contract > 6-mo visa cap" |

`pay_fx` is an object of currency code → AUD rate for the pay estimate (above).

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

## `config.toml` (machine settings, outside the Workspace)

`$XDG_CONFIG_HOME/rolecall/config.toml` (when `XDG_CONFIG_HOME` is an absolute path), else
`~/.config/rolecall/config.toml`, is per machine and never part of a Workspace. The Settings
screen edits its keys in place and keeps comments. Two keys change how the TUI draws:

- `ascii` (bool, default `false`): ASCII glyphs.
- `inherit_terminal` (bool, default `false`): inherit the terminal's colours. Status and role
  colours become ANSI-16 indices, all readable text (muted included) takes the terminal's
  default foreground and bright black is kept for rules, no background is painted anywhere,
  and the selection is the gutter mark plus reverse video. A missing key means `false`,
  so an existing file draws exactly as before. `NO_COLOR` wins over it. It combines with
  `ascii` (glyph set), and `ROLECALL_THEME` is ignored while it is on. Saving it in Settings
  takes effect at once.

## `jobs/roles.json` (optional role catalogue)

Read by the Sweep (`sweep.py`) and the `find-jobs` skill. It is not an Application, and
the TUI ignores it. Absent means the Sweep falls back to its built-in title keywords;
invalid data stops the Sweep with the reason.

| Key | Type | Required | Meaning |
|---|---|---|---|
| `version` | int | ✅ | `1` |
| `roles` | list | ✅ | non-empty; one object per role (below) |
| `additional_sources` | object | — | Platform name → list of targets, added to the shipped source plan. Ignored when `workspace.json` has `sources` |
| `manual_search_sources` | list of strings | — | Sources the agent searches by hand. Older form of the registry's `manual` entries |

Each role:

| Field | Type | Required | Meaning |
|---|---|---|---|
| `id` | string | ✅ | unique within the file |
| `category` | string | ✅ | e.g. `Engineering` |
| `track` | string | ✅ | must be one of `workspace.json` → `tracks` |
| `evidence` | string | ✅ | the Profile facts that justify the role |
| `gaps` | string | ✅ | what the Profile is missing for it |
| `priority` | `primary` · `secondary` · `stretch` | — | |
| `titles` | list of strings | ✅ | non-empty |
| `search_terms` | list of strings | ✅ | non-empty |

```json
{
  "version": 1,
  "roles": [
    {"id": "backend", "category": "Engineering", "track": "SWE",
     "priority": "primary", "titles": ["Software Engineer"],
     "search_terms": ["software engineer"], "evidence": "…", "gaps": "…"}
  ]
}
```

## `jobs/sweep-candidates.md` (Sweep output)

Written, and overwritten, by `sweep.py` on each Sweep. It is a report for the Owner and
the `find-jobs` skill and is not an Application: the TUI ignores it and no reader parses it
as data. It holds a summary line with the scan time, source health (a FAILED source names
its access problem: `403`, `login wall` or `cloudflare`), what each Search rule filtered out,
role coverage, a `## To assess` list (one `### <title> — <company>` block per candidate), a
`## Blocked` list, and a `## Not reachable by this script` note.

## `cv/` (CVs)

A CV is a resume written from the Profile for no Application: the Owner's everyday,
untailored resume, or a long master version. Each `cv/<name>.md` is one, in the same
Markdown as an Application's `resume.md`. Nothing else in `cv/` is a CV, so the resume or
export `create-profile` imported stays there untouched.

- `build.py --cv <name>` builds `cv/<name>.md` into `cv/out/<name>/`, replacing the PDF that
  folder held: `<Owner> Resume.pdf` for `general`, `<Owner> Resume <Name>.pdf` for any other.
  It runs every gate that needs no JD (Contact, Key Skills, facts, the PDF text check) and
  writes no `meta.yml` and no Status.
- The headline is the Profile's `General` row in its `## Headlines` table, unless
  `--headline` names one. With neither, the build fails and says which to add.
- The Profile screen lists each CV, `general` first: `b` builds it, `d` copies its PDF to the
  downloads folder, `e` opens the Markdown. A CV edited after its last build is marked ⚠.

## `templates/` (optional document Template overrides)

The Plugin's default document Template is three files in `plugin/templates/`: `brand.typ`
(colours, fonts and the header band, shared by both documents), `resume.typ` and
`cover-letter.typ`. To customize one for a Workspace, copy it to `<workspace>/templates/`
under the same name and edit the copy. The next Build uses each Workspace file it finds and
the Plugin default for the rest, and names the overrides it used.

- `<workspace>/brand.typ` at the Workspace root is the older spot for the `brand.typ`
  override. It still applies when `templates/brand.typ` is absent.
- Keep the `#let` names that `resume.typ` and `cover-letter.typ` import from `brand.typ`, and
  the `{{BODY}}`, `{{CONTACT}}` and `{{NAME}}` markers in the document templates, or the
  Build fails.
- Plugin updates to a file never reach a Workspace copy of it. `rolecall doctor` names every
  override in use.
- Nothing else is read from `templates/`. `rolecall doctor` warns about any other file
  there, such as a `style.md` (writing rules belong in `profile/writing-preferences.md`) or
  `fonts/` (fonts load from the Plugin).

## Fixtures

`fixtures/workspace/` holds a fictional workspace that covers: every Status; Ready, Written and
no docs; `resume_only`; unknown keys; `approx` entries; an empty log; one malformed
`meta.yml`; and a Profile with a Contact table. The Go store tests and the Python `build.py`
tests both load it. Any change to this file updates the fixtures in the same commit.

`examples/workspace/` is the showcase: a fictional Owner with a complete Profile and CV, and
Applications at every Status with built PDFs (`examples/README.md`). The README, the demo GIF
(`docs/demo.tape`) and bug reports use it; tests use `fixtures/workspace/`, apart from the
showcase tests listed in `examples/README.md`, which check it still covers what it promises. A change to
this file updates it in the same commit too.

`fixtures/conformance.json` holds this file's rules as vectors that the Go and Python tests
both assert: `meta.yml` lines and whole texts (malformed or not), folder names, Follow-up due
and posting age. The expected values follow this file, not either implementation; a side
that does not meet one yet lists it, by audit finding, in its test's known-drift table.

`fixtures/dashboard-era/` is the dashboard-era migration fixture: tracker, shortlist and dismissed
tables, jd front matter, legacy keys, bulk-stamped logs and an unmappable Fit. It is used by
`plugin/tests/test_migrate.py` and `internal/store/tests/migrate_integration_test.go`. It is
input to the one-time migration only, and the store does not scan it.
