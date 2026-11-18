# How the rolecall app works

Reference for the `help` skill. For the exact keys on one screen, the app's `?` help is
authoritative; for the Owner's own paths, `rolecall doctor` is.

## Setup

```sh
rolecall init [DIR]   # creates the Workspace (default ./rolecall) and its local git repo; asks nothing
rolecall              # opens the app; the first time, it asks for the Contact (Name, Email required)
rolecall doctor       # checks the Workspace, typst, pandoc, Python, fonts, Plugin and Harness
rolecall move FOLDER STATUS   # moves an Application to a Status as the app does; Applied also saves sent/
rolecall now [--days N] [--list]   # one plain line: Follow-up due items, next steps and closing dates within N days (7)
rolecall export [--format csv|json | --ics] [--out FILE]   # every Application as CSV or JSON, or next steps and close dates as .ics
```

`now` and `export` only read the Workspace and take `--dir PATH`; `export` writes only where `--out` says.
`e` on an Application also takes a **next step** (a date and what it is) and its first
**contact**; moving one to Rejected or Withdrawn asks why it ended (optional), which Stats
shows as where Applications dropped off.

Esc skips the Contact form; it returns on the next launch until Name and Email are saved.
Tracks, the Search area, Search rules and Sources are set in Settings. The Profile comes from the `create-profile` skill.

## Workspace layout

```text
<workspace>/
├─ workspace.json                 # Tracks, Search area, Search rules, Sources
├─ profile/master-profile.md      # the Profile; its ## Contact table is on every document
├─ cv/                            # CVs (general.md …) and the resume create-profile imported
│  └─ out/<name>/*.pdf            # built CVs; b builds and d downloads them on the Profile screen
└─ jobs/<track>/<folder>/         # one folder per Application
   ├─ meta.yml                    # title, company, track, fit, posted, status_log
   ├─ jd.md · resume.md · cover-letter.md · notes.md
   └─ out/*.pdf                   # built PDFs
```

Every Application, a Lead included, is its own folder: the app lists the folders and reads
nothing else. `jobs/SHORTLIST.md`, `TRACKER.md`, `DISMISSED.md` and `INBOX.md` (in `jobs/` or the
root) are from an older rolecall and are never read; the app's footer and `rolecall doctor`
warn about them, and `${CLAUDE_PLUGIN_ROOT}/scripts/migrate.py` converts them (see `<plugin>/README.md`).
`jobs/sweep-candidates.md` is the current Sweep's raw output, triaged into Leads by find-jobs.
`jobs/_archive/` holds retired files, locked and never read: editing them changes nothing.

## Statuses

Lead → Draft → Applied → Interviewing → Offer, or Rejected, Withdrawn, Dismissed. The Owner moves
an Application with `m`; the only automatic move is Lead → Draft when its PDFs are built.
Dismissed keeps the Application, restorable, and future Sweeps skip it. A Lead a Search rule
removed (too old, over a Source's applicant cap, a blocker in the ad) is Dismissed with its
reason and listed under Leads → **Filtered**; restore it with `m` and no rule removes it again.
A Lead with a posting date shows its age, and warns as it nears the age limit.

**Deadlines** (Drafts screen, `s`) lists every Lead and Draft with a closing date, or a Lead
nearing the posting age limit, soonest first, then the passed ones, then those whose date was unclear in the ad. A Lead or Draft shows its
pay as an annual AUD estimate (flagged when a figure looks mislabelled, such as $900 an hour),
its contract term against the visa cap in Settings → Rules, whether it needs sponsorship, and
"ad summary only" when `jd.md` is not the full ad. An agency ad reads "AGL via Chandler Macleod".
Add warns before saving a likely duplicate. **Insights**, at the end of Stats, counts the gaps
listed across every `fit.md` and how many Applications each would unlock. **Follow-up due** is a
reminder (`⚑ 2w` on the row) on an Applied or Interviewing Application that has gone
`follow_up_days` (default 14) without a Status change and without you recording that you
followed up; it is not a Status. `F` hands it to draft-follow-up, which writes `follow-up.md`
and never sends. Once you have sent it, `l` records that you followed up today
(`followed_up: YYYY-MM-DD` in `meta.yml`): the `⚑` clears and comes back `follow_up_days` after
that day, and `u` takes it back. A Status change restarts the count too.

**Documents** are the Resume and the Cover letter (no Cover letter when the Application is
Resume only), each written (`resume.md` has text) then built (its PDF in `out/`). The DOCS
meter and a Draft row's `○ 2/4 ready` count those steps; Ready means every needed PDF is built.
The JD and Gaps are inputs on their own tabs and are not counted.

## Screens and keys

| Key | Does |
| --- | --- |
| `1`–`6` | Leads (and Dismissed, Filtered), Drafts (and Deadlines: `s` on Drafts), Pipeline (Applied, Interviewing, Offer, Closed: Rejected and Withdrawn), Stats, Profile, Settings |
| `←`/`→` or `s` · `j`/`k` | change view within a screen (`Tab` is an unlisted alias of `→`, except on Profile where it switches pane) · move between rows |
| `Enter` / `v` | open the Application's details |
| `a` · `e` · `m` · `x` | add · edit · move to another Status · dismiss |
| `*` | mark or unmark the row; with rows marked, `x` and `m` act on all of them (`Esc` clears) |
| `n` | notes: the Application's `notes.md` |
| `f` · `p` | Handoff to the Harness: assess-fit (any row) · prepare-documents (Lead or Draft) |
| `F` · `l` | on a Follow-up due row, Handoff to draft-follow-up · record that you followed up today (Applied, Interviewing) |
| `r` | re-read the Workspace |
| `b` · `d` | build PDFs · copy the PDFs to the `downloads` folder |
| `w` · `c` | open the job ad's link · copy it |
| `/` · `t`/`T` · `s` · `o` | search · next / previous Track filter · change view · sort Leads |
| `z` · `h` · `J`/`K` | compact rows (wide sizes) · connect a Harness (while none is set) · next / previous row's detail |
| `u` · `space` · `,` · `?` · `q` | undo · command palette (`:` and `Ctrl-K` open it too) · Settings · help · quit |

A Handoff runs the `harness` command from `config.toml` with the skill and the Application's
folder, started in the Workspace wherever rolecall was launched; the app steps aside and re-reads the folder when the Harness exits. Without `harness`
set, `?` and the palette list the Handoff keys dimmed as `(needs a Harness)`, pressing one says to
set it, and `h` opens Settings › This machine on the Harness row.

## Settings

Machine settings live in `~/.config/rolecall/config.toml`: `workspace`, `editor`,
`follow_up_days`, `downloads`, `plugin`, `harness`, `ascii`, `inherit_terminal`, `compact_rows`
(list rows on a single line at wide sizes; `z` toggles it). Screen `6`
shows each value and where it came from; select This machine and press `e` to edit the file.
Workspace settings (Tracks, Search area, Search rules, Sources) live in `workspace.json` and
are edited on the same screen. **Rules** sets the age limit for postings, the applicant cap per
Source, how borderline labels such as "1mo ago" are decided and the preferred contract length.
**Sources** lists every place the Sweep or the Harness searches; add one, edit one (`Enter`),
check it (`c`: its access status, works, 403, login wall or cloudflare) or turn it off (`o`) there. A changed rule removes nothing
by itself: the find-jobs skill runs a dry run (`rules.py check`) and asks before dismissing.

## When something does not show

- **A new Lead is missing.** Check the Track filter (`t` → All) and the view (`s`).
  Then compare the folder the skill printed with the Workspace in `rolecall doctor`: Plugin
  scripts use `--data` or `ROLECALL_WORKSPACE`, not `config.toml`, so a mismatch writes
  elsewhere. A footer warning about `SHORTLIST.md` means the Harness has old instructions.
- **The app changes live** when files under `jobs/` change; `workspace.json` and `profile/`
  are re-read when their screen opens.
- **A row reads "Can't read …"**: its `meta.yml` is malformed; the row says the line and fix.
- **Builds fail on Contact**: the Profile is missing Name or Email (screen `5` warns).
