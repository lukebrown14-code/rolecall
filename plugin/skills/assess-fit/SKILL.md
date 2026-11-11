---
name: assess-fit
description: Decide whether an Application is worth pursuing and write the verdict. Use after a job is captured, when the Owner asks how well they fit a role, to assess, grade or score a job, or when a task names assess-fit. Reads jd.md against the Profile, writes fit.md, and sets fit in meta.yml.
---

# Assess fit

The first deliverable for a new Application folder is `fit.md` — the verdict — not a resume.
Read the ad against the Owner's locked facts and decide, before any time goes into tailoring. You
write one file, `<application folder>/fit.md`, and set one key, `fit` in `meta.yml`. Nothing else.

**Decide; do not sell.** The Owner relies on this to avoid wasting a day on a role that cannot
work. A padded verdict costs them that day and an avoidable rejection; over-caution costs them a
job they could have had. Neither is the safe option, so just be accurate.

## 1. Read the facts, then the ad

- `profile/master-profile.md` and `profile/projects.md` in the Workspace — the only facts you may
  use.
- The plugin writing rules and style (`plugin/writing-rules.md`, `plugin/styles/au.md`), then the
  Workspace's preferences (`<workspace>/profile/writing-preferences.md`); the later file wins.
- `<application folder>/jd.md` — the ad. `new-job.py` printed its visa and clearance signals at
  capture; start from those.

Every script takes the Workspace from `--data <path>` or `ROLECALL_WORKSPACE`. Resolve it before
running anything, and do not guess a path.

## 2. Rules

- **Facts only.** State nothing the Profile cannot evidence — no invented employer, date, title,
  tool, metric or claim. An unevidenced must-have goes in *unmet*, never in *met*.
- **Weigh must-haves met, not total requirements.** Count only what the ad frames as essential.
- **A hard blocker is decisive.** Citizenship, a security clearance, permanent residency, or
  sponsorship explicitly unavailable on a permanent role: say so and the verdict is *Poor*
  regardless of everything else. Name the blocker.
- **Do not downgrade a role for being junior to the Owner.** A step sideways or back is fine; judge
  it on must-haves met like anything else. A *seniority mismatch* means the role expects
  meaningfully more seniority than the Profile can evidence.
- **Not a job ad?** A rejection, acknowledgement or newsletter: verdict *Poor*, with the reason
  saying it is not an advertisement.

## 3. The verdict

There are exactly four values. Use these spellings and capitalisation, in `fit.md` and in
`meta.yml`:

| Verdict | `fit` value | Meaning |
| :-- | :-- | :-- |
| **Strong — apply** | `Strong` | No blockers; nearly all must-haves evidenced; a clear lead story |
| **Good — worth applying** | `Good` | No blockers; most must-haves met; gaps are learnable or peripheral |
| **Marginal — your call** | `Marginal` | No blockers, but two or more must-haves unevidenced, or a real seniority mismatch. Say what would have to be true for it to land |
| **Poor — don't apply** | `Poor` | A hard blocker, or a different discipline |

## 4. Write `fit.md`

The first `#` line is a title; the verdict must be on a line of its own in this exact shape,
naming one of the four values above:

```
**Verdict: <Strong|Good|Marginal|Poor>**
```

Then the shape below. The labelled `Summary:` and `Matches:` lines and the `## Must-haves` table
are what readers scan, so keep their names.

```markdown
# Fit — <Company>, <Role>

**Verdict: <verdict>**

Summary: <the case in one line, ~12 words>
Matches: <3–5 keywords the facts evidence, comma-separated>

<Employer/role meta line, " · "-separated if known>

<one paragraph: the case for or against, in plain prose>

## Blockers

<None stated, or the hard blocker named>

## Must-haves

| Requirement | Status |
| :-- | :-- |
| <essential requirement> | **Met**: <the fact that evidences it> |
| <essential requirement> | **Strong**: <...> |
| <essential requirement> | **Not evidenced**: <...> |

## Why not Strong

- <the honest reasons it is not the top verdict>

## Lead story

<the strongest evidence-backed angle, if applying>

## Gaps (for gaps.md if proceeding)

- <what the ad asks for that the Profile cannot evidence>
```

A row's Status cell in the table starts with **Met**, **Strong**, **Exceeds** or **Not evidenced**,
so the count of must-haves met is read correctly.

## 5. Set `fit` in `meta.yml`

Never edit `meta.yml` by hand. After `fit.md` is written, set only the `fit` key through the shared
writer:

```sh
python3 plugin/scripts/set-fit.py --data "$ROLECALL_WORKSPACE" \
  --folder jobs/SWE/acme-analytics-senior-developer-2026-11-06 \
  --fit Good
```

`--folder` accepts:

- an absolute path to the Application folder;
- a path relative to the Workspace, such as `jobs/SWE/<slug>`;
- a path relative to `jobs/`, such as `SWE/<slug>`;
- a bare folder name, such as `<slug>`, when exactly one track holds it.

A bare name that exists in more than one track is refused as ambiguous — pass the track too. The
script refuses a folder with no `meta.yml`, a malformed `meta.yml`, or anything other than the four
Fit values, and changes nothing when it refuses.

`set-fit.py` patches `fit` only. It preserves unknown keys, comments and every other byte, writes
atomically, and never appends a Status.

## 6. Writer rules

- Never append to `status_log`. Fit is not a Status.
- Never touch `notes.md`, `sent/`, or the Profile fact table (`profile/projects.md`).
- Never rewrite `meta.yml`; go through `set-fit.py` so comments and unknown keys survive.
- Do not skip to documents. If the Owner proceeds, the next step is
  `plugin/skills/prepare-documents/SKILL.md`.

## 7. Worked example

A complete `fit.md` for a fictional capture — the shape and standard to hold to:

```markdown
# Fit — Acme Analytics, Senior Developer

**Verdict: Good**

Summary: Most essentials evidenced; the contract length is the real risk.
Matches: TypeScript, Node, PostgreSQL, CI/CD

Acme Analytics · contract, 6 months · Australia · closes 2026-11-20

The ad wants a product-minded developer to own a data-heavy service end to end, and the Profile
evidences that directly: five years building and operating Node services, PostgreSQL schema design,
and CI/CD ownership. The six-month term fits the Owner's work rights exactly, so the usual
sponsorship question never comes up. The one real gap is Kubernetes, which the ad lists as
essential and the Profile does not evidence.

## Blockers

**None stated.** No citizenship, residency, clearance or sponsorship wording.

## Must-haves

| Requirement | Status |
| :-- | :-- |
| 5+ years building production services | **Strong**: owned and operated Node services in production for five years |
| TypeScript and Node | **Met**: TypeScript and Node are the primary stack across recent roles |
| Relational database design | **Met**: designed and tuned PostgreSQL schemas for a data-heavy service |
| CI/CD ownership | **Met**: built and ran the pipeline for the team |
| Kubernetes | **Not evidenced**: containerised services, but no Kubernetes operation |

## Why not Strong

- **Kubernetes is named as essential** and the Profile does not evidence it.
- **Contract length.** Six months is the point of the role for the Owner, but it means the ad's
  "grow with us" framing may not survive the term.

## Lead story

**The data-heavy service, end to end.** Design through to production operation, with the database
and the pipeline owned along the way — the same shape as this ad's core problem.

## Gaps (for gaps.md if proceeding)

- Kubernetes
- Data-pipeline tooling beyond PostgreSQL
```

Setting it is then one command:

```sh
python3 plugin/scripts/set-fit.py --data "$ROLECALL_WORKSPACE" \
  --folder jobs/SWE/acme-analytics-senior-developer-2026-11-06 --fit Good
```

## 8. Hand back

State the verdict, the blocker if any, and the one or two things that decided it. Confirm that
`fit.md` was written and `fit:` was set. Leave the decision to the Owner — they choose whether to
proceed to `plugin/skills/prepare-documents/SKILL.md`. A `Marginal` or `Poor` verdict is a complete,
correct result, not a failure to write documents.
