---
status: accepted
---
# Owner data never leaves the Workspace

When rolecall goes public, it still collects nothing. It has no server, account, sync, telemetry or crash reporter; every Profile, Application and document stays in the Owner's Workspace on their own machine. We chose this over a hosted or opt-in-analytics model because it removes our privacy obligations entirely and keeps the tool as small as ADR-0001 intends. Any future hosted, sync or analytics feature needs a new ADR and must be opt-in.

## Consequences

- The Harness sends Profile, CV and job text to the Owner's own AI provider. rolecall discloses this (README, `init`) but does not gate or redact it; that relationship is the Owner's.
- Bug reports come through GitHub issues, reproduced with the example workspace. Error messages show paths, never file contents.
- `init` creates the Workspace with private permissions and a local git repo with no remote. `doctor` warns, but does not block, on a public remote or loose permissions.
- The public code repo was to be a fresh repo with no history, with this one kept as the private `rolecall-archive`. Superseded by ADR-0009: this repo was already public, so it stays public with its history, and the guard scans that history and the tracker.
- The personal-data guard covers the whole repo. Generic patterns (emails, phone numbers, profile URLs) live in the repo and run on every PR. Owner-specific terms live outside it (a gitignored file for the pre-commit hook, an Actions secret for CI on `main`), so the guard does not leak what it guards.
- The README states the privacy position; `SECURITY.md` covers private vulnerability reports. There is no privacy policy because nothing is collected.
- Fixtures and the example workspace use only fictional people. Real company boards may appear as Sources.
