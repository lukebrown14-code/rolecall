"""Visa, clearance and contract signals found in a job ad.

Shared by new-job.py (capture) and sweep.py (triage), so the two agree on what a
blocker is and no script imports another by path.
"""

import re

# Phrases that decide an application before the writing starts. Detected at
# capture so a blocker is known before any time goes into tailoring.
#
# BLOCKERS are near-certain rejections for a short-contract visa: AU security
# clearance requires citizenship, and a stated refusal to sponsor makes a term
# limit terminal for a permanent role.
DEFAULT_BLOCKERS = [
    (r"australian citizens?(hip)?\b", "requires Australian citizenship"),
    (
        r"\b(nv1|nv2|nv-1|nv-2|agsva|negative vetting)\b|baseline clearance|security clearance",
        "requires an AU security clearance (citizens only)",
    ),
    # "PR" alone is also "PR review"/"pull request": only the capitalised
    # abbreviation next to residency wording counts.
    (
        r"permanent resident|permanent residency|(?-i:\bPR\b)\s+(holders?|status|only|required|or\s+citizen)"
        r"|(citizen(ship)?|resident)\s+or\s+(?-i:PR)\b|\b(?:AU|Australian)\s+(?-i:PR)\b",
        "asks for permanent residency",
    ),
    (
        r"(no|not|unable to|cannot|won't|will not)\s+(offer\s+|provide\s+)?(visa\s+)?sponsor",
        "states sponsorship is not available",
    ),
    (
        r"unrestricted work(ing)? rights|full working rights without restriction",
        "asks for unrestricted work rights (short-contract visas have a term limit)",
    ),
]

# FAVOURABLE signals: a short contract fits a short-contract visa exactly, with
# no awkward conversation needed.
DEFAULT_FAVOURABLE = [
    (
        r"\b(3|three|6|six)[\s-]months?\s+(contract|fixed|initial|engagement|assignment|term|temporary|rolling)"
        r"|\b(contract|term|engagement|assignment)\s+(of|for)\s+(up to\s+)?(3|three|6|six)\s+months",
        "short contract term — suits a short-contract visa",
    ),
    (
        r"\bcontract\b|\bfixed[\s-]term\b|\bday rate\b|\bcontractor\b",
        "contract role — the contract-term limit is a non-issue",
    ),
]


def _compiled(items):
    return [
        (re.compile(pattern, re.I) if isinstance(pattern, str) else pattern, label)
        for pattern, label in items
    ]


BLOCKERS = _compiled(DEFAULT_BLOCKERS)
FAVOURABLE = _compiled(DEFAULT_FAVOURABLE)

# A sentence that says the requirement does NOT apply ("citizenship is not
# required") must not count as a blocker.
NEGATION = re.compile(
    r"(?:\bnot|n't|\bno longer)\s+(?:be\s+)?(?:required|necessary|needed|essential|mandatory|a requirement)"
    r"|\b(?:no|without)\s+(?:\w+\s+){0,2}(?:requirement|need)\b"
    r"|\bno\b[^.;]{0,40}\b(?:required|necessary|needed)\b"
    r"|\b(?:do|does|did)(?:n't|\s+not)\s+(?:need|require|have to)"
    r"|\bneed not\b|\bnot\s+need\s+to\b|\bnot\s+(?:a\s+)?(?:pre-?requisite|condition)",
    re.I,
)
# Equal-opportunity boilerplate lists citizens/residents as groups welcomed,
# not as a condition of the job.
EEO = re.compile(
    r"\b(?:equal opportunity|encourage[sd]?|welcomes?|welcomed|diversity|diverse|inclusive|inclusion|"
    r"all applicants|regardless|indigenous|aboriginal|torres strait|underrepresented)\b",
    re.I,
)


def blockers_for(configured):
    """The compiled blockers for a Workspace's configured list, or the defaults."""
    return _compiled(configured or DEFAULT_BLOCKERS)


def _sentences(text):
    return [s for s in re.split(r"(?<=[.!?])\s+|\n+", text) if s.strip()]


def _search(pat, t):
    return pat.search(t) if isinstance(pat, re.Pattern) else re.search(pat, t, re.I)


def blocker_hits(text, blockers=None):
    """Labels of blockers present as real requirements: matched sentence by
    sentence, ignoring negated requirements and equal-opportunity boilerplate."""
    blockers = BLOCKERS if blockers is None else blockers
    sentences = _sentences(text)
    out = []
    for pattern, label in blockers:
        sponsor = "sponsor" in (
            pattern.pattern if isinstance(pattern, re.Pattern) else pattern
        )
        for sentence in sentences:
            if not _search(pattern, sentence):
                continue
            if not sponsor and (NEGATION.search(sentence) or EEO.search(sentence)):
                continue
            out.append(label)
            break
    return out


def scan(jd, blockers=None):
    """Surface visa and clearance signals before any writing time is spent."""
    hits = [(label, "BLOCKER") for label in blocker_hits(jd, blockers)]
    hits += [(label, "in your favour") for pattern, label in FAVOURABLE if pattern.search(jd)]
    return hits
