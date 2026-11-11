# rolecall: spec

A terminal job tracker. The Owner moves Applications between Statuses, reads them, and gets
their documents. A harness agent, following the rolecall plugin's skills, does the AI work
(capture, fit, documents, finding roles) by writing files. The TUI and the agent never talk to
each other directly: files are the only link.

- **Decisions:** ADR-0001 (personal local TUI replaces the dashboard), ADR-0002 (Status lives
  in each folder), ADR-0003 (files are the only harness contract).
- **Data contract:** [`SCHEMA.md`](../SCHEMA.md). **Terms:** [`GLOSSARY.md`](../GLOSSARY.md).
- **Status of this doc:** agreed in the grilling session of 2026-11-11. Covers v0.1 in full;
  later versions are listed as scope only.

## Roadmap

| Version | Theme | Scope |
|---|---|---|
| **v0.1** | personal, works without a harness | TUI layout and behaviour, `SCHEMA.md`, plugin port + rewrite for the schema, data migration |
| **v0.1.1** | styling | colour palette, light/dark, status colours, layout tweaks, empty states, header, demo GIF |
| **v0.2** | harness + extras | `f`/`p` keys that hand off to the harness, URL fetch in Add, Stats screen (`4`), `b` build key |
| **v0.3** | reading | Markdown preview of `resume.md` / `cover-letter.md` in the Docs tab |
| **v1.0** | public, anyone can use it | see [v1.0](#v10-public) |

## Non-goals for v0.1

- Any AI action, harness detection or harness command in the TUI. It must feel fully usable with no harness installed.
- Colour, theming or visual polish (v0.1.1).
- Hosting, multiple users, email intake, phone access.
- Fetching JDs from URLs (v0.2), building PDFs (v0.2), stats (v0.2).
- Being usable by anyone but the Owner (v1.0).

## Repositories

```
rolecall/  (new, private until v1.0)          <workspace>/  (private data repo)        
├─ cmd/rolecall/main.go                       ├─ workspace.json
├─ internal/                                  ├─ profile/   (+ writing-preferences.md)
│  ├─ store/   meta.yml read/patch, scan      ├─ cv/
│  ├─ ui/      Bubble Tea models, views       └─ jobs/<track>/<slug>/
│  └─ config/
├─ plugin/
│  ├─ .claude-plugin/plugin.json
│  ├─ skills/  capture-job, find-jobs, assess-fit, prepare-documents
│  ├─ scripts/ build.py, sweep.py, new-job.py, sync-github.py, migrate.py
│  ├─ templates/ resume.typ, cover-letter.typ, brand.typ, fonts/
│  ├─ styles/au.md
│  └─ writing-rules.md
├─ fixtures/workspace/
├─ SCHEMA.md · GLOSSARY.md · docs/adr/
└─ .github/workflows/  (go test, pytest, ruff)
```

- The code repo holds **no personal data**. The data repo stays private for good.
- **Dev loop:** register `rolecall/plugin` as a local-path marketplace in Claude Code, so skill
  edits take effect without reinstalling. Other harnesses read `plugin/skills/` by path.
- In the dashboard-era repo, `src/web/`, `src/email-worker/` and the hosted deploy are frozen now and deleted once v0.1
  is in daily use.

## TUI (v0.1)

### Stack

Go · Bubble Tea · Bubbles (table, viewport, help, key) · Lip Gloss (layout only in v0.1) ·
Huh (forms, move popup) · Glamour (Markdown in the detail view) · fsnotify · a TOML reader for
config. Use the latest stable major versions at setup.

### Config

`~/.config/rolecall/config.toml` (`--dir` overrides `workspace`):

```toml
workspace = "~/job-search"
editor = ""            # empty = $VISUAL, then $EDITOR, then vi
follow_up_days = 14
downloads = "~/Downloads"
```

If there is no workspace, rolecall exits with a one-line message naming the config path. No wizard until v1.0.

### Screens

| Key | Screen | Tables (`Tab` cycles) | Holds Status |
|---|---|---|---|
| `1` | Leads | **Leads** │ **Dismissed** | Lead │ Dismissed |
| `2` | Draft | — | Draft |
| `3` | Applied | **Applied** │ **Interviewing** │ **Offer** │ **Closed** | Applied │ Interviewing │ Offer │ Rejected + Withdrawn |
| `4` | *(reserved: Stats, v0.2)* | | |

Each table shows a count in its tab label. The active screen and table stay put across
refreshes, and the cursor stays on the same Application (matched by folder) when rows shift.

### Columns and sort

| Table | Columns | Sort |
|---|---|---|
| Leads, Draft | Title · Company · Track · Closes · Fit (+ Docs on Draft) | `closes` soonest first. No date goes last. Past dates show as `past` |
| Dismissed | Title · Company · Track · Dismissed (date) | newest first |
| Applied, Interviewing, Offer, Closed | Title · Company · Track · Applied · Fit · Docs · Follow-up | Applied date, newest first |

- **Docs:** `●` Ready · `○` Written · blank (SCHEMA → Documents state). Glyphs only in v0.1; colour comes in v0.1.1.
- **Follow-up:** `⚑` when the Status is Applied or Interviewing and the last change is more than `follow_up_days` old.
- **Fit:** the enum word, or `—`.
- **Malformed** `meta.yml`: the row shows `⚠` and the folder name. The detail view shows the path and why parsing failed.
- **Approx dates:** `~` prefix.
- Columns shrink to fit the terminal width, truncating Title first, then Company.

### Keys

| Key | Scope | Action |
|---|---|---|
| `1` `2` `3` | global | switch screen |
| `Tab` / `Shift-Tab` | screen | next / previous table |
| `↑↓` / `j` `k`, `g` `G` | table | move cursor, top / bottom |
| `a` | global | Add form |
| `v` / `Enter` | row | detail view |
| `e` | row | Edit form (fields). In the detail view: open the current tab's file in `$EDITOR` |
| `n` | row | open `notes.md` in `$EDITOR` (created if missing) |
| `m` | row | Move popup |
| `u` | global | undo the last move made in this session |
| `x` | row (unsent) | Dismiss. Same as Move → Dismissed |
| `d` | row | Download |
| `t` | global | cycle the track filter: All → each `workspace.json` track |
| `/` | table | text filter on Title + Company. `Esc` clears it |
| `r` | global | rescan the workspace |
| `?` | global | full help |
| `q` / `Ctrl-C` | global | quit |

- **Reserved, unbound in v0.1:** `f`, `p`, `b`, `4`.
- The bottom help bar shows only the keys valid for the current row (e.g. `d` hidden when nothing is built).

### Move popup (`m`)

Lists **every Status except the current one**, likely next steps first, each with a letter:

| From | Listed first | Then |
|---|---|---|
| Lead | `d` Draft · `a` Applied | `x` Dismissed, others |
| Draft | `a` Applied | `l` Lead · `x` Dismissed, others |
| Applied | `i` Interviewing · `r` Rejected · `w` Withdrawn | `o` Offer, others |
| Interviewing | `o` Offer · `r` Rejected · `w` Withdrawn | `a` Applied, others |
| Offer | `w` Withdrawn · `r` Rejected | others |
| Rejected / Withdrawn | `i` Interviewing · `a` Applied | others |
| Dismissed | **the Status before Dismissed** (default, `Enter`) | others |

Letters stay fixed per Status across all rows: `l` Lead, `d` Draft, `a` Applied,
`i` Interviewing, `o` Offer, `r` Rejected, `w` Withdrawn, `x` Dismissed. Pressing a letter
appends to `status_log` (SCHEMA rules) and closes the popup. The row then moves to its new
table, and a one-line toast says `Moved to Interviewing · u to undo`.

**Side effects when moving to Applied:**
1. If the documents are not Ready, ask first: `Docs not Ready. Mark Applied anyway? y/n`.
2. If `sent/` does not exist, copy `resume.md`, `cover-letter.md`, `jd.md`, `fit.md`, `gaps.md` and `out/*.pdf` into it. Never overwrite an existing `sent/`.

**Undo (`u`):** a session stack of `(folder, previous Status)`. Undo appends the previous
Status (the log stays append-only) and shows a toast. It does not remove `sent/`.

### Add form (`a`)

Huh form fields: **Title*** · **Company*** · **Track*** (select from `workspace.json`) · URL ·
Closes (`YYYY-MM-DD`) · Fit (select, blank allowed).

On save:
1. If the URL matches another Application's `url`, warn and offer to open that one instead.
2. Create `jobs/<track>/<slug>/` (SCHEMA → Folder name), write `meta.yml` (`schema: 1`, the
   fields, `status_log` with one `Lead` entry), and an empty `jd.md`.
3. Open `jd.md` in `$EDITOR` so the description can be pasted. On return, select the new row.

### Edit form (`e` in a table)

The same fields as Add, plus **Resume only** (yes/no). Saving patches only the keys that
changed. Changing Track does **not** move the folder.

### Detail view (`v` / `Enter`)

Header: `Company · Title`, then `Status · Track · closes <date> · applied <date>`.
Tabs (`←` `→`): **JD** · **Notes** · **Fit** · **Docs**.

| Tab | Shows | `e` |
|---|---|---|
| JD | `jd.md` rendered with Glamour, scrollable | `$EDITOR` on `jd.md` |
| Notes | `notes.md` rendered | `$EDITOR` on `notes.md` |
| Fit | the `fit` value + `fit.md` rendered | `$EDITOR` on `fit.md` |
| Docs | list: `resume.md`, `cover-letter.md`, `gaps.md` (present/absent), each PDF in `out/`, and `sent/` if it exists | `Enter` on a PDF opens it in the system viewer. On a `.md`, opens `$EDITOR` |

`Esc` goes back to the table. `m`, `n`, `d` and `u` work in the detail view too.

### Download (`d`)

Copies every `out/*.pdf` into `downloads` under its existing name (already upload-ready). If a
file of the same name exists, it is overwritten after a `y/n` prompt. Then the Downloads folder
is revealed (`open -R` on macOS). Toast: `2 PDFs → ~/Downloads`.

### `$EDITOR` and the v0.2 seam

Every external program runs through one helper, `ui.runExternal(cmd)`: Bubble Tea
`tea.ExecProcess` suspends the TUI, runs the command, resumes, then rescans the affected
folder. v0.1 uses it for `$EDITOR` and the system viewer. v0.2 adds harness command templates
on the same helper. Nothing else in the TUI should launch processes.

### Live refresh

- Watch `jobs/` and each track and Application folder with fsnotify. Add watches as new folders appear.
- Debounce events (≈200 ms), then rescan only the affected folders.
- `r` rescans everything.
- When a row's file changes under an open detail view, re-render it and keep the scroll position.

### Writes

The store package is the only code that writes. It follows SCHEMA → Format rules: re-read,
patch, write atomically. It refuses to touch malformed files and shows the error.

### Styling policy for v0.1

Layout only. Use terminal default colours, plus bold for headers and reverse video for the
selected row. Put every Lip Gloss style in one file (`internal/ui/styles.go`) as neutral
styles, so v0.1.1 changes look by editing that file alone.

## Plugin (v0.1)

### Port: sort while moving

| Today | Goes to | Change |
|---|---|---|
| `src/templates/resume.typ`, `cover-letter.typ` | `plugin/templates/` | paths |
| `src/templates/brand.typ` | `plugin/templates/` (default brand) | none. It becomes a workspace override at v1.0 |
| `src/templates/fonts/` (OFL) | `plugin/templates/fonts/` | none |
| `src/templates/style.md` | `plugin/styles/au.md` | remove the Owner's name and personal facts. Those move to `writing-preferences.md` |
| `docs/writing-rules.md` | `plugin/writing-rules.md` | rewrite the pipeline, tracker and command sections for the schema |
| `src/scripts/build.py`, `new-job.py`, `sweep.py`, `sync-github.py` | `plugin/scripts/` | see below |
| `.agents/skills/{capture-job,find-jobs,assess-fit,prepare-documents}` | `plugin/skills/` | see below |
| `docs/writing-preferences.md` | data repo `profile/writing-preferences.md` | none |
| `profile/*`, `cv/` | stay in the data repo | none |

Other skills in `.agents/skills/` are dev tooling. Each repo keeps what it needs.

### Tone layering

`prepare-documents` reads, in order, with later files winning:
`plugin/writing-rules.md` → `plugin/styles/au.md` → `<workspace>/profile/writing-preferences.md`.
Output must match today's documents. Check by rebuilding two existing Applications and diffing the Markdown.

### Script changes

| Script | Change |
|---|---|
| `build.py` | **Stop** writing `TRACKER.md` rows and adding empty legacy keys. **Add**: append `Draft` when the current Status is `Lead` and PDFs were built. Read `resume_only` as now. Keep the gates. Remove the `sent/` snapshot (the TUI owns it). Takes the workspace path from `--data` or `ROLECALL_WORKSPACE` |
| `new-job.py` | Writes `meta.yml` (SCHEMA) + front-matter-free `jd.md` instead of jd front matter + tracker row |
| `sweep.py` | Dedupe and skip by reading every `meta.yml` `url` (including Dismissed) instead of `SHORTLIST.md` / `DISMISSED.md` |
| `migrate.py` | new, one-time. See Migration |

### Skill changes

| Skill | Writes |
|---|---|
| `capture-job` | new folder: `meta.yml` (`title`, `company`, `track`, `url`, `closes` if stated, first `Lead` entry) + `jd.md` |
| `find-jobs` | runs `sweep.py`, triages, creates a Lead folder per role worth a look (as capture-job, `jd.md` may be a stub with the URL) |
| `assess-fit` | `fit.md` + `fit:` in `meta.yml` |
| `prepare-documents` | `resume.md`, `cover-letter.md`, `gaps.md`, sets `resume_only` if chosen, runs `build.py` |

All skills: never append to `status_log` unless the Owner explicitly asks. Never touch
`notes.md`, `sent/`, or the profile fact table.

## Migration (one-time, `migrate.py`)

Runs against the dashboard-era workspace, with `--dry-run` printing every change first.

| Source | Becomes |
|---|---|
| Each existing job folder + its `TRACKER.md` row | `meta.yml` gains `schema`, `title`, `company`, `track`, `url` (from jd front matter / tracker), `fit` (mapped below). The jd front matter is removed |
| Tracker Status → `status_log` | `Draft` → `Draft`, `Screening`/`Interview` → `Interviewing`, others same. If the log only holds a single bulk-stamped entry, prepend `Applied` with `at` = the `applied` key, or else the tracker Date, plus `approx: true` |
| `SHORTLIST.md` rows at `New` | new Lead folders: `meta.yml` + stub `jd.md` holding the URL and the triage reason |
| `DISMISSED.md` rows | new folders, `status_log`: `Dismissed` (`approx: true`, the row date) |
| Shortlist rows at `Passed` / other | dropped |
| `TRACKER.md`, `SHORTLIST.md`, `DISMISSED.md`, `INBOX.md`, `sweep-candidates.md` | moved to `jobs/_archive/` (ignored by readers) |

**Fit mapping:** text starting `Strong` → `Strong`. `Worth applying` / `Good` → `Good`.
`Marginal` → `Marginal`. `Weak` / `Poor` / `Don't` → `Poor`. Anything else is listed for the
Owner to set by hand.

## Testing and acceptance

**Automated**
- Go `internal/store`: parse and patch round-trips against `fixtures/workspace/` (unknown keys and comments preserved byte for byte; malformed files refused; atomic write).
- Go `internal/ui`: `teatest` flows for Add → Move → Undo, and Dismiss → restore.
- Python: `build.py` tests on the same fixtures (Lead → Draft append, no tracker writes, gates unchanged). `ruff` clean.
- CI runs both.

**v0.1 is done when the Owner can, with no harness running:**
1. Launch `rolecall` on the migrated workspace and see every current Application on the right screen and table.
2. Add a Lead, paste its JD, edit it, and see it sorted by close date.
3. Move it Lead → Draft → Applied (and see `sent/` created) → Interviewing → Rejected, then undo once.
4. Dismiss a Lead, find it in Dismissed, and restore it to its earlier Status.
5. Download a Ready Application's PDFs and see them in Finder.
6. Watch a row update live while an agent (in another terminal) writes `fit:` or builds PDFs.
7. Filter by track and by text.
8. Run `capture-job` and `prepare-documents` from the plugin and get output matching today's quality.

## Later versions (scope only)

### v0.1.1: styling
Colour palette and status colours (green `●` Ready, Fit colours, dimmed past dates), light/dark,
header bar, empty-state messages, layout tweaks, VHS demo GIF. All in `styles.go` plus view tweaks.

### v0.2: harness and extras
- `f` assess fit, `p` prepare documents: command templates in config, run through `runExternal` (interactive handoff, ADR-0003).
- URL fetch in Add.
- Stats screen on `4`.
- `b` build key (runs `build.py`).

### v0.3: reading
Markdown preview of `resume.md` and `cover-letter.md` in the Docs tab (Glamour).

### v1.0: public
| Area | Features |
|---|---|
| Onboarding | `rolecall init` setup wizard (name, contact, location, tracks, spelling, paper size, workspace path, `git init`) · `rolecall doctor` (typst, pandoc, Python, fonts, harness) · plugin install guide per harness |
| Profile | Profile screen (contact and preferences form, experience in `$EDITOR`) · `import-profile` skill (CV PDF/DOCX/LinkedIn export → profile) |
| Settings | Settings screen: tracks, gates, follow-up days, downloads, editor, theme, harness commands |
| Documents | neutral default template + workspace brand override · style packs AU/UK/US · 2–3 template choices |
| Data | `schema` migrations (`rolecall migrate`) · CSV/spreadsheet import · CSV/JSON export · optional auto-commit on Status change |
| Finding | per-workspace sources and locations, no Melbourne default |
| Platform | Linux/Windows file opening · `NO_COLOR` · colour-blind-safe markers · small terminals |
| Release | goreleaser + Homebrew tap · README, demo, fictional example workspace · MIT licence · CONTRIBUTING · personal-data CI guard · no telemetry |

## Open questions

- **Bubble Tea v2:** confirm it's stable at setup. If not, use v1 and plan the upgrade.
- **Leads from `find-jobs` with no JD text:** keep stub `jd.md` files, or have the skill fetch and fill them? Default is stub plus URL. Revisit after a week of use.
