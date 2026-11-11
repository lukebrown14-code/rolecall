---
status: accepted
---
# Status lives in each Application's folder, not in shared tables

Every Application, including a Lead, is a job folder from the moment it is added, and its `meta.yml` holds its one Status as an append-only log. The earlier design kept Status in shared Markdown tables (tracker, shortlist, dismissed list). The TUI and the Harness write at the same time; with one shared table they would overwrite each other's rows, while a file per Application means each writer only touches the Application it is working on. It also lets a Lead carry its job description, Fit and notes before the Owner commits to it.

## Consequences

- Dashboard-era tables need a one-time migration into folders.
- The Sweep skips Dismissed roles by reading folder URLs, not a dismissed list.
