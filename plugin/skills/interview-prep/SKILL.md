---
name: interview-prep
description: Write interview preparation for an Application: likely questions, one STAR story from the Profile for each gap, and questions to ask the employer. Use when the Owner has an interview coming, or asks to prep, prepare for, or practise an interview for a role. Reads jd.md, fit.md, gaps.md and the Profile; writes prep.md. Uses nothing that is not in the Profile.
---

# Interview prep

> **Plugin paths.** `${CLAUDE_PLUGIN_ROOT}` is this plugin's folder (the one holding `skills/`
> and `scripts/`). Claude Code fills it in; in another harness, or if it reaches the shell
> unexpanded, use the absolute path of the folder two levels above this file. Never run a
> script by a path relative to the Workspace, and never hand-write `meta.yml` because a
> script was not found: stop and say which path failed.

Write the Owner's preparation sheet for one Application and stop. You write one file,
`<application folder>/prep.md`. Nothing else: no `meta.yml` key, no Status, no other document.
`prep.md` is an optional extra (SCHEMA.md); the app's Documents and Ready state ignore it.

**Nothing that is not in the Profile.** Every story, number and claim the Owner might say out
loud must be evidenced in `profile/master-profile.md` or `profile/projects.md`. A STAR story is
built from facts the Profile holds: a real employer, date, tool, action or result, in the Owner's
own wording. Where the Profile has no evidence for a gap, **say so; do not write a story.** The
Owner will be asked about it in the room, so an invented answer is a trap, not help. The same
rule as `prepare-documents` applies, and it overrides the style guidance below.

## 1. Read

- `<application folder>/jd.md` — the ad: its must-haves, tools, team and seniority.
- `<application folder>/fit.md` — the verdict, its Must-haves table and its `## Gaps`.
- `<application folder>/gaps.md`, when present — what the documents could not say.
- `profile/master-profile.md`, `profile/projects.md` and `profile/phrasing-bank.md` — the only
  facts, and the Owner's own wording of them.
- `<application folder>/meta.yml` through `python3 "${CLAUDE_PLUGIN_ROOT}/scripts/meta.py" show --folder <folder>`:
  the role, the real employer and the Status. If the Status is not Applied or Interviewing, say
  so; prepare anyway when the Owner asks.
- `${CLAUDE_PLUGIN_ROOT}/writing-rules.md`, `${CLAUDE_PLUGIN_ROOT}/styles/au.md`, then
  `<workspace>/profile/writing-preferences.md`; the later file wins on tone, never on facts.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it before
running anything, and do not guess a path. If `fit.md` is missing, work from `jd.md` and the
Profile, and say there was no fit assessment.

## 2. Write `prep.md`

```markdown
# Interview prep — <Company>, <Role>

## What they will want to hear
<3–5 bullets: the ad's own priorities, each tied to the Profile fact that answers it>

## Likely questions
<8–12 questions, grouped (role and technical, behavioural, motivation). Each has one line of
"answer from": the Profile fact or project to draw on. A question the Profile cannot answer
says "no Profile evidence: prepare with the Owner".>

## Gap stories (STAR)
### <Gap name, as in fit.md>
**Situation:** … **Task:** … **Action:** … **Result:** …
<one story per gap, from the Profile, or the line "No Profile evidence for this. Say so
plainly, and name the nearest thing the Profile does show: …">

## Questions to ask them
<5–7 questions about the team, the work and what the first months look like, drawn from what
the ad says or leaves out>

## Watch for
<Blockers and weak spots from fit.md, and how to speak about each truthfully>
```

- **One STAR story per gap**, taking the gaps from `fit.md` `## Gaps` (and `gaps.md`), using the
  same names. Choose the Profile experience that is nearest to the gap and honest about the
  distance: transferable evidence, never a claim of the skill itself.
- **Never state a result, number, team size, tool or date the Profile does not hold.** Leave a
  blank `[ask the Owner]` rather than a plausible figure.
- Questions to ask are about the employer, so they may use the ad; they never assert the
  Owner's facts.
- Plain, spoken sentences the Owner can say. No buzzwords.

Overwrite an existing `prep.md` only after saying you are replacing it.

## 3. Report

Say where the file is, how many gaps got a story and how many had no Profile evidence (list
them: those are the Owner's homework), and that nothing was sent or changed elsewhere.

## 4. Writer rules

- Write only `prep.md`. Never touch `meta.yml`, `status_log`, `notes.md`, `sent/`, the documents,
  `fit.md` or the Profile.
- Do not send, book or email anything.
