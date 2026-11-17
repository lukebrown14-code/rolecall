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
| **v0.1.1** | styling | colour palette (dark and light), status colours, layout tweaks, empty states, header, demo GIF |
| **v0.2** | harness + extras | `f`/`p` keys that hand off to the harness, URL fetch in Add, Stats screen (`4`), `b` build key |
| **v0.3** | reading | Markdown reader tabs for Resume, Letter and Gaps |
| **v0.4** | search rules and Sources | `search_rules` and the Source registry in `workspace.json`, Settings → Rules and Sources, posting age on Leads, Filtered view, `rules.py` dry run, coverage report (ADR-0007) |
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
├─ internal/store/   meta.yml read/patch, scan         ├─ profile/   (+ writing-preferences.md)
├─ internal/ui/      Bubble Tea models, views          ├─ cv/
├─ internal/config/                                    └─ jobs/<track>/<slug>/
├─ plugin/
│  ├─ .claude-plugin/plugin.json
│  ├─ skills/  create-profile, capture-job, find-jobs, assess-fit, prepare-documents, draft-follow-up,
│  │            import-applications, interview-prep, answer-questions
│  ├─ scripts/ build.py, sweep.py, new-job.py, sync-github.py, migrate.py, import_csv.py
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

## TUI (v0.1)

### Stack

Go · Bubble Tea · Bubbles (viewport, help, key) · Lip Gloss (layout, colour and layered
pop-ups from v0.1.1) · Glamour (Markdown in the detail view) · fsnotify · a TOML reader for
config. v0.1 used Huh for the forms and move popup; v0.1.1 draws them itself (see below). Use the latest stable major versions at setup.

### Config

`$XDG_CONFIG_HOME/rolecall/config.toml` when `XDG_CONFIG_HOME` is an absolute path, else
`~/.config/rolecall/config.toml` (`--dir` overrides `workspace`). Every reader and writer (`init`,
Settings, `doctor`, the no-Workspace message) uses the same path:

```toml
workspace = "~/job-search"
editor = ""            # empty = $VISUAL, then $EDITOR, then vi
follow_up_days = 14
downloads = "~/Downloads"
harness = ""           # v0.2: Handoff command template, e.g. 'claude "/{skill} {dir}"'
plugin = ""            # v0.2: path to rolecall/plugin; falls back to $ROLECALL_PLUGIN
inherit_terminal = false  # use the terminal's ANSI-16 colours, no painted background; NO_COLOR still wins
ascii = false          # ASCII glyphs; ROLECALL_ASCII=1 forces them on, a non-UTF-8 locale or TERM=linux turns them on
compact_rows = false   # one-line list rows at wide sizes; z toggles it and saves it here
```

Settings (key `6`) edits `editor`, `downloads`, `harness`, `plugin`, `follow_up_days` (1-90), `ascii` and `inherit_terminal` in place, keeping comments and other lines; `workspace` is never written. Environment switches with no config key: `ROLECALL_THEME`, `ROLECALL_ASCII`, `ROLECALL_PLAIN` and `NO_COLOR` (see **v0.1.1 look**).

If there is no workspace, rolecall exits with a one-line message naming the config path. No wizard until v1.0.
An invalid `workspace.json` (bad JSON, a wrong-shaped key, or a `tracks` list that breaks the Track
name rules) does not stop startup: the app opens with no Track filter and the footer says
`workspace.json: invalid Workspace settings · 6 then e to fix it` (`e` on a table screen edits the
row, so the hint names the screen first; on Settings it reads `e to fix it`). While `workspace.json`
or `config.toml` cannot be used, the Settings tab keeps a `⚠` at every width, after the banner is
dismissed too. Only a newer `schema` stops it.

### Screens

| Key | Screen | Views (`←` `→` or `s` cycle) | Holds Status |
|---|---|---|---|
| `1` | Leads | **Leads** │ **Dismissed** | Lead │ Dismissed |
| `2` | Drafts | — | Draft |
| `3` | Applied | **All** │ **Applied** │ **Interviewing** │ **Offer** │ **Closed** | All = every Status below, one section each · Applied │ Interviewing │ Offer │ Rejected + Withdrawn |
| `4` | Stats (v0.2) | — | — |
| `5` | Profile (v1.0) | — | — |
| `6` | Settings (v1.0) | groups, `←` `→` | — |

Each view shows a count in its chip on the context row (`VIEW s [Leads 4] Dismissed 2`). The active screen and table stay put across
refreshes, and the cursor stays on the same Application (matched by folder) when rows shift.

### Columns and sort

| Table | Columns | Sort |
|---|---|---|
| Leads, Draft | Title · Company · Track · Closes · Fit (+ Docs on Draft) | by deadline, soonest first: the close date, or for a Lead the day its ad reaches `max_posting_age_days` (`posted` + the rule), whichever is sooner. No deadline goes after every dated row. Ties go by Fit (Strong, Good, Marginal, Poor, not assessed), then `posted`, newest first. A passed deadline (`expired 1 Sep`, or `⚠ past the age limit`) sorts under an **Expired** section. `o` on Leads switches to newest added first, which mixes expired Leads back in; the Leads header names the order in force (`◌ Leads · 23 · by deadline`, `· newest first`) |
| Dismissed | Title · Company · Track · Dismissed (date) | newest first |
| Applied, Interviewing, Offer, Closed | Title · Company · Track · Applied · Fit · Docs · Follow-up | Applied date, newest first. In Applied and Interviewing, Follow-ups due come first, most overdue first |

- **Docs:** `✓ ready` once SCHEMA Ready, else how many are ready, `○ 2/4 ready` (SCHEMA → Documents state).
- **v0.1.1** shows these as two-line rows with relative dates; see **v0.1.1 look**. There,
  Docs shows on Draft only (the Applied screen's rows show Fit and the follow-up flag, and
  Closed rows name the outcome on the second line, `rejected ~7d ago` / `withdrawn ~7d ago`,
  under the Status glyph `✕` / `↩`).
- **Follow-up:** `⚑` when the Status is Applied or Interviewing and the later of the last change and the last recorded follow-up (`l`, `followed_up`) is more than `follow_up_days` old.
- **Fit:** in list rows the enum word alone (`Good`), in its own colour, left-aligned in a column as wide as `Marginal`, and blank when not assessed. The preview, card and detail keep the label (`fit Good`, `fit not assessed`).
- **Malformed** `meta.yml`: the row reads `⚠ Can't read SWE/malformed`, then the failing line and how to fix it
  (`meta.yml line 4: indented lines must be status_log entries: …`). The detail view says the same with the path.
  `e` opens the file in the Editor at that line (`+N` for vi, Emacs and nano, `path:N` for Sublime, Zed and
  Helix, `-g path:N` for VS Code; others open at the top). A search matches a malformed row by its folder only.
- **Approx dates:** `~` prefix, on exact dates (`~2026-01-01`) and relative ones (`applied ~3mo ago`).
- Columns shrink to fit the terminal width, truncating Title first, then Company.

### Keys

| Key | Scope | Action |
|---|---|---|
| `1`–`6` | global | switch screen: Leads, Drafts, Applied, Stats, Profile, Settings |
| `←` `→` | screen | previous / next view; in the detail view, previous / next tab; in Settings, previous / next group |
| `s` | table screen | next view, the same cycle as `→` (listed where a screen has more than one view; help, palette, menu and an empty view all say `s change view`) |
| `Tab` / `Shift-Tab` | screen | next / previous pane on Profile, the only screen where it is listed; elsewhere an unlisted alias of `→` / `←` (views, detail tabs, Settings groups) |
| `↑↓` / `j` `k`, `g` `G` | table | move cursor, top / bottom. In the detail view: scroll (`pgup` `pgdn` page) |
| `J` / `K` | detail view | next / previous row's detail, on the same tab (help and palette only, not the key line). In Settings › Tracking they reorder Tracks |
| `a` | table screen, Stats | Add form |
| `v` / `Enter` | row | detail view. On the Docs tab: open the selected document |
| `e` | row | Edit form (fields). On a malformed row: open `meta.yml` at the failing line (`e fix in editor`). In the detail view: open the current tab's file in the Editor |
| `n` | row | open `notes.md` in the Editor (created if missing) |
| `m` | row | Move popup. On a malformed row it shows the parse error instead, as `x` does |
| `u` | global | undo the last move or recorded follow-up made in this session (on the key line while there is one, after the toast has cleared; with none it says `Nothing to undo`). In Settings: put back the last Settings change |
| `x` | row (Lead, Draft) | Dismiss. Same as Move → Dismissed. On the key line on a Lead or Draft row; elsewhere it says `x: only Leads and Drafts can be dismissed · m to move` |
| `l` | row (Applied, Interviewing) | record that you followed up today (`followed_up` in `meta.yml`), without a Status change: the `⚑` clears and returns `follow_up_days` later. `u` takes it back. Elsewhere it says `l: only on Applied and Interviewing Applications` |
| `d` | row with PDFs | Download |
| `w` / `c` | row with a `url` (table, detail) | open the job ad's link in the browser / copy it to the clipboard |
| `t` / `T` | tables, Stats | next / previous Track filter: All → each `workspace.json` Track → any other Track a row is on |
| `o` | Leads view | sort by deadline, soonest first (the default) ↔ newest added first, which mixes expired Leads back in |
| `z` | tables, wide sizes | compact rows: one line per row (title, company and track in aligned columns, then the dates) ↔ two-line rows. Saved to `config.toml` as `compact_rows`. `z` is free on every list and is vim's view-adjusting prefix |
| `/` | table | text filter on Title + Company. `Esc` clears it on any screen (on Stats and Profile it says `Cleared search "/fe"`) |
| `r` | global | rescan the Workspace, on every screen (Settings › Tracking renames a Track with `Enter`, so `r` keeps this one meaning) |
| `,` | global | open Settings (screen 6); `Esc` returns |
| `h` | global, while no Harness is set | connect a Harness: Settings › This machine on the Harness row (every screen but Settings, with or without Applications). The no-Harness error offers it |
| `space` | global | Workspace menu: the screens, then Add, Search, Change view, Track filter, Rescan, Commands and Help, then a **This row** group (Open, Move, Dismiss, Undo, Edit, Notes where they apply); each key first, named as the help names it. An item's key runs it, `space` and `q` close |
| `?` | global | help for where you are: this context's keys, then the global ones, then the glyphs. `Esc`, `?` and `q` close it; any other key closes it and runs on the screen beneath |
| `:` / `Ctrl-K` | global | command palette: type to filter this context's actions (and one `<Screen> screen` entry per screen), `Enter` runs one |
| `q` / `Ctrl-C` | global | quit. `Ctrl-C` over typed text in a form or Settings field asks `Discard changes? y/n` first; a second `Ctrl-C` quits |

- **v0.2 adds** `4` (Stats), `f`, `p`, `F`, `b`: see [v0.2](#v02-harness-and-extras).
- The key line shows only the keys valid for the current row (e.g. `d` hidden when nothing is built), and
  is lean: the core keys (`enter`, `m`, `a`, `/`), `x dismiss` on a Lead or Draft, `u undo` while a move
  can be undone, then `esc` and `? help`; every other key is in `?`, the palette and the menu. Key
  names are lower case everywhere (`enter`, `esc`), drawn bare in the accent colour on the key line, in
  `?`, the palette and the menu (brackets mean "active" only), and each key has one name on all four.
- A known row key that does not apply on this row says why in a note instead of doing nothing (`o: sorts
  the Leads view only`, `d: no PDFs built for this Application yet`, `F: only on a Follow-up due
  Application`); an unknown key stays silent. The detail view gives the same notes, from the same
  table, for the row and view keys it does not run (`x` on an Applied row, `o`, `s`, `l`), and `x` on
  a Lead or Draft there says `x: dismisses from the list only · m to move`.
- **Settings keys never shadow a global key** with another meaning: `t` stays the Track filter
  and `r` rescan, so a Source is checked with `c` and a Track renamed with `Enter`.
- While no Harness is set, `?` and the palette still list `f` and `p` (and `F` on a Follow-up due row),
  dimmed and marked `(needs a Harness)`; pressing one gives `✘ No Harness set · …   h connect a Harness`.
- **One keymap:** each screen, focus and pop-up declares its keys with a priority (`internal/ui/help.go`). The key line,
  the `?` help and the command palette all read it, so a key is declared once. While a pop-up is open the key
  line shows the pop-up's keys. The `space` menu offers a key only where the registry lists it for the
  screen it was opened from.
- **Mouse:** click a screen tab, a view or a row; the wheel scrolls whatever `↑↓` moves; click the search
  chip's `×` to clear it. `ROLECALL_PLAIN=1` turns mouse capture off.

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
table, and the feedback line says `Moved to Interviewing · <Title> (<Company>)` with `u undo`: the Status first, so a narrow footer cuts the name, never where it went.

**Side effects when moving to Applied:**
1. If the documents are not Ready, ask first: a `⚠ Docs not Ready` pop-up, `y` mark Applied, `n`/`Esc` cancel.
2. If `sent/` does not exist, copy `resume.md`, `cover-letter.md`, `jd.md`, `fit.md`, `gaps.md` and `out/*.pdf` into it. Never overwrite an existing `sent/`.

**Moving to Rejected or Withdrawn** asks a second, optional question once the move is done: a
`Why did it end?` pop-up with one field cycling `skip` · `no-response` · `after-screen` ·
`after-interview` · `salary` · `role-closed` · `other` (`←` `→`; `enter` saves, `esc` or `skip`
leaves it as it is). The answer is `close_reason` (SCHEMA); free text can be typed into `meta.yml`.
The move stays whatever the answer. It follows the `filter_reason` pattern: a reason that explains a
Status travels with the entry that made it, and goes when the Application leaves that Status (the
undo of the move included). The card's `CLOSED` row shows it.

**Undo (`u`):** a session stack of `(folder, previous Status)`. Undo appends the previous
Status (the log stays append-only) and says so on the feedback line (`Undone · back to Applied · Title (Company)`). It deletes nothing: when the undone move to Applied created `sent/`, the undo renames it to `sent.undone-<date>/`, so the next real Applied freezes a fresh `sent/`.
Undoing an Application's first move back to Lead leaves it as it was: the card says `Not moved
yet` and the row gains no age (an empty effective log reads as never moved). A recorded
follow-up (`l`) sits on the same stack: its undo puts back the `followed_up` day it replaced, or
removes the key.

**Moved out of view:** when a move takes a view's last row elsewhere, the centre says where it
is now and the one key that goes there (`Now in Drafts · 2 to go there`, or `← → to follow it`
when another view of this screen holds it). The toast already says what moved, so the centre
does not repeat it.

### Add form (`a`)

Form fields: URL · **Title*** · **Company*** · **Track** (choice from `workspace.json`) ·
Closes (`YYYY-MM-DD`) · Fit (choice, blank allowed). The form opens on Title; with a Plugin
set it opens on the URL instead, where `Enter` fetches the ad. The URL is optional, and a
value that is not a web address is refused when the field is left (`✘ URL must start with
https://`); Edit never refuses a saved URL it did not change. Track needs no `*`: it is a choice and
always holds a track. With no Tracks yet (a fresh `rolecall init` Workspace), Track is a
required text field instead (`Track*`, a dim placeholder `e.g. Nursing, Marketing` that is an example, not a value), and saving adds that Track to
`workspace.json` before the Lead's folder is made. `Enter` checks the field and moves on; on the last field it checks them
all and saves; `ctrl+s` checks them all and saves from any field, and the keys in the border say
which enter does. `↑` `↓`, `tab` and `shift+tab` move between fields on every field; a choice
changes only with `←` `→` (or `space`), so passing over one never changes it. `ctrl+u` empties
the focused text field. Pasting fills the focused text field;
a long value shows its end, and a field's error sits on its own line below it (`✘ Title is
required`), its `*` turning red; the `*` is otherwise muted, with `* required` in the border.
Leaving a field checks its format (Closes, URL). `Esc` on a form with typed changes asks `⚠
Discard changes?` first, drawn as a warning (the Warn border, as `⚠ Docs not Ready`) with `Your
typed values will be lost.` above the fields and no `* required` note; `y` discards, `n`/`esc`
keeps editing. Every form pop-up (Add, Edit, Search area, gates, machine settings, Track
add and rename, Contact) follows these rules. A Settings pop-up opens with its current value
selected, so typing replaces it and `backspace` clears it; any other key keeps it.

On save:
1. If the URL matches another Application's `url`, warn (naming an open match before a closed
   one, with `+N more` when several share it): `o` discards this form and opens that one's
   Detail, `s` saves anyway, `esc` goes back to the form with every value kept.
2. Create `jobs/<track>/<slug>/` (SCHEMA → Folder name), write `meta.yml` (`schema: 1`, the
   fields, `status_log` with one `Lead` entry), and an empty `jd.md`.
3. Open `jd.md` in the Editor so the description can be pasted. On return, select the new row.
   With no Editor chosen (config, `$VISUAL`, `$EDITOR`) nothing opens, since the `vi` fallback
   traps a first-time user: the new row is selected and the toast reads `Paste the JD: pick an
   Editor (6 › This machine) or add a URL` (without the last clause when a URL was given).
   If the save fails, the form stays open with its values and the error under the field.

### Edit form (`e` in a table)

The same fields as Add, in the same order, plus **Resume only** (yes/no), **Next step** (a date,
`YYYY-MM-DD` with an optional `THH:MM`, under the Closes rules: it must be a real date), **What** (the next step in
words), and the first Contact (**Contact**, **Role**, **Email / URL**). A What needs a date, a Role needs a name or
address, and a Contact's address is an email or an `https://` link; the form stays open with the reason under the
field. Clearing the date removes `next_step`. Only the first Contact is edited here; the rest of `contacts` is kept
as it is, and saving a list drops the legacy `contact` key (its value is read into the list first). Saving patches
only the keys that changed. Changing Track does **not** move the folder.

### Detail view (`v` / `Enter`)

Header: one line, `‹ esc  Title · Company` (as the list reads) with `applied 25 Sep 2026 · 6d ago` or
`closes in <n>d` on the right (shorter when narrow). Dates in the detail read `25 Sep 2026`; a timeline
that spans years gives every stop its year.
Tabs (`←` `→`, or `tab` / `shift+tab`): **JD** · **Fit** · **Notes** · **Docs** (Fit beside JD: they
are read together), the active one bracketed (`[JD]`).
How the detail opens depends on the size class:

- **Wide:** the tabs take the list's place and the preview's card (Status, next step, facts,
  documents, timeline, notes, then the link as one `LINK  https://…  w open` row) stays where
  the preview was, on the right at its width, so the divider does not move.
- **Medium:** the detail opens as a bottom sheet under the top of the list, when the height
  holds at least 5 list lines and 8 detail lines; otherwise full-screen, as narrow. The
  sheet's title carries the Status chip and the line under its tabs the next step, on every tab.
- **Narrow:** full-screen, with the card folded into a two-line strip under the header: the
  timeline as a track (`◌ Lead 24 Sep ─ ● Applied 25 Sep ─ ◆ Interviewing ─ ★ Offer`), then
  documents · Fit · close date · Track, each shedding detail to fit rather than wrapping (the
  Track, the close date, then the documents fold into their meter, so the Fit stays). The line
  under the tabs is the next step, on every tab.

Where the card is not shown (the sheet and full-screen), an **Overview** tab comes first and
holds the card's rows, less what the frame already says: the next step (under the tabs), the
Status (the sheet's chip; it keeps the age and Fit line) and, full-screen, the Status, Fit and
documents (the strip). The next step thus sits in one place per size class: under the Status
line in the card, under the tabs without it.

The detail opens on the tab last chosen in the session where the Application has it (Fit
stays Fit from one Lead to the next), otherwise on its first tab. `J` / `K` show the next /
previous row of the current view without leaving the detail, on the same tab; they stop at
either end.

| Tab | Shows | `e` |
|---|---|---|
| JD | `jd.md` rendered with Glamour, scrollable | `$EDITOR` on `jd.md` |
| Notes | `notes.md` rendered | `$EDITOR` on `notes.md` |
| Fit | the `fit` value + `fit.md` rendered | `$EDITOR` on `fit.md` |
| Docs | list, under **Documents**: JD, Resume, Cover letter (`✓` ready or `○` missing) and Gaps (`ready · not counted`: the DOCS meter leaves it out); under **PDFs · out/** each PDF, or `○ PDF  not built yet` while `out/` is empty, so the tab lists every Document the meter counts; then `sent/` if it exists | `Enter` on a PDF opens it in the system viewer. On a `.md`, opens `$EDITOR` |

`Esc` goes back to the table. `m`, `n`, `d`, `u`, `r`, `w`, `c`, `l` and the v0.2 keys (`f`, `p`, `F`, `b`) work in the
detail view too; `x`, `s` and `o` do not, and say so in a note (`x: dismisses from the list only · m to move`).

### Next step, contacts and exports

Added after 0.6.0 (SCHEMA: `next_step`, `contacts`, `close_reason`).

- **Next step** (`next_step: {at, what}`): shown as `next 4 Oct 10:30 · Phone screen` on Interviewing and Offer
  rows (amber within 7 days, `▲` within 2, `was …` struck through once past), as a `NEXT STEP` row in the card and
  Overview, and in Stats → Now. Edited with `e`.
- **Contacts** (`contacts:`): a `CONTACTS` card row, one line each (`Sam Lee, Recruiter · sam@…`), below the
  timeline. `e` edits the first. The `draft-follow-up` skill addresses the first. The old `contact` key still
  reads as the one Contact; no writer adds it.
- **`rolecall now [--dir PATH] [--days N] [--list]`**: the Now panel as one plain line, `2 follow-ups due | 1 next
  step within 7 days | 3 closing within 7 days`, no colour, nothing printed when nothing is due, exit 0. The
  follow-up period is Settings'. `--list` prints `kind<TAB>folder<TAB>when<TAB>title<TAB>company` per item. It is
  the same code as the panel, so the two never count differently.
- **`rolecall export [--dir PATH] [--format csv|json | --ics] [--out FILE]`**: every well-formed Application
  (core fields, dates from the status log with undone moves left out, next step, contacts, `close_reason`, the
  log), or with `--ics` an iCalendar file of next steps (a one-hour event with a time, otherwise all day) and the
  close dates of Leads and Drafts. Malformed ones are skipped and counted on stderr. Both commands are
  read-only. `export` writes only to `--out` (ADR-0005), through a temp file, mode 0600; without `--out` it prints
  to stdout. A CSV cell starting `=` `+` `-` `@` gets a leading `'`.

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
selected row. Keep shared colour roles in `internal/ui/theme/` and UI-specific styles in
`internal/ui/styles.go` as neutral
styles, so v0.1.1 changes look by editing that file alone.

### v0.1.1 look (built)

Settled with a throwaway prototype (branch `prototype/v011-styling`) and two rounds of
context-free review. Shared colours and terminal appearance live in `internal/ui/theme/`; UI-specific styles live in `internal/ui/styles.go`.

- **Theme:** colours are named by role (text, muted, faint, rule, accent, ok, info, warn,
  danger, marginal for a Marginal Fit, one per Status), each with a dark and a light value. The theme follows the
  terminal's background (asked before start-up; `COLORFGBG` as the fallback), and
  `ROLECALL_THEME=auto|dark|light` overrides it. The view paints the theme's background
  (`#0D1117` or white), both as the terminal background (OSC 11) and with SGR on every cell,
  so a terminal that ignores OSC 11 still shows the theme's text on the theme's background
  rather than the light theme's dark text on its own dark one. Accent **Mint** (`#5EEAD4`, `#0F766E` on light) is the colour of keys
  (bold) and of focus: the selection gutter, the focused pane's rule and a just-moved row's
  flash. Labels, chips, borders and the active tab use other roles. Colour always pairs with a
  word or glyph.
- **Active markers:** the active screen tab, view, track, detail tab and Settings group are
  always bracketed — `[1 Leads 4]`, `[Leads 4]`, `[All]`, `[JD]`, `[Search]` — as well as filled,
  so they read without colour (and underlined where there is no fill: NO_COLOR, inherit,
  ASCII). Inactive options hold a space in each bracket's cell, so nothing shifts.
- **Accessibility:** `NO_COLOR` drops all colour but keeps bold, faint, underline,
  strikethrough and every word and glyph. `ROLECALL_ASCII=1` maps every drawn glyph to a
  one-cell ASCII stand-in, a different one for each meaning; a locale that is not UTF-8,
  or `TERM=linux`, turns ASCII on by itself. `ROLECALL_PLAIN=1` is for screen readers and
  braille displays: it turns off mouse capture (so the terminal's own selection and a
  screen reader's review cursor work) and the tab bar's sparkline, draws one pane at every
  width (no preview; the detail opens full-screen on `enter`), draws no rules or dividers
  (a line of rules is blank; a section header's trailing rule is spaces), says a Status
  glyph that stands without its word as the word (`Applied Software Engineer`), and puts
  the terminal cursor on the selected row (or the pop-up's selection). Its limits: the
  Profile screen keeps its section list beside the Markdown at 72 columns and up, the
  filter row keeps its ` │ ` between VIEW and TRACK, pop-up borders stay, and a row that
  had no spare cells for a Status word loses cells at its right end. Without colour
  (`NO_COLOR`) a pop-up is set apart by weight and space instead: the frame behind it
  drops its bold, and two cells either side and a row above and below are cleared. Text keeps a 4.5:1 contrast floor on either theme's background, pop-up
  fill and selection bar (coloured words on the bar take the secondary colour), and rules,
  pane dividers and Status glyphs 3:1, including the frame behind a pop-up, which dims
  without going below the floor. No text is drawn in the faint role or terminal faint (SGR 2). The inherit-terminal-colours option draws all readable
  text in the terminal's default foreground; ANSI 8 is for rules only. The `?` help is the
  keys (Navigate · Act · Filter & find), then **AI skills** (each Plugin skill in one line: what it
  does and the key that hands the row to it, `f` assess-fit, `p` prepare-documents (`b` builds the
  PDFs), `F` draft-follow-up; capture-job, find-jobs, create-profile and help run in the Harness;
  without a Harness it says `connect a Harness first: h or Settings › This machine`), then the
  glyph legend, then on a list **Row words**: `posted 12d` (age of the ad), `past the age limit`,
  `ad summary only`, `contract fits visa cap` / `12-mo contract > 6-mo visa cap`, `sponsors visa` /
  `no sponsorship`, `⚑ 6w` (Follow-up due), the Fit values in their own colours, and the
  Dismissed and Filtered views (Filtered: `dismissed by rules; restorable`). The
  legend is generated from the glyph table the frame draws with (`internal/ui/theme/glyphs.go`),
  one meaning per glyph:

  | Meaning | Glyph | ASCII |
  |---|---|---|
  | Lead · Draft · Applied · Interviewing · Offer | `◌ ✎ ● ◆ ★` | `( = * @ $` |
  | Rejected · Withdrawn · Dismissed | `✕ ↩ ⊘` | `\ { ;` |
  | approximate date | `~` | `~` |
  | Document ready · missing · not needed | `✓ ○ —` | `+ _ -` |
  | ready meter cell ready · missing | `■ □` | `# :` |
  | agent writing (spinner) | `⠹` | `%` |
  | needs attention · closes within 2 days · Follow-up due | `⚠ ▲ ⚑` | `! ^ }` |
  | form field changed, not saved | `Δ` | `&` |

  No stand-in is a letter, a digit, `/`, `?`, `<` or `>`: in the Move pop-up a Status's
  stand-in sits beside its key letter (`r  \ Rejected`, `x  ; Dismissed`), so a letter
  there would read as a key. Chrome carries no meaning of its own and may share
  stand-ins: separator `·` (`.`), ellipsis `…` (`.`, so cut text never ends in an arrow),
  selection gutter `▌` (`>`), rules and borders (`- | +`), arrows (`‹ ›` `← →` `↑ ↓` as
  `< > ^ v`), the wordmark `▞` (`R`), the sparkline's baseline `▁` (`_`) and bars (`. - #`)
  and the error mark `✘` (`X`), which always sits beside its words. A success
  toast, an undo included, leads with `✓` (`+`).
- **Status visual system:** each Status has a glyph and a colour on a cold-to-gold ramp —
  Lead `◌` grey-green (sage) · Draft `✎` pale blue · Applied `●` blue · Interviewing `◆`
  magenta on dark, purple on light (so it parts from Applied's blue under red-green colour
  blindness) · Offer `★` gold — and the closed Statuses are muted, never red: Rejected
  `✕`, Withdrawn `↩`, Dismissed `⊘` (ASCII `( = * @ $ \ { ;`). The glyph leads list rows, group headers
  (label and rule in the Status colour), the preview and detail chip, the timeline and
  the Move pop-up. A timeline shows each Status by its own glyph: the current one in its
  colour and bold, the others dim. When two panes share the
  frame, the focused pane's part of the top rule is the accent.
- **Other colours:** Fit Strong green · Good blue · Marginal yellow (its own role, apart
  from the urgency amber) · Poor grey. Close dates by urgency: `closes in 6d` amber and
  bold within 7 days, `closes in 2d ▲` red and bold within 2 (and today), `expired 3 Sep` struck through and muted once past — in rows, the preview and the
  detail view. Follow-up due is `⚑ 9mo` orange, bold once more than a week overdue. Documents are named JD · Resume ·
  Cover letter · Gaps · PDF everywhere, each `✓ ready` or `○ missing`; a Draft row reads
  `✓ ready` once SCHEMA Ready, else how many are ready (`○ 2/4 ready`), and the preview and
  detail card add a meter, `■■□□ 2/4 ready` (GLOSSARY avoids "built").
- **Size classes (`internal/ui/layout.go`):** wide ≥ 100 columns, medium 72–99, narrow < 72; short
  ≤ 20 rows; below 56×14 a resize notice. Wide has the preview beside the list, the card beside
  the detail, the wordmark, the sparkline and `space menu  ? help` in the tab bar. Medium has
  no preview, opens the detail as a bottom sheet and keeps Profile's and Settings' side lists.
  Narrow has one pane everywhere and the detail full-screen; `ROLECALL_PLAIN` draws the
  narrow class's single pane at every width. At any width the Track filter
  collapses to its active option (`t [All 4]▸`) when the context row is short of room. Short
  drops the rule under the tab bar. Every screen and pop-up takes its sizes from one budget. Side panels (preview,
  detail card, Profile sections, Settings groups) are a share of the width between a
  minimum and a maximum. Content starts at column 3 (markers and the selection bar at
  column 1) and the last column stays clear.
- **Layout, top to bottom (one fixed frame):** tab bar (screens; at wide sizes the
  wordmark, the Applied-per-week sparkline beside the Stats tab and `space menu  ? help` on the
  right) · a rule (dropped when short) · context row (`VIEW s [Leads 4] Dismissed 2 │ TRACK t
  [All 4] SWE 3  Platform 1`, left-anchored, the same grammar on Stats: `STATS │ TRACK t …`; on
  the right the search chip and scope) · top rule · body (with a preview pane when wide) ·
  bottom rule · footer. The rules and the `┬`/`┴` joins are
  always drawn; the footer is the breadcrumb line (with any toast or error) over the key
  line or the search input (actions left, navigation pinned right).
- **Tab bar:** every tab has the same padding, active or not, so nothing shifts between
  screens; the active tab is filled and bracketed (`[1 Leads 4]`), so it reads without
  colour. The wordmark (`▞ rolecall`) leads it at wide sizes only. As it narrows it drops
  other screens' counts (and the sparkline with them), the word "rolecall", then other
  screens' names (their digits stay), then the mark; the active screen keeps its name and
  count, and a space always parts its chip from the digit before it (`1 2 3 [4 Stats] 5 6`).
  It sheds to the level every screen's bar fits at, so switching screens never sheds
  or restores a count. The sparkline is the same series Stats draws, over 8 weeks, drawn muted
  on a faint baseline, so an empty week shows as `▁` rather than a gap, and labelled
  `applied/wk` while the bar has room (the label goes before the counts); it is hidden when
  no week has an Applied and under `ROLECALL_PLAIN`. Below 72 columns Settings, which then
  has no group list, pins `←→ section` on the key line.
- **Track filter:** on the context row of the screens it changes (Leads, Draft, Applied,
  Stats), read once by its key: `TRACK t [All 4] SWE 3  Platform 1`, or `t [All 4]▸` when short
  of room. A Track with no rows in the current view is left out unless chosen, and `t`/`T`
  pass over it, so the cycle never lands on an empty list (All always has its turn).
- **Scope line:** whenever a track or text filter is on, the context row says what it
  hides — `/pl ×   3 of 4` — and the screen tab counts read `3 of 4` rather than a filtered
  number that looks like a total (so other tabs shed their counts sooner). The view chips
  beside it show the filtered counts.
- **Footer:** two lines on every screen, so the bottom rule never moves: a breadcrumb
  (`Applied › All · All tracks · row 2 of 9`, `· detail` while a detail is open, `no rows` on an
  empty view; `Stats › All tracks`, `Profile › Contact`, `Settings › Search`), then the key
  line. Every position reads `N of M` (rows, tabs, Profile and Settings, `page 1 of 7` in
  help); only the documents meter keeps `2/4 ready`.
- **Feedback:** on the breadcrumb line's right; where it cannot sit whole beside it, it takes the whole line. Its keys are never cut (a success drops `esc hide` first). Every
  move, dismiss, save and Settings change says what happened, the Status first, then the
  title and company (`✓ Moved to Applied · Software Engineer (Orbit Labs)   u undo`,
  `✓ Dismissed · …`, `✓ Undone · back to Lead · …`, `✓ Saved Editor: nano · config.toml`), and
  a moved row is lit for 600 ms so the eye can follow it.
  A toast (successes clear on the next key) or an error (stays until `Esc`)
  shows on the breadcrumb line; it never replaces the bottom rule. Paths read relative to the
  Workspace. A move to Offer celebrates once, in gold: `★ OFFER · Orbit Labs ★  12
  applied · 41 days · first offer`; a rejection stays quiet.
- **Agent writing:** when the file watcher sees a document's file written (`jd.md`,
  `resume.md`, `cover-letter.md`, a PDF in `out/`), its chip reads `⠹ Resume writing…`
  for 4 s after the last write, beside the `■□□□ 1/4 ready` meter. That is all the TUI can
  honestly see of a Harness working in another terminal; a Handoff run hands the terminal
  over, so nothing is drawn during it.
- **First run:** while the Profile's Contact lacks Name or Email the app opens on the Contact
  form, titled `Welcome to rolecall · first, your Contact`, with a line on why (`rolecall puts
  these on every Resume and Cover letter. Name and Email are required; the rest is optional.
  You can change them any time: 5 Profile, then c.`; opened from the Profile screen, the last
  sentence is left out). It asks for Name, Email, Phone, Location, LinkedIn, GitHub and Work
  rights (SCHEMA's Contact fields), with no per-field `optional`, which wrapped at 80 columns
  and `esc skip` in its border; esc skips it until the next launch. Its placeholders say what
  goes in a field (`your full name`, `your email address`), never a plausible person's details.
  A Workspace with no Applications shows a centred welcome instead of
  empty panes: `▞ rolecall · no Applications yet`, then `a` add your first Lead, a short
  "Set up" checklist of the steps not yet done (`e` pick an Editor and `h` connect a
  Harness, each opening Settings › This machine on that row; `5` set up your Profile, until
  the Profile has a section besides Contact, worded as the Profile screen words it: `fill in
  your Contact, then run create-profile in your Harness`, or without a Harness `fill in your
  Contact, then write the rest in your Editor`), then
  `?` see every key, in one aligned key column. Each step shows only while it is not done (an
  Editor counts as set from config, `$VISUAL` or `$EDITOR`). Search and sort keys are not
  offered on it; Stats says `No Applications yet`. The wordmark's `▞` (ASCII `R`) is no
  Status's glyph.
- **Search:** while a text filter is on, a chip (`/pl ×`) shows on every screen, beside the detail title too;
  `Esc` clears it. While typing, the search input takes the key line; after `Enter` the scope
  line shows the chip and the count. A search with no match here says where it does match:
  `No match for "orbit" in Leads · 1 in Applied`, with the key that goes there (`3 go to
  Applied`, or `s` when the next view holds it).
- **Rows:** two lines (title + right-hand columns, then company · track · dates), always
  dense: rows in a section sit together, and only a section's header has a blank line above
  it (none in a bottom sheet, which cannot spare it). A blank Company is left out rather than
  leaving a separator hanging; an empty title reads `(untitled)`. Right-hand columns are
  the Follow-up due flag and Fit, two cells apart. The flag, on Applied and Interviewing
  rows only, says how long the row has been quiet (`⚑ 6w`, years past two years: `⚑ 7y`),
  flush right in a column as wide as its widest value in the view. Fit is the word alone
  (`Strong`, `Good`, `Marginal`, `Poor`), left-aligned in a column as wide as `Marginal`,
  blank when not assessed. A column empty for the whole view is hidden. A row has no age
  column: the second line says the dates (`applied 6d ago`, `posted 12d`), and the preview
  says how long the row has been in its Status. No close date shows nothing. Dates older
  than about three months read as the month (`applied Jan 2025`), not `21mo ago`. Expired
  Leads and Drafts sort last under an **Expired** section. Titles that would not read as a
  name (a bare slug, an empty title) are quoted in toasts and the detail breadcrumb.
- **Row badges:** after a Lead's or Draft's dates, what the ad says against the Owner's
  visa and about itself, blockers first so a narrow row drops the lighter ones:
  `⚠ no sponsorship` and `⚠ 12-mo contract > 6-mo visa cap` (bold, in the text colour),
  `sponsors visa` and `contract fits visa cap` (green), `ad summary only` (muted). Amber and
  red in a row mean time only: close dates, the posting age warning and the Follow-up flag.
- **Compact rows (`z`):** at wide sizes, one line per row: Status glyph, title, company
  and track in columns as wide as the view's widest (company at most 16 cells, track 12;
  a column that would squeeze the title below 16 cells is left out), then the dates and
  badges cut at the end, and the flag and Fit as above. About twice the rows fit (32 or
  more at 166×44). Below the wide class rows stay two lines. Saved as `compact_rows`.
- **Next step:** the preview, the detail card (or, without it, the line under the detail
  tabs) say what to do next and the key that does it, worked out from Status and
  documents: `NEXT  add the JD   enter open`, `FOLLOW-UP  due · no change in 9mo · l once you
  have followed up   l followed up` (`F draft follow-up` with a Harness; `due · followed up 2w
  ago` once one was recorded), and after `l` a `FOLLOW-UP  sent 3d ago · next due in 11d` row
  under NEXT, `NEXT  decide on the offer   m move`. A key
  shows once per card: a DOCS, CLOSES or NOTES row whose key NEXT offers leaves it to NEXT. A key is offered only
  where it works here (`f` with a Harness, `p` with a Harness and a JD, `F` with a Harness on a Follow-up
  due row, `b` with a Plugin on a buildable row). Without a Harness, "write the resume" offers
  `enter open` (the `.md` opens in the Editor) instead of `p`.
- **Timeline:** one track of Status glyphs everywhere — the preview, the detail strip, and
  the detail card, where it flows onto more lines. An empty log reads `Not moved yet · m
  to move`. Empty tabs say what fills them (`No JD saved · e to open jd.md`); the Fit tab
  is one line, `fit Good · no reasoning saved` (`fit not assessed · …` before one is assessed, with
  `f to assess fit` when a Harness is set).
- **Card** (the preview, the detail card and the Overview tab): the title, then `Company ·
  Track` alone (the close date, ages and badges each have their own row), then the Status line,
  `◌ Lead · added 1d ago`, `● Applied · 6w in Applied` (`today` on the day of a move, `3d ago`
  once closed or dismissed; short of room `● Applied · 6w`), with `fit Good` or `fit not
  assessed` at its right edge: the age the list rows leave out. Every `LABEL value` row shares
  one label column, wide enough for SPONSORSHIP and two spaces, so every value starts at the
  same cell. Block headings (TIMELINE, FIT, JD) are field labels on a line of their own. There
  is no CLOSES row without a close date. A figure the Sweep took on another day says which:
  `POSTED 2026-09-04 · 27d ago · site said "27d ago" on 24 Sep`, `FILTERED too old · as of 24
  Sep: posted …, 54 days ago; limit 30`. Beside the list, the room under the rows holds the
  start of `fit.md` (headings, tables and the verdict line dropped; at most 10 lines), then of
  `jd.md`, each cut with `↓ 5 more · enter open`.
- **Pop-ups:** one template — title in the top border, keys in the bottom border, solid
  fill, amber border and ⚠ for warnings. A pop-up is at least as wide as its key hints, so
  no key is cut, and is centred on the full width between the rules: it never covers the
  tab bar, context row or footer. Only its footprint and one cell either side are cleared
  (widened to the end of any word they would cut; a pane divider is never cut), so the
  dimmed frame stays in view beside it. A pop-up taller than the body scrolls inside its border, keeping
  the selection in view. On a short terminal a pop-up sheds its hints before the last
  (`esc`), latest first, rather than cut one, and the Move pop-up pins its subject (title,
  `Company · now Status`) above the scrolling options. Body lines that overflow wrap at
  spaces. `?` is a pop-up when wide; below the wide size class (and always under
  `ROLECALL_PLAIN`) it is a screen of its own, since a pop-up between the rules is too short
  for it: the page fills every line but the last, which holds its title (`Keys · Leads · page 1
  of 2`) on the left and its keys on the right. Either way `esc`, `?` and `q` close it, any
  other key closes it and runs beneath (`? m` opens Move), and it pages (`↑↓`, `pgup` `pgdn`,
  the wheel) only when it has a second page: at most two pages from 72×24, five at 56×14. Its
  key sections sit three side by side where they fit, two below that (from about 56 columns),
  one under another only on the narrowest; full-screen, where that saves a page, they run on
  down three columns like a newspaper's. Too narrow for three glyph cells a line, each legend
  row packs its items. A page under 10 lines drops its blank spacers and padding rows, and a
  spacer goes rather than a section moving to the next page. The `space` menu is centred like every pop-up, key first. The Move
  pop-up always moves the Application it shows, even if a live refresh moved the cursor.
- **Overflow:** one cue everywhere — `↓ 3 more` / `↑ 3 more` at the text column — on lists,
  Stats, Profile, Settings, readers and pop-ups. Stats taller than the body scrolls with
  `j k ↑↓`, `g G` and the wheel. Each `?` section (keys, AI skills, glyphs, row words) starts a
  page of its own rather than split when it fits on one, unless keeping sections whole would
  take more pages, and a section heading never ends a page or a column.
  Text wraps only at spaces.
- **Narrow terminals:** the tab bar sheds detail, the preview and detail card fold away
  below the wide class, pop-ups shrink to fit. The key line drops actions from the right
  (`x dismiss` first) but keeps `enter`, `m move`, `a add`, `/ search` and `u undo` (while there is
  one) to the last, and `? help` and `esc` always; navigation hints (`1-6 screen`, `←→ view`) go first. The detail view keeps `←→
  tab` and `↑↓` over its actions. Below 56×14 a centred notice — `⚠ rolecall
  needs 56×14`, then `now 48×12 · resize to continue · q quit` — replaces the layout.

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
| `create-profile` | creates or updates the Owner's Profile from a resume or guided questions, then gets Owner approval |
| `find-jobs` | runs `sweep.py`, triages, creates a Lead folder per role worth a look (as capture-job, `jd.md` may be a stub with the URL) |
| `assess-fit` | `fit.md` + `fit:` in `meta.yml` |
| `prepare-documents` | `resume.md`, `cover-letter.md`, `gaps.md`, sets `resume_only` if chosen, runs `build.py` |
| `draft-follow-up` | `follow-up.md` (a message for the Owner to send; the TUI never sends it) |
| `import-applications` | new folders from a CSV export through `import_csv.py` (approximate `status_log`, `notes.md` from the notes column); dry run first, Owner confirms the column and status mapping |
| `interview-prep` | `prep.md` (likely questions, a STAR story from the Profile per gap, questions to ask; nothing the Profile cannot evidence) |
| `answer-questions` | `answers.md` (answers to pasted application-form questions from `jd.md` and the Profile) |

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
Built: see **v0.1.1 look** above. Dark and light themes, following the terminal. Still to do: VHS demo GIF.

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
- Unset `harness` hides `f`/`p`/`F` everywhere (help bar, NEXT, the DOCS row); pressing one shows
  `No Harness set · 6 Settings › This machine › Harness`. Unset `plugin` does the same for `b` and
  fetch (Add still works by hand).

#### Handoff: `f`, `p` and `F`

| Key | Skill | Applications | From |
|---|---|---|---|
| `f` | `assess-fit` | any | table row, detail view |
| `p` | `prepare-documents` | Lead, Draft | table row, detail view |
| `F` | `draft-follow-up` | Follow-up due | table row, detail view. Its own key, so `r` is always rescan |

Interactive mode: the Owner talks to the agent in their terminal (ADR-0003). On exit the
folder is rescanned. A non-zero exit shows `Harness exited with status N` in the feedback
line. No pop-up, no retry.

#### Build: `b`

- Runs `python3 <plugin>/scripts/build.py <dir> --data <workspace>` in captured mode. It needs only
  the Plugin, not a Harness: a `resume.md` written by hand builds the same way.
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

Answers "how is my search going?". No tables: `Tab` and row keys do nothing (`a` still adds). The `t` track
filter applies. Top to bottom:

Stats reads each well-formed Application's `status_log` in order (`approx` entries
included; malformed ones are left out of every figure and of Stats' `t` counts). A move and
the undo that restores the Status before it within 15 minutes cancel out. A later Status
implies the ones it cannot happen without: Interviewing, Offer, Rejected and Withdrawn
imply Applied, and Offer implies Interviewing, so the numbers agree with the screens. An
empty log is Lead.

Every bar on Stats is one grammar: a solid run in the role colour on a faint `░` track of
24 cells (ASCII `#` on `.`), so the funnel, the Response rate and the weekly chart read on
one scale of width. From about 128 columns Stats takes two columns: the funnel and the
Response rate on the left, the weekly chart and By track on the right; Now leads and
Insights follows, full width.

0. **Now**: what wants doing today, each count with the key that goes there:
   `3 follow-ups due → 3 Applied · 1 next step within 7 days → 3 Applied · 1 closing within 7
   days → 2 Deadlines`. A next step counts from today to 7 days ahead on an Application that is not
   Rejected, Withdrawn or Dismissed. Hidden when nothing is due. `rolecall now` prints the same
   counts (see [Next step, contacts and exports](#next-step-contacts-and-exports)).
1. **Funnel**: Applications that **ever reached** each Status: Lead → Draft → Applied →
   Interviewing → Offer, one bar per Status in its glyph and colour. Lead and Draft are
   optional starts, so a rung can be higher than the one before and there is no drop-off;
   where one is, the caption says so (`They can start at any Status, so a bar can outgrow
   the one above`). A rung that all came through the previous one says what share of it
   reached it (`29% of Applied`); a rung with Applications that never were on the previous
   one says how they arrived, as counts (`1 from Leads · 2 started at Draft`, `3 via Draft
   · 8 never a Draft`, `11 never a Draft`), never `0% of` or `none`. Nothing when the
   previous rung is empty. Rejected, Withdrawn and Dismissed on a muted `closed` line. Under it,
   once any closed Application has a `close_reason`, a `Dropped off` line counts them by reason,
   most first (`no-response 4 · after-screen 2 · no reason 1`).
2. **Response rate** (GLOSSARY → Response): reached Interviewing or Offer ÷ ever Applied,
   leaving out Applications whose Applied date (an implied one takes the date of the Status
   implying it) is within `follow_up_days`, on the funnel's bar (`36% · 4 of 11`). A line
   says how many were left out as too new.
3. **Applied per week**: last 12 weeks, Monday-start, local time, by logged Applied date,
   two cells a week; an empty week sits on the faint baseline `▁`, so all 12 show. The
   first and last week's dates sit under the chart's ends; this week's count, the busiest
   week (`max 3/wk`) and the 12-week average beside it. The tab bar's sparkline beside
   Stats is the same series over 8 weeks.
4. **By track**: track × Applied (as the funnel counts it) · Counted (the Response
   denominator) · Responses · Rate, busiest Track first (ties in `workspace.json` order,
   off-list Tracks after), then one line naming the Tracks with nothing applied (`No
   Applications sent: Platform`) rather than rows of zeros, and a line saying what Counted
   means when there is room. Hidden when the filter is on one track.
5. **Insights** (GLOSSARY → Insights): the gaps `fit.md` names most, with how many
   Applications name each and how many it would unlock.

The `t` filter's options are All, the `workspace.json` Tracks, then any other Track a row
is on (a legacy `Unknown`, or a renamed or removed one), so the options add up to All.
This holds on the Application screens too.

With no Applications, Stats says `No Applications yet · a to add a Lead`. The fixtures fit
80×24 without scrolling.

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
The detail view opens on JD and shows these tabs in order:

| Tab | Source | Visibility |
|---|---|---|
| JD | `jd.md` | Always |
| Fit | `fit.md` | Always |
| Resume | `resume.md` | Has non-whitespace text |
| Letter | `cover-letter.md` | Has non-whitespace text |
| Gaps | `gaps.md` | Has non-whitespace text |
| Notes | `notes.md` | Always |
| Docs | Document inventory | Always |

Reader tabs render the Markdown source as written, using the JD tab's Glamour
renderer, reading width and scroll keys. `e` opens the reader's file in `$EDITOR`.
The current reader tab is tracked by file name. Left/right wraps through visible tabs.
Live refresh keeps the current tab and scroll offset when text changes; newly
written documents add tabs without moving the reader. If the current reader's
file is deleted or emptied, move to Docs with `<file> is gone`. Moving to an
Application without the current tab falls back to JD.

Docs keeps the source files, PDFs and `sent/` inventory. Enter on a written
Markdown document opens its reader tab; on a missing document it says so.
`e` still opens the Markdown file in `$EDITOR`, and
Enter on a PDF opens the system viewer. A source file with no text reads `missing`.
For a Resume only Application without letter text, the card and Docs show
`not needed · Resume only` for the cover letter, and no Letter tab appears.
Reader scroll/edit hints match JD; help explains Enter in Docs.

Acceptance:

1. A Draft with all three written documents shows JD · Fit · Resume · Letter · Gaps · Notes · Docs; Resume scrolls and `e` edits it.
2. A Lead without documents shows JD · Fit · Notes · Docs.
3. A resume written through `p` adds its tab on return without moving the current tab.
4. Enter on `resume.md` in Docs opens Resume; a missing document reads `missing`.
5. Resume only Applications show no Letter tab and show `not needed · Resume only` in the card and Docs.
6. Tabs fit at 80 columns and readers work with `harness` and `plugin` unset.
7. `scripts/check-go.sh` and `go test -race -count=3 ./internal/ui` pass.

These are readers; preview continues to mean the side pane beside the tables.
Documents state rules, Build and the Plugin are unchanged.

### v1.0: public
| Area | Features |
|---|---|
| Onboarding | `rolecall init [DIR]` creates the Workspace and runs `git init` with no prompts; the app's first-run form asks for the Contact (spelling and paper size are fixed at AU and A4 for v1) · `rolecall doctor` (typst, pandoc, Python, fonts, harness) · plugin install guide per harness |
| Profile | read-only Profile screen (screen 5; editing in 1.x) · `create-profile` skill (resume, LinkedIn export or guided questions → approved Profile) |
| Settings | Settings screen (screen 6): Tracks, Search area, gates, follow-up days, downloads, editor, harness and plugin (the theme follows the terminal or ROLECALL_THEME, so no row) |
| Documents | neutral default template + workspace brand override · 2–3 template choices (style packs AU/UK/US move to 1.x) |
| Data | `schema` migrations (`rolecall migrate`) · CSV/spreadsheet import · CSV/JSON export · optional auto-commit on Status change |
| Finding | per-workspace sources and locations, no default city |
| Platform | Linux/Windows file opening · `NO_COLOR` · colour-blind-safe markers · small terminals |
| Release | goreleaser + Homebrew tap · README, demo, fictional example workspace · MIT licence · CONTRIBUTING · personal-data CI guard · no telemetry |

#### What v1.0 still needs (agreed 2026-11-12)

Agreed in the v1.0 grilling session of 2026-11-12. Every item below is ticketed, and the
public-repo cut (#82) is blocked by all of them.

**In 1.0:** privacy and the public-repo cut (ADR-0005, #73), the Profile screen (#49), the
Settings screen (#58, including the Search area filter in the Sweep), the `create-profile`
skill, and the release blockers in the table.

**Moved to 1.x:** UK/US style packs and spelling, template choices, CSV import/export,
auto-commit on Status change, `rolecall migrate`, and #59–#61.

**Platforms:** macOS and Linux are supported; Windows through WSL. Native Windows file
opening stays but is best-effort, with no CI.

| # | Item | Decision |
|---|---|---|
| 1 | Ownership | v1 stays under `lukebrown14-code`: public repo `lukebrown14-code/rolecall`; module path `github.com/lukebrown14-code/rolecall`; tap `lukebrown14-code/homebrew-tap`. The cut renames the private repo to `rolecall-archive` and creates the fresh public `rolecall`; no organization transfer. Moving to an organization is deferred past v1 |
| 2 | Licence · CONTRIBUTING | MIT, "the rolecall authors". CONTRIBUTING: checks and hooks, reproduce with the example workspace, no personal data, proposals via issues |
| 3 | README | Rewrite for the public release, including "Make your documents yours" (the Privacy section is #77) |
| 4 | Template | "Header Band" stays the default with neutral comments. A Workspace `brand.typ` overrides the Plugin's. `init`'s closing line and a `doctor` info line point to the README |
| 5 | goreleaser | macOS and Linux, amd64 and arm64, to GitHub Releases and the tap. Archives include the Plugin; `plugin` defaults to the installed path |
| 6 | Harness guides | Claude Code in full (installed Plugin as a local marketplace); one generic guide with `harness` examples for the rest |
| 7 | `init` and first run | `rolecall init [--switch] [DIR]` asks nothing: it creates the Workspace (default `./rolecall`) and records it in `config.toml`. When `config.toml` already names another existing Workspace, that stays the default and init prints `rolecall --dir DIR` and `rolecall init --switch DIR`; `--switch` repoints `config.toml` (also for an existing Workspace). It writes a starter `profile/master-profile.md` (the Contact table, then each expected `##` heading with a one-line how-to as a comment, so a Profile can be written by hand without a Harness). The app opens on a Contact form (Name and Email required) while either is missing; Esc skips it until the next launch. Tracks and Search area are set in Settings; the Profile comes from the Harness `create-profile` skill. Replaces the Huh wizard |
| 8 | `create-profile` skill | From a resume (copied into `cv/`) or from scratch by asking questions; gap-filling questions, then the Owner approves the Profile |
| 9 | `doctor` tool checks | typst, pandoc, Python 3, Plugin path, Template fonts, Harness command. Warnings with install hints; non-zero exit only when the Workspace can't be opened. Builds on #78 |
| 10 | Schema guard | A missing `schema` in `workspace.json` means 1; a file with a newer schema is refused with a clear message |
| 11 | Codebase review | Full review of the codebase against `CODING_STANDARDS.md` and the ADRs once all code work has merged |
| 12 | Design review | Full design review of the TUI, `init` and the documents once all UI work has merged |
| 13 | Showcase workspace | One fictional Owner, about 10 Applications across every Status, with built PDFs. `fixtures/workspace` stays test-only |
| 14 | Demo GIF | Recorded with VHS from the showcase workspace |

#### Profile screen (v1.0)

Read-only in v1.0; editing inside rolecall comes in 1.x. Spelling and paper size are not
settings in v1 (AU and A4 are fixed; style packs move to 1.x), and writing preferences stay
prose, so the 1.x edit form covers the Contact only.

- **Screen 5**, after Stats. A section list on the left, built from the `##` headings of
  `profile/master-profile.md`, then a divider and the other Profile files (`projects.md`,
  `writing-preferences.md`, `phrasing-bank.md`). The selected section renders as Markdown on
  the right.
- **Counts** beside each section: entries for `###` sections, rows for tables, bullets for
  lists, items for comma lists, none for prose. Contact shows ✓, or ○ (missing) with the reason
  in the content pane when Name or Email is missing (builds would fail).
- **Keys:** `j`/`k` section (scroll once `tab` moves focus to the content; the key line says
  which), `g`/`G` first and last section (top and bottom of the content), `tab` focus (on the
  key line while the list shows), `e` opens the current file in `$EDITOR` (as on the Resume
  tab), `c` opens the Contact form (`c fill in Contact` on the key line while Name or Email is
  missing; saving writes only the Contact table). `e edit file` shows once, on the section row,
  not on each Contact field.
- **Focus:** the context row names both panes and brackets the focused one (`[SECTIONS]
  CONTENT`), and the focused pane's rule takes the accent. While the content has focus the
  list's selected row keeps its `▌` mark but loses its fill, as the list does under a sheet.
- **No Harness:** a Profile with only a Contact lists each expected section (`Headlines`,
  `Experience`, `Key Skills keyword pool`, `Summary`, `Facts that are NOT available`) with how to
  fill it by hand: under the Contact when the file lacks the headings, or on each section's own
  page when `rolecall init`'s starter file has them (its `<!-- -->` how-to comment is a hint,
  not content, so a section holding only it counts as empty). A Contact with no fields shows `No
  Contact details yet · c to fill them in`, never an empty Field│Value table.
- **Edge cases:** no `master-profile.md` shows `No Profile yet: profile/master-profile.md not
  found`, then the welcome's instruction (`Set up your Profile: fill in your Contact, then …`,
  naming the Harness only when one is set) and `c fill in your Contact · r read it again`;
  `e` does nothing; an empty file or section reads `Nothing here yet · e to
  write it…`; a file with no `##` headings is one section, "Profile"; below 80 columns the
  section list hides and the header names the section.
- **Refresh:** re-read on entering the screen, after `$EDITOR` closes, and on `r`. `profile/` is
  not watched.
- The Contact table is a file contract: see `SCHEMA.md`.

#### Settings screen (v1.0)

Laid out like Profile. Parent spec: #58; the read-only first slice was #66, and the editing slices
below are built.

- **Screen 6**, after Profile (`6 Settings` in the header). The groups (**Search ·
  Documents · Tracking · This machine**) are tabs in the context row, the active one bracketed
  (`[Search]`); every group is named while they fit, else only the active one and its place
  (`[Search] 1/4`). At wide sizes an about pane beside the rows says, in one plain sentence,
  what the focused setting is, with an example and the file it is kept in; narrower, the
  sentence sits under the rows and is the first thing a short terminal drops.
- **Values shown**, each from the file that owns it:
  - Search: Places and Remote rule (`workspace.json` → `search_area`), then `Sources · N` and
    `Blockers · N` count rows.
  - Documents: cover letter word cap and minimum, Key Skills minimum and maximum
    (`workspace.json`; the Build defaults 350, 150, 8 and 12 when unset).
  - Tracking: Tracks (`workspace.json`), then any Track Applications use that
    `workspace.json` does not list (`Unknown · 6 applications · not listed`; Enter adds it, after
    which it renames like any other; a name the list could not hold, such as a case twin of a
    listed Track, is refused and nothing is written), and follow-up days (`config.toml`).
  - This machine (`config.toml`): workspace path (shown, never editable), editor, downloads,
    harness, plugin.
- **Keys:** `←`/`→` switch the group, as they switch views elsewhere (`tab` and `[` `]` too);
  `j`/`k` move the row, `g`/`G` go to the first and last; `k` above the first row rests on the
  tabs. The cursor skips read-only rows (Workspace, Sources, Blockers), which say what they are
  and why they are fixed. Tracking lists each Track with its Application count and a `+ add a Track`
  row; `Enter` renames the selected Track, and `a` add, `x` remove and `J`/`K` reorder act on it. `e`
  opens `workspace.json` in the Editor, or `config.toml` while This machine is selected; `r`
  rescans, on a Track row too. Each Track row's count reads like `10 applications · 7
  active · 2 closed · 1 dismissed`, leaving out the parts that are 0. One save model in every group: Enter saves at once (a pop-up's last field, an
  inline field, a Yes/No toggle), says what and where (`Saved Editor: nano · config.toml`), and
  `u` puts the last Settings change back (not a Track rename). There is no staged save. An
  inline field over a set value opens with it selected, so typing replaces it; over an unset
  value it opens empty with the default in effect as its placeholder. Undoing a save over an
  unset value names what it falls back to, as the row did (`Editor back to $VISUAL (nvim)`,
  `Follow-up due after back to the default (14)`); a Rules undo names the value as its row
  showed it (`Needs sponsorship back to Yes`, `Max contract length back to 12 months`). Emptying a text setting
  clears it, so the default applies again; emptying Places clears the Search area (every
  location kept). `ctrl+u` empties an inline field or a pop-up's text field (`ctrl+u clear` on
  the key line while a field is open). Settings names are drawn in the body text colour and
  values a step quieter. `ctrl+c` over typed text in a field or Settings pop-up asks `Discard changes?
  y/n` first (`⚠`, in the Warn colour); a second `ctrl+c` quits.
- **Sources:** each row is the Source's name and what it is (`api · greenhouse · 2 targets ·
  works`), with no file column: every Source is kept in `workspace.json`, which the about pane
  names once. `Enter` opens the Source's form (name, type, platform, targets, URL, access) and
  saving rewrites those keys in place, keeping its others; `o` turns it off or on (`o turn off`
  on the key line); both save at once with `u` to put it back, and `Enter` never changes whether
  a Source is on. `c` checks it (`sweep.py --test-source`) and reports `greenhouse: works · 41
  jobs`; `t` stays the Track filter.
- **`,`** opens Settings from any screen, including the detail view and Stats. `Esc` returns to
  the screen, table, cursor, open detail and filter it was opened from, whether it was opened
  with `,` or `6`. In a text filter `,` is typed, not a key.
- **Source:** every value is followed by where it came from: `workspace.json`, `config.toml`,
  `from $VISUAL`, `from $EDITOR` (Editor only), `from $ROLECALL_PLUGIN` (Plugin only), `from
  --dir` (Workspace only) or `default`. Unset values show their effective default (Editor `vi`,
  follow-up 14 days); a value with no default reads `not set` and gives an `example:`. Values
  carry their unit (`14 days`, `350 words`).
- **Editor:** shows the command in effect. If the first word is not on `PATH` (quoted paths
  honoured) the row adds `⚠ not found on PATH`. This never blocks launch; `rolecall doctor`
  reports the same condition as a warning. A Harness whose program is not on `PATH`, and a
  Plugin folder that is missing or has no `skills/`, warn the same way and never block a save.
- **Config reload:** `e` on This machine creates `config.toml` empty if missing, opens it in
  the Editor, and re-reads it when the Editor exits. On a TOML error the old values stay, the
  feedback line shows the error with its line and column, This machine shows the path and the
  parser's reason above its rows, Enter on a `config.toml` row is refused, and `e` opens
  `config.toml` from any group (unless `workspace.json` is broken too). The Workspace is fixed
  for the session. At launch a broken `config.toml` stops `rolecall` with the same detail
  (the Workspace path is in it), unless `--dir` names the Workspace: then the app opens on
  defaults and shows the error. `rolecall doctor` reports it as a PROBLEM and runs its other checks.
- **Editor examples:** the Editor's about text names `nvim`, `hx`, `code --wait`, `zed --wait`,
  `subl --wait`. GUI Editors need their wait flag, or rolecall rescans before editing finishes.
- An absent `follow_up_days` means the default 14. A hand-edited value outside 1-90 (0
  included) is clamped into range on load; the Follow-up row, the feedback line and
  `rolecall doctor` say so.
- **Invalid `workspace.json`:** the screen shows its path and the parse error (for a bad Track
  list, `workspace.json lists a Track rolecall can't use` and the reason, e.g. `Track SWE is
  listed twice`), with `e` as the only action. The program still starts, with no Track filter.
  Nothing is written.
- **Refresh:** both files are re-read on entering the screen, on `r`, and after `$EDITOR` closes.
- **Editing (built):** Tracks (add, rename, remove, reorder), the Search area pop-up, the gates
  and the machine settings are editable rows. The TUI writes `workspace.json`'s `tracks`,
  `search_area` and gates (`SCHEMA.md` → `workspace.json`). Renaming a Track asks first
  (`1 Application changes its Track…` / `N Applications change their Track…`) and `u` does not
  undo it.

## Open questions

- **Bubble Tea v2:** confirm it's stable at setup. If not, use v1 and plan the upgrade.
- **Leads from `find-jobs` with no JD text:** keep stub `jd.md` files, or have the skill fetch and fill them? Default is stub plus URL. Revisit after a week of use.
- **Clearing a Follow-up due without a Status change:** decided (design review 2026-11-16, C16):
  `l` writes `followed_up: YYYY-MM-DD` to `meta.yml` and the flag measures from the later of the
  last change and `followed_up`. The Python reader reads it too (`meta.py follow-up` reckons Follow-up
  due the same way). The draft-follow-up skill does not write it: recording a follow-up is the
  Owner's action, so the skill says when they last followed up and tells them to press `l` once sent.
