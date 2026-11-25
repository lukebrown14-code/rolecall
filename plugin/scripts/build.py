#!/usr/bin/env python3
"""Build an Application's resume and cover letter to PDF.

    python3 plugin/scripts/build.py <job_dir> --data <workspace>
    python3 plugin/scripts/build.py --cv general --data <workspace> [--headline TEXT]

Pipeline per document: Markdown --pandoc--> Typst markup --> injected into the
template --typst--> PDF. Output names carry the Owner's name from the Profile,
e.g. "<Owner name> Resume Acme Backend Engineer.pdf".

The Application's title, company, track, current Status and ``resume_only``
come from ``meta.yml`` through the shared ``meta`` reader; the JD body comes
from ``jd.md`` and the document sources from ``resume.md`` and
``cover-letter.md``. This script writes ``out/`` and, only when the current
Status is Lead and PDFs were produced, appends ``Draft``. It never writes
``TRACKER.md``, ``sent/`` or legacy meta keys.

``--cv NAME`` builds a CV instead: a general resume with no Application,
written at ``cv/<NAME>.md`` and built to ``cv/out/<NAME>/``. Its headline is
the Profile's ``General`` headline unless ``--headline`` names one. It runs
every gate that needs no JD (Contact, facts, Key Skills, the PDF text check)
and writes no ``meta.yml`` or Status.
"""

import argparse
import json
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from datetime import date, datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import contact as owner_contact_module  # noqa: E402
import meta as meta_module  # noqa: E402
import urlcheck  # noqa: E402
from workspace import add_data_argument, load, workspace_from_cli  # noqa: E402

PLUGIN = Path(__file__).resolve().parent.parent
TEMPLATES = PLUGIN / "templates"
STYLE = PLUGIN / "styles" / "au.md"
# The Template files a Workspace may override in <workspace>/templates/.
TEMPLATE_FILES = ("brand.typ", "resume.typ", "cover-letter.typ")

# Set by main() from --data / ROLECALL_WORKSPACE. Tests repoint it after import.
DATA = None
# Tests set this to a fake fetcher so the URL check never touches the network.
URL_FETCHER = None

LETTER_MAX_WORDS = 350
LETTER_MIN_WORDS = 150
KEY_SKILLS_MIN = 8
KEY_SKILLS_MAX = 12


def load_config():
    """Per-workspace limits from workspace.json, keeping the defaults."""
    global LETTER_MAX_WORDS, LETTER_MIN_WORDS, KEY_SKILLS_MIN, KEY_SKILLS_MAX
    gates = load(DATA).gates
    LETTER_MAX_WORDS = gates.get("letterWordCap", LETTER_MAX_WORDS)
    LETTER_MIN_WORDS = gates.get("letterWordMin", LETTER_MIN_WORDS)
    KEY_SKILLS_MIN = gates.get("keySkillsMin", KEY_SKILLS_MIN)
    KEY_SKILLS_MAX = gates.get("keySkillsMax", KEY_SKILLS_MAX)


def need(tool):
    if not shutil.which(tool):
        sys.exit(f"{tool} not found. Run: brew install pandoc typst poppler")


def _read(path):
    """A file's text, or "" when it is absent."""
    path = Path(path)
    return path.read_text() if path.exists() else ""


@dataclass
class Profile:
    """The Profile text, read once per run and passed into every gate.

    ``master`` carries the Contact table, headline table, Experience facts and
    the Key Skills pool; ``projects`` the manual-project metrics. Reading once
    keeps the contact gate, the headline and the fact gates on the same text.
    """

    master: str = ""
    projects: str = ""

    @classmethod
    def read(cls, data):
        directory = Path(data) / "profile"
        return cls(_read(directory / "master-profile.md"), _read(directory / "projects.md"))

    def contact(self):
        """The Owner's Contact from the Contact table in ``master``."""
        return owner_contact_module.Contact(owner_contact_module.parse_contact(self.master))


def titlecase(s):
    small = {"of", "and", "the", "for", "in", "on", "at", "to", "a", "an"}

    def cap(w):
        # Uppercase the first *letter*, not the first character -- "(cyber"
        # must become "(Cyber", and w.capitalize() would leave it alone.
        return re.sub(r"[a-z]", lambda m: m.group().upper(), w, count=1)

    # A word already carrying a capital after its first letter is a deliberate
    # spelling ("UpGuard", "GitLab") -- lowercasing it first would lose that.
    def styled(w):
        return w.isupper() or any(c.isupper() for c in w[1:])

    words = re.split(r"[\s_-]+", s.strip())
    return " ".join(
        w if styled(w) else (w.lower() if i and w.lower() in small else cap(w.lower()))
        for i, w in enumerate(words)
        if w
    )


def to_typst(text, name):
    """Markdown -> Typst markup via pandoc, reading the already-loaded text."""
    r = subprocess.run(
        ["pandoc", "--from", "markdown+smart", "--to", "typst", "--wrap", "none", "-"],
        input=text,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        sys.exit(f"pandoc failed on {name}:\n{r.stderr}")
    return r.stdout


# A salutation or sign-off line ("Dear Sam,", "Kind regards,") whose next line
# is the name or the first sentence, not a blank line.
_LETTER_LINE = re.compile(
    r"^(?:dear\s.+|(?:(?:kind|warm|warmest|best|with\s+(?:best|kind))\s+)?regards"
    r"|yours\s+(?:sincerely|faithfully|truly)|sincerely|respectfully|(?:many\s+)?thanks"
    r"|thank\s+you|cheers|best(?:\s+wishes)?|all\s+the\s+best),\s*$",
    re.I,
)


def keep_line_breaks(md_text):
    """Markdown with a hard line break where the Owner meant a new line.

    Markdown joins a line to the one before it unless a blank line separates
    them, so two mistakes would render as one run-on line:

    - ``Kind regards,`` with the name on the next line ("Kind regards, Sam").
    - consecutive bold lines such as ``**Languages:** Python`` then
      ``**Cloud:** AWS`` (one paragraph instead of one line each).

    Both get a trailing backslash, Pandoc's hard line break. Code fences,
    headings, list items and table rows are left alone.
    """
    lines = md_text.split("\n")
    fenced = False
    for i, line in enumerate(lines[:-1]):
        if line.lstrip().startswith("```"):
            fenced = not fenced
        if fenced or not line.strip() or line.endswith(("\\", "  ")):
            continue
        following = lines[i + 1]
        if not following.strip() or following.lstrip().startswith("```"):
            continue
        stripped = line.strip()
        if stripped.startswith(("#", "|")):
            continue
        if _LETTER_LINE.match(stripped) or (
            following.startswith("**") and stripped.startswith(("**", "- **", "* **"))
        ):
            lines[i] = line.rstrip() + "\\"
    return "\n".join(lines)


def strip_first_heading(typst_body):
    """Drop a leading level-1 heading -- the template already prints the name."""
    return re.sub(r"^\s*=\s+[^\n]*\n+", "", typst_body, count=1)


def letter_word_count(text):
    """Words in the cover letter body, excluding the markdown scaffolding.

    The practical note counts -- it is part of the letter the reader receives,
    and excluding it would just let the letter creep past the cap.
    """
    text = re.sub(r"^#.*$", "", text, flags=re.M)  # headings (the name line)
    text = re.sub(r"^>.*$", "", text, flags=re.M)  # any block quotes
    text = re.sub(r"[*_`#>|-]", " ", text)  # inline markup
    return len(text.split())


def check_letter_length(text):
    n = letter_word_count(text)
    if n > LETTER_MAX_WORDS:
        sys.exit(
            f"\n  cover-letter.md is {n} words. The hard cap is {LETTER_MAX_WORDS}"
            f" (letterWordCap in workspace.json).\n  Cut {n - LETTER_MAX_WORDS} words"
            " and build again."
        )
    if n < LETTER_MIN_WORDS:
        print(f"        WARNING: {n} words, under the {LETTER_MIN_WORDS}-word guide.")
    else:
        print(f"        length: {n} words (cap {LETTER_MAX_WORDS})")
    return n


def kw_in(kw, text):
    """Word-boundary match, so "Go" does not match "going" or "Java" "JavaScript".

    Keywords of three letters or fewer are case-sensitive ("Go", "AWS", "SQL").
    """
    flags = 0 if len(kw) <= 3 else re.I
    return (
        re.search(r"(?<![A-Za-z0-9])" + re.escape(kw) + r"(?![A-Za-z0-9])", text, flags)
        is not None
    )


def key_skills(md_text):
    """(keywords, offset where the block ends) from resume.md, or None."""
    m = re.search(r"^## Key Skills\s*\n+(.*?)(?=\n## |\Z)", md_text, re.S | re.M)
    if not m:
        return None
    return [k.strip() for k in m.group(1).replace("\n", " ").split(",") if k.strip()], m.end()


def check_key_skills(text, profile, name="resume.md"):
    """The Key Skills block is 8-12 keywords and every one must be evidenced.

    It is the most ATS-visible part of the resume and so the easiest place to
    overclaim by accident. A resume with no block is an error: it is the
    primary ATS-scanned section on Seek.
    """
    found = key_skills(text)
    if found is None:
        sys.exit(
            f"\n  {name} has no '## Key Skills' block. AU resumes need one under the"
            " summary (8-12 keywords in the ad's wording)."
        )
    kws, end = found

    if not (KEY_SKILLS_MIN <= len(kws) <= KEY_SKILLS_MAX):
        sys.exit(
            f"\n  Key Skills has {len(kws)} keywords; the range is"
            f" {KEY_SKILLS_MIN}-{KEY_SKILLS_MAX} (plugin/styles/au.md)."
        )

    body = text[end:]
    # Strip the gap list before matching. It names skills the Owner does NOT
    # have, so leaving it in would let the exact terms the list exists to
    # exclude pass as evidence.
    pool = profile.split("\n## Facts that are NOT available")[0]
    orphans = [k for k in kws if not kw_in(k, body) and not kw_in(k, pool)]
    if orphans:
        sys.exit(
            "\n  Key Skills keywords with no evidence below them or in the profile:\n    "
            + "\n    ".join(orphans)
            + "\n  Every keyword in that block must be backed. Cut them or evidence them."
        )
    print(f"        key skills: {len(kws)} keywords, all evidenced")


# --- fact gate -----------------------------------------------------------------

PLACEHOLDERS = re.compile(
    r"\bTODO\b|\bTBD\b|\bFIXME\b|\bXXX+\b|lorem ipsum|\[[^\]\n]{0,40}\](?!\()|\{\{|"
    r"<[A-Z][A-Za-z ]{1,30}>|\b(?:Company|Employer|Role|Hiring Manager) Name\b",
    re.I,
)
VISA_WORDS = re.compile(r"subclass|working holiday", re.I)
NUM_RE = re.compile(r"\d[\d,]*(?:\.\d+)?\+?%?")
# Extra terms that the NOT-available bullets imply but do not spell out.
EXTRA_FORBIDDEN = ["certification", "incident response", "threat detection", "security operations"]


def numeric_tokens(text):
    text = re.sub(r"https?://\S+|\b[\w.+-]+@[\w.-]+\b|\b\S+\.(?:dev|com|io|au)\b", " ", text)
    return [t.rstrip(",.") for t in NUM_RE.findall(text)]


def not_available_terms(profile):
    """Searchable terms from the profile's 'Facts that are NOT available' bullets."""
    m = re.search(
        r"^## Facts that are NOT available\s*\n(.*?)(?=\n## |\n---|\Z)", profile, re.S | re.M
    )
    if not m:
        return list(EXTRA_FORBIDDEN)
    terms = []
    for line in m.group(1).splitlines():
        if not line.lstrip().startswith("-"):
            continue
        phrase = re.sub(r"\(.*?\)", "", re.sub(r"^\s*-\s*No\s+", "", line)).strip()
        caps = [
            w
            for w in re.findall(r"[A-Za-z][A-Za-z0-9+#]*", phrase)
            if (w.isupper() and len(w) > 2) or (w[0].isupper() and len(w) > 3)
        ]
        terms += caps or [re.sub(r"\s+experience$", "", phrase)]
    return list(dict.fromkeys(terms + EXTRA_FORBIDDEN))


def term_re(term):
    parts = [re.escape(p) for p in re.split(r"[\s-]+", term.strip()) if p]
    flags = 0 if term.isupper() else re.I
    return re.compile(r"(?<![A-Za-z0-9])" + r"[\s-]+".join(parts) + r"(?![A-Za-z0-9])", flags)


def _headline_rows(profile):
    m = re.search(r"^## Headlines[^\n]*\n(.*?)(?=\n## |\Z)", profile, re.S | re.M)
    rows = []
    for line in (m.group(1) if m else "").splitlines():
        cells = [cell.strip() for cell in re.split(r"(?<!\\)\|", line.strip().removeprefix("|"))]
        if len(cells) < 2 or not cells[0] or cells[0] == "Track":
            continue
        if set(cells[0]) <= set(":- "):
            continue
        rows.append((cells[0], cells[1].replace("\\|", "|").strip()))
    return rows


def approved_headlines(profile):
    return [headline for _, headline in _headline_rows(profile)]


def headline_for(profile, track):
    """The Profile's approved headline for a track, else its first one."""
    rows = _headline_rows(profile)
    for name, headline in rows:
        if name == track:
            return headline
    return rows[0][1] if rows else "Software Engineer"


def check_headline(headline, jd_text, profile):
    """A headline is approved per track; a narrowed one must echo a title the JD uses."""
    approved = approved_headlines(profile)
    if not approved or headline in approved:
        return []
    title = headline.split("|")[0].strip().lower()
    if title and title in jd_text.lower():
        return []
    return [f"headline {headline!r} is not an approved headline and its title is not in the JD"]


def _norm_dates(s):
    return re.sub(r"\s*[–—-]\s*", "-", re.sub(r"\s+", " ", s.strip().lower()))


def profile_roles(profile):
    """[(key, dates, block text)] from the profile's Experience section."""
    m = re.search(r"^## Experience\s*\n(.*?)(?=\n## )", profile, re.S | re.M)
    roles = []
    for blk in re.split(r"^### ", m.group(1) if m else "", flags=re.M)[1:]:
        head, _, rest = blk.partition("\n")
        key = re.split(r"\s+—\s+", head)[0].strip()
        d = re.search(r"\*\*Dates:\*\*\s*(.+)", rest)
        roles.append((key, d.group(1).strip() if d else "", blk))
    return roles


def check_experience_headings(md_text, profile):
    m = re.search(r"^## Professional Experience\s*\n(.*?)(?=\n## |\Z)", md_text, re.S | re.M)
    if not m:
        return []
    roles = profile_roles(profile)
    if not roles:
        return []
    errs = []
    for h in re.findall(r"^### (.+)$", m.group(1), re.M):
        left, _, dates = h.partition(" | ")
        hit = next((r for r in roles if term_re(r[0]).search(left)), None)
        if not hit:
            errs.append(f"employer not in the profile: {h!r}")
            continue
        if dates and hit[1] and _norm_dates(dates) != _norm_dates(hit[1]):
            errs.append(f"dates {dates!r} for {hit[0]} differ from the profile's {hit[1]!r}")
        title = re.split(r"\s+—\s+", left)[0].strip()
        if title.lower() not in hit[2].lower():
            errs.append(f"job title {title!r} is not an approved title for {hit[0]}")
    return errs


DISCLAIM = re.compile(
    r"\bno\b|\bnot\b|n't\b|\bwithout\b|\bnever\b|\bgaps?\b|\bthe ad\b|\byour ad\b|"
    r"\bthe role\b|\byou (?:ask|want|need)",
    re.I,
)


def _disclaimed(text, pos):
    """True when the sentence around `pos` negates the term or attributes it to the ad."""
    start = max(text.rfind(c, 0, pos) for c in ".!?\n") + 1
    ends = [i for i in (text.find(c, pos) for c in ".!?\n") if i != -1]
    return bool(DISCLAIM.search(text[start : min(ends) if ends else len(text)]))


def fact_errors(text, kind, profile, projects=""):
    """Every reason this resume/letter text breaks the fact rules (empty = clean)."""
    errs = []
    body = text
    if kind == "letter":
        # The "Company -- Role (Ref 1234)" subject line carries the ad's own reference.
        body = re.sub(r"^(?!Dear)[^\n#>].*\(.*\d.*\)\s*$", "", body, count=1, flags=re.M)
    allowed = set(numeric_tokens(profile + "\n" + projects))
    cores = {t.rstrip("%+") for t in allowed}
    for t in dict.fromkeys(numeric_tokens(body)):
        if t in allowed or (not t.endswith("%") and t in cores) or t.rstrip("+") in allowed:
            continue
        errs.append(f"number {t!r} is not in the metrics register or the experience facts")
    for term in not_available_terms(profile):
        for m in term_re(term).finditer(text):
            if kind == "letter" and _disclaimed(text, m.start()):
                continue  # the letter owning a gap, or quoting the ad, is not a claim
            errs.append(f"'{term}' is on the profile's 'Facts that are NOT available' list")
            break
    for m in PLACEHOLDERS.finditer(text):
        errs.append(f"placeholder left in the text: {m.group().strip()!r}")
    if kind == "resume":
        for m in VISA_WORDS.finditer(text):
            errs.append(f"visa wording on a resume: {m.group()!r}")
        errs += check_experience_headings(text, profile)
    return errs


def check_facts(name, text, kind, profile, projects="", extra_errs=()):
    errs = list(extra_errs) + fact_errors(text, kind, profile, projects)
    if errs:
        sys.exit(
            f"\n  {name} fails the fact gate:\n    - "
            + "\n    - ".join(errs)
            + "\n  Facts come only from profile/. Cut the line or add the fact to the profile."
        )
    print("        facts: ok")


# --- cover-letter checks -------------------------------------------------------

DEFAULT_BANNED = [
    "I am writing to apply for",
    "I believe I would be a great fit",
    "This role is an exciting opportunity for me",
    "I was excited to see",
    "I am passionate about",
    "Thank you for considering my application",
    "Please find attached",
]


def banned_phrases():
    if STYLE.exists():
        m = re.search(
            r"### Banned openings and phrases\s*\n(.*?)(?=\n### |\n---|\Z)",
            STYLE.read_text(),
            re.S,
        )
        if m:
            found = [
                re.split(r"…|\.\.\.", q)[0].strip()
                for q in re.findall(r'"([^"]+)"', m.group(1).replace("“", '"').replace("”", '"'))
            ]
            found = [q for q in found if q]
            if found:
                return found
    return DEFAULT_BANNED


def _acronym_expanded(stem, text):
    """ABC counts as mentioned when "Australian Broadcasting Corporation" is."""
    if not (stem.isupper() and 2 <= len(stem) <= 6):
        return False
    words = re.findall(r"[A-Za-z]+", text)
    n = len(stem)
    return any(
        "".join(w[0] for w in words[i : i + n]).upper() == stem
        and all(w[0].isupper() for w in words[i : i + n])
        for i in range(len(words) - n + 1)
    )


def letter_errors(text, company):
    errs = []
    paras = [
        p.strip()
        for p in re.split(r"\n\s*\n", text)
        if p.strip() and not p.lstrip().startswith("#")
    ]
    if not paras:
        return ["cover letter is empty"]
    if not any(p.startswith("Dear") for p in paras) and len(paras) < 2:
        errs.append("no 'Dear ...' line or subject line")
    stem = re.sub(r"\s+(pty\.?\s*)?(ltd|limited)\.?$", "", company or "", flags=re.I).strip()
    if stem and not re.search(re.escape(stem), text, re.I) and not _acronym_expanded(stem, text):
        errs.append(f"company name {company!r} is not mentioned")
    n = len(re.findall(r"One practical note", text))
    if n != 1:
        errs.append(f"'One practical note' paragraph appears {n} times, expected exactly once")
    flat = re.sub(r"\s+", " ", text.replace("’", "'")).lower()
    for ph in banned_phrases():
        if ph.lower() in flat:
            errs.append(f"banned phrase (plugin/styles/au.md): {ph!r}")
    return errs


def check_letter_content(text, company):
    errs = letter_errors(text, company)
    if errs:
        sys.exit(
            "\n  cover-letter.md fails the letter checks:\n    - " + "\n    - ".join(errs)
        )
    print("        letter: ok")


def skills_as_chips(typst_body, md_text):
    """Swap the Key Skills paragraph for a skill-chips() call (resume.typ).

    The keywords come from the Markdown, not pandoc's output, so each one is a
    clean string literal. JSON string syntax is valid Typst string syntax.
    """
    found = key_skills(md_text)
    if not found:
        return typst_body
    kws = found[0]
    call = "#skill-chips((" + ", ".join(json.dumps(k, ensure_ascii=False) for k in kws) + ",))"
    return re.sub(
        r"(^== Key Skills\n+)(.+?)(?=\n\n|\Z)",
        lambda m: m.group(1) + call,
        typst_body,
        count=1,
        flags=re.S | re.M,
    )


def lift_subject(typst_body):
    """Split the letter's opening "Company — Role (Ref)" line from the body.

    cover-letter.typ prints it as a heading beside the date. A letter that
    opens straight on the salutation has no subject line; leave it whole.
    """
    first, _, rest = typst_body.lstrip().partition("\n\n")
    if first.startswith("Dear") or not rest:
        return "", typst_body
    return first.strip(), rest


def template_path(name):
    """The Template file the build uses for ``name``: the Workspace's own copy
    when it has one, else the Plugin's.

    ``<workspace>/templates/<name>`` overrides any of the TEMPLATE_FILES. A
    ``brand.typ`` at the Workspace root is the older override spot and still
    works when ``templates/brand.typ`` is absent.
    """
    candidates = [Path(DATA) / "templates" / name]
    if name == "brand.typ":
        candidates.append(Path(DATA) / "brand.typ")
    for candidate in candidates:
        if candidate.is_file():
            return candidate
    return TEMPLATES / name


def overridden_templates():
    """Workspace-relative paths of the Template overrides this build uses."""
    found = []
    for name in TEMPLATE_FILES:
        path = template_path(name)
        if path.parent != TEMPLATES:
            found.append(path.relative_to(Path(DATA)).as_posix())
    return found


def build(md_text, md_name, template_name, out_pdf, substitutions):
    body = strip_first_heading(to_typst(keep_line_breaks(md_text), md_name))
    if template_name == "resume.typ":
        body = skills_as_chips(body, md_text)
    else:
        substitutions["SUBJECT"], body = lift_subject(body)
    doc = template_path(template_name).read_text()
    substitutions["BODY"] = body
    # BODY, CONTACT and NAME must exist in every template; HEADLINE and DATELINE
    # are per-template (the letter has no headline, the resume has no dateline).
    for marker, value in substitutions.items():
        if marker == "HEADLINE":
            value = value.replace("\\", "\\\\").replace('"', '\\"')
        token = "{{" + marker + "}}"
        if token in doc:
            doc = doc.replace(token, value)
        elif marker in ("BODY", "CONTACT", "NAME"):
            sys.exit(f"{template_name} is missing the {token} marker.")

    # macOS APFS is case-insensitive: writing "(Cyber ...)" over an existing
    # "(cyber ...)" keeps the OLD directory entry's casing, so the filename goes
    # stale while the contents update. Unlink first to force the new name.
    if out_pdf.exists():
        out_pdf.unlink()

    # Typst wants the source file and its root-relative imports
    # (#import "/templates/brand.typ") under one --root. The templates are
    # staged into a scratch root per build so nothing is ever written into the
    # plugin; fonts still load from the real dir.
    with tempfile.TemporaryDirectory() as tmp:
        shutil.copytree(
            TEMPLATES,
            Path(tmp) / "templates",
            ignore=shutil.ignore_patterns("fonts"),
        )
        shutil.copy2(template_path("brand.typ"), Path(tmp) / "templates" / "brand.typ")
        src = Path(tmp) / f".{out_pdf.stem}.typ"
        src.write_text(doc)
        r = subprocess.run(
            [
                "typst",
                "compile",
                "--root",
                tmp,
                "--font-path",
                str(TEMPLATES / "fonts"),
                str(src),
                str(out_pdf),
            ],
            capture_output=True,
            text=True,
        )
        if r.returncode != 0:
            sys.exit(f"typst failed on {md_name}:\n{r.stdout}\n{r.stderr}")
    return out_pdf


def contact_failure(text, name, email):
    """Why a built PDF's extracted text fails the Profile contact check, or None.

    The document must carry the Owner's own name and email, so a stale template
    that still prints someone else's is caught before the PDF is sent.
    """
    if name and name.upper() not in text.upper():
        return f"the Profile name {name!r} is not in the extracted text"
    if email and email not in text:
        return f"the Profile email {email!r} is not in the extracted text"
    return None


# --- PDF text check (what an ATS reads) --------------------------------------------

# Section headings an ATS files a resume's text under. A heading outside this list
# reads as part of the section above it. Compared case-insensitively.
STANDARD_HEADINGS = {
    "summary", "professional summary", "profile", "career summary", "objective",
    "key skills", "skills", "technical skills", "core skills", "core competencies",
    "experience", "professional experience", "work experience", "employment history",
    "relevant experience", "career history",
    "education", "education and training", "qualifications", "training",
    "professional development",
    "certifications", "certificates", "licences", "licenses",
    "certifications and licences", "certifications and licenses",
    "projects", "selected projects", "personal projects", "open source",
    "publications", "awards", "honours", "honors", "achievements",
    "volunteering", "volunteer experience", "languages", "interests",
    "referees", "references", "additional information",
}

# Merged-letter glyphs (ligatures), and the marks of text a PDF reader could not map
# back to letters: an ATS sees these as unknown characters, not words.
BAD_CHARS = re.compile("[\ufb00-\ufb06\ufffd\ue000-\uf8ff]")


def _flat(s):
    """Text compared across Markdown and PDF: one space, plain dashes and quotes."""
    s = s.replace("\u2019", "'").replace("\u2018", "'").replace("\u201c", '"').replace("\u201d", '"')
    s = re.sub(r"\s*[\u2013\u2014-]\s*", "-", s)
    return re.sub(r"\s+", " ", s).strip().lower()


def _markdown_plain(s):
    """A Markdown heading's text without inline marks or escapes."""
    return re.sub(r"[*_`\\]", "", s).replace("--", "\u2013").strip()


def role_line_problems(md_text, lines):
    """Each `### Role | Dates` whose dates are not on the role's extracted line.

    A role wrapped over two lines may carry its dates on the second.
    """
    flat = [_flat(line) for line in lines]
    problems = []
    for left, dates in re.findall(r"^###\s+(.+?)\s+\|\s+(.+?)\s*$", md_text, re.M):
        role, when = _flat(_markdown_plain(left)), _flat(_markdown_plain(dates))
        found = False
        for i, line in enumerate(flat):
            joined = line + " " + flat[i + 1] if i + 1 < len(flat) else line
            if role in line:
                found = True
                if when in line[line.index(role) + len(role):]:
                    break
            elif role in joined and when in flat[i + 1]:
                found = True
                break
        else:
            label = _markdown_plain(left)
            problems.append(
                f"the dates of {label!r} are not on its line in the PDF text"
                if found
                else f"the role {label!r} is not in the PDF text as one line"
            )
    return problems


def heading_problems(md_text, lines):
    """Level-2 headings that are not standard, or that the PDF does not yield as words."""
    flat = {_flat(line) for line in lines}
    problems = []
    for heading in re.findall(r"^##\s+(.+?)\s*#*\s*$", md_text, re.M):
        plain = _markdown_plain(heading)
        if _flat(plain) not in STANDARD_HEADINGS:
            problems.append(
                f"section heading {plain!r} is not one an ATS recognises; use a standard one"
                " such as Professional Summary, Key Skills, Professional Experience, Skills"
                " or Education"
            )
        elif _flat(plain) not in flat:
            problems.append(f"section heading {plain!r} does not come out of the PDF as one line")
    return problems


def skills_order_problems(md_text, raw_text):
    """The Key Skills keywords as the PDF yields them: each one, after the heading, in
    the Markdown's order. Tags a reader takes for columns come out shuffled."""
    found = key_skills(md_text)
    if not found:
        return []
    flat = _flat(raw_text)
    pos = flat.find("key skills")
    if pos < 0:
        return []
    for keyword in found[0]:
        at = flat.find(_flat(keyword), pos)
        if at < 0:
            return [f"Key Skills come out of the PDF out of order, from {keyword!r} on"]
        pos = at + len(_flat(keyword))
    return []


def char_problems(text):
    """Words holding a merged-letter glyph or an unmapped character."""
    words = sorted({w for w in text.split() if BAD_CHARS.search(w)})
    if not words:
        return []
    return [
        "merged-letter or unreadable characters in the PDF text: "
        + ", ".join(repr(w) for w in words[:6])
    ]


def hyphen_problems(layout_text):
    """Words the PDF breaks across lines at a hyphen. A PDF reader joins the two
    halves ("full-" + "platform" reads as "fullplatform"), so the word is lost."""
    lines = [line.rstrip() for line in layout_text.splitlines()]
    problems = []
    for i, line in enumerate(lines[:-1]):
        m = re.search(r"([A-Za-z]+)-$", line)
        nxt = re.match(r"\s*([A-Za-z]+)", lines[i + 1])
        if m and nxt:
            problems.append(f"{m.group(1)}-{nxt.group(1)} is split across two lines at its hyphen")
    return problems


def font_problems(pdffonts_output):
    """Fonts pdffonts lists as not embedded. The table's emb column is found from its
    header, since a font type may hold a space ("CID TrueType")."""
    lines = pdffonts_output.splitlines()
    if len(lines) < 2 or "emb" not in lines[0]:
        return []
    column = lines[0].index("emb")
    missing = [
        line.split()[0]
        for line in lines[2:]
        if line.strip() and line[column : column + 3].strip() == "no"
    ]
    return [f"font not embedded: {name}" for name in missing]


def ats_problems(md_text, raw_text, layout_text, pdffonts_output, letter=False):
    """Every reason the PDF's text would read badly to an ATS (empty = clean).

    ``raw_text`` is pdftotext's reading order, ``layout_text`` its -layout view, and
    ``pdffonts_output`` pdffonts' table, or "" when pdffonts is not installed.
    """
    lines = [line for line in raw_text.splitlines() if line.strip()]
    problems = []
    if not letter:
        problems += role_line_problems(md_text, lines)
        problems += heading_problems(md_text, lines)
        problems += skills_order_problems(md_text, raw_text)
    problems += char_problems(raw_text)
    problems += hyphen_problems(layout_text)
    problems += font_problems(pdffonts_output)
    return problems


def _pdf_text(*args):
    return subprocess.run(["pdftotext", *args], capture_output=True, text=True).stdout


def ats_check(pdf, md_name, contact, md_text="", letter=False):
    """Confirm the PDF yields clean, ordered text for an ATS parser, and carries
    the Owner's Contact from the Profile. Any problem fails the build."""
    if not shutil.which("pdftotext"):
        return None
    text = _pdf_text("-layout", str(pdf), "-")
    reason = contact_failure(text, contact.name, contact.email)
    if reason:
        sys.exit(
            f"\n  {md_name} fails the contact gate: {reason}."
            "\n  The document must carry the Profile's Contact -- check the workspace templates."
        )
    fonts = ""
    if shutil.which("pdffonts"):
        fonts = subprocess.run(["pdffonts", str(pdf)], capture_output=True, text=True).stdout
    problems = ats_problems(md_text, _pdf_text(str(pdf), "-"), text, fonts, letter)
    if problems:
        sys.exit(
            f"\n  {md_name} fails the ATS check (the PDF's text, as an ATS reads it):\n    - "
            + "\n    - ".join(problems)
            + "\n  Fix a heading in the Markdown; anything else comes from the Template"
            " (a Workspace override in templates/ is the first suspect)."
        )
    pages = max(1, text.count("\f"))
    checked = "dates, headings, Key Skills order, characters, hyphens" + (", fonts" if fonts else "")
    if letter:
        checked = "characters, hyphens" + (", fonts" if fonts else "")
    return f"ok ({checked}), {len(text.split()):,} words, {pages} page(s)", pages


def report_pdf(pdf, md_name, contact, md_text, letter=False):
    """Run the PDF text check and print it with the page-count notes."""
    result = ats_check(pdf, md_name, contact, md_text, letter)
    if not result:
        return
    message, pages = result
    print(f"        ATS: {message}")
    # AU convention: the resume runs to two pages, three only for
    # senior/technical roles. The cover letter is always one page.
    if letter:
        if pages > 1:
            print(f"        OVERFLOW: {pages} pages, the cover letter must be one.")
        return
    limit = 3
    if pages > limit:
        print(f"        OVERFLOW: {pages} pages, limit is {limit}.")
    elif pages < 2:
        print(
            f"        UNDERFULL: {pages} page. The AU norm is two —"
            " a one-page CV reads as junior."
        )


# --- CVs -------------------------------------------------------------------------

# The headline row a CV uses in the Profile's Headlines table.
CV_HEADLINE_ROW = "General"
CV_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_-]*")


def cv_source(name):
    """The CV Markdown for ``--cv NAME``: ``cv/NAME.md``. ``cv/NAME.md`` and
    ``NAME.md`` are accepted too. Exits naming the problem."""
    stem = re.sub(r"\.md$", "", re.sub(r"^cv/", "", name.strip()))
    if not CV_NAME.fullmatch(stem):
        sys.exit(f"rolecall: --cv {name!r} -- name a CV in cv/, such as --cv general")
    path = Path(DATA) / "cv" / f"{stem}.md"
    if not path.is_file():
        sys.exit(f"rolecall: no CV at {path}. Write cv/{stem}.md first.")
    return path


def cv_headline(profile, headline):
    """The headline a CV is built with: ``headline`` when given, else the
    Profile's General headline. Exits when there is neither."""
    if headline is not None:
        if not headline.strip():
            sys.exit("rolecall: --headline is empty")
        return headline.strip()
    for track, approved in _headline_rows(profile):
        if track == CV_HEADLINE_ROW:
            return approved
    sys.exit(
        f"\n  headline: profile/master-profile.md has no {CV_HEADLINE_ROW} row in its"
        f" ## Headlines table.\n  Add `| {CV_HEADLINE_ROW} | <headline> |` or pass --headline."
    )


def cv_pdf_name(owner, stem):
    """``<Owner> Resume.pdf`` for cv/general.md, else ``<Owner> Resume <Name>.pdf``."""
    if stem.lower() == "general":
        return f"{owner} Resume.pdf"
    return f"{owner} Resume {titlecase(stem)}.pdf"


def build_cv(name, headline, profile, contact):
    """Build one CV to cv/out/<name>/, replacing what that folder held.

    Gates first, as for an Application: a failure leaves the folder untouched.
    """
    source = cv_source(name)
    md_name = f"cv/{source.name}"
    text = source.read_text()
    if not text.strip():
        sys.exit(f"\nNothing built. {md_name} is empty.")
    subs = {
        "NAME": contact.name,
        "CONTACT": contact.contact_line(),
        "CONTACT2": contact.work_rights,
        "DATELINE": date.today().strftime("%d %B %Y"),
        "HEADLINE": cv_headline(profile.master, headline),
    }
    print(f"  {md_name}")
    check_key_skills(text, profile.master, md_name)
    check_facts(md_name, text, "resume", profile.master, profile.projects)

    out = source.parent / "out" / source.stem
    out.parent.mkdir(exist_ok=True)
    staging = Path(tempfile.mkdtemp(dir=out.parent, prefix=".build-"))
    try:
        pdf = build(text, md_name, "resume.typ", staging / cv_pdf_name(contact.name, source.stem), subs)
        print(f"  built {pdf.name}")
        report_pdf(pdf, md_name, contact, text)
        # One PDF per CV: an older one (a renamed Owner) is replaced, not kept.
        out.mkdir(exist_ok=True)
        for old in out.glob("*.pdf"):
            if old.name != pdf.name:
                old.unlink()
        pdf.replace(out / pdf.name)
    finally:
        shutil.rmtree(staging, ignore_errors=True)
    print(f"\nDone. {out / pdf.name}")


def fit_verdict(job_dir):
    """The verdict line out of fit.md, for callers that summarise an Application."""
    f = Path(job_dir) / "fit.md"
    if not f.exists() or not f.read_text().strip():
        print("        WARNING: fit.md is empty -- the fit assessment was skipped.")
        return "—"
    # Tolerate both shapes seen in practice:
    #   **Verdict: Strong — apply**
    #   ## Verdict: **Strong — apply**
    m = re.search(
        r"^\s*#*\s*\*{0,2}Verdict\*{0,2}\s*[:\-]\s*\*{0,2}([^*\n]+)",
        f.read_text(),
        re.I | re.M,
    )
    return m.group(1).strip().rstrip(".") if m else "—"


# --- document gates ------------------------------------------------------------


def run_gates(md_name, text, company, subs, profile, jd_text):
    """Every hard document gate for one file, in the build's own order.

    The Profile and the JD text were read once in ``main`` and are passed in.
    """
    if md_name == "cover-letter.md":
        check_letter_length(text)
        check_letter_content(text, company)
        check_facts(md_name, text, "letter", profile.master, profile.projects)
    else:
        check_key_skills(text, profile.master)
        check_facts(
            md_name,
            text,
            "resume",
            profile.master,
            profile.projects,
            check_headline(subs["HEADLINE"], jd_text, profile.master),
        )


def check_urls(documents, fetcher=None):
    """Exit on a definitely dead link; warn when links are unverified or the network is down."""
    found = {name: urlcheck.urls_in(text) for name, text in documents.items()}
    results = urlcheck.check({u for urls in found.values() for u in urls}, **fetcher_args(fetcher))
    if urlcheck.offline(results):
        print("        WARNING: no URL could be reached (offline?); links were not checked.")
        return
    for url in urlcheck.unverified(results):
        print(f"        WARNING: {url} could not be verified ({results[url][1]}).")
    bad = set(urlcheck.failures(results))
    lines = [
        f"{url} in {name}: {results[url][1]}"
        for name, urls in found.items()
        for url in urls
        if url in bad
    ]
    if lines:
        sys.exit(
            "\n  dead link(s):\n    - " + "\n    - ".join(lines)
            + "\n  Fix or remove the link, or build with --skip-url-check."
        )
    print("        links: ok")


def fetcher_args(fetcher):
    return {"fetcher": fetcher} if fetcher else {}


def check_letter_present(application, letter_text):
    """Require a letter unless the Application explicitly chose Resume only."""
    if letter_text.strip():
        return
    if application is None:
        sys.exit("\nGate: cover-letter.md is empty and the folder has no meta.yml.")
    if application.malformed:
        sys.exit(f"\nGate: {application.malformed}")
    if not application.resume_only:
        sys.exit(
            "\nGate: cover-letter.md is empty. Write cover-letter.md or set"
            " resume_only: true in meta.yml."
        )


def main(argv=None):
    global DATA
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("job_dir", nargs="?", help="the Application folder to build")
    parser.add_argument(
        "--cv", metavar="NAME", help="build the CV cv/NAME.md instead of an Application"
    )
    parser.add_argument(
        "--headline", help="with --cv: the headline (default: the Profile's General headline)"
    )
    parser.add_argument(
        "--skip-url-check",
        action="store_true",
        help="do not check that the URLs in the documents resolve",
    )
    add_data_argument(parser)
    args = parser.parse_args(argv)
    if bool(args.cv) == bool(args.job_dir):
        parser.error("give an Application folder or --cv NAME, not both")
    if args.headline is not None and not args.cv:
        parser.error("--headline applies only with --cv")

    need("pandoc")
    need("typst")

    DATA = workspace_from_cli(args.data)
    load_config()

    # Who the Owner is comes from the Profile. A missing Name or Email is a
    # hard gate: it would otherwise build documents that identify nobody. The
    # Profile is read once here and passed into the headline and every gate.
    profile = Profile.read(DATA)
    contact = profile.contact()
    missing = contact.missing()
    if missing:
        sys.exit(
            "\n  contact gate: profile/master-profile.md has no "
            + ", ".join(missing)
            + " in its ## Contact table.\n  Add the field(s) and build again."
        )

    if args.cv:
        build_cv(args.cv, args.headline, profile, contact)
        return

    try:
        d = meta_module.resolve_folder(DATA, args.job_dir)
    except meta_module.FolderError as error:
        sys.exit(f"rolecall: {error}")
    if not (d / "jd.md").exists():
        sys.exit(f"{d} does not look like an Application folder (no jd.md).")

    application = meta_module.rescan(d)
    if application is None:
        sys.exit(f"{d} is not an Application (no meta.yml).")
    if application.malformed:
        sys.exit(f"\n  meta.yml is malformed and was not touched:\n  {application.malformed}")

    company = titlecase(application.company)
    role = titlecase(application.title)
    label = f"{company} {role}"

    # Each source Document is read once here and passed into the gates and build.
    jd_text = (d / "jd.md").read_text()
    omit = load(DATA).contact_omit.get(application.track, ())
    documents = {name: _read(d / name) for name in ("resume.md", "cover-letter.md")}

    subs = {
        "NAME": contact.name,
        "CONTACT": contact.contact_line(omit=omit),
        "CONTACT2": contact.work_rights,
        "DATELINE": date.today().strftime("%d %B %Y"),
        "HEADLINE": headline_for(profile.master, application.track),
    }

    check_letter_present(application, documents["cover-letter.md"])

    candidates = [
        ("resume.md", "resume.typ", f"{contact.name} Resume {label}.pdf"),
        ("cover-letter.md", "cover-letter.typ", f"{contact.name} {label} Cover Letter.pdf"),
    ]
    jobs = []
    for job in candidates:
        if not documents[job[0]].strip():
            print(f"  skip  {job[0]} is empty -- ask the agent to write it first")
            continue
        jobs.append(job)
    if not jobs:
        sys.exit("\nNothing built. Write resume.md and cover-letter.md first.")

    # Gate every document before anything is compiled: a later gate failure
    # must leave no PDF of an earlier one behind.
    for md_name, _template, _pdf_name in jobs:
        run_gates(md_name, documents[md_name], company, subs, profile, jd_text)

    if not args.skip_url_check:
        check_urls({name: documents[name] for name, _t, _p in jobs}, URL_FETCHER)

    overrides = overridden_templates()
    if overrides:
        print("  templates: Workspace " + ", ".join(overrides))

    # Build into a staging folder in the Application. Only a fully successful
    # build publishes anything into out/, so a failure leaves out/ untouched.
    built = []
    staging = Path(tempfile.mkdtemp(dir=d, prefix=".build-"))
    try:
        for md_name, template, pdf_name in jobs:
            pdf = build(documents[md_name], md_name, template, staging / pdf_name, dict(subs))
            print(f"  built {pdf.name}")
            report_pdf(
                pdf, md_name, contact, documents[md_name], letter=md_name == "cover-letter.md"
            )
            built.append(pdf)

        out = d / "out"
        out.mkdir(exist_ok=True)
        for pdf in built:
            pdf.replace(out / pdf.name)
    finally:
        shutil.rmtree(staging, ignore_errors=True)

    # SCHEMA.md: build.py appends Draft only when the current Status is Lead
    # and the build produced PDFs. It never touches any other Status.
    if meta_module.append_status(
        d, "Draft", datetime.now(timezone.utc), only_if_current="Lead"
    ):
        print("\n  status: Lead -> Draft")

    print(f"\nDone. {len(built)} PDF(s) in {out}/")


if __name__ == "__main__":
    main()
