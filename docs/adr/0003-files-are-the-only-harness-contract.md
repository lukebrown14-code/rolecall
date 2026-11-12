---
status: accepted
---
# Files are the only contract with the Harness; v0.1 has no Harness link

The TUI and the Harness never talk to each other: the agent writes files in a job folder, and the TUI watches the folders and re-reads them. There is no task file, no Agent Client Protocol (ACP) client and no task state. v0.1 is fully usable without any Harness and shows no AI actions.

v0.2 may add keys that hand off to the Harness interactively: the TUI suspends, runs a configured command template (e.g. `claude "/prepare-documents {dir}"`), and resumes and re-reads when the agent exits. The earlier design rejected command templates in favour of ACP because a headless agent cannot ask questions or request permissions. An interactive handoff avoids that, since the Owner talks to the agent in their own terminal, and it needs no protocol client or per-harness adapter. v0.1 builds the same suspend-and-resume path for opening `$EDITOR`, so the later handoff reuses it.

All external process starts go through the `process` package. The TUI delegates its suspend-and-resume and captured runs there, and `rolecall init` uses it to create the Workspace's local Git repository.
