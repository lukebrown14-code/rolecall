<!-- Generic AU style guide. Keep it free of personal data: the Owner's facts and
     preferences live in the workspace's profile/writing-preferences.md. -->

# Style Rules

Binding on every generated document.

**These rules are tuned for the Australian market.** Several of them differ deliberately from
US convention — length, the Key Skills block, and the register. Do not "correct" them back
toward what a US resume guide would say.

---

## Australian register

AU hiring culture is warier of visible self-promotion than the US. Understatement reads as
confidence here; the same sentence that lands as assured in New York can read as skiting in an
Australian context. The fix is register, not substance.

**Understatement means dropping adjectives, not hedging facts.** Where the Profile shows the
Owner held sole ownership, that claim is true and it is among the strongest things on the CV.
Keep it. Strip the swagger around it.

| Retire | Use |
| :-- | :-- |
| Solo founder-engineer | Sole developer |
| Spearheaded / orchestrated / drove | Built / ran / led |
| Deeply experienced in | *(cut — the bullets show it)* |
| Extensive experience with | *(cut, or give the number of years)* |
| World-class, cutting-edge, best-in-class | *(cut entirely)* |
| Passionate about, excited by | *(cut — see banned phrases)* |
| Rare combination of… | *(retired; the reader decides what is rare)* |

Also:

- **Let the metric carry the sentence.** "Lifted unit coverage from 70% to 80%" needs no
  adverb in front of it. An intensifier next to a number weakens the number.
- **Credit the team where the work was shared.** Team-delivered work should read as work the
  Owner led within a team, not as personal triumphs. Where the Owner genuinely held sole
  ownership, say so plainly, once, and move on.
- **No superlatives about the Owner.** Describe what was done and at what scale; the judgement
  is the reader's.
- Australian English throughout: organise, prioritise, analyse, specialise, centre, licence
  (noun) / license (verb).

---

## Cover letter voice

**The letter is an offer of help, not a request for consideration.**

The subject of almost every sentence should be the employer, their problem, or the work —
not the Owner's feelings about the job. A reader should finish the first paragraph knowing what
the Owner thinks their challenge is, and finish the letter knowing what the Owner would do
about it.

### Register

**Default register is technical and analytical** — precise, systems-framed language rather
than conversational connective tissue. Treat the reader's problem as an engineering or systems
question and describe it in those terms.

- Frame the opening problem in systems terms — "The binding constraint in that kind of system
  is…", "structurally the same problem as…" — rather than a chattier restatement ("I'd guess…",
  "reading the ad, this squad exists to…").
- Prefer precise technical nouns over looser paraphrase: "root-cause analysis via log and
  trace inspection" not "figuring out what broke".
- Keep sentences declarative and complete. Avoid asides that soften a claim into a
  conversational hedge beyond the one permitted in the opening (see Structure, step 1).
- This still obeys AU understatement — the formality comes from word choice and sentence
  structure, not from claiming more than the evidence supports.

### Structure

1. **Open on their problem — as a reading, not a verdict.** Name the challenge the ad implies
   this hire exists to solve, in their terms. Not "I am writing to apply for…", but not
   telling them their own business either. Frame it as what the ad suggests:

   > From the ad, the thing this hire has to solve looks like keeping reliability steady while
   > the platform scales.

   > Reading the ad, the constraint seems less about throughput than about making failure
   > modes visible early enough to design around.

   One or two sentences. No throat-clearing. The hedge ("looks like", "seems", "reading the
   ad") is doing real work in the AU market — it keeps the confident structure without the
   presumption of diagnosing a company the Owner has not worked at.
2. **Answer it.** The specific capability that addresses that problem, evidenced by one
   concrete thing actually done. Evidence beats adjectives: "lifted coverage 70% to 80% across
   a live service" not "strong testing skills".
3. **Second and third priority.** Same shape, shorter. Two or three of these paragraphs
   total — pick the JD's top priorities, do not walk the whole list.
4. **Close on the work.** What the Owner would pick up early, or where the Owner would be
   most useful first. Not gratitude, not availability-for-interview boilerplate.
5. **The practical note.** A short closing paragraph, below the close and above the
   sign-off. It states a working-rights or availability fact from the Profile, framed as a
   proposal rather than an apology. It opens with "One practical note" so the template can
   set it in a tinted box.

### Naming a gap

When the ad has a must-have the Owner cannot evidence, **name it in one sentence and move on.**
Do not hide it and do not apologise for it. A named gap with an offer against it beats a
reader discovering it themselves, and in the AU market the directness reads well.

Never claim adjacency as equivalence — application security is not incident response, and a
high-assurance environment is not a SOC. Saying so plainly is what makes the rest of the
letter credible.

### The practical note

The paragraph is for a working-rights or availability fact that belongs in the letter rather
than on the resume. The fact itself comes from `profile/master-profile.md`; never invent one.

Shape:

> One practical note: [the fact from the Profile]. [How the Owner proposes to work with it].
> [What comes next, if anything].

Rules for this paragraph:

- **Two or three sentences. Never more.** It is a footnote, not a section.
- It goes last, immediately before `Kind regards,`.
- **Propose, do not apologise.** No "unfortunately", "I realise this may be an issue",
  "I understand if this is a barrier". A trial framing turns a constraint into a low-risk
  offer.
- It counts toward the 350-word maximum, like everything else in the letter.
- **Omit it** when there is no relevant fact, when the role is itself short-term, or when the
  ad rules the relevant arrangement out — note the latter in `gaps.md` instead.

### Hard rules

- **350 words maximum. Hard cap.** Seek's guidance is 150–350; 350 is a ceiling, not a
  target to creep up to. The practical note **counts toward it**. In practice the body runs to
  roughly 280–300 words so the closing note fits inside the cap. `build.py` refuses to build
  a letter over 350 words.
- Name the company and the exact role title.
- Every factual claim traces to `master-profile.md` or `projects.md`.
- No paragraph whose grammatical subject is the Owner's enthusiasm.
- Australian English: organise, prioritise, analyse, programme (not program, for initiatives).

### Banned openings and phrases

- "I am writing to apply for…"
- "I believe I would be a great fit…"
- "This role is an exciting opportunity for me…"
- "I was excited to see…"
- "I am passionate about…"
- "Thank you for considering my application" *(as a closing line — just stop)*
- "Please find attached…"

### Sign-off

`Kind regards,` followed by the Owner's name from `profile/master-profile.md`.

---

## Resume rules

- **Two pages.** This is the Australian norm and the default for every track. Three pages
  only for genuinely senior or unusually technical roles. **Never one page** — it reads as
  junior here — and never a page and a bit.
- Bullets are **one or two lines**. A three-line bullet is two bullets or a cut.
- **No pronouns.** Bullets start with a verb from the track's register in `phrasing-bank.md`.
- **Never repeat a metric** in both the summary and a bullet.
- **Reverse chronological**, always. Never reorder roles by relevance — reorder *bullets*
  within a role instead.
- Dates as the master profile writes them. Do not convert `2022 – 2025` to months.
- Australian English throughout.
- Serial comma off, en dash (–) for ranges, em dash (—) for asides.

### Section order

```
Contact (incl. work rights) → Professional Summary → Key Skills
→ Professional Experience → Projects → Education → Technical Skills → Referees
```

**The second page is earned space, not default space.** Fill it with evidence or tighten back
to a clean two pages. Never pad to reach it, and never drop real evidence to avoid it — with
a two-page budget, Projects now stays in by default rather than being the first cut.

#### Key Skills

A flat, comma-separated block of **8–12 keywords**, directly after the summary. Not grouped,
not bulleted — one dense line the eye and the parser both take in at once.

This is the primary ATS-scanned section on Seek and what recruiter keyword searches hit, so
**lift the wording from the ad itself**: if they write "CI/CD pipelines", do not write
"continuous delivery". It is deliberately redundant with Technical Skills at the foot of the
document — the top block is for the machine and the ten-second scan, the bottom one is for
the person who reads properly.

**Every keyword in this block must be evidenced below it**, in a bullet or in Technical
Skills. The block is a summary of the document, never a place to park terms the Owner cannot
back.

#### Technical Skills

The grouped detail, as in `master-profile.md`. Order the groups to match the ad's priorities.

#### Referees

A plain italic line at the very end — *Referees available on request.* — **not** a section
with its own heading and rule. Four words do not justify the heading, the rule and the space
above them, and on a tight document that section alone pushes a page.

#### Certifications

There is no Certifications section. If one is ever added, give it its own heading rather than
folding it into Education — a certification listed as a bullet under the last degree reads as
if the university awarded it.

### ATS safety

The Typst templates already enforce this; do not undo it.

- Real text only — no images, icons, or text inside graphics
- Single-column body; no tables used for layout
- Standard section headings ("Professional Experience", not "Where I've Been")
- Contact details as text in the document body, never in a header/footer region
- Verify after every build: `pdftotext out/<file>.pdf -` must read in the right order

---

## Tailoring

Allowed:

- Reorder bullets within a role
- Drop bullets that do not serve the JD
- Reword using `phrasing-bank.md`, or into the JD's own vocabulary
- Swap the headline and summary for the track's approved variant
- Swap a role's job title for an approved per-track variant
- Reorder and regroup skills

Not allowed:

- Any fact, tool, employer, date or metric absent from the profile
- A job title not in that role's approved variant list
- Inflating scope ("led a team of 10" when the profile says 4–6)
- Claiming a tool encountered only in study or reading as production experience. A cloud
  service seen in a tutorial is real, but do not broaden it to services the Owner has not run
  in production

Anything the JD wants that the profile cannot evidence goes to `gaps.md`. Never onto the page.
