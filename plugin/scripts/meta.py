#!/usr/bin/env python3
"""Read Applications from a Workspace, mirroring the Go store.

The read side of SCHEMA.md: no YAML library, stock Python 3. A folder is an
Application if and only if it holds ``meta.yml``. A malformed file is returned
as an Application carrying the error, never as an exception that stops the scan.

Usage:  python3 meta.py [--data <workspace>]
"""

import json
import os
import re
import sys
from datetime import datetime
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from workspace import workspace_from_cli  # noqa: E402

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
            "StatusLog": [entry.to_dict() for entry in self.status_log],
            "AppliedDate": self.applied_date.to_dict() if self.applied_date else None,
            "LastChange": self.last_change.to_dict() if self.last_change else None,
            "Malformed": self.malformed is not None,
            "Error": self.malformed or "",
        }


def _scalar_token(raw):
    """Split a scalar from its exact whitespace/comment suffix."""
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
    return raw[start:end]


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
            if value != "1":
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


def main():
    workspace = workspace_from_cli()
    json.dump(
        [app.to_dict() for app in scan(workspace)],
        sys.stdout,
        ensure_ascii=False,
        indent=2,
    )
    print()


if __name__ == "__main__":
    main()
