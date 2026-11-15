---
name: help
description: Answer the Owner's questions about rolecall itself — what each Plugin skill does and which one to use, how the app's screens, keys, Statuses and Settings work, where Workspace files live, and why something is not showing or not working. Use when the Owner asks how rolecall or a skill works, what to do next, or to troubleshoot setup. Reads only; changes nothing.
---

# rolecall help

Answer the Owner's question about rolecall from its sources, then stop. You **read only**: no
file in the Workspace or the Plugin changes, and no other skill runs. When the answer is "run
this skill" or "press this key", say so and let the Owner do it.

## 1. Find the sources

The Plugin root is two folders above this file (`<plugin>/skills/help/SKILL.md`). Read the source that matches the question:

| Question about | Source of truth |
| --- | --- |
| A skill: what it does, when to use it, what it reads and writes | `<plugin>/skills/*/SKILL.md`, read live. The front matter `description` says when; the body says what it touches. List the folders: never answer from memory of which skills exist. |
| The app: screens, Statuses, keys, Settings, setup, troubleshooting | [`app.md`](app.md) beside this file. For the exact keys on one screen, the app's own `?` help is authoritative. |
| Plugin scripts and the one-time migration | `<plugin>/README.md`, and `python3 <plugin>/scripts/<script>.py --help`. |
| The Owner's own setup: which Workspace, Plugin, Harness, Editor | `rolecall doctor`, when `rolecall` is on the PATH. It changes nothing. |
| The Owner's own data | The Workspace (`--data` or `ROLECALL_WORKSPACE`; `rolecall doctor` names it): `jobs/<track>/<folder>/meta.yml`, `profile/`. Read, never write. |

Read every source the question touches before answering. A question about "which skill"
needs every `SKILL.md` description, not the first match.

## 2. Answer

- Lead with the direct answer in one or two sentences, then the steps: the key to press, the
  skill to ask for, or the command to run, exactly as the Owner would type it.
- Use rolecall's words: Lead, Draft, Applied, Dismissed, Fit, Track, Sweep, Profile, Handoff.
  When the Owner uses an old word (shortlist, tracker, stage), name the current one.
- Say where each fact came from (`find-jobs/SKILL.md`, `app.md`, `rolecall doctor`) so the
  Owner can check it.
- When the sources do not answer it, say so plainly and point to
  https://github.com/lukebrown14-code/rolecall/issues rather than guessing.

The answer is done when every part of the question has an answer traced to a source, or is
named as unanswered.
