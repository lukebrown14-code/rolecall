---
name: find-jobs
description: Sweep the Owner's configured Sources for roles worth applying for and save each one as a Lead Application. Use when the Owner asks to find, search for or look for jobs, wants a fresh sweep, or asks what is out there. Runs plugin/scripts/sweep.py over the Workspace tracks and Sources, reads the ads, judges each against the Profile, and creates a Lead folder only for roles genuinely worth a look.
---

# Find jobs

Find roles worth applying for and save each as a **Lead** Application.

**The script collects; you judge.** `plugin/scripts/sweep.py` fetches, filters and dedupes. It
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

Never hardcode a track, a Source or a preference. If a Workspace value is missing, say so
rather than inventing a default.

## 1. Read the config

Read `jobs/roles.json` before searching or judging. Use its `titles` for manual searches,
`search_terms` for mechanical title matching, and `priority` to order effort: primary,
secondary, then stretch. Its evidence and gaps guide assessment; the Profile remains the
fact authority. Edit the JSON catalogue to change search titles, priorities or sources.

Read `workspace.json` when present. The script uses its tracks, or its built-in title keywords
when absent; say when defaults are being used. `sources`, when present, wholly replaces
the source plan. Otherwise the script combines built-in targets with the catalogue's
`additional_sources`. Inspect the selected Workspace's catalogue with:

```sh
python3 plugin/scripts/sweep.py --roles
```

`null` means the catalogue is absent; a malformed catalogue must be fixed before sweeping.

Run `python3 plugin/scripts/sweep.py --platforms` to see which Platforms the script knows and
the targets each one defaults to. Platforms are code; which of them a Workspace sweeps is
config.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Use the
environment variable in this skill so the commands stay short.

## 2. Sweep

The script defaults to all Workspace tracks. Pass tracks explicitly when narrowing scope:

```sh
python3 plugin/scripts/sweep.py --track <track> --track <track>   # one --track per configured track
```

`--all-locations` widens or drops the script's location filter; use it only when the Owner
asks, or when a track is producing nothing and the filter is the reason.

Writes `jobs/sweep-candidates.md`. It has already dropped any role whose URL belongs to an
existing Application, whatever its Status — Dismissed included — so everything listed is new.
Its **Source health** table is part of the result: a **FAILED** source means those roles are
missing from this Sweep, not absent from the market. Say so rather than reporting a quiet run.

## 3. Fill the gaps the script cannot reach

Use the catalogue's `titles` to search beyond configured boards, including
`manual_search_sources`. Cover at least one title per role family in the requested tracks,
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
python3 plugin/scripts/meta.py create \
  --track SWE \
  --title "Senior Backend Engineer" \
  --company "Acme" \
  --url "https://boards.example.com/acme/senior-backend" \
  --closes 2027-01-30
```

- `--track` is the Workspace track the role belongs to.
- `--url` is the ad URL exactly as found. Never type one you have not seen.
- `--closes` only when the ad states a date in `YYYY-MM-DD` form. Omit it otherwise.
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

- Never append any other Status from this skill. The only entry it writes is the new folder's
  first `Lead`.
- Never touch `notes.md`, `sent/` or the Profile fact table (`profile/projects.md`).
- Never rewrite `meta.yml` by hand; set keys through the shared writer so unknown keys and
  comments survive byte for byte.
- Never create a duplicate. If the Sweep kept a URL that is already an Application, report the
  existing folder instead of making a second one.
- Find jobs never decides the final Fit verdict. Save the Lead; assessment comes next.

## 6. Hand back

Report how many roles you saved as Leads, the top pick and why, and anything notable about the
market — a track producing nothing usable is a real finding, not a failure. Say plainly what
you could not reach, and name any **FAILED** source. List roles you screened out with one line
each so the Owner can check whether the filtering was too aggressive.

The Owner picks which Lead to pursue; then the next steps run in order:

```sh
# capture any missing ad text, then
# assess-fit writes fit.md, then prepare-documents writes resume.md and cover-letter.md
```

Never start building an application from this skill. The note you wrote when saving the Lead is
triage; `fit.md` is the verdict, and it can still come back weak once the ad is read properly.
