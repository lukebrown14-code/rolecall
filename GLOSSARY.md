# rolecall

rolecall: a terminal job tracker plus a plugin that lets the Owner's coding harness write tailored resumes and cover letters from a locked fact set.

## Language

**Owner**:
The individual job seeker whose Profile and Applications live in a Workspace. An Owner can pursue multiple career tracks using one Profile.

**Workspace**:
The Owner's private folder holding their Profile, settings and Applications. Never part of rolecall's code.
_Avoid_: data dir, project

**Track**:
A kind of role the Owner is pursuing (e.g. SWE, Platform). Every Application belongs to one, and a Sweep searches each one.
_Avoid_: category, stream, career path

### Profile

**Profile**:
The locked fact set every document is written from: career facts plus the rules for using them. Nothing goes in a document that is not in the Profile.
_Avoid_: personal details, master profile (for the whole set), CV

**Contact**:
The Owner's name and how to reach them, printed on every document. Part of the Profile.
_Avoid_: personal details, identity

**Experience**:
One employment entry in the Profile: employer, dates, approved titles, bullets.
_Avoid_: job, role (for these)

### Sweeping

**Platform**:
A board vendor with a public API or scrapeable structure the Sweep knows how to read (e.g. Greenhouse, Ashby). Each Platform has its own collector.
_Avoid_: board type, provider

**Source**:
A single company's board on a Platform, configured per Workspace (e.g. greenhouse:cultureamp).
_Avoid_: board (alone), feed

**Sweep**:
A mechanical run of all Sources: fetch, filter by Track and Search area, dedupe. Judges nothing; its output is candidates the Harness triages into Leads.
_Avoid_: search, scan

**Search area**:
The places a role must be in, and whether remote roles count, for a Sweep to keep it. Not the Owner's own location, which is part of their Contact.
_Avoid_: location (alone), region

### Applying

**Application**:
A role saved with its own job folder, from the moment it is added, whatever its Status.
_Avoid_: job, draft (as a noun for the whole thing)

**Status**:
The single value saying where an Application is: Lead, Draft, Applied, Interviewing, Offer, Rejected, Withdrawn, Dismissed. The Owner's to set; the only automatic change is Lead → Draft when documents are built.
_Avoid_: stage, lane, section (for these)

**Lead**:
The Status of an Application saved as worth a look that the Owner has not committed to. From a Sweep or added by hand.
_Avoid_: shortlist row, candidate (once saved)

**Draft**:
The Status of an Application the Owner has committed to but not yet sent.

**Rejected**:
The Status of a sent Application the employer turned down.
_Avoid_: dismissed (for this)

**Dismissed**:
The Status of an unsent Application the Owner chose not to pursue. Kept and restorable, and skipped by future Sweeps.
_Avoid_: rejected, deleted, passed (for this)

**Follow-up due**:
A sent Application whose Status hasn't changed for the follow-up period. It is a reminder, not a Status.
_Avoid_: stale, reminder

**Response**:
A sent Application that reached Interviewing or Offer. A rejection without an interview is not a Response. Response rate leaves out Applications sent within the follow-up period, because they are too new to judge.
_Avoid_: reply, callback

**Fit**:
The verdict on how well an Application matches the Profile: Strong, Good, Marginal or Poor. Empty means not yet assessed; the reasoning lives beside it, not in it.
_Avoid_: score, match

**Resume**:
The resume tailored for one Application, written from the Profile. It is one of the Application's documents, beside the cover letter.
_Avoid_: CV

**Written**:
The documents have text in them; the build has not produced the required PDFs yet. It does not mean a PDF exists.
_Avoid_: done

**Build**:
The Plugin's script that turns an Application's documents into PDFs. The verb is "build", and "Built N PDFs" reports one. It is not a synonym for Written, which is about the text alone; a build of a Lead appends Draft.
_Avoid_: compile, render

**Ready**:
The documents can be sent: the resume PDF exists, and the cover letter PDF too unless the Application is deliberately Resume only.
_Avoid_: Stage (retired: Prepared, Assessed, Written, Ready as one pipeline)

**Resume only**:
An Application sent without a cover letter, by choice. It is Ready with the resume PDF alone. The choice is stored on the Application; it is never inferred from an empty letter.

### Agent work

**Harness**:
The AI coding tool the Owner already uses (Claude Code, Codex, Gemini CLI, opencode…). It does all AI work using the Owner's own credentials and model.
_Avoid_: provider (for the harness), AI connection

**Handoff**:
The Owner passing one Application to the Harness from the TUI to run a Plugin skill. The TUI steps aside while the Owner and the agent talk, then re-reads the folder when the agent exits.
_Avoid_: AI action, job (for this), task

**Plugin**:
The skills, scripts and templates rolecall ships for the Harness. It is how the Harness knows the Workspace's files and rules.
_Avoid_: extension, integration
