---
name: design-review
description: Cold-read review of the TUI by N context-free agents that see only snapshots, then a ranked list of what they agree on.
disable-model-invocation: true
---

A **cold read**: reviewers who know nothing about rolecall judge only what a user sees. Their value is that they are not anchored on the code or on your reasons, so give them snapshots and a lens, never your conclusions.

## Steps

1. **Snapshot.** Run `scripts/snap.sh <scratchpad>/snaps` (add sizes, e.g. `120x36 80x24 60x16`, when layout at a width is the question). Done when the folder holds a `.txt` and `.ansi` per state and size. To add a state, extend `snapshotStates` in `ui/snapshot_test.go`. Read two or three `.txt` files yourself: a snapshot that shows the wrong screen wastes every reviewer.

2. **Brief.** One prompt per reviewer, each with a distinct lens; default to the user's count, else 5:
   - visual design: hierarchy, alignment, spacing, consistency, truncation, stray characters
   - interaction: discoverability, state visibility, feedback, pop-up keys, first-time confusion
   - accessibility: meaning by colour alone, contrast (read `38;2;`/`48;2;` codes in `.ansi`), glyph rendering
   - the user's own focus, if they named one

   Every brief carries: the snapshot folder; what the files are (plain vs colour, file name = keys pressed, sizes); the fixed test clock (`fixtureNow` in `ui/clock_test.go`) so dates are not reported as bugs; "do not read the source code; do not edit files"; and the return shape: defects most severe first citing file and line, up to six small improvements, ONE big change with a justification, under 400 words.

   Add code reviewers only when asked. They read `git diff <base>...HEAD`. Any reviewer that writes files (throwaway tests, probes) runs with `isolation: "worktree"`, so it cannot collide with reviewers running the suite in the shared checkout.

3. **Dispatch** every reviewer in one message so they run in parallel. Done when all have reported.

4. **Synthesise.** Merge the reports into one list: bugs first (mark each confirmed when two reviewers agree or you reproduced it, citing the snapshot), then design issues, then each reviewer's big idea. End with a recommendation: what to fix now, and what to file as issues. Done when every reported defect is either in the list or dropped with a one-line reason.
