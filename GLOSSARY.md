# rolecall

rolecall: a terminal job tracker plus a plugin that lets the Owner's coding harness write tailored resumes and cover letters from a locked fact set.

## Language

**Owner**:
The individual job seeker whose Profile and Applications live in a Workspace. An Owner can pursue multiple career tracks using one Profile.

**Workspace**:
The Owner's private folder holding their Profile, settings and Applications. Never part of rolecall's code.
_Avoid_: data dir, project

**Settings**:
The Owner's local preferences (Workspace location, editor, follow-up threshold, Downloads folder, Plugin, Harness), kept in a private config file outside the Workspace. The Owner reads and changes them on the Settings screen, where Enter saves a change at once and u puts it back, or by editing that file.
_Avoid_: preferences, options, config (for the Owner-facing idea)

**Editor**:
The program the Owner chooses to open an Application's files. Any program that takes a file works; if none is chosen, the Owner's environment default applies.
_Avoid_: text editor, IDE

**Track**:
A kind of role the Owner is pursuing (e.g. SWE, Platform). Every Application belongs to one, and a Sweep searches each one.
_Avoid_: category, stream, career path

### Screens

**Leads** (`1`):
The screen of Leads, with views for Dismissed and Filtered Applications.

**Drafts** (`2`):
The screen of Drafts, with a Deadlines view.

**Pipeline** (`3`):
The screen of sent Applications: views All, Applied, Interviewing, Offer and Closed. The screen is Pipeline so that Applied names only the Status: "Pipeline › Applied" is the Applied view.
_Avoid_: Applied (for the screen)

**Stats** (`4`), **Profile** (`5`), **Settings** (`6`):
The report on every Application, the Owner's fact set, and the Owner's preferences.

### Status groups

**Closed**:
Rejected plus Withdrawn: a sent Application that ended. The Pipeline screen's Closed view and Stats count the same two Statuses. Dismissed is never Closed.
_Avoid_: ended, over (for these)

**Active**:
Every readable Application that is not Closed, Dismissed or Filtered. The tab bar's readiness share counts active Applications only.

**Ever reached / currently at**:
How a count reads a Status. The Stats funnel counts the Applications that ever reached a Status ("Ever reached each Status"); a tab or view counts the Applications at it now. So a funnel rung is never below its tab.

### Profile

**Profile**:
The locked fact set every document is written from: career facts plus the rules for using them. Nothing goes in a document that is not in the Profile.
_Avoid_: personal details, master profile (for the whole set)

**Contact**:
The Owner's name and how to reach them, printed on every document: Name and Email are required; Phone, Location, LinkedIn, GitHub and Work rights are optional. Part of the Profile. Only ever the Owner's: someone the Owner writes to about an Application is a Person.
_Avoid_: personal details, identity

**Experience**:
One employment entry in the Profile: employer, dates, approved titles, bullets.
_Avoid_: job, role (for these)

### Sweeping

**Platform**:
A board vendor with a public API or scrapeable structure the Sweep knows how to read (e.g. Greenhouse, Ashby). Each Platform has its own collector.
_Avoid_: board type, provider

**Source**:
One place the Owner looks for roles, as an entry in the Source registry (workspace.json `sources`): an api Source the Sweep fetches (a Platform and its target boards, e.g. greenhouse with cultureamp), or a browser or manual Source (LinkedIn, Jora) the Harness searches by hand. Each has an access status (works, 403, login wall, cloudflare) and may carry its own Search rules.
_Avoid_: board (alone), feed

**Search rule**:
A setting in workspace.json `search_rules` that rules a role out without judgement: a posting-age limit, an applicant cap per Source, a contract-length preference. Applied by the Sweep, by the Lead writer and by a rules check over saved Applications. Writing style is not a Search rule.
_Avoid_: filter (for the setting), preference

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

**Filtered**:
A Dismissed Application a Search rule removed, with the reason (expired, blocker, applicant_cap…) recorded beside it. Listed apart from the ones the Owner dismissed, and restorable; once restored no rule dismisses it again.
_Avoid_: expired (as a Status), discarded

**Recruiter**:
The agency that posted an ad for a client. The client is the Application's company, the real employer, and is what Fit, duplicate checks and Search rules use.
_Avoid_: agency (as a field), employer (for the recruiter)

**Person**:
Someone the Owner writes to about one Application, such as a recruiter, the hiring manager or a referrer: a name, their role, and an email or link (`contacts` in `meta.yml`). The card lists them under PEOPLE and the Edit form edits the first under Person. Never the Owner's Contact.
_Avoid_: contact (for these)

**Likely duplicate**:
A saved Application that is probably the same role as a new one: the same employer and normalised title, or the same ad text. Refused unless the Owner says it is a different role.
_Avoid_: dupe

**Deadlines**:
The view of every Lead and Draft with a deadline still to come, soonest first: a closing date, or a Lead whose ad is within the expiry warning of the max posting age. Those whose closing date has passed follow, then the ones whose date was unclear.
_Avoid_: due list

**Insights**:
The Stats panel that counts the gaps recorded across every Fit write-up, to show which skills would unlock the most roles.
_Avoid_: analytics

**Follow-up due**:
A sent Application (Applied or Interviewing) that has gone the follow-up period (`follow_up_days`, due on the day it ends) without a Status change or a recorded follow-up; a move undone within 15 minutes is no change. It is a reminder, not a Status. It clears when the Status changes or when the Owner records that they followed up (`l`, kept as `followed_up` in `meta.yml`), and comes back once the follow-up period passes again. A next step booked for today or later holds it until that day has passed.
_Avoid_: stale, reminder

**Next step**:
What the Owner does next on an Application and when (`next_step`: a date, optionally a time, and a few words). Shown on Applied, Interviewing and Offer rows, in the detail and in Stats → Now. Not the card's `NEXT` row, which suggests the key to press.

**Close reason**:
Why a Rejected or Withdrawn Application ended (`close_reason`), asked when it moves there and optional, and changed later from Edit. Stats counts Applications by it, as where they dropped off.

**Response**:
A sent Application that reached Interviewing or Offer. A rejection without an interview is not a Response. An Application at Interviewing, Offer, Rejected or Withdrawn was sent, whether or not its Applied was recorded. Response rate leaves out Applications sent within the follow-up period, because they are too new to judge. A move the Owner undid never happened.
_Avoid_: reply, callback

**Fit**:
The verdict on how well an Application matches the Profile: Strong, Good, Marginal or Poor. Empty means not yet assessed; the reasoning lives beside it, not in it.
_Avoid_: score, match

**Resume**:
The resume tailored for one Application, written from the Profile. It is one of the Application's two Documents, beside the Cover letter.
The Owner's general, untailored resume is a CV, not a Resume.

**Cover letter**:
The letter tailored for one Application, written from the Profile: its second Document, not needed when the Application is Resume only.
_Avoid_: letter, cover (for these)

**CV**:
A resume written from the Profile for no Application: the Owner's everyday one (`cv/general.md`) or another kept in `cv/`, such as a long master version. It is built and downloaded from the Profile screen, has no Status and is never one of an Application's documents.
_Avoid_: general resume, master resume (as names for the file)

**Document**:
The Resume or the Cover letter of one Application, always called by that name. Each is Written, then Built: the Resume and, unless the Application is Resume only, the Cover letter, so four steps or two. The DOCS meter, the Docs tab and Draft rows all count that one set ("○ 2/4 ready"). Its file name (resume.md) shows only where it is opened, muted.
_Avoid_: letter, cover, present, absent, empty (as a Document's state) (for these)

**Input**:
A file a Document is written from that is never counted as one: the JD (`jd.md`, the ad's text) and Gaps (`gaps.md`, what the Profile lacks for the role). Each has its own tab and is ready (it has text) or missing; the Docs tab lists them under Inputs, not counted.
_Avoid_: document (for these)

**Written**:
A Document's first step: its file has text. It does not mean a PDF exists; an empty file is not written.
_Avoid_: done

**Built**:
A Document's second step: its PDF is in `out/`, drawn as "✓ PDF" beside the Document's name ("✓ Resume ○ PDF"). Every needed Document Built is Ready.

**Template**:
The design every document is built with. The Plugin ships a default; a Workspace can override it with its own.
_Avoid_: brand, theme

**Build**:
The Plugin's script that turns an Application's documents into PDFs. The verb is "build", and "Built N PDFs" reports one. It is not a synonym for Written, which is about the text alone; a build of a Lead appends Draft.
_Avoid_: compile, render

**Ready**:
The Documents can be sent: the Resume is Built, and the Cover letter too unless the Application is deliberately Resume only (ADR-0004). The list says "✓ ready"; until then it counts the Document steps done, "○ 2/4 ready".
_Avoid_: Stage (retired: Prepared, Assessed, Written, Ready as one pipeline)

**Resume only**:
An Application sent without a cover letter, by choice. It is Ready with the resume PDF alone. The choice is stored on the Application; it is never inferred from an empty letter.

### Agent work

**Bring your own agent (BYOA)**:
The design in which rolecall has no AI of its own. The Owner's Harness does all AI work and meets rolecall only through the Workspace's files. Unlike ACP-style BYOA, rolecall never launches or talks to the agent over a protocol; the files are the only contract (ADR 0003).
_Avoid_: built-in AI, AI integration

**Harness**:
The AI coding tool the Owner already uses (Claude Code, Codex, Gemini CLI, opencode…). It does all AI work using the Owner's own credentials and model. First-run copy explains it in those words: "connect a Harness, the AI coding tool you already use".
_Avoid_: provider (for the harness), AI connection

**Handoff**:
The Owner passing one Application to the Harness from the TUI to run a Plugin skill. The TUI steps aside while the Owner and the agent talk, then re-reads the folder when the agent exits.
_Avoid_: AI action, job (for this), task

**Plugin**:
The skills, scripts and templates rolecall ships for the Harness. It is how the Harness knows the Workspace's files and rules.
_Avoid_: extension, integration
