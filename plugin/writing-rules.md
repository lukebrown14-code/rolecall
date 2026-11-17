# Writing rules

General rules for producing job-application documents. They apply to any Owner and name no
personal facts or preferences. The Owner's own facts live in the Workspace Profile; the plugin
ships only the general rules, the AU style guide and the skills that apply them.

## What this is

A pipeline for tailoring job applications. A job description goes in; a tailored resume and
cover letter come out, as Markdown source and as PDF, generated only from a locked fact set.

## Tone layering

Documents are written by layering three files, in this order:

1. `plugin/writing-rules.md` — these general rules.
2. `plugin/styles/au.md` — the Australian style guide: register, voice, structure, resume rules.
3. `<workspace>/profile/writing-preferences.md` — the Owner's own preferences and facts.

**Later files win.** Where the style guide and the general rules disagree, the style guide's
presentation rules win. Where the Owner's preferences disagree with either, the preferences win
in tone and convention — but the facts-only rule below is absolute and none of the three may
override it.

## Commands

Every script resolves the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`, in that
order. Run the commands from the repository root (or point at the scripts by path), and always
pass the Workspace explicitly when it is not the current directory.

```sh
# One-time setup
brew install pandoc typst poppler

# 1. Capture a job description
python3 plugin/scripts/new-job.py --track <track> --url https://...
python3 plugin/scripts/new-job.py --track <track> --paste            # reads stdin
python3 plugin/scripts/new-job.py --track <track> --file ~/Desktop/jd.txt
#    --company and --role override what is parsed from the JD
#    --source <url> records the ad URL when pasting

# 2. (the agent writes fit.md -- the verdict -- and the Owner decides whether to proceed)

# 3. (the agent writes resume.md, cover-letter.md and gaps.md)

# 4. Build PDFs
python3 plugin/scripts/build.py jobs/<track>/<company>-<role>-<date> --data "$ROLECALL_WORKSPACE"

# Refresh GitHub-derived project facts
python3 plugin/scripts/sync-github.py --data "$ROLECALL_WORKSPACE"

# Sweep for new roles -- collects candidates, judges nothing
python3 plugin/scripts/sweep.py --data "$ROLECALL_WORKSPACE" --track <track> --track <track>
```

## The pipeline stages

Every Application is a folder under `jobs/<track>/` that holds a `meta.yml`. Status lives in
that file's append-only `status_log`; the current Status is its last entry. The vocabulary is
fixed by `SCHEMA.md`: `Lead`, `Draft`, `Applied`, `Interviewing`, `Offer`, `Rejected`,
`Withdrawn`, `Dismissed`.

- **Lead** is a captured job nobody has worked on yet.
- **Draft** means the documents are built but not sent. `build.py` appends `Draft` only when
  the current Status is `Lead` and it produced PDFs. Every later Status is the Owner's decision.
- **Applied** and beyond record what happened. The first `Applied` entry supplies the applied
  date; nothing is stored twice.

Two further states are worked out from the files, never stored:

- **Written** — `resume.md` has text.
- **Ready** — `out/` holds a resume PDF, plus a cover-letter PDF unless `resume_only: true`.

**Resume only** is a deliberate per-application choice stored as `resume_only: true` in
`meta.yml`. It is never inferred from an empty letter. An empty letter without it is an
incomplete application: write the letter, or ask the Owner whether this one is resume only.

## Finding roles

`sweep.py` does the mechanical half of the search: it fetches the configured boards, filters by
track and location, dedupes against every `meta.yml` `url` including Dismissed Applications, and
flags visa signals using the same `BLOCKERS` patterns as `new-job.py`. It writes
`jobs/sweep-candidates.md` and **decides nothing** — the judgement is the Owner's.

The `find-jobs` skill reads that candidate list, adds manual searches, and creates a Lead
folder for each role worth pursuing. Capture is what turns a candidate into an Application:
`new-job.py` writes the `meta.yml` and the `jd.md`. A candidate is not an Application until
that folder exists.

When present, `jobs/roles.json` is the master search catalogue: title phrases, track mappings,
priorities, fit guidance and additional sources. The script uses its title phrases; the
`find-jobs` skill uses its titles and fit guidance for wider searches and assessment. Without a
catalogue, built-in title keywords apply. Sweeps default to all workspace tracks; `--track`
narrows them.

To widen coverage, add Sources to the Source registry, `workspace.json` `sources` (Settings →
Sources): `api` entries for supported boards, `browser` or `manual` entries for sites searched
by hand, each with its access status and parsing hints. The registry replaces the entire
source plan. Without it, the catalogue's `additional_sources` extend the built-in defaults and
its `manual_search_sources` name the manual searches.

Rules that rule a role out (posting age, applicant caps, contract length) are Search rules in
`workspace.json` `search_rules`, never prose. `writing-preferences.md` is for writing style
only: a rule written there is not applied.

The script prints its effective source plan through
`python3 plugin/scripts/sweep.py --data <workspace> --sources`. Adding or removing a Source
materialises that plan, preserving catalogue additions unless explicitly removed.

Sweep candidates include the matching role families, priorities and title phrases. The Role
coverage table counts unique fetched postings per family, split into known, outside-location,
blocked and to-assess outcomes. Counts are not fit verdicts or evidence of market-wide
coverage. Excluded tracks and runs with no Sources are marked Not searched; failed Sources or a
platform-filtered run make coverage Partial. Manual searches remain part of the `find-jobs`
skill's separate coverage report.

## Assess fit before writing anything

**When a new job folder appears, the first deliverable is `fit.md`, not `resume.md`.** Read
the JD against `profile/master-profile.md` and give the role a verdict before spending the
Owner's time or yours on tailoring. The Owner decides whether to proceed; never skip straight to
the documents, and never talk the Owner into a role to be agreeable.

`new-job.py` already prints visa and clearance signals at capture. Start from those.

### The verdict

Weigh **must-haves met**, not total requirements. Count only what the ad frames as essential.

The verdict is one of exactly four Fit values, the same word everywhere: in `fit.md`, in
`meta.yml`, in the TUI and in conversation. No other labels ("worth applying", "don't apply").

| Fit | Meaning |
| :-- | :-- |
| `Strong` | No blockers; nearly all must-haves evidenced; a clear lead story |
| `Good` | No blockers; most must-haves met; gaps are learnable or peripheral |
| `Marginal` | No blockers, but two or more must-haves unevidenced, or a real seniority mismatch (the role expects meaningfully more seniority than the Profile can evidence). Say what would have to be true for it to land |
| `Poor` | A hard blocker, or the role is a different discipline |

`fit.md` holds: the verdict and one-line reason, the blockers, must-haves met vs unmet, the
strongest lead story if applying, and what is missing if not. **Be blunt.** A padded verdict
costs a day of work and an avoidable rejection; over-caution costs a job the Owner could have
had. Neither is the safe option, so just be accurate.

## The non-negotiable rule

**Read `profile/master-profile.md` and `profile/projects.md` before writing any application
document. Never state a fact, employer, date, job title, tool or metric that is not in one of
them.**

If the JD asks for something the Profile cannot evidence, it goes in the job's `gaps.md`.
Never onto the page. This is the whole point of the pipeline — the documents go to real
employers and every line has to be stood behind.

## Architecture

```
plugin/
  skills/        one folder per skill: capture-job, find-jobs, assess-fit, prepare-documents
  scripts/       new-job.py, build.py, sweep.py, meta.py, set-fit.py, sync-github.py
  templates/     brand.typ, resume.typ, cover-letter.typ, fonts/ (Hanken Grotesk)
  styles/au.md   the Australian style guide (the presentation layer)
  writing-rules.md   these general rules
<workspace>/
  profile/       the fact layer -- master-profile.md, projects.md, phrasing-bank.md,
                 writing-preferences.md
  jobs/<track>/<company>-<role>-<date>/   one folder per application
    jd.md → fit.md → resume.md + cover-letter.md + gaps.md → out/*.pdf
```

The separation matters: **facts never live in a job folder, and presentation never lives in the
profile.** A job folder holds only the JD, the selection of facts made for it, and the output.
The code repo holds no Owner data at all.

### How a build works

`build.py` runs Markdown → Typst → PDF:

1. `pandoc --to typst` converts `resume.md` to Typst markup.
2. The leading top-level heading (the Owner's name) is stripped — the template prints the name
   itself.
3. The markup is injected into `plugin/templates/*.typ` at `{{BODY}}`, alongside `{{CONTACT}}`,
   `{{HEADLINE}}` and `{{DATELINE}}`. For the resume, the Key Skills paragraph is swapped for
   a `#skill-chips(...)` call built from the Markdown keywords. For the letter, the first
   paragraph (the `Company — Role (Ref)` line) is lifted into `{{SUBJECT}}` and printed
   beside the date; a letter that opens on `Dear …` keeps its body whole.
4. `typst compile` produces the PDF, named `<Owner name> Resume <Company> <Role>.pdf` and
   `<Owner name> <Company> <Role> Cover Letter.pdf`.
5. The ATS check reads the PDF back with `pdftotext` (and `pdffonts`) and fails the build on
   any problem: the Owner's Contact missing, a role's dates off the role's line, Key Skills
   out of order, a non-standard or unreadable `##` heading, merged-letter characters, a word split at a
   hyphen, or an unembedded font. The page count is then checked (2 pages, 3 for a senior
   role; a 1-page resume warns as UNDERFULL).
6. If the current Status in `meta.yml` is `Lead` and PDFs were produced, `build.py` appends
   `Draft`. It writes only `out/` and that one Status entry; it never rewrites other keys.

**Markers are braced (`{{BODY}}`) deliberately.** They were bare words once and the word
`BODY` in a template comment got substituted too, injecting the resume twice.

### Markdown contract for a job document

The templates depend on this structure. Deviating from it breaks the layout silently.

```markdown
# <Owner name>                                 <- stripped by the build
## Professional Summary                        <- level 2 = ruled section heading
## Professional Experience
### Job Title — Employer | Dates               <- level 3, " | " splits title from dates
#### Optional context sub-line                 <- level 4 = italic grey
- Bullet
## Skills
**Group** — comma, separated, list
```

The ` | ` separator in a level-3 heading is what puts the dates flush right. Omit it and the
whole string renders as the title.

### Typst template gotchas

- **`@` in content mode is a label reference.** An address written as content fails to compile.
  The contact line is therefore a Typst *string* (`#let contact-line = "…"`), not markup.
- **Letter-spacing breaks text extraction.** Above ~0.08em tracking, pdftotext splits
  capitals into single letters (`P R O F E S S I O N A L`). Keep heading tracking at 0.08em.
- **The letter's practical note is styled by its opening words.** A `show par` rule in
  `cover-letter.typ` boxes the paragraph starting `One practical note`; keep that phrasing.
- **Heading bodies are sequences, not strings.** Pandoc's smart punctuation splits them, so
  `it.body.text` throws. `brand.typ` defines `to-plain()` to flatten content recursively.
  There is no public `space` variable; it is detected via `[ ].func()`.

### Build-time gates

`build.py` refuses to produce a document that breaks a hard rule, rather than warning:

- **Cover letter over the configured word cap** (default 350), closing note included. Hard
  failure.
- **Empty cover letter without Resume only** — hard failure, naming `cover-letter.md`. With
  `resume_only: true` in `meta.yml` the letter is skipped and the resume still builds.
- **Key Skills outside the configured count** (default 8–12), or any keyword with no evidence in
  the resume body or the profile. Hard failure — that block is the most ATS-visible part of the
  document and so the easiest place to overclaim by accident. Matching is by word boundary. A
  resume with no Key Skills block is a hard failure.
- **Fact gate**, on `resume.md` and `cover-letter.md`: any number or percentage not in
  `master-profile.md` or `projects.md`; any term from the Profile's "Facts that are NOT
  available" section (a letter may name one only in a sentence that negates it, e.g. "I have no
  SOC experience", or attributes it to the ad); placeholders (`TODO`, `[Company]`); on resumes,
  an employer, date or job title not in the Profile, and an unapproved headline.
- **Cover letter structure**: a `Dear` line or a subject line, the company named, the
  `One practical note` paragraph exactly once, and none of the banned phrases in
  `plugin/styles/au.md`.

It warns, but still builds, on page count: OVERFLOW past the limit, UNDERFULL at one page.
Fix page count by changing content, not by shrinking the template.

## Writing the documents

`plugin/styles/au.md` is binding, layered under these rules and over the Owner's preferences as
described above. The two things that matter most:

**Cover letter voice.** The letter is an offer of help, not a request for consideration. Open
on the employer's problem, answer it with one concrete piece of evidence, repeat for their
second and third priority, and close on what the Owner would take on first. Never open with
wanting the job. Never close with gratitude. Respect the configured word cap, which is hard.

**Tailoring is selection, not invention.** Reorder bullets, drop what does not serve the JD,
reword using `profile/phrasing-bank.md` — which holds the Owner's own phrasing for each fact, so
the output sounds like the Owner rather than like a language model. Each track has its own verb
register there; mixing registers is what makes a resume read as generic.

## Keeping the profile current

`profile/master-profile.md` is hand-maintained. `profile/projects.md` has a synced fact table
(commit counts, languages, dates) regenerated by `sync-github.py`, and hand-written prose below
the `---` that the script preserves. Editing the table by hand is pointless; it will be
overwritten.

Never quote a project's age from that table: a repository migrated from another host has
GitHub commit dates that start at the migration and understate the real timeline.
