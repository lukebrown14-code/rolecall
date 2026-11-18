---
name: draft-follow-up
description: Draft a short follow-up message for an Application that is Follow-up due, and write it to follow-up.md. Use when the Owner asks to chase, nudge or follow up on an application they sent, or when a task names draft-follow-up. Reads meta.yml, jd.md and the Profile; writes follow-up.md. Never sends anything.
---

# Draft follow-up

> **Plugin paths.** `${CLAUDE_PLUGIN_ROOT}` is this plugin's folder (the one holding `skills/`
> and `scripts/`). Claude Code fills it in; in another harness, or if it reaches the shell
> unexpanded, use the absolute path of the folder two levels above this file. Never run a
> script by a path relative to the Workspace, and never hand-write `meta.yml` because a
> script was not found: stop and say which path failed.

The Owner sent this Application and has heard nothing for the follow-up period: no Status
change, and no follow-up recorded, for `follow_up_days` (default 14). Write the message they
could send to nudge it along, and stop. You write one file,
`<application folder>/follow-up.md`. Nothing else.

**You never send.** No email, no form, no network call, no calendar entry. The Owner reads the
message, edits it and sends it themselves.

## 1. Read the facts

- `<application folder>/meta.yml` — the title, company, `contacts` if there are any (or the
  older single `contact`), `next_step` if one is set, the `status_log` and `followed_up`. The log says when the Owner applied and when the Status last
  changed; `followed_up: YYYY-MM-DD`, when present, is the day the Owner last recorded a
  follow-up (`l` in rolecall).
- When it is due: `python3 ${CLAUDE_PLUGIN_ROOT}/scripts/meta.py --data <workspace> follow-up --folder <application folder>`
  (add `--days <n>` when the Owner changed `follow_up_days` in Settings). It prints `FollowedUp`,
  `From` (the later of the last Status change and `followed_up`; a move undone within 15
  minutes does not count), `Since` (which of the two), `HeldBy`, `DueOn` and `Due`, reckoned
  as the app's `⚑` flag is. `HeldBy` is the `next_step` date when one is booked today or
  later: nothing is due while it stands.
- `<application folder>/jd.md` — the role, so the message names it correctly.
- `profile/master-profile.md` in the Workspace, only for the Owner's name and one fact worth
  repeating. The only facts you may use are in the Profile.
- The plugin writing rules and style (`${CLAUDE_PLUGIN_ROOT}/writing-rules.md`, `${CLAUDE_PLUGIN_ROOT}/styles/au.md`), then the
  Workspace's preferences (`<workspace>/profile/writing-preferences.md`); the later file wins.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it before
running anything, and do not guess a path.

Before drafting, tell the Owner when they last followed up, if `FollowedUp` is set. When `Due`
is false, the follow-up is not due yet: say so, give `DueOn` (or, when `HeldBy` is set, the
booked next step that holds it), and draft only if the Owner still wants a message now.

## 2. Write `follow-up.md`

- Address the first entry of `contacts` in `meta.yml` by name (use its `role` to pitch the
  message: a recruiter and a hiring manager want different things). Fall back to the older
  single `contact` key. With no contact, write `Hello,` and say so in the report so the Owner
  can fill the name in.
- When `next_step` is set (a date and what it is), mention it rather than asking about a step
  already booked.
- Three to five sentences. Name the role and the date the Owner applied (or their last
  contact, when the Status is Interviewing), ask for an update, and repeat one fact from the
  Profile that is relevant to the ad. Add nothing the Profile cannot evidence.
- When `Since` is `followed_up`, this is a second nudge: say briefly that the Owner followed up
  on that day, rather than writing as if it were the first.
- Polite and brief. No apology, no pressure, no invented urgency or competing offers.
- Put a one-line `Subject:` first, then a blank line, then the body.

Overwrite an existing `follow-up.md` only after saying you are replacing it.

## 3. Report

Say where the file is, whether a contact was found (and which), and that nothing was sent. Then tell the
Owner: once they have sent it, press `l` on the Application in rolecall to record the follow-up.
That writes `followed_up` and clears the `⚑` until `follow_up_days` after that day.

Do not append to `status_log` and do not edit `meta.yml`, `followed_up` included, even when the
Owner says the message went: recording a follow-up is the Owner's action in the app, as a
Status change is, and SCHEMA.md does not list `followed_up` among the keys an agent writes.
