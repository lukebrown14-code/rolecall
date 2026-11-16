---
name: capture-job
description: Capture a job advertisement into a new rolecall Application. Use when the Owner gives a job ad URL or pastes ad text and asks to capture, save, add, or start an application for it.
---

# Capture job

> **Plugin paths.** `${CLAUDE_PLUGIN_ROOT}` is this plugin's folder (the one holding `skills/`
> and `scripts/`). Claude Code fills it in; in another harness, or if it reaches the shell
> unexpanded, use the absolute path of the folder two levels above this file. Never run a
> script by a path relative to the Workspace, and never hand-write `meta.yml` because a
> script was not found: stop and say which path failed.

Turn a job advertisement into an Application folder. Capture is mechanical and is not a
decision — the Fit assessment is, and it comes next.

**Do not capture and then jump to documents.** Capture is the first of three steps:
capture → assess Fit → prepare documents. The file contract is `SCHEMA.md` at the repository
root; this skill only records what the schema allows.

## 1. Check the URL first

Every writer resolves the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Before
capturing a URL, ask whether it is already in the Workspace:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/meta.py" --data "$ROLECALL_WORKSPACE" find-url https://example.test/jobs/123
```

An empty list means the URL is new. A non-empty list names the Application folders that already
belong to it, whatever their Status. **Do not create a duplicate.** Report the existing folder
and stop, unless the Owner explicitly wants a second Application (a re-advertised role, for
example) — then say so in the report.

## 2. Pick the source

`new-job.py` takes exactly one source of ad text:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/new-job.py" --track SWE --url https://example.test/jobs/123   # fetch a URL
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/new-job.py" --track SWE --file ~/Desktop/jd.txt              # read a file
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/new-job.py" --track SWE --paste                              # read stdin
```

- `--track` is required and names one of the Workspace's tracks.
- `--paste` reads the JD from stdin, so pipe or paste the ad text in.

## 3. Override what the ad text gets wrong

The script parses Company and Title from the ad. When that is wrong or missing, override both
and record the ad URL:

```sh
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/new-job.py" --track SWE --paste --source https://example.test/jobs/123 \
  --company "Acme" --role "Senior Developer"
```

- `--company` and `--role` override what the text parses to and change the folder name.
- `--source <url>` records the ad URL when the text comes from `--paste` or `--file`.
- `--posted YYYY-MM-DD` and `--posted-label "14d ago"` record when the ad was posted, when it
  says; `--applicants` and `--from linkedin` record the applicant count and the registry
  Source it came from. A Search rule the ad breaks is reported, not enforced: the Owner chose
  this ad.

Never type a URL you have not seen. If the ad cannot be fetched, capture it from pasted text.

## 4. What it writes

`new-job.py` creates `jobs/<track>/<slug>/` through the shared `meta.create` and returns that
folder. It writes exactly two files:

- `meta.yml` — `schema: 1`, `title`, `company`, `track`, and `url` and `closes` only when they
  are known (an ad that never states a close date leaves the key out). The `status_log`
  holds one `Lead` entry at the capture time.
- `jd.md` — the job description as plain Markdown, **no front matter**.

It writes no `jobs/TRACKER.md` row, no `fit.md`, `resume.md`, `cover-letter.md` or `gaps.md`,
and no `out/`.

Do not edit the captured `jd.md` by hand. If the ad looked wrong, capture it again and dismiss
the mistaken folder rather than patching the text.

## 5. Writer rules

- Never append any other Status from this skill. The only entry capture writes is the new
  folder's first `Lead`. Append another Status only when the Owner explicitly asks in that
  session, and only through the TUI or the shared writer, never by editing the log by hand.
- Never touch `notes.md`, `sent/` or the Profile fact table (`profile/projects.md`).
- Never rewrite `meta.yml`; set keys through the shared writer so unknown keys and comments
  survive byte for byte.
- Capture never decides Fit. Leave that to the assessment step.

## 6. Hand back

Report the folder path, the keys recorded (including whether `closes` was supplied), and any
visa or clearance signals the script printed — those are the usual reason a role is a
don't-apply. Then assess Fit before any tailoring happens.
