---
name: prepare-documents
description: Write an Application's tailored resume, cover letter and gaps.md from the Owner's Profile, then run the build and fix any gate failures. Use when a fit verdict says to proceed, when the Owner asks to prepare, tailor, write or rewrite the documents for a role, or when a task names prepare-documents. Reads jd.md, fit.md and the Profile; writes resume.md, cover-letter.md and gaps.md, then proves them with build.py.
---

# Prepare documents

Tailoring is **selection, not invention**: everything on the page is already in the Owner's
Profile. Your job is to choose what this ad asks for, in the Owner's own phrasing, and to prove
the result by building it.

You write exactly three files, all in the Application folder:

    resume.md
    cover-letter.md
    gaps.md

Nothing else in the folder is yours to write.

## 1. One-time Workspace setup

The Owner's writing preferences are personal data and never live in this repository. Before the
first run in a Workspace, make sure they are at:

    <workspace>/profile/writing-preferences.md

If the Owner has an older `docs/writing-preferences.md` (or `docs/application-rules.md`) outside
the Workspace, move it to `profile/writing-preferences.md`. Do not copy it into the code repo,
do not paste its contents into a commit, and do not recreate it from memory. If the file is
missing, write the documents from the general rules and the AU style guide alone, and say so in
the hand-back.

## 2. Read the inputs, in this order

- `profile/master-profile.md` and `profile/projects.md` — the only facts you may use.
- `plugin/writing-rules.md`, then `plugin/styles/au.md`, then
  `<workspace>/profile/writing-preferences.md` — the layering order. **Later files win:** the
  style guide overrides the general rules, and the Owner's preferences override both in tone and
  convention. The facts-only rule below overrides all three.
- `<application folder>/jd.md` and, when present, `<application folder>/fit.md` — the ad and its
  verdict. The verdict's must-haves and lead story are your outline.
- `profile/phrasing-bank.md` — the Owner's own wording for each fact. Use it: it is what makes the
  output sound like the Owner rather than like a model. Each track has its own verb register; do
  not mix them.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it
before running anything, and do not guess a path.

## 3. The rule that overrides all others

**Never state a fact, employer, date, job title, tool, metric or claim the Profile cannot
evidence.** If the ad asks for something the Profile cannot evidence, it goes in `gaps.md`, never
on the page. The documents go to real employers and every line has to be stood behind.

## 4. Write `resume.md`

The templates depend on this structure; deviate and the layout breaks silently.

```markdown
# <the Owner's name>                          <- stripped by the build; keep the line
## Professional Summary                       <- level 2 = ruled section heading
## Key Skills                                 <- 8–12 keywords, in the ad's own wording
## Professional Experience
### Job Title — Employer | Dates              <- level 3; the ` | ` puts the dates flush right
#### Optional context sub-line                <- level 4 = italic grey
- Bullet
## Skills
**Group** — comma, separated, list
```

- **Reorder bullets, drop what does not serve this ad, reword from the phrasing bank.** Do not
  invent to fill a heading.
- **The Key Skills block is the most ATS-visible part of the resume.** 8–12 keywords taken from the
  ad's wording, each one evidenced in the body below it or in the Profile. A keyword with nothing
  behind it is an overclaim, and the build catches it.
- The Owner's conventions (length, work-rights line, approved job titles, metrics) live in
  `profile/writing-preferences.md` and `plugin/styles/au.md`. Follow them exactly.

## 5. Write `cover-letter.md`

The letter is an **offer of help, not a request for consideration**.

- Open on the employer's problem — what this hire has to solve — not on wanting the job.
- Answer it with one concrete piece of evidence, then repeat for their second and third priority.
- Close on what the Owner would take on first.
- Keep every ownership claim the Profile supports, but match the register the preferences file and
  the style guide ask for.
- Carry the closing practical note exactly once; the build styles it by its opening words,
  `One practical note`.
- **Respect the configured word cap** (default 350), the closing note included. It is hard.
- Use none of the banned phrases in `plugin/styles/au.md` ("I am writing to apply…", "I believe I
  would be a great fit…", and the rest of that section).

## 6. Write `gaps.md`

One bullet per requirement the ad names that the Profile cannot evidence. Be specific and honest:
this is what the Owner prepares an answer for, not a place to hide a weakness.

## 7. Resume only — only when the Owner says so

`resume_only` is an explicit choice, never an inference. A missing `cover-letter.md` is
an incomplete application, not evidence that this one is resume only. Set it **only when the
Owner chooses it in this session** (ADR-0004); never infer it from an empty letter, from the ad,
or from a previous Application.

When — and only when — the Owner has chosen it, set the key through the shared writer:

```sh
python3 plugin/scripts/meta.py --data "$ROLECALL_WORKSPACE" \
  patch --folder <application folder> --set resume_only=true
```

Use `resume_only=true`; keep the key absent or `false` otherwise. Never edit `meta.yml` by hand.

## 8. Prove it: run the build

The build turns the Markdown into PDFs **and** is what proves the documents are sound:

```sh
python3 plugin/scripts/build.py <job_dir> --data <workspace>
```

`build.py` **refuses** to produce a document that breaks a hard rule, naming the file and the fix.
Fix the document and build again until it prints `built …`. The hard gates are:

- **Cover letter over the word cap** (default 350, `letterWordCap` in `workspace.json`). Cut words;
  never shrink the template.
- **Key Skills outside 8–12** keywords, or a keyword with no evidence below it or in the Profile.
- **Fact gate** — a number, employer, date or job title the Profile does not evidence, a
  placeholder, or a term the Profile lists as *not available*.
- **Cover-letter structure** — a `Dear` line or a subject line, the company named, the closing note
  exactly once, and no banned phrase.

Page count is a **warning**, not a failure: the AU norm is two pages (three for senior roles), and
a one-page resume reads as junior. Fix it with content, not by shrinking the template.

## 9. Writer rules

- **Never append to `status_log` from this skill.** `build.py` appends `Draft` on its own when the
  current Status is `Lead` and the build produced PDFs. That is the only Status change a document
  run makes; any other Status is the Owner's decision, taken in the TUI.
- Never touch `notes.md`, `sent/`, or the Profile fact table (`profile/projects.md`).
- Never rewrite `meta.yml`; the only key you may set is `resume_only`, and only through `meta.py`
  after the Owner's explicit choice.
- Never touch `out/` by hand; `build.py` owns it.
- Do not skip the build. A document that has not passed the gates is not finished.

## 10. Worked example

A fictional Application for a six-month backend contract: a resume and a cover letter that pass
the gates. Take the shape and the standard, not the facts.

```markdown
<!-- resume.md -->
# Alex Quinn

## Professional Summary

Backend engineer who ships Python and TypeScript services end to end, owning the database, the
pipeline and the on-call rotation along the way.

## Key Skills

Python, TypeScript, PostgreSQL, Docker, Terraform, REST, CI/CD, SQL

## Professional Experience

### Backend Engineer — Northwind Logistics | 2022 – 2025

- Redesigned the PostgreSQL schema behind a high-volume shipment service.
- Built Python and TypeScript services with REST contracts and CI/CD ownership.
- Ran the service in production on Docker, with tracing and alerting the team relied on.

## Skills

**Languages** — Python, TypeScript, SQL
**Platform** — Docker, Terraform, CI/CD
```

```markdown
<!-- cover-letter.md -->
Acme Analytics — Senior Backend Engineer

Dear Acme Analytics team,

From the ad, the thing this hire has to solve looks like a shipment service that has outgrown its
first schema, alongside a platform the team wants to keep boring to operate.

That is the work I have done most recently. At Northwind Logistics I redesigned the PostgreSQL
schema behind a high-volume shipment service, moving the heaviest reporting off the write path
without changing the API. The Python and TypeScript services either side of it stayed REST and
stayed deployable, which is how a small team keeps shipping.

Your ad also asks for someone to own the pipeline and the on-call rotation. I have run both: CI/CD
as a habit rather than a gate, and tracing and alerting tuned so a page means something when it
fires. If platform cost is your second priority, the Terraform-managed environment is where I
would start.

One practical note: I hold full working rights in Australia, so there is no sponsorship
conversation to have.

Kind regards,
Alex Quinn
```

The two things the gates most often catch are visible here: the Key Skills keywords each appear in
the body below the block, and the letter stays well inside the word cap with the closing note
carried exactly once.

## 11. Hand back

State what you wrote, that the build passed, and anything the ad asked for that had to go to
`gaps.md`. Say whether `resume_only` was set and that the Owner chose it. Report any Status the
build appended. Never change a Status beyond that yourself: the pipeline records the Owner's
decisions, not yours.
