"""Read Applications from a Workspace, mirroring the Go store.

The read side of SCHEMA.md. A folder is an Application if and only if it holds
``meta.yml``. A malformed file is returned as an Application carrying the error,
never as an exception that stops the scan. The line grammar is ``metayaml``'s.
"""

import os
import re
from datetime import datetime
from pathlib import Path

from metayaml import (
    DATE,
    KEY,
    MetaError,
    blank,
    flow_map,
    scalar,
    valid_date,
    valid_next_step_at,
    valid_timestamp,
    whole,
)

ENTRY = re.compile(
    r"^  - \{status: ([A-Za-z]+), at: ([^,}]+)(, approx: (true|false))?\}"
    r"([ \t]+#.*)?$"
)

STATUSES = {
    "Lead",
    "Draft",
    "Applied",
    "Interviewing",
    "Offer",
    "Rejected",
    "Withdrawn",
    "Dismissed",
}
FITS = {"", "Strong", "Good", "Marginal", "Poor"}
# Why a Search rule dismissed an Application (filter_reason). SCHEMA.md lists them.
FILTER_REASONS = {
    "expired", "blocker", "applicant_cap", "location", "contract_length",
    "closed", "pulled", "duplicate",
}
NUMBER = re.compile(r"^[0-9]+(\.[0-9]+)?$")

# The ad facts (SCHEMA.md): key -> (attribute, JSON name, kind). A kind is a set of allowed
# values, "text", "int" (whole number >= 1), "number", "bool", "date" or "currency".
FACTS = {
    "recruiter": ("recruiter", "Recruiter", "text"),
    "employment_type": ("employment_type", "EmploymentType",
                        {"permanent", "contract", "fixed-term", "casual", "part-time"}),
    "term_months": ("term_months", "TermMonths", "int"),
    "extension": ("extension", "Extension", {"yes", "no", "possible"}),
    "sponsorship": ("sponsorship", "Sponsorship", {"offered", "not-offered", "unknown"}),
    "pay_min": ("pay_min", "PayMin", "number"),
    "pay_max": ("pay_max", "PayMax", "number"),
    "pay_unit": ("pay_unit", "PayUnit", {"hour", "day", "week", "month", "year"}),
    "currency": ("currency", "Currency", "currency"),
    "closes_unclear": ("closes_unclear", "ClosesUnclear", "bool"),
    "jd_completeness": ("jd_completeness", "JDCompleteness", {"full", "summary", "blurb"}),
    "added_by": ("added_by", "AddedBy", {"sweep", "agent", "owner"}),
    "search_query": ("search_query", "SearchQuery", "text"),
    "listing": ("listing", "Listing", {"live", "gone", "closed"}),
    "listing_checked": ("listing_checked", "ListingChecked", "date"),
}
_FACT_DEFAULTS = {"text": "", "int": None, "number": None, "bool": False, "date": "",
                  "currency": ""}


def fact_value(key, value):
    """The typed value of an ad fact, or raise MetaError naming the key."""
    _, _, kind = FACTS[key]
    if value == "" and kind != "bool":
        return _FACT_DEFAULTS.get(kind, "") if not isinstance(kind, set) else ""
    if isinstance(kind, set):
        if value not in kind:
            raise MetaError(f"invalid {key}")
        return value
    if kind == "int":
        number = whole(value)
        if number is None or number < 1:
            raise MetaError(f"invalid {key}")
        return number
    if kind == "number":
        if not NUMBER.fullmatch(value):
            raise MetaError(f"invalid {key}")
        number = float(value)
        return int(number) if number.is_integer() else number
    if kind == "bool":
        if value not in ("true", "false"):
            raise MetaError(f"invalid {key}")
        return value == "true"
    if kind == "date":
        if not valid_date(value):
            raise MetaError(f"invalid {key}")
        return value
    if kind == "currency":
        if not re.fullmatch(r"[A-Z]{3}", value):
            raise MetaError(f"invalid {key}")
    return value


class StatusEntry:
    def __init__(self, status, at, approx=False):
        self.status = status
        self.at = at
        self.approx = approx

    def to_dict(self):
        return {"Status": self.status, "At": self.at, "Approx": self.approx}


class Application:
    """One Application folder, with the values readers derive from it."""

    def __init__(self, folder):
        self.folder = folder
        self.title = ""
        self.company = ""
        self.track = ""
        self.status = "Lead"
        self.documents = ""
        self.pdfs = []
        self.has_jd = False
        self.has_notes = False
        self.has_fit = False
        self.has_gaps = False
        self.has_resume = False
        self.has_cover_letter = False
        self.has_sent = False
        self.schema = 1
        self.url = ""
        self.closes = ""
        self.fit = ""
        self.resume_only = False
        self.posted = ""
        self.posted_label = ""
        self.applicants = None
        self.source = ""
        self.filter_reason = ""
        self.filter_note = ""
        self.filter_overruled = False
        self.followed_up = ""
        self.next_step = None
        self.contacts = []
        self.close_reason = ""
        for attribute, _, kind in FACTS.values():
            setattr(self, attribute, "" if isinstance(kind, set) else _FACT_DEFAULTS[kind])
        self.status_log = []
        self.applied_date = None
        self.last_change = None
        self.unknown = {}
        self.malformed = None

    def to_dict(self):
        """The Go store's JSON shape, plus the malformed error for readers."""
        return {
            "Folder": str(self.folder),
            "Title": self.title,
            "Company": self.company,
            "Track": self.track,
            "Status": self.status,
            "Documents": self.documents,
            "PDFs": list(self.pdfs),
            "HasJD": self.has_jd,
            "HasNotes": self.has_notes,
            "HasFit": self.has_fit,
            "HasGaps": self.has_gaps,
            "HasResume": self.has_resume,
            "HasCoverLetter": self.has_cover_letter,
            "HasSent": self.has_sent,
            "Schema": self.schema,
            "URL": self.url,
            "Closes": self.closes,
            "Fit": self.fit,
            "ResumeOnly": self.resume_only,
            "Posted": self.posted,
            "PostedLabel": self.posted_label,
            "Applicants": self.applicants,
            "Source": self.source,
            "FilterReason": self.filter_reason,
            "FilterNote": self.filter_note,
            "FilterOverruled": self.filter_overruled,
            "FollowedUp": self.followed_up,
            "NextStep": self.next_step,
            "Contacts": list(self.contacts),
            "CloseReason": self.close_reason,
            **{name: getattr(self, attribute) for attribute, name, _ in FACTS.values()},
            "StatusLog": [entry.to_dict() for entry in self.status_log],
            "AppliedDate": self.applied_date.to_dict() if self.applied_date else None,
            "LastChange": self.last_change.to_dict() if self.last_change else None,
            "Malformed": self.malformed is not None,
            "Error": self.malformed or "",
        }


LEGACY_CONTACT = "contact"


def _read_next_step(raw):
    """next_step's value: ``{"At", "What"}`` (the Go store's shape), or None when empty."""
    if blank(raw):
        return None
    pairs, _ = flow_map(raw)
    at = pairs.get("at", "")
    if not valid_next_step_at(at):
        raise MetaError("invalid next_step at (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
    return {"At": at, "What": pairs.get("what", "")}


def _read_contact(line):
    """One ``  - {name: …}`` line of ``contacts`` as ``{"Name", "Role", "Email", "URL"}``."""
    if not line.startswith("  - "):
        raise MetaError("invalid contacts entry or unexpected nesting")
    pairs, _ = flow_map(line[4:])
    contact = {
        "Name": pairs.get("name", ""),
        "Role": pairs.get("role", ""),
        "Email": pairs.get("email", ""),
        "URL": pairs.get("url", ""),
    }
    if not (contact["Name"] or contact["Email"] or contact["URL"]):
        raise MetaError("empty contacts entry")
    return contact


def _legacy_contact(value):
    """The dashboard era's ``contact: Name`` as one Contact, or None when blank."""
    value = value.strip()
    if not value:
        return None
    contact = {"Name": "", "Role": "", "Email": "", "URL": ""}
    if value.startswith(("http://", "https://")):
        contact["URL"] = value
    elif "@" in value and not re.search(r"[ \t]", value):
        contact["Email"] = value
    else:
        contact["Name"] = value
    return contact


def read_meta(folder, data):
    app = Application(folder)
    meta_path = Path(folder) / "meta.yml"

    def fail(line, message):
        app.malformed = f"{meta_path}:{line}: {message}"
        return app

    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return fail(1, "expected UTF-8, LF and trailing newline")
    if "\r" in text or not text.endswith("\n"):
        return fail(1, "expected UTF-8, LF and trailing newline")

    seen = set()
    in_log = False
    in_contacts = False
    legacy = ""
    for number, line in enumerate(text[:-1].split("\n"), start=1):
        if blank(line) or line.startswith("#"):
            continue
        if line.startswith((" ", "\t")):
            if in_contacts:
                try:
                    app.contacts.append(_read_contact(line))
                except MetaError as error:
                    return fail(number, str(error))
                continue
            match = ENTRY.match(line)
            if not in_log or match is None:
                return fail(number, "invalid status_log entry or unexpected nesting")
            status, at, _, approx = match.group(1), match.group(2), match.group(3), match.group(4)
            if status not in STATUSES:
                return fail(number, f"unknown Status {status!r}")
            if not valid_timestamp(at):
                return fail(number, "invalid status timestamp")
            entry = StatusEntry(status, at, approx == "true")
            app.status_log.append(entry)
            app.status = entry.status
            app.last_change = entry
            if entry.status == "Applied" and app.applied_date is None:
                app.applied_date = entry
            continue

        key, separator, raw = line.partition(":")
        if not separator or not KEY.fullmatch(key) or key in seen:
            return fail(number, "invalid or duplicate key")
        seen.add(key)
        in_log = key == "status_log"
        in_contacts = key == "contacts"
        if in_contacts:
            stripped = raw.strip(" \t")
            if stripped and stripped != "[]":
                return fail(number, "invalid contacts")
            in_contacts = not stripped
            continue
        if key == "next_step":
            try:
                app.next_step = _read_next_step(raw)
            except MetaError as error:
                return fail(number, str(error))
            continue
        if in_log:
            stripped = raw.strip(" \t")
            if stripped and stripped != "[]":
                return fail(number, "invalid status_log")
            if stripped == "[]":
                in_log = False
            continue
        try:
            value = scalar(raw)
        except MetaError as error:
            return fail(number, str(error))
        if key == "schema":
            version = whole(value)
            if version is None:
                return fail(number, "unsupported schema")
            if version > 1:
                return fail(
                    number,
                    f"schema {version} is newer than this version; upgrade rolecall",
                )
            if version != 1:
                return fail(number, "unsupported schema")
        elif key == "title":
            app.title = value
        elif key == "company":
            app.company = value
        elif key == "track":
            app.track = value
        elif key == "url":
            app.url = value
        elif key == "closes":
            if value and not DATE.fullmatch(value):
                return fail(number, "invalid closes date")
            if value:
                try:
                    datetime.strptime(value, "%Y-%m-%d")
                except ValueError:
                    return fail(number, "invalid closes date")
            app.closes = value
        elif key == "posted":
            if value and not valid_date(value):
                return fail(number, "invalid posted date")
            app.posted = value
        elif key == "posted_label":
            app.posted_label = value
        elif key == "applicants":
            applicants = whole(value) if value else None
            if value and applicants is None:
                return fail(number, "invalid applicants")
            app.applicants = applicants
        elif key == "source":
            app.source = value
        elif key == "filter_reason":
            if value and value not in FILTER_REASONS:
                return fail(number, "invalid filter_reason")
            app.filter_reason = value
        elif key == "filter_note":
            app.filter_note = value
        elif key == LEGACY_CONTACT:
            legacy = value
            app.unknown[key] = value
        elif key == "close_reason":
            app.close_reason = value.strip()
        elif key == "followed_up":
            # Only times a reminder: a value that is not a date reads as unset, as in the
            # Go store, rather than making the file malformed. The line itself is kept.
            app.followed_up = value if valid_date(value) else ""
        elif key == "filter_overruled":
            if value not in ("true", "false"):
                return fail(number, "invalid filter_overruled")
            app.filter_overruled = value == "true"
        elif key in FACTS:
            try:
                setattr(app, FACTS[key][0], fact_value(key, value))
            except MetaError as error:
                return fail(number, str(error))
        elif key == "fit":
            if value not in FITS:
                return fail(number, "invalid Fit")
            app.fit = value
        elif key == "resume_only":
            if value not in ("true", "false"):
                return fail(number, "invalid resume_only")
            app.resume_only = value == "true"
        else:
            app.unknown[key] = value

    for required in ("title", "company", "track"):
        if required not in seen:
            return fail(1, f"missing {required}")
    # The dashboard era's single ``contact`` still reads, as the one Contact.
    old = _legacy_contact(legacy)
    if old is not None and "contacts" not in seen:
        app.contacts = [old]
    return app


def _read_folder(app):
    folder = Path(app.folder)
    flags = (
        ("jd.md", "has_jd", False),
        ("notes.md", "has_notes", False),
        ("fit.md", "has_fit", False),
        ("gaps.md", "has_gaps", False),
        ("resume.md", "has_resume", False),
        ("cover-letter.md", "has_cover_letter", False),
        ("sent", "has_sent", True),
    )
    for name, attribute, is_dir in flags:
        path = folder / name
        if not path.exists() and not path.is_symlink():
            continue
        setattr(app, attribute, path.is_dir() == is_dir)

    out = folder / "out"
    try:
        entries = sorted(out.iterdir(), key=lambda entry: os.fsencode(entry.name))
    except FileNotFoundError:
        entries = []
    resume = letter = False
    for entry in entries:
        if entry.is_dir() or entry.suffix != ".pdf":
            continue
        app.pdfs.append(entry.name)
        resume = resume or "Resume" in entry.name
        letter = letter or "Cover Letter" in entry.name
    if resume and (letter or app.resume_only):
        app.documents = "Ready"
        return
    if app.has_resume:
        try:
            resume_text = (folder / "resume.md").read_text()
        except OSError:
            resume_text = ""
        if resume_text.strip():
            app.documents = "Written"


def rescan(folder):
    """Read one folder; None when it is not an Application."""
    meta_path = Path(folder) / "meta.yml"
    try:
        data = meta_path.read_bytes()
    except FileNotFoundError:
        return None
    app = read_meta(Path(folder), data)
    _read_folder(app)
    return app


def scan(workspace):
    """Return every Application under ``jobs/<track>/<folder>``, sorted."""
    workspace = Path(workspace)
    root = workspace / "jobs"
    if not root.is_dir():
        return []
    applications = []
    for track in sorted(root.iterdir(), key=lambda entry: os.fsencode(entry.name)):
        if not track.is_dir() or track.name == "_archive":
            continue
        for folder in sorted(track.iterdir(), key=lambda entry: os.fsencode(entry.name)):
            if not folder.is_dir():
                continue
            app = rescan(folder)
            if app is not None:
                applications.append(app)
    return applications
