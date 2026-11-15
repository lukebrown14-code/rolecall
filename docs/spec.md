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
| **v0.3** | reading | Markdown reader tabs for Resume, Letter and Gaps |
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
│  ├─ skills/  create-profile, capture-job, find-jobs, assess-fit, prepare-documents, draft-follow-up
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
inherit_terminal = false  # use the terminal's ANSI-16 colours, no painted background; NO_COLOR still wins
ascii = false          # ASCII glyphs; ROLECALL_ASCII=1 forces them on, a non-UTF-8 locale or TERM=linux turns them on
```

Settings (key `6`) edits `editor`, `downloads`, `harness`, `plugin`, `follow_up_days` (1-90), `ascii` and `inherit_terminal` in place, keeping comments and other lines; `workspace` is never written.

If there is no workspace, rolecall exits with a one-line message naming the config path. No wizard until v1.0.

### Screens

| Key | Screen | Tables (`Tab` cycles) | Holds Status |
|---|---|---|---|
| `1` | Leads | **Leads** │ **Dismissed** | Lead │ Dismissed |
| `2` | Drafts | — | Draft |
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
- **Malformed** `meta.yml`: the row reads `⚠ Can't read SWE/malformed`, then the failing line and how to fix it
  (`meta.yml line 4: indented lines must be status_log entries: …`). The detail view says the same with the path.
  `e` opens the file in the Editor at that line (`+N` for vi, Emacs and nano, `path:N` for Sublime, Zed and
  Helix, `-g path:N` for VS Code; others open at the top). A search matches a malformed row by its folder only.
- **Approx dates:** `~` prefix, on exact dates (`~2026-01-01`) and relative ones (`applied ~3mo ago`).
- Columns shrink to fit the terminal width, truncating Title first, then Company.

### Keys

| Key | Scope | Action |
|---|---|---|
| `1` `2` `3` | global | switch screen |
| `←` `→` | screen | previous / next view (table); in the detail view, previous / next tab |
| `Tab` / `Shift-Tab` | screen | next / previous pane (Profile); next / previous section (Settings); on a table screen and in the detail view, which have one pane, the same as `→` / `←` |
| `↑↓` / `j` `k`, `g` `G` | table | move cursor, top / bottom |
| `a` | global | Add form |
| `v` / `Enter` | row | detail view |
| `e` | row | Edit form (fields). In the detail view: open the current tab's file in `$EDITOR` |
| `n` | row | open `notes.md` in `$EDITOR` (created if missing) |
| `m` | row | Move popup |
| `u` | global | undo the last move made in this session |
| `x` | row (unsent) | Dismiss. Same as Move → Dismissed |
| `d` | row | Download |
| `t` / `T` | global | next / previous track filter: All → each `workspace.json` track |
| `s` | Leads | sort by close date, soonest first (the default) ↔ newest added first, which mixes expired Leads back in |
| `/` | table | text filter on Title + Company. `Esc` clears it |
| `r` | global | rescan the workspace |
| `,` | global | open Settings (screen 6); `Esc` returns |
| `?` | global | help for where you are: this context's keys, then the global ones, then the glyphs |
| `:` / `Ctrl-K` | global | command palette: type to filter this context's actions, `Enter` runs one |
| `q` / `Ctrl-C` | global | quit |

- **v0.2 adds** `4` (Stats), `f`, `p`, `b`: see [v0.2](#v02-harness-and-extras).
- The bottom help bar shows only the keys valid for the current row (e.g. `d` hidden when nothing is built).
- **One keymap:** each screen, focus and pop-up declares its keys with a priority (`internal/ui/help.go`). The key line,
  the `?` pop-up and the command palette all read it, so a key is declared once. While a pop-up is open the key
  line shows the pop-up's keys.
- **Mouse:** click a screen tab, a view or a row; the wheel scrolls whatever `↑↓` moves; click the search
  chip's `×` to clear it.

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
Status (the log stays append-only) and says so on the feedback line (`Undone · Title back to Applied`). It deletes nothing: when the undone move to Applied created `sent/`, the undo renames it to `sent.undone-<date>/`, so the next real Applied freezes a fresh `sent/`.

### Add form (`a`)

Form fields: URL · **Title*** · **Company*** · **Track** (choice from `workspace.json`) ·
Closes (`YYYY-MM-DD`) · Fit (choice, blank allowed). Track needs no `*`: it is a choice and
always holds a track. `Enter` checks the field and moves on; on the last field it checks them
all and saves; `ctrl+s` checks them all and saves from any field, and the keys in the border say
which enter does. Choices change with `←` `→` (or `↑` `↓`). Pasting fills the focused text field;
a long value shows its end, and a field's error sits on its own line below it (`✕ Title is
required`), its `*` turning red; the `*` is otherwise muted, with `* required` in the border.
Leaving a field checks its format (Closes). `Esc` on a form with typed changes asks `Discard
changes? y/n` first. Every form pop-up (Add, Edit, Search area, gates, machine settings, Track
add and rename) follows these rules.

On save:
1. If the URL matches another Application's `url`, warn (naming an open match before a closed
   one, with `+N more` when several share it): `o` discards this form and opens that one's
   Detail, `s` saves anyway, `esc` goes back to the form with every value kept.
2. Create `jobs/<track>/<slug>/` (SCHEMA → Folder name), write `meta.yml` (`schema: 1`, the
   fields, `status_log` with one `Lead` entry), and an empty `jd.md`.
3. Open `jd.md` in `$EDITOR` so the description can be pasted. On return, select the new row.
   If the save fails, the form stays open with its values and the error under the field.

### Edit form (`e` in a table)

The same fields as Add, in the same order, plus **Resume only** (yes/no). Saving patches only the keys that
changed. Changing Track does **not** move the folder.

### Detail view (`v` / `Enter`)

Header: `Title · Company` (as the list reads) with `applied <date> · <ago>` or `closes in
<n>d` on the right, then `Status · Track · closes <date>`. The applied date shows once.
Tabs (`←` `→`): **JD** · **Fit** · **Notes** · **Docs** (Fit beside JD: they are read together).
In the wide size class the preview's card (status, documents, timeline, link) stays on the
left. Below it the card folds into a two-line strip under the title: the timeline as a track
(`○ Lead 24 Sep ─ ● Applied 25 Sep ─ ◌ Interviewing ─ ◌ Offer`), then documents · Fit ·
close date · Track, each shedding detail to fit rather than wrapping.

| Tab | Shows | `e` |
|---|---|---|
| JD | `jd.md` rendered with Glamour, scrollable | `$EDITOR` on `jd.md` |
| Notes | `notes.md` rendered | `$EDITOR` on `notes.md` |
| Fit | the `fit` value + `fit.md` rendered | `$EDITOR` on `fit.md` |
| Docs | list: `resume.md`, `cover-letter.md`, `gaps.md` (`✓` ready or `○` missing), each PDF in `out/`, and `sent/` if it exists | `Enter` on a PDF opens it in the system viewer. On a `.md`, opens `$EDITOR` |

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
selected row. Keep shared colour roles in `internal/ui/theme/` and UI-specific styles in
`internal/ui/styles.go` as neutral
styles, so v0.1.1 changes look by editing that file alone.

### v0.1.1 look (built)

Settled with a throwaway prototype (branch `prototype/v011-styling`) and two rounds of
context-free review. Shared colours and terminal appearance live in `internal/ui/theme/`; UI-specific styles live in `internal/ui/styles.go`.

- **Theme:** colours are named by role (text, muted, faint, rule, accent, ok, info, warn,
  danger, one per Status), each with a dark and a light value. The theme follows the
  terminal's background (asked before start-up; `COLORFGBG` as the fallback), and
  `ROLECALL_THEME=auto|dark|light` overrides it. The view paints the theme's background
  (`#0D1117` or white). Accent **Mint** (`#5EEAD4`, `#0F766E` on light) means focus and
  selection only; key names are bold text. Colour always pairs with a word or glyph.
- **Active markers:** the active track, view tab and detail tab are bracketed — `[All]`,
  `[Leads 4]`, `[JD]` — as well as coloured, so they read without colour.
- **Accessibility:** `NO_COLOR` drops all colour but keeps bold, underline and every word
  and glyph. `ROLECALL_ASCII=1` maps every drawn glyph to an ASCII stand-in; a locale that
  is not UTF-8, or `TERM=linux`, turns ASCII on by itself. Text keeps a 4.5:1 contrast
  floor on either theme's background, pop-up fill and selection bar, and rules and pane
  dividers 3:1, including the frame behind a pop-up, which dims without going below the
  floor. The `?` pop-up carries a legend, one meaning per glyph: Statuses `◌ ◐ ● ◆ ★ ✕ ↩
  ⊘`; Timeline, the current Status's glyph in its colour and earlier ones faint, `~`
  approximate; Docs `✓` ready, `○` missing; Flags `⚠ ⚑`; Fit values and `—` not assessed.
- **Status visual system:** each Status has a glyph and a colour on a cold-to-gold ramp —
  Lead `◌` grey-blue · Draft `◐` pale blue · Applied `●` blue · Interviewing `◆` purple ·
  Offer `★` gold — and the closed Statuses are muted, never red: Rejected `✕`, Withdrawn
  `↩`, Dismissed `⊘` (ASCII `o = * @ $ x < -`). The glyph leads list rows, group headers
  (label and rule in the Status colour), view tabs, the preview and detail chip, the
  timeline and the Move pop-up. A timeline shows each Status by its own glyph: the
  current one in its colour and bold, earlier ones faint. When two panes share the
  frame, the focused pane's part of the top rule is the accent.
- **Other colours:** Fit Strong green · Good blue · Marginal/Poor grey. Close dates by
  urgency: `closes in 6d` amber within 7 days, `closes in 2d ▲` red and bold within 2 (and
  today), `closed 3 Sep` struck through and muted once past — in rows, the preview and the
  detail view. `⚑ quiet` orange. Documents are named JD · Resume ·
  Cover letter · Gaps · PDF everywhere, each `✓ ready` or `○ missing`; a Draft row reads
  `✓ ready` once SCHEMA Ready, else how many are ready (`○ 2/4 ready`), and the preview and
  detail card add a meter, `■■□□ 2 of 4 ready` (GLOSSARY avoids "built").
- **Size classes (`internal/ui/layout.go`):** wide ≥ 110 columns, medium 80–109, narrow < 80; short
  ≤ 20 rows. Every screen and pop-up takes its sizes from one budget. Side panels (preview,
  detail card, Profile sections, Settings groups) are a share of the width between a
  minimum and a maximum. Content starts at column 3 (markers and the selection bar at
  column 1) and the last column stays clear.
- **Layout, top to bottom (one fixed frame):** tab bar (screens; at wide sizes a pulse,
  `10 live · 1 due · 1 closes ≤2d`) · a blank line (dropped when short) · context row (the
  screen's views; on the right the scope and the track filter) · top rule · body (with a
  preview pane when wide) · bottom rule · status row. The rules and the `┬`/`┴` joins are
  always drawn; the status row holds a toast, an error, the search input or the key line,
  one at a time (actions left, navigation pinned right).
- **Tab bar:** every tab has the same padding, active or not, so nothing shifts between
  screens; the active tab is filled and bracketed (`[1 Leads 4]`), so it reads without
  colour. As it narrows it drops other screens' counts, the word "rolecall" (kept down to
  80 columns), then other screens' names (their digits stay); the active screen keeps its
  name and count.
- **Track filter:** on the context row of the screens it changes (Leads, Draft, Applied,
  Stats), read once by its key: `t  [All]  SWE  Platform`, or `t [All]▸` when narrow.
- **Scope line:** whenever a track or text filter is on, the context row says what it
  hides — `/pl ×   3 of 4` — and the tab and view counts read `3/4` rather than a filtered
  number that looks like a total.
- **Feedback:** the message on the left, cut first; its keys on the right, never cut. Every
  move, dismiss, save and Settings change says what happened (`✓ Moved to Applied ·
  Software Engineer   u undo`), and a moved row is lit for 600 ms so the eye can follow it.
  A toast (successes clear on the next key) or an error (stays until `Esc`)
  shows on the status row; it never replaces the bottom rule. Paths read relative to the
  Workspace. A move to Offer celebrates once, in gold: `★ OFFER · Orbit Labs ★  12
  applied · 41 days · first offer`; a rejection stays quiet.
- **Agent writing:** when the file watcher sees a document's file written (`jd.md`,
  `resume.md`, `cover-letter.md`, a PDF in `out/`), its chip reads `⠹ Resume writing…`
  for 4 s after the last write, beside the `1 of 4 ready` meter. That is all the TUI can
  honestly see of a Harness working in another terminal; a Handoff run hands the terminal
  over, so nothing is drawn during it.
- **First run:** a Workspace with no Applications shows a centred welcome instead of
  empty panes: `▞ rolecall · no Applications yet`, then `a` add your first Lead, a short
  "Set up" checklist of the steps not yet done (`e` pick an Editor and `h` connect a
  Harness, each opening Settings › This machine on that row; `5` create your Profile with
  the Harness's create-profile skill), then `?` see every key. Search and sort keys are not
  offered on it. The wordmark's `▞` (ASCII `#`) is no Status's glyph.
- **Search:** while a text filter is on, a chip (`/pl ×`) shows on every screen, beside the detail title too;
  `Esc` clears it. While typing, the search input is on the status row; after `Enter` the scope
  line shows the chip and the count.
- **Rows:** two lines (title + right-hand columns, then company · track · dates). A blank
  line between rows when they all fit; dense when they don't. Right-hand columns are
  state · flag · Fit, Fit flush right and labelled (`fit Good`, `fit —`); a column empty
  for the whole view is hidden. No close date is a muted `—`. Dates older than about three
  months read as the month (`applied Jan 2025`), not `21mo ago`. Expired Leads and Drafts
  sort last under an **Expired** section. Titles that would not read as a name (a bare
  slug, an empty title) are quoted in toasts and the detail breadcrumb.
- **Next step:** the preview, the detail card (or, without it, the line under the detail
  tabs) say what to do next and the key that does it, worked out from Status and
  documents: `NEXT  add the JD   enter open`, `NEXT  follow up · quiet 9mo   n note`,
  `NEXT  decide on the offer   m move`. A key is offered only where it works here (`f` and
  `p` with a Harness, `b` with a Plugin).
- **Timeline:** one track of Status glyphs everywhere — the preview, the detail strip, and
  the detail card, where it flows onto more lines. An empty log reads `Not moved yet · m
  to move`. Empty tabs say what fills them (`No JD saved · e to open jd.md`, `No fit
  assessment yet`); a Fit without reasoning saved says so (`Good · no reasoning saved`).
- **Pop-ups:** one template — title in the top border, keys in the bottom border, solid
  fill, amber border and ⚠ for warnings. A pop-up is at least as wide as its key hints, so
  no key is cut, and is centred on the full width between the rules: it never covers the
  tab bar, context row or status row. The rows it covers are blank either side; the rest
  of the screen is dimmed. A pop-up taller than the body scrolls inside its border, keeping
  the selection in view. `?` is a pop-up; any key closes it, `Esc` included, and it pages
  only when it has a second page. The Move
  pop-up always moves the Application it shows, even if a live refresh moved the cursor.
- **Overflow:** one cue everywhere — `↓ 3 more` / `↑ 3 more` at the text column — on lists,
  Stats, Profile, Settings, readers and pop-ups. Stats taller than the body scrolls with
  `j k ↑↓`, `g G` and the wheel. The `?` legend starts a page of its own rather than split. A section heading never sits alone above
  it. Text wraps only at spaces.
- **Narrow terminals:** the tab bar sheds detail, the preview and detail card fold away
  below the wide class, pop-ups shrink to fit. The key line drops actions from the right but
  keeps `enter`, `m move`, `a add` and `/ search` to the last, and `? help` and `esc`
  always; navigation hints (`1-6 screen`, `←→ view`) go first. The detail view keeps `←→
  tab` and `↑↓` over its actions. Below 56×14 a centred notice — `rolecall
  needs 56×14 · now 48×12 · resize to continue` — replaces the layout.

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

Stats reads each well-formed Application's `status_log` in order (`approx` entries
included; malformed ones are left out of every figure and of Stats' `t` counts). A move and
the undo that restores the Status before it within 15 minutes cancel out. A later Status
implies the ones it cannot happen without: Interviewing, Offer, Rejected and Withdrawn
imply Applied, and Offer implies Interviewing, so the numbers agree with the screens. An
empty log is Lead.

1. **Funnel**: Applications that **ever reached** each Status: Lead → Draft → Applied →
   Interviewing → Offer, one bar per Status in its glyph and colour. Lead and Draft are
   optional starts, so a rung can be higher than the one before and there is no drop-off:
   each rung says what share of the previous rung reached it first (`29% of Applied`, left
   out when that rung is empty) and how many reached it without the previous one before
   it (`5 weren't Drafts first`). Rejected, Withdrawn and Dismissed on a muted `closed` line.
2. **Response rate** (GLOSSARY → Response): reached Interviewing or Offer ÷ ever Applied,
   leaving out Applications whose Applied date (an implied one takes the date of the Status
   implying it) is within `follow_up_days`, with a meter. A line says how many were left
   out as too new.
3. **Applied per week**: last 12 weeks, Monday-start, local time, by logged Applied date. One
   sparkline with the week range, this week's count and the 12-week average. The tab bar's
   sparkline beside Stats is the same series over 8 weeks.
4. **By track**: track × Applied (as the funnel counts it) · Counted (the Response
   denominator) · Responses · Rate, then a line saying what Counted means when there is
   room. Hidden when the filter is on one track.

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
| 7 | `init` and first run | `rolecall init [DIR]` asks nothing: it creates the Workspace (default `./rolecall`) and records it in `config.toml`. The app opens on a Contact form (Name and Email required) while either is missing; Esc skips it until the next launch. Tracks and Search area are set in Settings; the Profile comes from the Harness `create-profile` skill. Replaces the Huh wizard |
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
  lists, items for comma lists, none for prose. Contact shows ✓, or ✗ with the reason in the
  content pane when Name or Email is missing (builds would fail).
- **Keys:** `j`/`k` section (scroll once `tab` moves focus to the content; the key line says
  which), `g`/`G` first and last section (top and bottom of the content), `tab` focus, `e`
  opens the current file in `$EDITOR` (as on the Resume tab). `e edit file` shows once, on the
  section row, not on each Contact field.
- **Edge cases:** no `master-profile.md` shows `No Profile yet: profile/master-profile.md not
  found` and `e` does nothing; a file with no `##` headings is one section, "Profile"; below
  80 columns the section list hides and the header names the section.
- **Refresh:** re-read on entering the screen, after `$EDITOR` closes, and on `r`. `profile/` is
  not watched.
- The Contact table is a file contract: see `SCHEMA.md`.

#### Settings screen (v1.0)

Laid out like Profile. Parent spec: #58; this section covers the read-only first slice (#66).

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
    which it renames like any other), and follow-up days (`config.toml`).
  - This machine (`config.toml`): workspace path (shown, never editable), editor, downloads,
    harness, plugin.
- **Keys:** `←`/`→` switch the group, as they switch views elsewhere (`tab` and `[` `]` too);
  `j`/`k` move the row, `g`/`G` go to the first and last; `k` above the first row rests on the
  tabs. The cursor skips read-only rows (Workspace, Sources, Blockers), which say what they are
  and why they are fixed. Tracking lists each Track with its Application count and a `+ add a Track`
  row; `a` add, `r` rename, `x` remove and `J`/`K` reorder act on the selected Track. `e`
  opens `workspace.json` in the Editor, or `config.toml` while This machine is selected; `r`
  rescans. One save model in every group: Enter saves at once (a pop-up's last field, an
  inline field, a Yes/No toggle), says what and where (`Saved Editor: nano · config.toml`), and
  `u` puts the last Settings change back (not a Track rename). There is no staged save. An
  inline field over a set value opens with it selected, so typing replaces it; over an unset
  value it opens empty with the default in effect as its placeholder. Emptying a text setting
  clears it, so the default applies again; emptying Places clears the Search area (every
  location kept). `ctrl+c` over typed text in a field or Settings pop-up asks `Discard changes?
  y/n` first; a second `ctrl+c` quits.
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
  the Editor, and re-reads it when the Editor exits. On a TOML error the old values stay and
  the feedback line shows the error. The Workspace is fixed for the session.
- **Editor examples:** the Editor's about text names `nvim`, `hx`, `code --wait`, `zed --wait`,
  `subl --wait`. GUI Editors need their wait flag, or rolecall rescans before editing finishes.
- An explicit `follow_up_days = 0` is honoured; only an absent key means the default 14.
- **Invalid `workspace.json`:** the screen shows its path and the parse error, with `e` as the
  only action. The program still starts, with no Track filter. Nothing is written.
- **Refresh:** both files are re-read on entering the screen, on `r`, and after `$EDITOR` closes.
- **Later slices:** editing Tracks (add, rename, remove, reorder), the Search area form, gates
  and machine settings as editable rows. `workspace.json` is then written by the TUI, a
  `SCHEMA.md` ownership change that lands with the first writing slice.

## Open questions

- **Bubble Tea v2:** confirm it's stable at setup. If not, use v1 and plan the upgrade.
- **Leads from `find-jobs` with no JD text:** keep stub `jd.md` files, or have the skill fetch and fill them? Default is stub plus URL. Revisit after a week of use.
