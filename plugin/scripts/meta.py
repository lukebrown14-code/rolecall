#!/usr/bin/env python3
"""Read Applications from a Workspace, mirroring the Go store.

The read side of SCHEMA.md: no YAML library, stock Python 3. A folder is an
Application if and only if it holds ``meta.yml``. A malformed file is returned
as an Application carrying the error, never as an exception that stops the scan.

Usage:  python3 meta.py [--data <workspace>]
"""

import argparse
import json
import os
import re
import shutil
import sys
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from workspace import add_data_argument, workspace_from_cli  # noqa: E402
from workspaceio import atomic_write, quote_scalar, unique_folder  # noqa: E402

KEY = re.compile(r"^[A-Za-z_][A-Za-z0-9_-]*$")
ENTRY = re.compile(
    r"^  - \{status: ([A-Za-z]+), at: ([^,}]+)(, approx: (true|false))?\}"
    r"([ \t]+#.*)?$"
)
DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
RFC3339 = re.compile(
    r"^(\d{4})-(\d{2})-(\d{2})T(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?"
    r"(Z|([+-])(\d{2}):(\d{2}))$"
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
        if not re.fullmatch(r"[0-9]+", value) or int(value) < 1:
            raise MetaError(f"invalid {key}")
        return int(value)
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
        if not _valid_date(value):
            raise MetaError(f"invalid {key}")
        return value
    if kind == "currency":
        if not re.fullmatch(r"[A-Z]{3}", value):
            raise MetaError(f"invalid {key}")
    return value
ESCAPES = {
    "0": "\x00",
    "a": "\x07",
    "b": "\x08",
    "t": "\x09",
    "n": "\x0a",
    "v": "\x0b",
    "f": "\x0c",
    "r": "\x0d",
    "e": "\x1b",
    " ": " ",
    '"': '"',
    "/": "/",
    "\\": "\\",
    "N": "\x85",
    "_": "\xa0",
    "L": "\u2028",
    "P": "\u2029",
}


class MetaError(Exception):
    """A meta.yml line that breaks the SCHEMA.md format."""


class FolderError(Exception):
    """A folder argument does not name exactly one Application."""


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
            **{name: getattr(self, attribute) for attribute, name, _ in FACTS.values()},
            "StatusLog": [entry.to_dict() for entry in self.status_log],
            "AppliedDate": self.applied_date.to_dict() if self.applied_date else None,
            "LastChange": self.last_change.to_dict() if self.last_change else None,
            "Malformed": self.malformed is not None,
            "Error": self.malformed or "",
        }


def _scalar_parts(raw):
    """Split a scalar into its token and exact whitespace/comment suffix."""
    start = len(raw) - len(raw.lstrip(" \t"))
    end = len(raw)
    if start < len(raw) and raw[start] == '"':
        end = -1
        i = start + 1
        while i < len(raw):
            if raw[i] == "\\":
                i += 2
                continue
            if raw[i] == '"':
                end = i + 1
                break
            i += 1
        if end < 0:
            raise MetaError("invalid quoted scalar")
        tail = raw[end:].lstrip(" \t")
        if tail and not tail.startswith("#"):
            raise MetaError("invalid quoted scalar suffix")
    else:
        i = start
        while i < len(raw):
            if raw[i] == "#" and (i == start or raw[i - 1] in " \t"):
                end = i
                break
            i += 1
        end = len(raw[:end].rstrip(" \t"))
    if end < start:
        end = start
    return raw[start:end], raw[end:]


def _scalar_token(raw):
    return _scalar_parts(raw)[0]


def _decode_quoted(token):
    out = []
    i = 1
    while i < len(token) - 1:
        if token[i] != "\\":
            out.append(token[i])
            i += 1
            continue
        i += 1
        if i >= len(token) - 1:
            raise MetaError("short YAML escape")
        c = token[i]
        if c in ESCAPES:
            out.append(ESCAPES[c])
            i += 1
            continue
        size = {"x": 2, "u": 4, "U": 8}.get(c)
        if size is None:
            raise MetaError("invalid YAML escape")
        if i + size >= len(token) - 1:
            raise MetaError("short YAML escape")
        digits = token[i + 1 : i + 1 + size]
        if not re.fullmatch(r"[0-9A-Fa-f]+", digits):
            raise MetaError("invalid Unicode escape")
        value = int(digits, 16)
        if value > 0x10FFFF or 0xD800 <= value <= 0xDFFF:
            raise MetaError("invalid Unicode escape")
        out.append(chr(value))
        i += size + 1
    return "".join(out)


def _scalar(raw):
    token = _scalar_token(raw)
    if token.startswith('"'):
        return _decode_quoted(token)
    if (
        "\t" in token
        or "\r" in token
        or token.startswith(("{", "[", "'"))
        or token in ("|", ">")
        or ": " in token
    ):
        raise MetaError("invalid scalar")
    return token


def _valid_date(value):
    if not DATE.fullmatch(value):
        return False
    try:
        datetime.strptime(value, "%Y-%m-%d")
    except ValueError:
        return False
    return True


def _valid_timestamp(value):
    if DATE.fullmatch(value):
        try:
            datetime.strptime(value, "%Y-%m-%d")
        except ValueError:
            return False
        return True
    match = RFC3339.fullmatch(value)
    if match is None:
        return False
    year, month, day, hour, minute, second = (
        int(match.group(index)) for index in range(1, 7)
    )
    try:
        datetime(year, month, day)
    except ValueError:
        return False
    if hour > 23 or minute > 59 or second > 59:
        return False
    if match.group(8):
        offset_hour, offset_minute = int(match.group(9)), int(match.group(10))
        if offset_hour > 23 or offset_minute > 59:
            return False
    return True


def _read_meta(folder, data):
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
    for number, line in enumerate(text[:-1].split("\n"), start=1):
        if not line.strip() or line.startswith("#"):
            continue
        if line.startswith((" ", "\t")):
            match = ENTRY.match(line)
            if not in_log or match is None:
                return fail(number, "invalid status_log entry or unexpected nesting")
            status, at, _, approx = match.group(1), match.group(2), match.group(3), match.group(4)
            if status not in STATUSES:
                return fail(number, f"unknown Status {status!r}")
            if not _valid_timestamp(at):
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
        if in_log:
            stripped = raw.strip()
            if stripped and stripped != "[]":
                return fail(number, "invalid status_log")
            if stripped == "[]":
                in_log = False
            continue
        try:
            value = _scalar(raw)
        except MetaError as error:
            return fail(number, str(error))
        if key == "schema":
            if not re.fullmatch(r"[0-9]+", value):
                return fail(number, "unsupported schema")
            version = int(value)
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
            if value and not _valid_date(value):
                return fail(number, "invalid posted date")
            app.posted = value
        elif key == "posted_label":
            app.posted_label = value
        elif key == "applicants":
            if value and not re.fullmatch(r"[0-9]+", value):
                return fail(number, "invalid applicants")
            app.applicants = int(value) if value else None
        elif key == "source":
            app.source = value
        elif key == "filter_reason":
            if value and value not in FILTER_REASONS:
                return fail(number, "invalid filter_reason")
            app.filter_reason = value
        elif key == "filter_note":
            app.filter_note = value
        elif key == "followed_up":
            # Only times a reminder: a value that is not a date reads as unset, as in the
            # Go store, rather than making the file malformed. The line itself is kept.
            app.followed_up = value if _valid_date(value) else ""
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
    app = _read_meta(Path(folder), data)
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


def _parse_moment(value):
    """An aware datetime from a datetime, YYYY-MM-DD or RFC 3339 string."""
    if isinstance(value, datetime):
        return value
    text = str(value).strip()
    if DATE.fullmatch(text):
        return datetime.strptime(text, "%Y-%m-%d").replace(tzinfo=timezone.utc)
    match = RFC3339.fullmatch(text)
    if match is None:
        raise MetaError(f"invalid timestamp {value!r}")
    year, month, day, hour, minute, second = (
        int(match.group(index)) for index in range(1, 7)
    )
    if match.group(7) == "Z":
        zone = timezone.utc
    else:
        offset = timedelta(hours=int(match.group(9)), minutes=int(match.group(10)))
        zone = timezone(-offset if match.group(8) == "-" else offset)
    return datetime(year, month, day, hour, minute, second, tzinfo=zone)


# The TUI's default follow_up_days. The setting lives in the TUI's config.toml, which the
# plugin does not read, so a caller passes the Owner's value when they changed it.
DEFAULT_FOLLOW_UP_DAYS = 14
FOLLOW_UP_STATUSES = ("Applied", "Interviewing")


def _day(value):
    """The day of a YYYY-MM-DD date or the date part of a timestamp, as written; or None."""
    text = str(value or "").split("T")[0]
    return datetime.strptime(text, "%Y-%m-%d").date() if _valid_date(text) else None


def follow_up_from(app):
    """The day Follow-up due counts from, and what it is, mirroring the Go TUI.

    It is the later of the last Status change and ``followed_up`` (a follow-up the Owner
    recorded with ``l``); on the same day the Status change wins. Returns
    ``(day, "followed_up" | "last change")``, or ``(None, "")`` when neither reads.
    """
    changed = _day(app.last_change.at) if app.last_change else None
    followed = _day(app.followed_up)
    if followed is not None and (changed is None or followed > changed):
        return followed, "followed_up"
    if changed is not None:
        return changed, "last change"
    return None, ""


def follow_up_status(app, today=None, days=DEFAULT_FOLLOW_UP_DAYS):
    """Follow-up due for one Application, as the TUI's ⚑ flag reckons it.

    Only Applied and Interviewing can be due. The Application is due ``days`` after
    ``follow_up_from``; ``days_past`` is whole days past that (0 on the day, negative
    before it).
    """
    today = today or date.today()
    start, since = follow_up_from(app)
    result = {
        "Status": app.status,
        "FollowedUp": app.followed_up,
        "From": start.isoformat() if start else "",
        "Since": since,
        "Days": days,
        "DueOn": "",
        "DaysPast": None,
        "Due": False,
    }
    if app.malformed or app.status not in FOLLOW_UP_STATUSES or start is None:
        return result
    due_on = start + timedelta(days=days)
    result["DueOn"] = due_on.isoformat()
    result["DaysPast"] = (today - due_on).days
    result["Due"] = result["DaysPast"] >= 0
    return result


def _utc_stamp(value):
    moment = _parse_moment(value)
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _update(folder, change):
    """Re-read, validate, apply ``change`` to the lines, validate, write."""
    path = Path(folder) / "meta.yml"
    data = path.read_bytes()
    application = _read_meta(Path(folder), data)
    if application.malformed:
        raise MetaError(application.malformed)
    lines = data.decode("utf-8")[:-1].split("\n")
    lines = change(lines)
    out = ("\n".join(lines) + "\n").encode("utf-8")
    checked = _read_meta(Path(folder), out)
    if checked.malformed:
        raise MetaError(checked.malformed)
    if out == data:
        return
    atomic_write(path, out, path.stat().st_mode & 0o777)


def _patch_lines(lines, keys):
    """Set scalar keys in place, keeping every other byte and inline comment."""
    for name in sorted(keys):
        if not KEY.fullmatch(name) or name == "status_log":
            raise MetaError(f"invalid scalar key {name!r}")
        value = quote_scalar(keys[name])
        found = False
        for index, line in enumerate(lines):
            if line.startswith(name + ":"):
                _, suffix = _scalar_parts(line[len(name) + 1 :])
                lines[index] = name + ": " + value + suffix
                found = True
                break
        if not found:
            index = len(lines)
            for position, line in enumerate(lines):
                if line.startswith("status_log:"):
                    index = position
                    break
            lines.insert(index, name + ": " + value)
    return lines


def patch(folder, keys):
    """Set scalar keys, keeping every other byte and inline comment."""
    _update(folder, lambda lines: _patch_lines(lines, keys))


def update(folder, keys, status_log):
    """Patch scalar keys and replace the status_log in one read-modify-validate-write.

    Migration's single write path: one validation and one atomic publication per
    Application, rather than a ``patch`` followed by a second ``set_status_log``.
    ``status_log`` of None leaves the existing log untouched.
    """
    entries = (
        None
        if status_log is None
        else [
            _entry_line(status, at, approx)
            for status, at, approx in dedupe_status_log(status_log)
        ]
    )

    def change(lines):
        lines = _patch_lines(lines, keys)
        return lines if entries is None else _set_status_log_lines(lines, entries)

    _update(folder, change)


def append_status(folder, status, at, only_if_current=None):
    """Append one Status entry, unless it repeats the current Status.

    ``only_if_current`` makes the append conditional on the current Status,
    re-read inside the writer at write time (not during an earlier scan).
    Returns ``True`` when an entry was appended, ``False`` otherwise.
    """
    if status not in STATUSES:
        raise MetaError(f"invalid Status {status!r}")
    if only_if_current is not None and only_if_current not in STATUSES:
        raise MetaError(f"invalid Status {only_if_current!r}")
    stamp = _utc_stamp(at)
    appended = False

    def change(lines):
        nonlocal appended
        current = _read_meta(Path(folder), ("\n".join(lines) + "\n").encode("utf-8"))
        if current.malformed:
            raise MetaError(current.malformed)
        if only_if_current is not None and current.status != only_if_current:
            return lines
        if current.status == status:
            return lines
        index = -1
        for position, line in enumerate(lines):
            if line.startswith("status_log:"):
                index = position
                break
        if index < 0:
            lines.append("status_log:")
            index = len(lines) - 1
        else:
            lines[index] = "status_log:"
        end = index + 1
        while end < len(lines) and (
            lines[end].startswith("  ")
            or not lines[end].strip()
            or lines[end].startswith("#")
        ):
            end += 1
        lines.insert(end, f"  - {{status: {status}, at: {stamp}}}")
        appended = True
        return lines

    _update(folder, change)
    return appended


def dismiss_by_rule(folder, reason, note, at=None):
    """Dismiss an Application because a Search rule rules it out.

    One read-modify-validate-write: ``filter_reason`` and ``filter_note`` are set and a
    ``Dismissed`` entry appended. An Application the Owner restored after a rule
    (``filter_overruled: true``), or one already Dismissed, is left alone. Returns True
    when it was written.
    """
    if reason not in FILTER_REASONS:
        raise MetaError(f"invalid filter_reason {reason!r}")
    stamp = _utc_stamp(at or datetime.now(timezone.utc))
    written = False

    def change(lines):
        nonlocal written
        current = _read_meta(Path(folder), ("\n".join(lines) + "\n").encode("utf-8"))
        if current.malformed:
            raise MetaError(current.malformed)
        if current.filter_overruled or current.status == "Dismissed":
            return lines
        lines = _patch_lines(lines, {"filter_reason": reason, "filter_note": note})
        index = next((i for i, line in enumerate(lines) if line.startswith("status_log:")), -1)
        if index < 0:
            lines.append("status_log:")
            index = len(lines) - 1
        else:
            lines[index] = "status_log:"
        end = index + 1
        while end < len(lines) and (
            lines[end].startswith("  ") or not lines[end].strip() or lines[end].startswith("#")
        ):
            end += 1
        lines.insert(end, f"  - {{status: Dismissed, at: {stamp}}}")
        written = True
        return lines

    _update(folder, change)
    return written


def rule_findings(workspace, fields, today=None):
    """The Search rules a new Lead with these fields breaks (rules.py), as Findings."""
    import rules  # noqa: PLC0415 -- rules imports meta lazily; keep the cycle out of import time

    applicants = fields.get("applicants")
    found = rules.evaluate(
        rules.load_rules(workspace),
        source=fields.get("source", ""),
        posted=fields.get("posted", ""),
        posted_label=fields.get("posted_label", ""),
        applicants=int(applicants) if str(applicants or "").isdigit() else None,
        today=today,
    )
    return [f for f in found if not f.advisory]


def duplicates_of(workspace, fields, jd_text=""):
    """Saved Applications that look like the role in ``fields`` (adfacts.likely_duplicates)."""
    import adfacts  # noqa: PLC0415

    return adfacts.likely_duplicates(
        scan(workspace), fields.get("company", ""), fields.get("title", ""), jd_text,
        fields.get("recruiter", ""),
    )


def dedupe_status_log(status_log):
    """``status_log`` with exact repeats removed: entries with the same Status and
    ``at`` collapse into one.

    The **last** occurrence is kept, so the current Status (the last entry), the
    Applied date (a repeat shares its ``at``) and the last change all read the
    same as before. Entries are ``(status, at, approx)`` tuples.
    """
    last = {(status, str(at)): index for index, (status, at, _) in enumerate(status_log)}
    return [
        entry
        for index, entry in enumerate(status_log)
        if last[(entry[0], str(entry[1]))] == index
    ]


def duplicate_status_entries(status_log):
    """The ``(status, at)`` pairs that appear more than once in a read log."""
    seen, repeated = set(), []
    for entry in status_log:
        key = (entry.status, entry.at)
        if key in seen and key not in repeated:
            repeated.append(key)
        seen.add(key)
    return repeated


def _entry_line(status, at, approx):
    if status not in STATUSES:
        raise MetaError(f"invalid Status {status!r}")
    if not _valid_timestamp(str(at)):
        raise MetaError(f"invalid status timestamp {at!r}")
    line = f"  - {{status: {status}, at: {at}"
    if approx:
        line += ", approx: true"
    return line + "}"


def _meta_text(fields, status_log):
    """Render a complete meta.yml, validating every Status entry."""
    text = "schema: 1\n"
    for key in CREATE_KEYS:
        value = fields.get(key, "")
        if value is not None and str(value) != "":
            text += key + ": " + quote_scalar(str(value)) + "\n"
    text += "status_log:\n"
    for status, at, approx in dedupe_status_log(status_log):
        text += _entry_line(status, at, approx) + "\n"
    checked = _read_meta("", text.encode("utf-8"))
    if checked.malformed:
        raise MetaError(checked.malformed)
    return text


# The scalar keys create writes, in file order.
CREATE_KEYS = (
    "title", "company", "track", "url", "closes", "fit",
    "posted", "posted_label", "applicants", "source", *FACTS,
)


def _required_fields(fields):
    title = fields.get("title", "")
    company = fields.get("company", "")
    track = fields.get("track", "")
    if (
        not title.strip()
        or not company.strip()
        or not track
        or track in (".", "..", "_archive")
        or "/" in track
        or "\\" in track
    ):
        raise MetaError("Title, Company and a single Track folder are required")


def resolve_folder(workspace, value):
    """The Application folder named by ``value`` under ``workspace``.

    An absolute path is taken as given. A relative path resolves against the
    Workspace, then ``jobs/``; a bare folder name also resolves to the one
    track that holds it. An ambiguous name is refused rather than guessed.
    """
    raw = Path(value).expanduser()
    if raw.is_absolute():
        return raw
    workspace = Path(workspace)
    jobs = workspace / "jobs"
    candidates = []
    for base in (workspace, jobs):
        candidate = base / raw
        if candidate.is_dir():
            candidates.append(candidate.resolve())
    if len(raw.parts) == 1 and jobs.is_dir():
        for track in sorted(jobs.iterdir()):
            candidate = track / raw
            if track.is_dir() and candidate.is_dir():
                candidates.append(candidate.resolve())
    unique = sorted(set(candidates))
    if not unique:
        raise FolderError(f"no Application folder {value!r} under {jobs}")
    if len(unique) > 1:
        listed = "\n  ".join(str(path) for path in unique)
        raise FolderError(f"--folder {value!r} is ambiguous; name one of:\n  {listed}")
    return unique[0]


def create(workspace, fields, now=None, status_log=None):
    """Create a Lead Application and return its folder."""
    _required_fields(fields)
    title = fields.get("title", "")
    company = fields.get("company", "")
    track = fields.get("track", "")
    moment = now if now is not None else datetime.now(timezone.utc)
    if status_log is None:
        status_log = [("Lead", _utc_stamp(moment), False)]
    text = _meta_text(fields, status_log)
    checked = _read_meta("", text.encode("utf-8"))
    if checked.malformed:
        raise MetaError(checked.malformed)

    parent = Path(workspace) / "jobs" / track
    parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    day = moment.strftime("%Y-%m-%d")
    while True:
        folder = unique_folder(parent, company, title, day)
        try:
            folder.mkdir(mode=0o700)
        except FileExistsError:
            continue
        break
    try:
        os.chmod(folder, 0o700)
        (folder / "jd.md").write_bytes(b"")
        os.chmod(folder / "jd.md", 0o600)
        atomic_write(folder / "meta.yml", text.encode("utf-8"), 0o600)
    except BaseException:
        shutil.rmtree(folder, ignore_errors=True)
        raise
    return folder


def create_migrated(folder, fields, status_log):
    """Write a complete meta.yml into an existing folder (migration only)."""
    _required_fields(fields)
    folder = Path(folder)
    if not folder.is_dir():
        raise MetaError(f"no such folder: {folder}")
    text = _meta_text(fields, status_log)
    atomic_write(folder / "meta.yml", text.encode("utf-8"), 0o600)


def _set_status_log_lines(lines, entries):
    """Replace the status_log entries in place, keeping every other byte and comment."""
    index = -1
    for position, line in enumerate(lines):
        if line.startswith("status_log:"):
            index = position
            break
    if index < 0:
        while lines and not lines[-1].strip():
            lines.pop()
        lines.append("status_log:")
        lines.extend(entries)
        return lines
    lines[index] = "status_log:"
    # Keep indented comments inside the block; replace only entry lines.
    kept = []
    end = index + 1
    while end < len(lines) and (
        lines[end].startswith((" ", "\t")) or not lines[end].strip()
    ):
        if lines[end].strip() and not lines[end].lstrip().startswith("- "):
            kept.append(lines[end])
        end += 1
    return lines[: index + 1] + entries + kept + lines[end:]


def set_status_log(folder, status_log):
    """Replace the status_log entries, keeping every other byte and comment.

    Exact repeats (same Status and ``at``) are written once; see
    ``dedupe_status_log``.
    """
    entries = [
        _entry_line(status, at, approx) for status, at, approx in dedupe_status_log(status_log)
    ]
    _update(folder, lambda lines: _set_status_log_lines(lines, entries))


def find_by_url(workspace, url):
    """Every exact URL match, regardless of Status."""
    return [application for application in scan(workspace) if application.url == url]


def _parser():
    parser = argparse.ArgumentParser(description=__doc__)
    add_data_argument(parser)
    commands = parser.add_subparsers(dest="command")

    patch_parser = commands.add_parser("patch", help="set scalar keys")
    patch_parser.add_argument("--folder", required=True)
    patch_parser.add_argument("--set", action="append", default=[], metavar="KEY=VALUE")

    append_parser = commands.add_parser("append-status", help="append a Status entry")
    append_parser.add_argument("--folder", required=True)
    append_parser.add_argument("--status", required=True)
    append_parser.add_argument("--at", help="RFC 3339 or YYYY-MM-DD (default: now)")
    append_parser.add_argument(
        "--only-if-current",
        help="append only when the Status read at write time is this value",
    )

    create_parser = commands.add_parser("create", help="create a Lead Application")
    create_parser.add_argument("--track", required=True)
    create_parser.add_argument("--title", required=True)
    create_parser.add_argument("--company", required=True)
    create_parser.add_argument("--url", default="")
    create_parser.add_argument("--closes", default="")
    create_parser.add_argument("--fit", default="")
    create_parser.add_argument("--at", help="RFC 3339 or YYYY-MM-DD (default: now)")
    create_parser.add_argument("--posted", default="", help="YYYY-MM-DD the ad was posted")
    create_parser.add_argument(
        "--posted-label", default="", help='the age as the site shows it, e.g. "14d ago"'
    )
    create_parser.add_argument("--applicants", default="", help="applicant count the site shows")
    create_parser.add_argument("--source", default="", help="Source registry name, e.g. linkedin")
    create_parser.add_argument(
        "--override-rules",
        action="store_true",
        help="save even though a Search rule rules it out (only when the Owner asks)",
    )

    for key in FACTS:
        create_parser.add_argument("--" + key.replace("_", "-"), dest="fact_" + key, default="")
    create_parser.add_argument(
        "--allow-duplicate",
        action="store_true",
        help="save even though a saved Application looks like the same role",
    )
    create_parser.add_argument("--jd-file", help="ad text, checked for a duplicate ad")

    show_parser = commands.add_parser("show", help="print one Application as JSON")
    show_parser.add_argument("--folder", required=True)

    follow_parser = commands.add_parser(
        "follow-up", help="print when one Application is Follow-up due, as JSON"
    )
    follow_parser.add_argument("--folder", required=True)
    follow_parser.add_argument(
        "--days",
        type=int,
        default=DEFAULT_FOLLOW_UP_DAYS,
        help=f"the Owner's follow_up_days (default {DEFAULT_FOLLOW_UP_DAYS})",
    )

    dup_parser = commands.add_parser("find-duplicates", help="saved Applications like this role")
    dup_parser.add_argument("--company", required=True)
    dup_parser.add_argument("--title", required=True)
    dup_parser.add_argument("--recruiter", default="")
    dup_parser.add_argument("--jd-file")

    dismiss_parser = commands.add_parser(
        "dismiss-by-rule", help="dismiss an Application a Search rule rules out"
    )
    dismiss_parser.add_argument("--folder", required=True)
    dismiss_parser.add_argument("--reason", required=True, choices=sorted(FILTER_REASONS))
    dismiss_parser.add_argument("--note", default="")

    find_parser = commands.add_parser("find-url", help="find Applications by URL")
    find_parser.add_argument("url")
    return parser


def main(argv=None):
    args = _parser().parse_args(argv)
    workspace = workspace_from_cli(args.data)
    try:
        if args.command is None:
            json.dump(
                [app.to_dict() for app in scan(workspace)],
                sys.stdout,
                ensure_ascii=False,
                indent=2,
            )
            print()
        elif args.command == "patch":
            keys = {}
            for setting in args.set:
                name, separator, value = setting.partition("=")
                if not separator:
                    raise MetaError("--set needs KEY=VALUE")
                keys[name] = value
            patch(args.folder, keys)
        elif args.command == "append-status":
            append_status(
                args.folder,
                args.status,
                args.at or datetime.now(timezone.utc),
                only_if_current=args.only_if_current,
            )
        elif args.command == "create":
            fields = {
                "title": args.title,
                "company": args.company,
                "track": args.track,
                "url": args.url,
                "closes": args.closes,
                "fit": args.fit,
                "posted": args.posted,
                "posted_label": args.posted_label,
                "applicants": args.applicants,
                "source": args.source,
                **{key: getattr(args, "fact_" + key) for key in FACTS},
            }
            for key in FACTS:
                if fields[key]:
                    fact_value(key, fields[key])  # refuse a bad value before any write
            if fields["closes"] and fields["closes_unclear"] == "true":
                raise MetaError("give --closes or --closes-unclear true, not both")
            if not args.allow_duplicate:
                jd_text = Path(args.jd_file).read_text() if args.jd_file else ""
                dupes = duplicates_of(workspace, fields, jd_text)
                if dupes:
                    listed = "\n  ".join(f"{app.folder} ({why})" for app, why in dupes)
                    raise MetaError(
                        "not saved, it looks like a saved Application:\n  " + listed +
                        "\nUse that one, or pass --allow-duplicate if it is a different role"
                    )
            if args.posted and not _valid_date(args.posted):
                raise MetaError(f"--posted wants YYYY-MM-DD, not {args.posted!r}")
            if args.applicants and not args.applicants.isdigit():
                raise MetaError(f"--applicants wants a whole number, not {args.applicants!r}")
            if not args.override_rules:
                findings = rule_findings(workspace, fields)
                certain = [f for f in findings if f.certain]
                if certain:
                    why = "; ".join(f"{f.reason}: {f.note}" for f in certain)
                    raise MetaError(
                        f"not saved, a Search rule rules it out ({why}). Leave it out, or pass "
                        "--override-rules if the Owner asked for it anyway"
                    )
                for f in findings:
                    print(f"rolecall: borderline, ask the Owner: {f.note}", file=sys.stderr)
            folder = create(
                workspace, fields, _parse_moment(args.at) if args.at else None
            )
            print(folder)
        elif args.command == "show":
            app = rescan(resolve_folder(workspace, args.folder))
            if app is None:
                raise MetaError(f"no meta.yml in {args.folder}")
            json.dump(app.to_dict(), sys.stdout, ensure_ascii=False, indent=2)
            print()
        elif args.command == "follow-up":
            if not 1 <= args.days <= 90:
                raise MetaError("--days wants follow_up_days, a whole number from 1 to 90")
            app = rescan(resolve_folder(workspace, args.folder))
            if app is None:
                raise MetaError(f"no meta.yml in {args.folder}")
            if app.malformed:
                raise MetaError(app.malformed)
            json.dump(follow_up_status(app, days=args.days), sys.stdout, indent=2)
            print()
        elif args.command == "find-duplicates":
            jd_text = Path(args.jd_file).read_text() if args.jd_file else ""
            found = duplicates_of(
                workspace,
                {"company": args.company, "title": args.title, "recruiter": args.recruiter},
                jd_text,
            )
            json.dump([{"Folder": str(app.folder), "Why": why} for app, why in found],
                      sys.stdout, indent=2)
            print()
        elif args.command == "dismiss-by-rule":
            folder = resolve_folder(workspace, args.folder)
            if not dismiss_by_rule(folder, args.reason, args.note):
                print("rolecall: left as it is (already Dismissed, or the Owner overruled the rules)")
        elif args.command == "find-url":
            json.dump(
                [app.to_dict() for app in find_by_url(workspace, args.url)],
                sys.stdout,
                ensure_ascii=False,
                indent=2,
            )
            print()
    except (MetaError, FolderError, ValueError, OSError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
