# How the rolecall app works

Reference for the `help` skill. For the exact keys on one screen, the app's `?` help is
authoritative; for the Owner's own paths, `rolecall doctor` is.

## Setup

```sh
rolecall init [DIR]   # creates the Workspace (default ./rolecall) and its local git repo; asks nothing
rolecall              # opens the app; the first time, it asks for the Contact (Name, Email required)
rolecall doctor       # checks the Workspace, typst, pandoc, Python, fonts, Plugin and Harness
```

Esc skips the Contact form; it returns on the next launch until Name and Email are saved.
Tracks, the Search area, Search rules and Sources are set in Settings. The Profile comes from the `create-profile` skill.

## Workspace layout

```text
<workspace>/
├─ workspace.json                 # Tracks, Search area, Search rules, Sources
├─ profile/master-profile.md      # the Profile; its ## Contact table is on every document
├─ cv/                            # the Owner's general resume, for create-profile
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
A Lead with a posting date shows its age, and warns as it nears the age limit. **Follow-up due** is a
reminder on a sent Application unchanged for `follow_up_days` (default 14), not a Status.

## Screens and keys

| Key | Does |
| --- | --- |
| `1`–`6` | Leads (and Dismissed, Filtered), Drafts, Applied (Interviewing, Offer, Closed), Stats, Profile, Settings |
| `Tab` · `j`/`k` | move between tables · rows |
| `Enter` / `v` | open the Application's details |
| `a` · `e` · `m` · `x` | add · edit · move to another Status · dismiss |
| `n` | the Application's notes |
| `f` · `p` | Handoff to the Harness: assess-fit (any row) · prepare-documents (Lead or Draft) |
| `r` | on a Follow-up due row, Handoff to draft-follow-up; elsewhere, re-read the Workspace |
| `b` · `d` | build PDFs · copy the PDFs to the `downloads` folder |
| `/` · `t` · `s` · `o` | search · Track filter · Status filter · sort |
| `u` · `space` · `,` · `?` · `q` | undo · menu · Settings · help · quit |

A Handoff runs the `harness` command from `config.toml` with the skill and the Application's
folder, started in the Workspace wherever rolecall was launched; the app steps aside and re-reads the folder when the Harness exits. Without `harness`
set, the Handoff keys leave the key line, and pressing one says to set it.

## Settings

Machine settings live in `~/.config/rolecall/config.toml`: `workspace`, `editor`,
`follow_up_days`, `downloads`, `plugin`, `harness`, `ascii`, `inherit_terminal`. Screen `6`
shows each value and where it came from; select This machine and press `e` to edit the file.
Workspace settings (Tracks, Search area, Search rules, Sources) live in `workspace.json` and
are edited on the same screen. **Rules** sets the age limit for postings, the applicant cap per
Source, how borderline labels such as "1mo ago" are decided and the preferred contract length.
**Sources** lists every place the Sweep or the Harness searches; add one, test it (its access
status: works, 403, login wall, cloudflare) or disable it there. A changed rule removes nothing
by itself: the find-jobs skill runs a dry run (`rules.py check`) and asks before dismissing.

## When something does not show

- **A new Lead is missing.** Check the Track filter (`t` → All) and the Status filter (`s`).
  Then compare the folder the skill printed with the Workspace in `rolecall doctor`: Plugin
  scripts use `--data` or `ROLECALL_WORKSPACE`, not `config.toml`, so a mismatch writes
  elsewhere. A footer warning about `SHORTLIST.md` means the Harness has old instructions.
- **The app changes live** when files under `jobs/` change; `workspace.json` and `profile/`
  are re-read when their screen opens.
- **A row reads "Can't read …"**: its `meta.yml` is malformed; the row says the line and fix.
- **Builds fail on Contact**: the Profile is missing Name or Email (screen `5` warns).
