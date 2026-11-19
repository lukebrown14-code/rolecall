---
status: accepted
---
# A personal, local TUI instead of a hosted web dashboard

Rolecall replaces an earlier SvelteKit dashboard that had grown hosted multi-user mode, cookie auth and email intake: far more code than one person's job search needs. Rolecall is a local terminal UI for a single Owner. It shows Applications, changes their Status and opens their documents. All AI work (capture, fit, documents, finding roles) happens in the Owner's own coding Harness through the Plugin, never in the app. Until v1.0 it serves its first Owner only; going public is a later, separate decision.

## Consequences

- No phone or remote access; the TUI runs where the Workspace is.
- No email intake; Leads are added by hand or by the Harness.
