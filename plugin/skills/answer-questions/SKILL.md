---
name: answer-questions
description: Answer an employer's application-form questions (screening questions, "why do you want this role", selection criteria) from the ad and the Profile, and write them to answers.md. Use when the Owner pastes questions from an application form and asks for answers, or when a task names answer-questions. Reads jd.md and the Profile; writes answers.md. Never states a fact the Profile cannot evidence.
---

# Answer questions

> **Plugin paths.** `${CLAUDE_PLUGIN_ROOT}` is this plugin's folder (the one holding `skills/`
> and `scripts/`). Claude Code fills it in; in another harness, or if it reaches the shell
> unexpanded, use the absolute path of the folder two levels above this file. Never run a
> script by a path relative to the Workspace, and never hand-write `meta.yml` because a
> script was not found: stop and say which path failed.

The Owner has an application form with questions to answer. Write answers they can paste, and
stop. You write one file, `<application folder>/answers.md`. Nothing else. `answers.md` is an
optional extra (SCHEMA.md); the app's Documents and Ready state ignore it. You never submit,
send or fill in a form.

**Locked facts.** Everything an answer claims about the Owner is in `profile/master-profile.md`
or `profile/projects.md`. The ad (`jd.md`) is where you learn what the employer asks and cares
about, never what the Owner has done. Do not state an employer, date, title, tool, metric,
qualification or work right the Profile cannot evidence. This is the `prepare-documents` rule,
and it overrides the style guidance.

## 1. Read

- The questions the Owner pasted. If they are not pasted yet, ask for them. Keep each one
  exactly as written, with its word or character limit when it states one.
- `<application folder>/jd.md`, and `fit.md` and `gaps.md` when present — what the ad asks for
  and what the Profile cannot evidence.
- `profile/master-profile.md`, `profile/projects.md` and `profile/phrasing-bank.md`.
- `${CLAUDE_PLUGIN_ROOT}/writing-rules.md`, `${CLAUDE_PLUGIN_ROOT}/styles/au.md`, then
  `<workspace>/profile/writing-preferences.md`; the later file wins on tone, never on facts.
- `meta.yml` through `python3 "${CLAUDE_PLUGIN_ROOT}/scripts/meta.py" show --folder <folder>`, for
  the role and the real employer.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it before
running anything, and do not guess a path.

## 2. Write `answers.md`

One section per question, in the order asked:

```markdown
# Application answers — <Company>, <Role>

## 1. <the question, as pasted>
<the answer, within the stated limit>

*Evidence: <the Profile entry or project each claim comes from>*

## 2. <the next question>
…
```

- **Answer the question asked**, first, in the Owner's voice and the Australian register of the
  style guide. Lead with the fact, not with enthusiasm.
- **Tie to the ad** where the question invites it ("why us", "why this role"): use what the ad
  says about the work. Never invent what the employer does, and never quote a value or product
  you did not read in `jd.md`.
- **Respect any limit.** Count it. A limit the question gives is hard.
- **Yes/no and eligibility questions** (work rights, clearance, notice period, salary, start date,
  relocation, a licence): answer only from the Profile or `workspace.json`. When neither holds
  the fact, write `[Owner to answer: <what is needed>]` and ask. Never default to "yes".
- **Where the Profile has no evidence** for what the question wants (a gap, or a selection
  criterion with no matching fact), do not stretch. Say what the Owner can honestly say, with the
  nearest evidenced experience, or mark `[Owner to answer]`. Do not claim the skill.
- End each answer with an *Evidence* line naming the Profile entries used, so the Owner can check
  each claim before pasting. The Owner removes the line when they paste.

Overwrite an existing `answers.md` only after saying you are replacing it.

## 3. Report

Say where the file is, which questions have a `[Owner to answer]` gap and why, and that nothing
was submitted. Remind the Owner to read each answer against the form's own limit before pasting.

## 4. Writer rules

- Write only `answers.md`. Never touch `meta.yml`, `status_log`, `notes.md`, `sent/`, the
  documents, `fit.md` or the Profile.
- Do not submit, send or fill in any form, and do not fetch the form's page.
