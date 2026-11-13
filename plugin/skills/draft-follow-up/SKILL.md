---
name: draft-follow-up
description: Draft a short follow-up message for an Application that is Follow-up due, and write it to follow-up.md. Use when the Owner asks to chase, nudge or follow up on an application they sent, or when a task names draft-follow-up. Reads meta.yml, jd.md and the Profile; writes follow-up.md. Never sends anything.
---

# Draft follow-up

The Owner sent this Application and has heard nothing for the follow-up period. Write the
message they could send to nudge it along, and stop. You write one file,
`<application folder>/follow-up.md`. Nothing else.

**You never send.** No email, no form, no network call, no calendar entry. The Owner reads the
message, edits it and sends it themselves.

## 1. Read the facts

- `<application folder>/meta.yml` — the title, company, `contact` if there is one, and the
  `status_log`. The dates in it say when the Owner applied and when the Status last changed.
- `<application folder>/jd.md` — the role, so the message names it correctly.
- `profile/master-profile.md` in the Workspace, only for the Owner's name and one fact worth
  repeating. The only facts you may use are in the Profile.
- The plugin writing rules and style (`plugin/writing-rules.md`, `plugin/styles/au.md`), then the
  Workspace's preferences (`<workspace>/profile/writing-preferences.md`); the later file wins.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it before
running anything, and do not guess a path.

## 2. Write `follow-up.md`

- Address the `contact` from `meta.yml` by name. With no contact, write `Hello,` and say so in
  the report so the Owner can fill the name in.
- Three to five sentences. Name the role and the date the Owner applied (or their last
  contact, when the Status is Interviewing), ask for an update, and repeat one fact from the
  Profile that is relevant to the ad. Add nothing the Profile cannot evidence.
- Polite and brief. No apology, no pressure, no invented urgency or competing offers.
- Put a one-line `Subject:` first, then a blank line, then the body.

Overwrite an existing `follow-up.md` only after saying you are replacing it.

## 3. Report

Say where the file is, whether a contact was found, and that nothing was sent. Do not append to
`status_log` and do not edit `meta.yml`.
