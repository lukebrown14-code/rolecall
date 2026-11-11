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
import tempfile
from datetime import datetime, timedelta, timezone
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


def quote_scalar(value):
    """Encode a scalar exactly like the Go store's quoteScalar."""
    out = ['"']
    for character in str(value):
        code = ord(character)
        if character in '"\\':
            out.append("\\" + character)
        elif code < 0x20 or 0x7F <= code <= 0x9F or code in (0x2028, 0x2029):
            out.append(f"\\u{code:04X}")
        else:
            out.append(character)
    out.append('"')
    return "".join(out)


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


def _utc_stamp(value):
    moment = _parse_moment(value)
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def atomic_write(path, data, mode):
    """Publish bytes through a same-folder temporary file, then rename."""
    path = Path(path)
    descriptor, name = tempfile.mkstemp(dir=str(path.parent), prefix=".meta-")
    try:
        with os.fdopen(descriptor, "wb") as handle:
            os.chmod(name, mode)
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(name, path)
    except BaseException:
        try:
            os.unlink(name)
        except FileNotFoundError:
            pass
        raise


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


def patch(folder, keys):
    """Set scalar keys, keeping every other byte and inline comment."""

    def change(lines):
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
    for key in ("title", "company", "track", "url", "closes", "fit"):
        value = fields.get(key, "")
        if value:
            text += key + ": " + quote_scalar(value) + "\n"
    text += "status_log:\n"
    for status, at, approx in status_log:
        text += _entry_line(status, at, approx) + "\n"
    checked = _read_meta("", text.encode("utf-8"))
    if checked.malformed:
        raise MetaError(checked.malformed)
    return text


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


def unique_folder(parent, company, title, day, reserved=None):
    """The first free ``<slug>-<day>`` / ``-2`` / ``-3`` path under ``parent``.

    Mirrors the Go store's slug and collision rule. ``reserved`` lets a planner
    claim names before they exist on disk.
    """
    if reserved is None:
        reserved = set()
    parent = Path(parent)
    slug = re.sub(r"[^a-z0-9]+", "-", (company + "-" + title).lower()).strip("-")
    slug += "-" + day
    number = 1
    while True:
        name = slug if number == 1 else f"{slug}-{number}"
        if name not in reserved and not (parent / name).exists():
            reserved.add(name)
            return parent / name
        number += 1


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


def set_status_log(folder, status_log):
    """Replace the status_log entries, keeping every other byte and comment."""
    entries = [_entry_line(status, at, approx) for status, at, approx in status_log]

    def change(lines):
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

    _update(folder, change)


def find_by_url(workspace, url):
    """Every exact URL match, regardless of Status."""
    return [application for application in scan(workspace) if application.url == url]


def _parser():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", help="Workspace path; or set ROLECALL_WORKSPACE")
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

    find_parser = commands.add_parser("find-url", help="find Applications by URL")
    find_parser.add_argument("url")
    return parser


def main(argv=None):
    args = _parser().parse_args(argv)
    workspace = workspace_from_cli()
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
            folder = create(
                workspace,
                {
                    "title": args.title,
                    "company": args.company,
                    "track": args.track,
                    "url": args.url,
                    "closes": args.closes,
                    "fit": args.fit,
                },
                _parse_moment(args.at) if args.at else None,
            )
            print(folder)
        elif args.command == "find-url":
            json.dump(
                [app.to_dict() for app in find_by_url(workspace, args.url)],
                sys.stdout,
                ensure_ascii=False,
                indent=2,
            )
            print()
    except (MetaError, ValueError, OSError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
