---
status: accepted
---
# Ad facts are structured, and company is the real employer

Contract terms, pay, sponsorship, closing-date doubts, how much of the ad was read, and where
a Lead came from all used to live in `notes.md`, so nothing could rank, badge or check them.
They are now optional `meta.yml` keys (SCHEMA.md), written by `meta.py create` and
`new-job.py` and validated by both readers. They stay optional and unknown to older readers,
so the schema stays 1.

`company` is always the real employer. An agency ad keeps the agency in `recruiter`. We chose
this over separate `client` and `recruiter` fields because every reader, sort and Track
breakdown already keys on `company`, and Fit, duplicate checks and Search rules are about the
employer, not the agency.

## Consequences

- Pay is stored as the ad states it (figure, unit, currency) and never converted on disk.
  Readers estimate an annual AUD figure and flag a figure whose unit looks wrong.
- A closing date that cannot be read is `closes_unclear: true`, never a guess.
- A likely duplicate (same employer and title, or same ad text) is refused at creation, and
  `rules.py check` dismisses the later of two with reason `duplicate`.
- `rules.py recheck` writes only `listing` and `listing_checked`. Dismissing a pulled ad still
  goes through the dry run (ADR-0007).
- assess-fit refuses a verdict on a `blurb`, and writes gaps as named bullets so Insights can
  count them across every `fit.md`.
