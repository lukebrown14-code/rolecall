# History

How rolecall got to its current shape. Everything here has shipped; the current behaviour is in
[`spec.md`](spec.md), and the data contract in [`../SCHEMA.md`](../SCHEMA.md). These sections
were moved out of the spec so it describes what the app does now.

## Porting the dashboard-era files (v0.1)

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

### Script changes

| Script | Change |
|---|---|
| `build.py` | Stop writing `TRACKER.md` rows and adding empty legacy keys. Append `Draft` when the current Status is `Lead` and PDFs were built. Read `resume_only` as now. Keep the gates. Remove the `sent/` snapshot (the TUI owns it). Takes the workspace path from `--data` or `ROLECALL_WORKSPACE` |
| `new-job.py` | Writes `meta.yml` (SCHEMA) + front-matter-free `jd.md` instead of jd front matter + tracker row |
| `sweep.py` | Dedupe and skip by reading every `meta.yml` `url` (including Dismissed) instead of `SHORTLIST.md` / `DISMISSED.md` |
| `migrate.py` | new, one-time. See Migration |

### Tone layering

`prepare-documents` reads, in order, with later files winning: `plugin/writing-rules.md` →
`plugin/styles/au.md` → `<workspace>/profile/writing-preferences.md`. See `plugin/README.md`.

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

`migrate.py` stays in `plugin/scripts/` and is described in `plugin/README.md`.

## Acceptance lists (all met)

**v0.1 was done when the Owner could, with no harness running:**
1. Launch `rolecall` on the migrated workspace and see every current Application on the right screen and table.
2. Add a Lead, paste its JD, edit it, and see it sorted by close date.
3. Move it Lead → Draft → Applied (and see `sent/` created) → Interviewing → Rejected, then undo once.
4. Dismiss a Lead, find it in Dismissed, and restore it to its earlier Status.
5. Download a Ready Application's PDFs and see them in Finder.
6. Watch a row update live while an agent (in another terminal) writes `fit:` or builds PDFs.
7. Filter by track and by text.
8. Run `capture-job` and `prepare-documents` from the plugin and get output matching today's quality.

**v0.2 was done when the Owner could**

1. Press `f` on a Lead, talk to the agent, and see Fit appear on return.
2. Press `p` on a Draft, and on return see Docs go to Written or Ready.
3. Press `b` on a Written Lead and see `Built 2 PDFs · moved to Draft` without leaving the
   Leads screen. Then see a failed gate's message in a pop-up.
4. Paste a Greenhouse or Ashby URL into Add, see the fields fill, save, and find `jd.md` filled.
   Paste a Seek URL and get the paste-by-hand path.
5. Open Stats, check the funnel and response rate against the fixtures, and filter by track.
6. Do all of the v0.1 list above with `harness` and `plugin` unset.

**v0.3 acceptance**

1. A Draft with all three written documents shows JD · Fit · Resume · Letter · Gaps · Notes · Docs; Resume scrolls and `e` edits it.
2. A Lead without documents shows JD · Fit · Notes · Docs.
3. A resume written through `p` adds its tab on return without moving the current tab.
4. Enter on `resume.md` in Docs opens Resume; a missing document reads `missing`.
5. Resume only Applications show no Letter tab and show `not needed · Resume only` in the card and Docs.
6. Tabs fit at 80 columns and readers work with `harness` and `plugin` unset.
7. `scripts/check-go.sh` and `go test -race -count=3 ./internal/ui` pass.
