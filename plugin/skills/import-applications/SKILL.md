---
name: import-applications
description: Import the Owner's existing applications from a CSV or TSV export (a job-tracker or spreadsheet) into Application folders. Use when the Owner asks to import, bring over, load or migrate a list of jobs they already applied to, or hands over a spreadsheet or CSV of applications. Maps the columns and statuses with the Owner first, dry-runs, then creates folders through the shared writer with approximate dates. Never invents a fact.
---

# Import applications

> **Plugin paths.** `${CLAUDE_PLUGIN_ROOT}` is this plugin's folder (the one holding `skills/`
> and `scripts/`). Claude Code fills it in; in another harness, or if it reaches the shell
> unexpanded, use the absolute path of the folder two levels above this file. Never run a
> script by a path relative to the Workspace, and never hand-write `meta.yml` because a
> script was not found: stop and say which path failed.

Bring an Owner's existing application list into the Workspace so it shows in Rolecall. The work
is done by `import_csv.py`, which writes each row through the shared `meta.create`. Your job is
the conversation around it: confirm the column mapping, ask about each status the script does not
know, show the dry run, and only then import. The file contract is `SCHEMA.md`.

**Nothing is invented.** A row becomes what the export says and no more: no `fit`, no `closes`,
no ad text, no dates the export does not give. A date the script cannot read is left out, not
guessed. A row without a title or a company is skipped and reported.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it before
running anything, and do not guess a path.

## 1. Get a CSV

The script reads CSV and TSV (comma, tab or semicolon separated; UTF-8). For a `.xlsx`, Numbers or
Google Sheets file, ask the Owner to export it as CSV first. The export is the Owner's own data:
read it where it is, and never copy it into the code repository.

## 2. Inspect, and confirm the mapping

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/import_csv.py" --data "$ROLECALL_WORKSPACE" \
  --file ~/Downloads/applications.csv --inspect
```

It prints the columns it matched to **title, company, status, date applied, URL, notes** (and
`track` when there is a column of that name), the columns it will not import, and every distinct
status value with the Status it maps to. A column is matched only by its header name, never by
what is in it.

Show this to the Owner and **ask them to confirm it**. Fix a wrong or missing column with
`--map field=Column` (`--map notes=` leaves a field out). The two the import cannot do without are
title and company.

## 3. Ask about unknown statuses

The Statuses are fixed (`Lead`, `Draft`, `Applied`, `Interviewing`, `Offer`, `Rejected`,
`Withdrawn`, `Dismissed`). For each status value marked `?`, **ask the Owner which one it means**:
"Ghosted" might be Rejected or Withdrawn, and only they know. Record each answer with
`--status-map "ghosted=Rejected"`. Do not choose for them. The import refuses to write while any
value is unmapped.

A row with no status at all is **Applied** when the export gives a date applied, and a **Lead**
when it does not: the date is the only evidence either way. Say so to the Owner.

Also ask which **Track** the applications belong to (`--track`), from the Workspace's tracks.
Different tracks in one file: use a `track` column, matched to the Workspace's tracks.

A date like 03/04/2026 reads day first, as in Australia. If the Owner's export is month first,
pass `--date-order mdy`.

## 4. Dry run

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/import_csv.py" --data "$ROLECALL_WORKSPACE" \
  --file ~/Downloads/applications.csv --track SWE --status-map "ghosted=Rejected" --dry-run
```

It lists every row as `create` or `skip` with the reason, and writes nothing. Read it with the
Owner. A skipped row is one of:

- a **duplicate**: the same URL, or the same employer and title, as a saved Application or as an
  earlier row. This is the existing duplicate rule. Leave it out unless the Owner says it is a
  different role, then pass `--allow-duplicate`;
- missing a title or company; or an unknown Track.

A `~` date is approximate: the export gave only a day. Each created Application gets a
`status_log` of date-only entries marked `approx: true`, the way the migration writes dates it
rebuilt, so the app shows them with a `~`. A sent row (Applied or later) with a date applied
starts at Applied on that day. Everything else, and a sent row with no date, is dated the import
day. Say so; the Owner can correct a date in the app.

## 5. Import

Run the same command without `--dry-run`. It creates `jobs/<track>/<slug>/` for each row through
`meta.create`, with an empty `jd.md`, the URL when it is a real web address, and the row's notes
copied verbatim into `notes.md`. Report how many were created and what was skipped and why.

Imported Applications have no ad text, so they have no Fit and no documents. Do not run
`assess-fit` or `prepare-documents` on a role the Owner already applied to unless they ask.

## 6. Writer rules

- Folders and `meta.yml` are written by `import_csv.py` only; never by hand.
- Append no Status of your own and do not run `rolecall move`. The `status_log` the script writes
  is the Owner's history.
- Never touch the Profile. Never write the export's contents anywhere but the Workspace.
- A second run on the same file creates nothing new: every row is then a duplicate.
