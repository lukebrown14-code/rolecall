---
status: accepted
---
# A local diagnostics log, never sent anywhere

Before v1.0 the Owner wants to see what goes wrong in daily use: panics, Builds and Handoffs that fail, errors on the feedback line, and updates slow enough to feel laggy. Most of this is gone once the screen redraws. rolecall therefore writes a diagnostics log, `rolecall/diagnostics.jsonl` under `$XDG_STATE_HOME` (else `~/.local/state`), and `rolecall diagnostics` shows it. Nothing is sent: there is no server, upload or crash reporter, so ADR-0005 stands. We chose this over hosted error tracking or analytics because those would supersede ADR-0005 and need an opt-in flow and a privacy policy, which is too much for a tool with one Owner.

## Consequences

- The log is on in development builds (`version` is `dev`) and off in releases. `ROLECALL_DIAGNOSTICS=1` or `0` overrides either. `rolecall doctor` says when it is on.
- An entry names programs, paths, screens, pop-ups, durations, exit codes and error text as the feedback line shows it. It never holds file contents or a program's arguments, which can carry a Harness prompt or job text; a shell script is named by its first word. The end of a failed program's stderr (2,000 bytes) is kept, so typst and pandoc errors survive.
- The feedback line can show an Application's title or a path, so an entry can too. `rolecall diagnostics` asks the Owner to review it before sharing.
- The file is private (0600, in a 0700 directory). At start-up a log of 1 MiB or more becomes `diagnostics.1.jsonl`, keeping one previous generation, and a session writes at most 4 MiB.
- A panic is recorded with its stack and then re-raised, so Bubble Tea still restores the terminal. Commands nested inside `tea.Batch` or `tea.Sequence` are not guarded; Bubble Tea still prints their panics.
- A log that cannot be opened never stops rolecall; it says why on stderr and carries on.
