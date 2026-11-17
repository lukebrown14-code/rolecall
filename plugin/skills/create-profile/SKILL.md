---
name: create-profile
description: Create or update the Owner's Profile from a resume or a guided interview. Use when the Owner asks to create, build, import or review their Profile.
---

# Create Profile

Build the Owner's Profile: the fact set every Application document must come from. The Owner
approves the finished Profile before it is treated as their locked fact set. All reading,
drafting and questions happen in the Owner's Harness; rolecall does not send Profile data to a
service.

The file contract is [`SCHEMA.md`](../../../SCHEMA.md). Profile files live in the Workspace's
`profile/` directory, not in the rolecall repository. Source documents (an imported resume or
export) go in the Workspace's `cv/` folder; never copy the Owner's personal data into this
repository.

## 1. Choose how to start

Ask which route the Owner wants if they have not said. They can switch routes at any time.

### From a resume or LinkedIn export

When invoked with `--resume <path>`, read that source file and copy it into the Workspace's
`cv/` folder without changing the original. Otherwise, ask the Owner to copy a PDF, DOCX or
LinkedIn export into `cv/` and provide its path. Read each provided file and draft the Profile
from evidence in it. Extract every Experience, including the
employer, dates, approved titles, work and results, then draft the other facts it supports, such
as skills, education and projects. Preserve the Owner's meaning; do not turn responsibilities
into results or fill gaps by guessing.

Show the draft and ask focused questions for missing, unclear or conflicting facts. Resolve
dates and titles with the Owner, and ask for concrete outcomes where the source only describes
responsibilities. Do not treat a LinkedIn export or a resume as authoritative when its facts
conflict; ask the Owner which is correct.

### From scratch

Ask about each Experience in turn. For every entry, collect the employer, dates, approved
title(s), what the Owner did and the results. Confirm each entry before moving on. Then ask about
skills and education, and ask whether there are projects or other career facts they want in the
Profile. Ask one clear question at a time and record only facts the Owner confirms.

## 2. Fill the Profile

Write `profile/master-profile.md` with a `## Contact` section and the career facts. The Contact
section must be a two-column table in the exact shape read by the Contact reader:

```markdown
## Contact

| Field | Value |
| :-- | :-- |
| Name | Alex Quinn |
| Email | alex.quinn@example.test |
| Phone | +61 400 000 000 |
| Location | Brisbane, QLD |
| LinkedIn | linkedin.com/in/alexquinn |
| GitHub | github.com/alexquinn |
| Work rights | Full working rights in Australia |
```

`Name` and `Email` are required. Include other known Contact fields and omit unknown ones; never
invent values. Use the field names and Contact table rules in `SCHEMA.md` so both the Go Profile
reader and Plugin Contact reader can read it. Keep headings as `## Contact` and other top-level
Profile sections as `## ` headings.

For every Experience, record the employer, dates, approved title(s) and evidence-based bullets
for the work and results. Keep skills, education, projects and other confirmed facts under clear
headings. Create `profile/projects.md` for project facts when useful, and create
`profile/phrasing-bank.md` only from wording the Owner supplied or approved. Do not create
`profile/writing-preferences.md` from assumptions; write preferences only when the Owner states
them.

Keep gaps and unresolved questions visible during drafting. Do not add estimates, inferred
metrics, unapproved titles, or facts merely because they would make the Profile look complete.

## 3. Review and approve

Show the Owner the complete proposed Profile, including Contact and every Experience, and list
any unanswered questions or conflicts. Ask the Owner to review the facts and approve the Profile.
Revise it until the Owner approves. If required Contact fields are missing, explain that the
build needs Name and Email and ask for them; do not fabricate them or claim the Profile is
complete.

Only after approval, save the confirmed files under the Workspace's `profile/` directory. The
approved Profile is the locked fact set: future application documents may select and rephrase its
facts, but must not add claims it cannot support. Report the files written and any questions the
Owner left unresolved.

## Writer rules

- Read the Workspace path from `--data <path>` or `ROLECALL_WORKSPACE`; never guess it.
- Write only Profile files in that Workspace. Never modify Applications, `workspace.json`, or
  files in the rolecall repository.
- Preserve the original files in `cv/`; do not overwrite or delete source material.
- The Owner's approval is required before replacing an existing approved Profile.
- Never invent facts or silently resolve conflicting sources.
