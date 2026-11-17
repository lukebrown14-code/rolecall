#!/usr/bin/env python3
"""Import existing applications from a CSV or TSV export into Application folders.

    python3 plugin/scripts/import_csv.py --data WS --file export.csv --inspect
    python3 plugin/scripts/import_csv.py --data WS --file export.csv --track SWE --dry-run
    python3 plugin/scripts/import_csv.py --data WS --file export.csv --track SWE \
        --map applied="Date Sent" --status-map "Phone screen=Interviewing"

``--inspect`` reads the header and prints the column mapping it detected, the distinct status
values and how each maps, and a few rows, so the Owner can confirm them. ``--dry-run`` prints
the plan and writes nothing. Without it, each row becomes a folder through the shared
``meta.create`` (Lead, Draft, Applied ... as the Owner's Status says), with an approximate
``status_log``: dates the export gives are written date-only with ``approx: true``, the way the
migration writes dates it rebuilt. Nothing is invented: a row without a title or a company is
skipped, a date that cannot be read is left out, and a status the script does not know stops
the import until the Owner maps it with ``--status-map``.
"""

import argparse
import csv
import io
import json
import re
import sys
from datetime import date, datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import adfacts  # noqa: E402
import httpfetch  # noqa: E402
import meta as meta_store  # noqa: E402
from workspace import add_data_argument, load, workspace_from_cli  # noqa: E402
from workspaceio import atomic_write  # noqa: E402

FIELDS = ("title", "company", "status", "applied", "url", "notes", "track")
MAX_BYTES = 5_000_000
MAX_ROWS = 5000

# Header names (lower case, punctuation as spaces) each field is detected from. Only these
# exact names match: a column is never guessed from its contents.
HEADERS = {
    "title": ("title", "job title", "position", "position title", "role", "job", "role title"),
    "company": ("company", "company name", "employer", "organisation", "organization"),
    "status": ("status", "stage", "application status", "state", "outcome"),
    "applied": ("date applied", "applied", "applied date", "applied on", "application date",
                "date of application", "date", "date sent"),
    "url": ("url", "link", "job url", "job link", "posting", "posting url", "ad url", "ad link"),
    "notes": ("notes", "note", "comments", "comment", "remarks"),
    "track": ("track",),
}

# What an Owner's status word means. Anything else is asked about, never guessed.
STATUS_WORDS = {
    "Lead": ("lead", "saved", "wishlist", "wish list", "interested", "to apply", "bookmarked",
             "watching", "not applied"),
    "Draft": ("draft", "preparing", "in progress"),
    "Applied": ("applied", "submitted", "sent", "pending", "awaiting response", "awaiting",
                "waiting"),
    "Interviewing": ("interviewing", "interview", "interviewed", "phone screen", "screening",
                     "screen", "technical", "technical interview", "assessment", "onsite",
                     "on site", "final round", "in process"),
    "Offer": ("offer", "offered", "offer received"),
    "Rejected": ("rejected", "rejection", "unsuccessful", "not selected", "not successful",
                 "turned down"),
    "Withdrawn": ("withdrawn", "withdrew", "cancelled", "canceled"),
    "Dismissed": ("dismissed", "skipped", "not applying", "passed on"),
}
_WORD_TO_STATUS = {word: status for status, words in STATUS_WORDS.items() for word in words}

_MONTHS = {m: i for i, m in enumerate(
    ("jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"), 1)}


SENT = ("Applied", "Interviewing", "Offer", "Rejected", "Withdrawn")


class ImportError_(Exception):
    """A problem with the file or the options, reported as one line."""


def _norm(text):
    return re.sub(r"[^a-z0-9]+", " ", str(text).lower()).strip()


def _one_line(value):
    return re.sub(r"\s+", " ", str(value or "")).strip()


# ----------------------------------------------------------------- reading


def read_table(path):
    """(headers, rows) of a CSV or TSV file; each row is a list of strings."""
    path = Path(path).expanduser()
    if path.suffix.lower() in (".xlsx", ".xls", ".numbers", ".ods"):
        raise ImportError_(f"{path.name} is a spreadsheet file: export it as CSV and import that")
    try:
        if path.stat().st_size > MAX_BYTES:
            raise ImportError_(f"{path.name} is over {MAX_BYTES // 1_000_000} MB: not a job list")
        raw = path.read_bytes()
    except OSError as error:
        raise ImportError_(f"cannot read {path}: {error.strerror or error}") from error
    text = raw.decode("utf-8-sig", errors="replace")
    sample = text[:4096]
    delimiter = max(",\t;", key=lambda d: sample.splitlines()[0].count(d) if sample else 0)
    try:
        table = list(csv.reader(io.StringIO(text, newline=""), delimiter=delimiter))
    except csv.Error as error:
        raise ImportError_(f"{path.name} is not readable as CSV: {error}") from error
    table = [row for row in table if any(cell.strip() for cell in row)]
    if len(table) < 2:
        raise ImportError_(f"{path.name} has no data rows under its header")
    if len(table) - 1 > MAX_ROWS:
        raise ImportError_(f"{path.name} has over {MAX_ROWS} rows: split it first")
    headers = [_one_line(cell) for cell in table[0]]
    return headers, [[cell.strip() for cell in row] for row in table[1:]]


def detect_mapping(headers):
    """{field: header} for every field whose header matches a known name (first wins)."""
    mapping = {}
    for field in FIELDS:
        for header in headers:
            if field not in mapping and _norm(header) in HEADERS[field]:
                if header not in mapping.values():
                    mapping[field] = header
    return mapping


def apply_overrides(headers, mapping, overrides):
    """``mapping`` with the Owner's ``--map field=Column`` choices; unknown names are refused."""
    mapping = dict(mapping)
    for item in overrides:
        field, sep, column = item.partition("=")
        field, column = field.strip().lower(), column.strip()
        if not sep or field not in FIELDS:
            raise ImportError_(f"--map wants field=Column with a field from: {', '.join(FIELDS)}")
        if column == "":
            mapping.pop(field, None)  # --map notes= : leave the field out
        elif column not in headers:
            raise ImportError_(f"--map {field}: no column named {column!r}; columns are: "
                               + ", ".join(headers))
        else:
            mapping[field] = column
    return mapping


def map_status(value, overrides=None):
    """The Status a word stands for, or None when it is not known (ask the Owner)."""
    key = _norm(value)
    if overrides and key in overrides:
        return overrides[key]
    return _WORD_TO_STATUS.get(key)


def status_overrides(items):
    out = {}
    for item in items:
        word, sep, status = item.rpartition("=")
        if not sep or not word.strip() or status.strip() not in meta_store.STATUSES:
            raise ImportError_("--status-map wants Value=Status, with a Status from: "
                               + ", ".join(sorted(meta_store.STATUSES)))
        out[_norm(word)] = status.strip()
    return out


def parse_date(value, order="dmy"):
    """(ISO date, "") for a date the export gives; (None, why) when it cannot be read.

    ``order`` says how 03/04/2025 reads: ``dmy`` (day first) or ``mdy``. A date that is only
    valid one way round is read that way whatever the order.
    """
    text = re.sub(r"(?<=\d)(st|nd|rd|th)\b", "", _one_line(value), flags=re.I).replace(",", "")
    if not text:
        return None, ""
    candidates = []
    match = re.fullmatch(r"(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})(?:[T ].*)?", text)
    if match:
        candidates.append((int(match[1]), int(match[2]), int(match[3])))
    match = re.fullmatch(r"(\d{1,2})[-/.](\d{1,2})[-/.](\d{2}|\d{4})", text)
    if match:
        a, b, year = int(match[1]), int(match[2]), int(match[3])
        year += 2000 if year < 100 else 0
        candidates += [(year, b, a), (year, a, b)] if order == "dmy" else [(year, a, b), (year, b, a)]
    match = re.fullmatch(r"(\d{1,2})[ -]([A-Za-z]{3,9})[ -](\d{2}|\d{4})", text)
    if match and match[2][:3].lower() in _MONTHS:
        year = int(match[3]) + (2000 if len(match[3]) == 2 else 0)
        candidates.append((year, _MONTHS[match[2][:3].lower()], int(match[1])))
    match = re.fullmatch(r"([A-Za-z]{3,9})\.? (\d{1,2}) (\d{4})", text)
    if match and match[1][:3].lower() in _MONTHS:
        candidates.append((int(match[3]), _MONTHS[match[1][:3].lower()], int(match[2])))
    for year, month, day in candidates:
        try:
            return date(year, month, day).isoformat(), ""
        except ValueError:
            continue
    return None, f"date {value!r} not understood, left out"


# ----------------------------------------------------------------- planning


def status_log_for(status, applied, today):
    """The approximate ``status_log`` for an imported row: (status, at, approx) tuples.

    Dates come only from the export, and only a date applied says when a row was sent: a row
    that was sent (Applied or later) with one starts at Applied on that day, and a later
    Status is written at the same day because the export gives no other. Every other entry
    (a Lead, a Draft, a Dismissed row, a sent row with no date) is dated the import day. Every
    entry is approximate.
    """
    if status in SENT and applied:
        if status == "Applied":
            return [("Applied", applied, True)]
        return [("Applied", applied, True), (status, applied, True)]
    return [(status, today, True)]


class RowPlan:
    def __init__(self, line, fields=None, status_log=None, notes="", skip="", warnings=()):
        self.line, self.fields, self.status_log = line, fields or {}, status_log or []
        self.notes, self.skip, self.warnings = notes, skip, list(warnings)

    @property
    def label(self):
        return f"{self.fields.get('company', '?')} - {self.fields.get('title', '?')}"

    def describe(self):
        if self.skip:
            return f"skip    row {self.line}: {self.skip}"
        status, at, _ = self.status_log[-1]
        stamp = "" if at.endswith("Z") else f" ~{at}"
        return f"create  row {self.line}: {self.label}  [{status}{stamp}]"


def _cell(row, headers, mapping, field):
    column = mapping.get(field)
    if column is None or column not in headers:
        return ""
    index = headers.index(column)
    return row[index] if index < len(row) else ""


def plan_rows(workspace, headers, rows, mapping, track, status_map=None, order="dmy",
              allow_duplicate=False, today=None):
    """One RowPlan per data row. Writes nothing."""
    today = today or date.today().isoformat()
    tracks = load(workspace).tracks
    by_lower = {name.lower(): name for name in tracks}
    existing = meta_store.scan(workspace)
    seen_urls = {app.url: app.folder for app in existing if app.url}
    seen_keys = {}
    plans = []
    for number, row in enumerate(rows, start=2):  # line 1 is the header
        warnings = []
        get = lambda field: _cell(row, headers, mapping, field)  # noqa: E731
        title, company = _one_line(get("title")), _one_line(get("company"))
        if not title or not company:
            missing = "title" if not title else "company"
            plans.append(RowPlan(number, skip=f"no {missing}, nothing to create from"))
            continue
        row_track = track
        if get("track"):
            row_track = by_lower.get(get("track").lower())
            if row_track is None:
                plans.append(RowPlan(number, skip=f"track {get('track')!r} is not one of: "
                                     + ", ".join(tracks)))
                continue
        if not row_track:
            plans.append(RowPlan(number, skip="no track: pass --track"))
            continue
        applied, why = parse_date(get("applied"), order)
        if why:
            warnings.append(why)
        raw_status = get("status")
        if raw_status:
            status = map_status(raw_status, status_map)
            if status is None:
                plans.append(RowPlan(number, skip=f"status {raw_status!r} is not mapped: "
                                     "pass --status-map", warnings=warnings))
                continue
        else:  # no status given: the date is the only evidence
            status = "Applied" if applied else "Lead"
        if applied and status not in SENT:
            warnings.append(f"date applied ignored: the Status is {status}, which is not sent")
        elif not applied and status in SENT:
            warnings.append("no date applied: dated the import day (approximate)")
        url = get("url")
        notes = get("notes")
        if url:
            try:
                httpfetch.check_url(url)
            except httpfetch.FetchError:
                warnings.append(f"url {url!r} is not a web address, kept in notes")
                notes = (notes + "\n" if notes else "") + f"Imported link: {url}"
                url = ""
        fields = {"title": title, "company": company, "track": row_track, "url": url,
                  "added_by": "agent"}
        skip = ""
        if not allow_duplicate:
            if url and url in seen_urls:
                skip = f"same URL as {seen_urls[url]}"
            else:
                key = (adfacts.title_key(title), adfacts.company_key(company))
                dupes = adfacts.likely_duplicates(existing, company, title)
                if dupes:
                    skip = f"looks like {dupes[0][0].folder} ({dupes[0][1]})"
                elif key in seen_keys:
                    skip = f"same role as row {seen_keys[key]} in this file"
                else:
                    seen_keys[key] = number
        if url and not skip:
            seen_urls[url] = f"row {number}"
        plans.append(RowPlan(number, fields, status_log_for(status, applied, today), notes,
                             skip, warnings))
    return plans


def apply_plan(workspace, plans, now=None):
    """Create a folder for each non-skipped row through ``meta.create``; the folders made."""
    now = now or datetime.now(timezone.utc)
    made = []
    for item in plans:
        if item.skip:
            continue
        folder = meta_store.create(workspace, item.fields, now, status_log=item.status_log)
        if item.notes:
            atomic_write(folder / "notes.md", (item.notes.strip() + "\n").encode("utf-8"), 0o600)
        made.append(folder)
    return made


# ----------------------------------------------------------------- command line


def inspect(headers, rows, mapping, status_map):
    status_column = mapping.get("status")
    statuses = {}
    if status_column:
        index = headers.index(status_column)
        for row in rows:
            value = row[index].strip() if index < len(row) else ""
            if value:
                entry = statuses.setdefault(value, {"count": 0, "status": map_status(value, status_map)})
                entry["count"] += 1
    return {
        "rows": len(rows),
        "columns": headers,
        "mapping": mapping,
        "unmapped_fields": [f for f in FIELDS if f not in mapping and f != "track"],
        "unused_columns": [h for h in headers if h not in mapping.values()],
        "statuses": statuses,
        "sample": [dict(zip(headers, row, strict=False)) for row in rows[:3]],
    }


def build_parser():
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    add_data_argument(parser)
    parser.add_argument("--file", required=True, help="the CSV or TSV export")
    parser.add_argument("--track", default="", help="Track for every row (or map a track column)")
    parser.add_argument("--map", action="append", default=[], metavar="FIELD=COLUMN",
                        help="override a detected column; FIELD= leaves the field out. "
                        "Fields: " + ", ".join(FIELDS))
    parser.add_argument("--status-map", action="append", default=[], metavar="VALUE=STATUS",
                        help="what a status value in the export means")
    parser.add_argument("--date-order", choices=("dmy", "mdy"), default="dmy",
                        help="how 03/04/2025 reads (default day first)")
    parser.add_argument("--inspect", action="store_true",
                        help="print the detected mapping and statuses, and stop")
    parser.add_argument("--dry-run", action="store_true", help="print the plan, write nothing")
    parser.add_argument("--allow-duplicate", action="store_true",
                        help="create rows that look like a saved Application")
    parser.add_argument("--json", action="store_true", help="machine-readable output")
    return parser


def main(argv=None):
    args = build_parser().parse_args(argv)
    workspace = workspace_from_cli(args.data)
    try:
        headers, rows = read_table(args.file)
        mapping = apply_overrides(headers, detect_mapping(headers), args.map)
        status_map = status_overrides(args.status_map)
        if args.track and args.track not in load(workspace).tracks:
            raise ImportError_(f"--track {args.track!r} is not one of: "
                               + ", ".join(load(workspace).tracks))
        if args.inspect:
            report = inspect(headers, rows, mapping, status_map)
            if args.json:
                print(json.dumps(report, indent=2))
            else:
                print_inspection(report)
            return
        missing = [f for f in ("title", "company") if f not in mapping]
        if missing:
            raise ImportError_("no column found for " + " and ".join(missing)
                               + ": pass --map title=Column (see --inspect)")
        plans = plan_rows(workspace, headers, rows, mapping, args.track, status_map,
                          args.date_order, args.allow_duplicate)
        unmapped = [p for p in plans if p.skip.startswith("status ")]
        if args.json:
            print(json.dumps([{"line": p.line, "label": p.label, "skip": p.skip,
                               "status_log": p.status_log, "warnings": p.warnings}
                              for p in plans], indent=2))
        else:
            print_plan(plans, args.dry_run)
        if args.dry_run:
            return
        if unmapped:
            raise ImportError_(f"{len(unmapped)} row(s) have a status that is not mapped; "
                               "nothing was written. Pass --status-map for each, or fix the file")
        made = apply_plan(workspace, plans)
        print(f"\nCreated {len(made)} Application folder(s); {sum(1 for p in plans if p.skip)} skipped.")
    except (ImportError_, meta_store.MetaError, OSError, ValueError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


def print_inspection(report):
    print(f"{report['rows']} rows, columns: " + ", ".join(report["columns"]))
    print("\nDetected mapping:")
    for field in FIELDS:
        if field in report["mapping"]:
            print(f"  {field:<8} <- {report['mapping'][field]}")
    for field in report["unmapped_fields"]:
        print(f"  {field:<8} <- (no column found)")
    if report["unused_columns"]:
        print("\nColumns not imported: " + ", ".join(report["unused_columns"]))
    if report["statuses"]:
        print("\nStatus values:")
        for value, info in report["statuses"].items():
            print(f"  {value!r:<28} x{info['count']:<4} -> {info['status'] or '? (ask the Owner)'}")


def print_plan(plans, dry_run):
    for item in plans:
        print(item.describe())
        for warning in item.warnings:
            print(f"        note: {warning}")
    created = sum(1 for p in plans if not p.skip)
    if dry_run:
        print(f"\nDry run: {created} would be created, {len(plans) - created} skipped. "
              "Nothing was written.")


if __name__ == "__main__":
    main()
