# Showcase Workspace

`examples/workspace` is a complete, fictional Workspace for trying rolecall, recording the demo
GIF and reproducing bugs. Everyone and every company in it is made up, and every email address
is at `example.test`.

```sh
go run ./cmd/rolecall --dir ./examples/workspace
```

The app writes to the Workspace it opens. To move or edit Applications without changing the
checked-in copy, copy it first: `cp -R examples/workspace /tmp/showcase`.

## What is in it

- **Owner:** Casey Marlow, a Melbourne software engineer. `profile/` holds a full Profile
  (Contact, Headlines, Experience, Key Skills, Facts that are NOT available) and the three other
  Profile files. `cv/general.md` is built to PDF.
- **Applications**, in `jobs/SWE` and `jobs/Platform`:

| Status | Application | Shows |
| --- | --- | --- |
| Lead | Kestrel Freight, Backend Engineer (Go) | Fit Good, `fit.md` with gaps, pay, close date |
| Lead | Lyrebird Media, Senior Platform Engineer | contract via a recruiter, day rate, Fit Marginal |
| Lead | Saltbush Labs, Software Engineer, Data Tools | ad summary only, not assessed |
| Draft | Corella Pay, Software Engineer, Payments Platform | Ready: resume and letter built |
| Draft | Bluegum Health, Full Stack Engineer | Written: resume written, letter not, no PDFs |
| Applied | Paperbark Software, Backend Engineer | Follow-up due, a contact |
| Applied | Mallee Robotics, Software Engineer, Fleet Tools | Resume only |
| Interviewing | Tidewater Analytics, Platform Engineer | next step, contacts, notes |
| Offer | Ironbark Energy, Software Engineer, Grid Systems | next step |
| Rejected | Wattlebird Games, Backend Developer | close reason `after-screen` |
| Withdrawn | Quandong Travel, Platform Engineer | close reason `salary` |
| Dismissed | Numbat Security, SOC Analyst | Fit Poor, dismissed by the Owner |
| Dismissed | Glasswing Insurance, Senior Backend Engineer | Filtered by the citizenship blocker |
| (malformed) | `jobs/SWE/hand-edited-example-2026-11-12` | the warning row for a broken `meta.yml` |

Every Application that was sent has its PDFs in `out/` and a frozen `sent/`. Dates are fixed
around November 2026, so relative labels ("applied 3w ago", Follow-up due, posting age) drift
as time passes.

Four tests keep it honest, all on a temporary copy:

| Test | Fails when |
| --- | --- |
| `TestShowcaseWorkspace` (`internal/store/tests`) | a Status goes missing, a sent Application loses its PDFs, or the malformed example is fixed by accident |
| `TestShowcasePythonReaderAgrees` (`internal/store/tests`) | the Plugin's `meta.py` reads an Application differently from the TUI |
| `TestShowcaseEveryScreenHasContent` (`internal/ui/tests`) | a screen or view is empty or loses the Applications it shows, at a fixed clock of 19 November 2026 |
| `plugin/tests/test_showcase.py` | `build.py` would now refuse a resume, letter or the CV, so the checked-in PDFs no longer match a rebuild |

## Rebuild the PDFs

After changing a `resume.md`, `cover-letter.md` or the Profile, rebuild with the Plugin (needs
pandoc, typst and poppler):

```sh
python3 plugin/scripts/build.py "$PWD/examples/workspace/jobs/<track>/<folder>" --data examples/workspace
python3 plugin/scripts/build.py --cv general --data examples/workspace
```

`build.py` appends `Draft` to a Lead it builds, so check `git diff` and put back any
`status_log` change it made. For a sent Application, copy the new `out/` PDFs into `sent/out/`.

## Re-record the demo GIF

`docs/demo.tape` records `docs/demo.gif` from a temporary copy of this Workspace. From the
repository root, with [VHS](https://github.com/charmbracelet/vhs) and Go installed:

```sh
vhs docs/demo.tape
```
