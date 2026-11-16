---
name: find-jobs
description: Sweep the Owner's configured Sources for roles worth applying for and save each one as a Lead Application. Use when the Owner asks to find, search for or look for jobs, wants a fresh sweep, or asks what is out there. Runs the plugin sweep script over the Workspace tracks and Sources, reads the ads, judges each against the Profile, and creates a Lead folder only for roles genuinely worth a look.
---

# Find jobs

> **Plugin paths.** `${CLAUDE_PLUGIN_ROOT}` is this plugin's folder (the one holding `skills/`
> and `scripts/`). Claude Code fills it in; in another harness, or if it reaches the shell
> unexpanded, use the absolute path of the folder two levels above this file. Never run a
> script by a path relative to the Workspace, and never hand-write `meta.yml` because a
> script was not found: stop and say which path failed.

Find roles worth applying for and save each as a **Lead** Application.

**The script collects; you judge.** `${CLAUDE_PLUGIN_ROOT}/scripts/sweep.py` fetches, filters and dedupes. It
does not decide anything. Deciding what is worth the Owner's time needs the full ad read against
the Profile, and that is this skill's whole job.

**Read the Workspace; assume nothing.** Everything this skill needs is config or Profile:

- `workspace.json` — `tracks` (which tracks a Sweep runs) and `sources` (which company boards
  to sweep, as `platform: target` entries).
- `jobs/roles.json`, when present — the master role families, title aliases, track mappings,
  priorities, evidence, gaps, additional boards and manual-search sources.
- `profile/master-profile.md` and `profile/projects.md` — the facts every judgement is measured
  against.
- The Owner's preferences in `profile/writing-preferences.md` and the general rules in
  the Workspace's writing rules.

Never hardcode a track, a Source or a Search rule. If a Workspace value is missing, say so
rather than inventing a default.

## 1. Read the config

Read `jobs/roles.json` before searching or judging. Use its `titles` for manual searches,
`search_terms` for mechanical title matching, and `priority` to order effort: primary,
secondary, then stretch. Its evidence and gaps guide assessment; the Profile remains the
fact authority. Edit the JSON catalogue to change search titles, priorities or sources.

Read `workspace.json` when present. The script uses its tracks, or its built-in title keywords
when absent; say when defaults are being used. `sources`, when present, wholly replaces
the source plan. Otherwise the script combines built-in targets with the catalogue's
`additional_sources`. In its list form, `sources` is the **Source registry**: every place the
Owner searches, each with a `type` (`api` the script fetches; `browser` or `manual` you search),
an `access` status, parsing hints (`age_format`, `applicants_format`) and its own `rules`.

Read `search_rules` too: the posting-age limit, the applicant caps per Source, the
labels the Owner has already ruled on (`ambiguous_age_labels`) and the longest contract
the Owner prefers. These are the rules. Do not look for them in `writing-preferences.md`, and do
not apply a rule that is only written in prose there: ask the Owner to add it to
`search_rules` (Settings → Rules) instead. Print the effective rules with:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/rules.py" show
```

Inspect the selected Workspace's catalogue with:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/sweep.py" --roles
```

`null` means the catalogue is absent; a malformed catalogue must be fixed before sweeping.

Run `python3 "${CLAUDE_PLUGIN_ROOT}/scripts/sweep.py" --platforms` to see which Platforms the script knows and
the targets each one defaults to. Platforms are code; which of them a Workspace sweeps is
config.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Use the
environment variable in this skill so the commands stay short.

## 2. Sweep

The script defaults to all Workspace tracks. Pass tracks explicitly when narrowing scope:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/sweep.py" --track <track> --track <track>   # one --track per configured track
```

The script filters by the Workspace's `search_area` (`places` and `remote` in `workspace.json`,
see SCHEMA). Without one it keeps every location and says `no Search area set: all locations
kept`; tell the Owner, as they may want to set one. `--all-locations` bypasses the filter; use
it only when the Owner asks, or when a track is producing nothing and the filter is the reason.

Writes `jobs/sweep-candidates.md`. It has already dropped any role whose URL belongs to an
existing Application, whatever its Status — Dismissed included — so everything listed is new.
Its **Source health** table is part of the result: a **FAILED** source means those roles are
missing from this Sweep, not absent from the market. Say so rather than reporting a quiet run.
A failure names its access problem (`403`, `login wall`, `cloudflare`) when the script can
tell. **Filtered by Search rules** counts what each rule removed; a role marked
**borderline age** is yours to raise with the Owner, never to drop on your own judgement.

## 3. Fill the gaps the script cannot reach

Use the catalogue's `titles` to search beyond configured boards, including every Source the
report lists under **Not reachable by this script** (the registry's enabled `browser` and
`manual` entries, then the catalogue's older `manual_search_sources`). Use each Source's
`age_format` and `applicants_format` to read the age and the applicant count, and apply its
rules: skip a role over that Source's applicant cap, or at or past the age limit. Check a
label with `python3 "${CLAUDE_PLUGIN_ROOT}/scripts/rules.py" label "1mo ago" --source jora`:
`discard` means leave it out, `borderline` means save it only if it is otherwise strong and
tell the Owner. For the manual searches (Seek, LinkedIn), read `search_area` from
`workspace.json` and search each of its `places` (add remote roles only when `remote` is not
`no`); without a Search area, ask the Owner where to search rather than assuming. Cover at least one title per role family in the requested tracks,
in priority order; vary aliases when results are sparse. For stretch families, assess the
actual duties and seniority before recommending. Record families searched with no suitable
results separately from families not searched or blocked by access.

Some large boards block automated fetching. Use WebSearch to surface these as leads, then
prefer employer career pages for the full ad. A zero-result board Sweep does not establish
that a role family has no vacancies.

Anything found this way arrived without a readable ad — say so when you save it.

## 4. Judge each candidate

Read the ad text in `sweep-candidates.md` — or fetch it where the entry says **(no ad text —
fetch it)** — against `profile/master-profile.md` and `profile/projects.md`.

Apply the Fit rubric in the Workspace's writing rules and the Owner's preferences. In short:

- Exclude anything the Owner cannot take: work rights, residency or sponsorship the ad rules
  out. The script pre-flags these in its *Blocked* section from the ad text — spot-check one
  before trusting it, because a page can mention them in boilerplate.
- Weigh **must-haves met**, counting only what the ad frames as essential.
- Judge against the Owner's own stated preferences, not a generic idea of seniority.

**Save a Lead only for a role you would actually recommend.** If the reason reads like faint
praise, leave it out of the Workspace entirely and say why in the hand-back. A padded set of
Leads wastes a day per entry.

## 5. Save each Lead

For every role worth a look, create one Application folder with the same rules as
capture-job. Do not hand-write `meta.yml`.

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/meta.py" create \
  --track SWE \
  --title "Senior Backend Engineer" \
  --company "Acme" \
  --url "https://boards.example.com/acme/senior-backend" \
  --closes 2027-01-30
```

- `--track` is the Workspace track the role belongs to.
- `--url` is the ad URL exactly as found. Never type one you have not seen.
- `--closes` only when the ad states a date in `YYYY-MM-DD` form. Omit it otherwise.
- `--posted YYYY-MM-DD` when the ad or the script gives a posting date, and
  `--posted-label` with the age exactly as the site shows it (`"14d ago"`,
  `"Posted 30+ Days Ago"`). Give the label even when you work the date out from it.
- `--applicants` with the count the site shows, and `--source` with the registry `name`
  you found it on (`linkedin`, `greenhouse`).
- The command **refuses** a Lead a Search rule rules out and names the rule: leave that role
  out and list it in the hand-back. Pass `--override-rules` only when the Owner asked for that
  role anyway. A borderline age prints a warning and saves; mention it in the hand-back.
- If the script cannot be found, stop and say which path failed. Never write `meta.yml` by
  hand instead.
- The command prints the new folder and writes `meta.yml` with `schema: 1`, those keys, and
  exactly one first `Lead` entry.

Then fill `jd.md` in that folder with the ad as plain Markdown, **no front matter**. Strip
navigation and page furniture; keep the duties, requirements, location and closing date.

When there is no readable ad text — a search-snippet Lead, or a board that would not render —
write `jd.md` as a short stub so the ad can still be opened:

```markdown
# Senior Backend Engineer — Acme

Ad: https://boards.example.com/acme/senior-backend

Saved as a Lead from a search snippet. The ad itself was not read: must-haves and work rights
are unconfirmed until it is opened.
```

The URL and the triage reason are enough. Do not invent ad text.

### Writer rules

- Never append any other Status from this skill except through the rules check below. The
  only other entry it writes is the new folder's first `Lead`.
- Never move a folder to discard it (no `_discarded/`, no `_archive/`), and never edit a file in
  `jobs/_archive/`: it is a read-only record the app does not read.

### Apply the rules to saved Leads

When the Owner adds or changes a Search rule, or asks to clear out old Leads, run the dry
run first and show the Owner its output:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/rules.py" check
```

It lists the Leads a rule would dismiss, the borderline ones it keeps, and the Drafts a rule
would remove, separately. Nothing is written. Only after the Owner confirms, run
`rules.py check --apply`. Add `--include-drafts` only when the Owner said yes to the Drafts as
well: they may already have documents. Each Application is dismissed in place with a
`filter_reason` and appears under Leads → Filtered, where the Owner can restore it.
- Never touch `notes.md`, `sent/` or the Profile fact table (`profile/projects.md`).
- Never rewrite `meta.yml` by hand; set keys through the shared writer so unknown keys and
  comments survive byte for byte.
- Never create a duplicate. If the Sweep kept a URL that is already an Application, report the
  existing folder instead of making a second one.
- Find jobs never decides the final Fit verdict. Save the Lead; assessment comes next.

## 6. Hand back

Report how many roles you saved as Leads, the top pick and why, and anything notable about the
market — a track producing nothing usable is a real finding, not a failure. Say plainly what
you could not reach, and name any **FAILED** source. Give a coverage report:

- every Source searched (by the script or by hand) and when;
- how many roles each Search rule filtered out (from the report, plus your manual searches);
- every Source that failed or could not be searched, and why (403, login wall, Cloudflare).

List roles you screened out with one line each so the Owner can check whether the filtering
was too aggressive. If a Source's access status in the registry is out of date, say so and
suggest the change (Settings → Sources).

The Owner picks which Lead to pursue; then the next steps run in order:

```sh
# capture any missing ad text, then
# assess-fit writes fit.md, then prepare-documents writes resume.md and cover-letter.md
```

Never start building an application from this skill. The note you wrote when saving the Lead is
triage; `fit.md` is the verdict, and it can still come back weak once the ad is read properly.
