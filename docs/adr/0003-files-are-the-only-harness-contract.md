---
status: accepted
---
# Files are the only contract with the Harness

The TUI and the Harness never talk to each other: the agent writes files in a job folder, and the TUI watches the folders and re-reads them. There is no task file, no Agent Client Protocol (ACP) client and no task state. Rolecall is fully usable without any Harness.

The later handoff keys (`f`, `p`, `F`) hand off to the Harness interactively: the TUI suspends, runs a configured command template (e.g. `claude "/rolecall:{skill} {dir}"`), and resumes and re-reads when the agent exits. The earlier design rejected command templates in favour of ACP because a headless agent cannot ask questions or request permissions. An interactive handoff avoids that, since the Owner talks to the agent in their own terminal, and it needs no protocol client or per-harness adapter. The suspend-and-resume path was first built for opening `$EDITOR`, and the handoff reuses it.

All external process starts go through the `process` package. The TUI delegates its suspend-and-resume and captured runs there, and `rolecall init` uses it to create the Workspace's local Git repository.
