# Email Inbox

Entries are written by the email endpoint. Pending entries wait for the Owner;
Processed is the dedupe record.

## Pending

### msg-1 — rejection — 2026-09-02T08:00:00Z
- from: jobs@example.test
- subject: Your application
- company: Globex
- role: Software Engineer
- match: jobs/SWE/globex-engineer-2026-02-01
- excerpt
  > Thank you for your interest.

## Processed

- 2026-09-02T09:00:00Z msg-1 rejection → Rejected `jobs/SWE/globex-engineer-2026-02-01`
