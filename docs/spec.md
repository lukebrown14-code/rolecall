# rolecall: spec

A terminal job tracker. The Owner moves Applications between Statuses, reads them, and gets
their documents. A harness agent, following the rolecall plugin's skills, does the AI work
(capture, fit, documents, finding roles) by writing files. The TUI and the agent never talk to
each other directly: files are the only link.

- **Decisions:** ADR-0001 (personal local TUI replaces the dashboard), ADR-0002 (Status lives
  in each folder), ADR-0003 (files are the only harness contract).
- **Data contract:** [`SCHEMA.md`](../SCHEMA.md). **Terms:** [`GLOSSARY.md`](../GLOSSARY.md).
- **Status of this doc:** v0.1 agreed in the grilling session of 2026-11-11; v0.2 agreed in a
  second session the same day. Covers v0.1 and v0.2 in full; later versions are listed as scope only.

## Roadmap

| Version | Theme | Scope |
|---|---|---|
| **v0.1** | personal, works without a harness | TUI layout and behaviour, `SCHEMA.md`, plugin port + rewrite for the schema, data migration |
| **v0.1.1** | styling | colour palette (dark only), status colours, layout tweaks, empty states, header, demo GIF |
| **v0.2** | harness + extras | `f`/`p` keys that hand off to the harness, URL fetch in Add, Stats screen (`4`), `b` build key |
| **v0.3** | reading | Markdown preview of `resume.md` / `cover-letter.md` in the Docs tab |
| **v1.0** | public, anyone can use it | see [v1.0](#v10-public) |

## Non-goals for v0.1

- Any AI action, harness detection or harness command in the TUI. It must feel fully usable with no harness installed.
- Colour, theming or visual polish (v0.1.1).
- Hosting, multiple users, email intake, phone access.
- Fetching JDs from URLs, building PDFs, stats (all v0.2).
- Being usable by anyone but the Owner (v1.0).

## Repositories

```
rolecall/  (new, private until v1.0)          <workspace>/  (private data repo)        
├─ cmd/rolecall/main.go                       ├─ workspace.json
├─ store/   meta.yml read/patch, scan         ├─ profile/   (+ writing-preferences.md)
├─ ui/      Bubble Tea models, views          ├─ cv/
├─ config/                                    └─ jobs/<track>/<slug>/
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

Go · Bubble Tea · Bubbles (viewport, help, key) · Lip Gloss (layout, colour and layered
pop-ups from v0.1.1) · Glamour (Markdown in the detail view) · fsnotify · a TOML reader for
config. v0.1 used Huh for the forms and move popup; v0.1.1 draws them itself (see below). Use the latest stable major versions at setup.

### Config

`~/.config/rolecall/config.toml` (`--dir` overrides `workspace`):

```toml
workspace = "~/job-search"
editor = ""            # empty = $VISUAL, then $EDITOR, then vi
follow_up_days = 14
downloads = "~/Downloads"
harness = ""           # v0.2: Handoff command template, e.g. 'claude "/{skill} {dir}"'
plugin = ""            # v0.2: path to rolecall/plugin; falls back to $ROLECALL_PLUGIN
```

If there is no workspace, rolecall exits with a one-line message naming the config path. No wizard until v1.0.

### Screens

| Key | Screen | Tables (`Tab` cycles) | Holds Status |
|---|---|---|---|
| `1` | Leads | **Leads** │ **Dismissed** | Lead │ Dismissed |
| `2` | Draft | — | Draft |
| `3` | Applied | **All** │ **Applied** │ **Interviewing** │ **Offer** │ **Closed** | All = every Status below, one section each · Applied │ Interviewing │ Offer │ Rejected + Withdrawn |
| `4` | Stats (v0.2) | — | — |

Each table shows a count in its tab label. The active screen and table stay put across
refreshes, and the cursor stays on the same Application (matched by folder) when rows shift.

### Columns and sort

| Table | Columns | Sort |
|---|---|---|
| Leads, Draft | Title · Company · Track · Closes · Fit (+ Docs on Draft) | `closes` soonest first. No date goes last. Past dates show as `past` (v0.1.1: `expired 4w ago`, under an **Expired** section) |
| Dismissed | Title · Company · Track · Dismissed (date) | newest first |
| Applied, Interviewing, Offer, Closed | Title · Company · Track · Applied · Fit · Docs · Follow-up | Applied date, newest first |

- **Docs:** `●` Ready · `○` Written · blank (SCHEMA → Documents state). Glyphs only in v0.1; colour comes in v0.1.1.
- **v0.1.1** shows these as two-line rows with relative dates; see **v0.1.1 look**. There,
  Docs shows on Draft only (the Applied screen's rows show Fit and the follow-up flag, and
  Closed rows show the outcome, `✕ rejected` / `↩ withdrawn`, beside Fit).
- **Follow-up:** `⚑` when the Status is Applied or Interviewing and the last change is more than `follow_up_days` old.
- **Fit:** the enum word, or `—`.
- **Malformed** `meta.yml`: the row shows `⚠` and the folder name. The detail view shows the path and why parsing failed.
- **Approx dates:** `~` prefix, on exact dates (`~2026-01-01`) and relative ones (`applied ~3mo ago`).
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

- **v0.2 adds** `4` (Stats), `f`, `p`, `b`: see [v0.2](#v02-harness-and-extras).
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
table, and the feedback line says `<Title> → Interviewing` with `u undo`.

**Side effects when moving to Applied:**
1. If the documents are not Ready, ask first: a `⚠ Docs not Ready` pop-up, `y` mark Applied, `n`/`Esc` cancel.
2. If `sent/` does not exist, copy `resume.md`, `cover-letter.md`, `jd.md`, `fit.md`, `gaps.md` and `out/*.pdf` into it. Never overwrite an existing `sent/`.

**Undo (`u`):** a session stack of `(folder, previous Status)`. Undo appends the previous
Status (the log stays append-only) and says so on the feedback line (`Undone · Title back to Applied`). It does not remove `sent/`.

### Add form (`a`)

Form fields: **Title*** · **Company*** · **Track** (choice from `workspace.json`) · URL ·
Closes (`YYYY-MM-DD`) · Fit (choice, blank allowed). Track needs no `*`: it is a choice and
always holds a track. `Enter` checks the field and moves on; on the last field it checks them
all and saves. Choices change with `←` `→` (or `↑` `↓`). Pasting fills the focused text field;
a long value shows its end, and a field's error sits on its own line below it.

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
Tabs (`←` `→`): **JD** · **Fit** · **Notes** · **Docs** (Fit beside JD: they are read together).
From 100 columns the preview's card (status, documents, timeline, link) stays on the left.

| Tab | Shows | `e` |
|---|---|---|
| JD | `jd.md` rendered with Glamour, scrollable | `$EDITOR` on `jd.md` |
| Notes | `notes.md` rendered | `$EDITOR` on `notes.md` |
| Fit | the `fit` value + `fit.md` rendered | `$EDITOR` on `fit.md` |
| Docs | list: `resume.md`, `cover-letter.md`, `gaps.md` (present/absent), each PDF in `out/`, and `sent/` if it exists | `Enter` on a PDF opens it in the system viewer. On a `.md`, opens `$EDITOR` |

`Esc` goes back to the table. `m`, `n`, `d` and `u` work in the detail view too.

### Download (`d`)

Copies every `out/*.pdf` into `downloads` under its existing name (already upload-ready). If a
file of the same name exists, a `⚠ File exists` pop-up asks: `y` overwrite, `a` all (this and
every later clash), `n` skip, `Esc` stop. Then the Downloads folder is revealed (`open -R` on
macOS). Feedback line: `Saved 2 PDFs → ~/Downloads`.

### `$EDITOR` and the v0.2 seam

Every external program runs through one helper, `ui.runExternal(cmd)`: Bubble Tea
`tea.ExecProcess` suspends the TUI, runs the command, resumes, then rescans the affected
folder. v0.1 uses it for `$EDITOR` and the system viewer. v0.2 adds a second, captured mode
on the same helper (see v0.2). Nothing else in the TUI should launch processes.

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
selected row. Put every Lip Gloss style in one file (`ui/styles.go`) as neutral
styles, so v0.1.1 changes look by editing that file alone.

### v0.1.1 look (built)

Settled with a throwaway prototype (branch `prototype/v011-styling`) and two rounds of
context-free review. Every colour and style is in `ui/styles.go`.

- **Theme:** dark only; the view paints the terminal background `#0D1117`. Accent **Mint**
  `#5EEAD4`. Colour always pairs with a word or glyph.
- **Active markers:** the active track, view tab and detail tab are bracketed — `[All]`,
  `[Leads 4]`, `[JD]` — as well as coloured, so they read without colour.
- **Accessibility:** `NO_COLOR` drops all colour but keeps bold, underline and every word
  and glyph. `ROLECALL_ASCII=1` maps every drawn glyph to an ASCII stand-in. The `?`
  pop-up carries a legend for `● ○ ✓ ⚠ ✕ ↩ ~ ⚑`.
- **Status colours:** Fit Strong green · Good blue · Marginal/Poor grey. Closes today red,
  within 7 days amber, expired dimmed. `⚑ quiet` amber. Interviewing lilac, Offer green,
  Rejected red. Documents read `● ready` · `○ written` · `not written` (SCHEMA Ready and
  Written; GLOSSARY avoids "built").
- **Layout, top to bottom (one fixed frame):** header (screens and the track filter) ·
  context row (the screen's views, and the scope line when a filter is on) · top rule ·
  body (with a preview pane from 100 columns) · bottom rule · status row. The rules and
  the `┬`/`┴` joins are always drawn; the status row holds a toast, an error, the search
  input or the key line, one at a time (actions left, navigation pinned right).
- **Scope line:** whenever a track or text filter is on, the context row names it once —
  `<Screen> › <View> · track SWE · search pl · 3 of 4` — and the header and tab counts read
  `3/4` rather than a filtered number that looks like a total.
- **Feedback:** a toast (successes clear on the next key) or an error (stays until `Esc`)
  shows on the status row; it never replaces the bottom rule. Paths read relative to the
  Workspace.
- **Search:** while typing, the search input is on the status row; after `Enter` the scope
  line shows `search pl` and the count.
- **Rows:** two lines (title + right-hand columns, then company · track · dates). A blank
  line between rows when they all fit; dense when they don't. Right-hand columns are
  state · flag · Fit, Fit flush right; a column empty for the whole view is hidden.
  Expired Leads and Drafts sort last under an **Expired** section.
- **Pop-ups:** one template — title in the top border, keys in the bottom border, solid
  fill, amber border and ⚠ for warnings, the screen behind dimmed with two blank cells
  either side of the box. A pop-up taller than the terminal drops middle lines and keeps
  its bottom border. `?` is a pop-up; any key closes it, `Esc` included. The Move pop-up
  always moves the Application it shows, even if a live refresh moved the cursor.
- **Narrow terminals:** the screens row sheds detail, the preview and detail card hide
  below 100 columns, pop-ups shrink to fit. The key line drops actions from the right but
  keeps `enter`, `a add` and `/ search` to the last. Below 50×12 a "Terminal too small"
  notice replaces the layout.

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
- Go `store`: parse and patch round-trips against `fixtures/workspace/` (unknown keys and comments preserved byte for byte; malformed files refused; atomic write).
- Go `ui`: `Update` driven directly (key in → model and disk state out) for Add → Move →
  Undo and Dismiss → restore, plus a few whole-program `teatest` smoke tests; rendering
  goldens are ANSI-stripped except one palette golden.
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
Built: see **v0.1.1 look** above. Dark only (light mode dropped). Still to do: VHS demo GIF.

### v0.2: harness and extras

Agreed in full (no longer scope only). Four items; the open polish issues (#29–33) are a
separate track.

#### Config and the process seam

- `harness`: one command template for every Handoff. `{skill}` is the Plugin skill name,
  `{dir}` the absolute Application folder, shell-quoted. Runs through `/bin/sh -c`, like the editor.
- `plugin`: path to `rolecall/plugin`, else `$ROLECALL_PLUGIN`. No auto-detection.
- `runExternal` gains a mode:
  - **interactive** (v0.1 behaviour): suspend, run, resume, rescan. Used by `$EDITOR`, the viewer, Handoffs.
  - **captured**: run in the background without suspending, keep stdout/stderr, report a
    message when done, then rescan the folder. Used by `b` and URL fetch. One at a time;
    other keys keep working.
- Unset `harness` hides `f`/`p` from the help bar; pressing one shows
  `Set harness in ~/.config/rolecall/config.toml`. Unset `plugin` does the same for `b` and
  fetch (Add still works by hand).

#### Handoff: `f` and `p`

| Key | Skill | Applications | From |
|---|---|---|---|
| `f` | `assess-fit` | any | table row, detail view |
| `p` | `prepare-documents` | Lead, Draft | table row, detail view |

Interactive mode: the Owner talks to the agent in their terminal (ADR-0003). On exit the
folder is rescanned. A non-zero exit shows `Harness exited with status N` in the feedback
line. No pop-up, no retry.

#### Build: `b`

- Runs `python3 <plugin>/scripts/build.py <dir> --data <workspace>` in captured mode.
- Shown only on Lead or Draft whose Documents state is Written or Ready (rebuilds Ready).
- Spinner in the feedback line, then `Built 2 PDFs`. If the build fails (e.g. a gate), a pop-up shows
  `build.py`'s output.
- A build of a Lead appends `Draft` (SCHEMA). The view stays on the current screen and says
  `Built 2 PDFs · moved to Draft`. This is not the Owner's move, so `u` does not undo it.

#### URL fetch in Add

- URL becomes the **first** field of the Add form.
- `Enter` on a URL: duplicate check first (as on save). If new, run
  `new-job.py --fetch-only --url <url> --data <workspace>` in captured mode, spinner on the field.
- On success: fill Title, Company and Closes where still empty, and keep the ad text. Visa
  and clearance signals show as a `⚠` line under the URL. They are not stored.
- On save after a successful fetch: `jd.md` gets the fetched text and `$EDITOR` is skipped.
- On failure (login wall, text under 200 characters, no Python): one line saying why. The form
  goes on by hand, and save opens `$EDITOR` as in v0.1.

#### Stats screen (`4`)

Answers "how is my search going?". No tables: `Tab` and row keys do nothing. The `t` track
filter applies. Top to bottom:

1. **Funnel**: Applications that **ever reached** each Status (from `status_log`, `approx`
   entries included): Lead → Draft → Applied → Interviewing → Offer, with drop-off
   percentages. Dismissed, Rejected and Withdrawn as side counts.
2. **Response rate** (GLOSSARY → Response): reached Interviewing or Offer ÷ ever Applied,
   leaving out Applications whose Applied date is within `follow_up_days`.
3. **Applied per week**: last 12 weeks, Monday-start, local time, by Applied date. Unicode
   block bars with counts.
4. **By track**: track × Applied · Responses · Rate. Hidden when the filter is on one track.

#### v0.2 is done when the Owner can

1. Press `f` on a Lead, talk to the agent, and see Fit appear on return.
2. Press `p` on a Draft, and on return see Docs go to Written or Ready.
3. Press `b` on a Written Lead and see `Built 2 PDFs · moved to Draft` without leaving the
   Leads screen. Then see a failed gate's message in a pop-up.
4. Paste a Greenhouse or Ashby URL into Add, see the fields fill, save, and find `jd.md` filled.
   Paste a Seek URL and get the paste-by-hand path.
5. Open Stats, check the funnel and response rate against the fixtures, and filter by track.
6. Do all of v0.1's acceptance list with `harness` and `plugin` unset.

**Automated:** config parsing for the new keys; captured-mode runner fake in `ui` tests (`b`
success/failure, fetch success/failure); Stats numbers as pure functions over the fixtures
(funnel, Response rate exclusion window, week bucketing across a Monday boundary).

### v0.3: reading
Markdown preview of `resume.md` and `cover-letter.md` in the Docs tab (Glamour).

### v1.0: public
| Area | Features |
|---|---|
| Onboarding | `rolecall init` setup wizard (name, contact, location, tracks, spelling, paper size, workspace path, `git init`) · `rolecall doctor` (typst, pandoc, Python, fonts, harness) · plugin install guide per harness |
| Profile | read-only Profile screen (screen 5; editing in 1.x) · `import-profile` skill (CV PDF/DOCX/LinkedIn export → profile) |
| Settings | Settings screen: tracks, gates, follow-up days, downloads, editor, theme, harness commands |
| Documents | neutral default template + workspace brand override · style packs AU/UK/US · 2–3 template choices |
| Data | `schema` migrations (`rolecall migrate`) · CSV/spreadsheet import · CSV/JSON export · optional auto-commit on Status change |
| Finding | per-workspace sources and locations, no Melbourne default |
| Platform | Linux/Windows file opening · `NO_COLOR` · colour-blind-safe markers · small terminals |
| Release | goreleaser + Homebrew tap · README, demo, fictional example workspace · MIT licence · CONTRIBUTING · personal-data CI guard · no telemetry |

#### What v1.0 still needs (as of 2026-11-12)

v0.1, v0.1.1 and v0.2 are merged to `main` (PR #41). Still scope only: each area needs
agreeing in a grilling session and splitting into issues before it is built.

| Area | Item | State |
|---|---|---|
| Onboarding | `rolecall init` wizard | To do. Today a missing workspace exits with a one-line message |
| | `rolecall doctor` | To do |
| | Plugin install guide per harness | To do |
| Profile | Profile screen | Designed (grilled 2026-11-12, see [Profile screen](#profile-screen-v10)). To do (#49) |
| | `import-profile` skill | To do |
| Settings | Settings screen | To do. Settings are only in `config.toml` and `workspace.json` |
| Documents | Neutral default template + workspace brand override | To do. `plugin/templates/brand.typ` is still the Owner's brand |
| | Style packs AU/UK/US | Partial. Only `plugin/styles/au.md` exists |
| | 2–3 template choices | To do |
| Data | `rolecall migrate` (schema versions) | To do. `migrate.py` is the one-time dashboard migration only |
| | CSV/spreadsheet import · CSV/JSON export | To do |
| | Optional auto-commit on Status change | To do |
| Finding | Per-workspace sources and locations | Partial. `find-jobs` reads `sources` from `workspace.json`, but `sweep.py` hard-codes an Australia location filter (`LOCATION_OK` / `LOCATION_REJECT`) |
| Platform | Linux/Windows file opening | To do. `ui/detail.go` and `ui/download.go` call macOS `open` |
| | `NO_COLOR`, glyph fallbacks, non-colour active markers | Done (#32, `ROLECALL_ASCII`) |
| | Colour-blind-safe markers | Partial. Every colour pairs with a word or glyph; the palette itself is not checked for colour blindness |
| | Small terminals | Partial. Below 50×12 shows a `too small` notice; no compact layout |
| Release | goreleaser + Homebrew tap | To do |
| | README | Partial. Still says v0.1 and private |
| | Demo GIF | To do (VHS, carried over from v0.1.1) |
| | Fictional example workspace | Partial. `fixtures/workspace` is fictional but built for tests, not as a showcase |
| | MIT licence · CONTRIBUTING | To do |
| | Personal-data CI guard | Partial. `plugin/tests/test_personal_data.py` covers `plugin/` only; extend to Go, docs and fixtures |
| | No telemetry | Done (none exists); state it in the README |

**Suggested order:** ship v0.3 first. Then the release blockers, so the repo can go public:
licence, README, personal-data guard across the repo, neutral template, location filter from
the workspace, cross-platform open, `init` and `doctor`, goreleaser. Profile editing, the Settings
screen, template choices, style packs, import/export and auto-commit can follow in 1.x if
needed.

**To decide in the v1.0 grilling:** whether the Settings screen is required for 1.0 or can come
in 1.x; whether `rolecall init` may write the Profile's Contact table; what `rolecall migrate`
versions (a `schema` field in `workspace.json`?); which harnesses get an install guide; and
Windows support in or out.

#### Profile screen (v1.0)

Read-only in v1.0; editing inside rolecall comes in 1.x. Spelling and paper size belong to
Settings, and writing preferences stay prose, so the 1.x edit form covers the Contact only.

- **Screen 5**, after Stats. A section list on the left, built from the `##` headings of
  `profile/master-profile.md`, then a divider and the other Profile files (`projects.md`,
  `writing-preferences.md`, `phrasing-bank.md`). The selected section renders as Markdown on
  the right.
- **Counts** beside each section: entries for `###` sections, rows for tables, bullets for
  lists, items for comma lists, none for prose. Contact shows ✓, or ✗ with the reason in the
  content pane when Name or Email is missing (builds would fail).
- **Keys:** `j`/`k` section, `tab` focus, `e` opens the current file in `$EDITOR` (as on the
  Resume tab).
- **Edge cases:** no `master-profile.md` shows `No Profile yet: profile/master-profile.md not
  found` and `e` does nothing; a file with no `##` headings is one section, "Profile"; below
  80 columns the section list hides and the header names the section.
- **Refresh:** re-read on entering the screen, after `$EDITOR` closes, and on `r`. `profile/` is
  not watched.
- The Contact table is a file contract: see `SCHEMA.md`.

## Open questions

- **Bubble Tea v2:** confirm it's stable at setup. If not, use v1 and plan the upgrade.
- **Leads from `find-jobs` with no JD text:** keep stub `jd.md` files, or have the skill fetch and fill them? Default is stub plus URL. Revisit after a week of use.
