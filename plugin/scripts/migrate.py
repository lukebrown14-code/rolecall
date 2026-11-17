#!/usr/bin/env python3
"""One-time migration from the dashboard-era Workspace to SCHEMA.md v1.

Plans every change before writing anything:

    python3 migrate.py [--data <workspace>] [--dry-run]

The plan phase reads ``TRACKER.md``, ``SHORTLIST.md``, ``DISMISSED.md`` and each
job folder. The apply phase then writes ``meta.yml`` through the same validating,
atomic path as the other writers, removes jd front matter and archives the
dashboard-era files under ``jobs/_archive/``. A folder whose ``meta.yml`` already
has a ``schema`` key is skipped, so a second run changes nothing.
"""

import argparse
import os
import re
import sys
from dataclasses import dataclass, field
from datetime import date
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import meta  # noqa: E402
from workspace import add_data_argument, load, workspace_from_cli  # noqa: E402

ARCHIVE = [
    "TRACKER.md",
    "SHORTLIST.md",
    "DISMISSED.md",
    "INBOX.md",
    "sweep-candidates.md",
]
STATUS_MAP = {
    "Lead": "Lead",
    "Draft": "Draft",
    "Applied": "Applied",
    "Screening": "Interviewing",
    "Interview": "Interviewing",
    "Offer": "Offer",
    "Rejected": "Rejected",
    "Withdrawn": "Withdrawn",
    "Dismissed": "Dismissed",
}
DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
LINK = re.compile(r"^\s*\[([^\]]*)\]\(([^)]*)\)\s*$")


@dataclass
class FolderChange:
    folder: Path
    fields: dict
    status_log: list
    jd_body: bytes | None
    had_meta: bool
    details: list = field(default_factory=list)

    @property
    def verb(self):
        return "update" if self.had_meta else "create"


@dataclass
class Archive:
    source: Path
    destination: Path


@dataclass
class Report:
    text: str


@dataclass
class ArchiveNotice:
    """The read-only notice beside archived files, so none is mistaken for a live list."""

    path: Path


ARCHIVE_NOTICE = """# Read-only archive

rolecall does not read anything in this folder. These are dashboard-era files kept as
history after the migration: editing them changes nothing in the app. The live data is
each Application's `meta.yml` (one folder per role under `jobs/<track>/`), shown by the
rolecall TUI. The files here are made read-only so they are not edited by mistake.
"""


def _rel(workspace, path):
    return Path(path).relative_to(workspace).as_posix()


def _iso_date(value):
    text = str(value or "").strip().strip('"').strip("'")
    if not DATE.fullmatch(text):
        return None
    try:
        date.fromisoformat(text)
    except ValueError:
        return None
    return text


def _folder_date(folder):
    match = re.search(r"-(\d{4}-\d{2}-\d{2})$", Path(folder).name)
    return _iso_date(match.group(1)) if match else None


def map_status(value):
    return STATUS_MAP.get(str(value or "").strip())


def map_fit(value):
    """Prefix mapping from the dashboard's free-text Fit to the v1 enum."""
    text = str(value or "").replace("**", "").strip()
    if text.lower().startswith("verdict:"):
        text = text.split(":", 1)[1].strip()
    low = text.lower().replace("\u2019", "'")
    if low.startswith("strong"):
        return "Strong"
    if low.startswith("worth applying") or low.startswith("good"):
        return "Good"
    if low.startswith("marginal"):
        return "Marginal"
    if low.startswith("weak") or low.startswith("poor") or low.startswith("don't"):
        return "Poor"
    return None


def _has_schema(path):
    try:
        text = Path(path).read_text()
    except OSError:
        return False
    return any(line.startswith("schema:") for line in text.splitlines())


def _unquote(raw):
    text = raw.strip()
    if len(text) >= 2 and text[0] == '"' and text[-1] == '"':
        try:
            return meta._decode_quoted(text)
        except meta.MetaError:
            return text[1:-1]
    if len(text) >= 2 and text[0] == "'" and text[-1] == "'":
        return text[1:-1]
    return text


def parse_front_matter(path):
    """Return (front matter dict, body bytes, had front matter)."""
    path = Path(path)
    if not path.exists():
        return {}, b"", False
    data = path.read_bytes()
    lines = data.split(b"\n")
    if not lines or lines[0].strip() != b"---":
        return {}, data, False
    for index in range(1, len(lines)):
        if lines[index].strip() == b"---":
            values = {}
            for raw in lines[1:index]:
                text = raw.decode("utf-8", "replace").strip()
                if not text or text.startswith("#"):
                    continue
                key, separator, value = text.partition(":")
                if separator:
                    values[key.strip()] = _unquote(value)
            return values, b"\n".join(lines[index + 1 :]), True
    return {}, data, False


def _table_rows(path, header_cells):
    path = Path(path)
    if not path.exists():
        return []
    lines = path.read_text().splitlines()
    head = None
    for index, line in enumerate(lines):
        if line.lstrip().startswith("|") and all(cell in line for cell in header_cells):
            head = index
            break
    if head is None:
        return []
    rows = []
    for line in lines[head + 2 :]:
        if not line.lstrip().startswith("|"):
            break
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        rows.append(cells)
    return rows


def read_tracker(workspace):
    rows = {}
    for cells in _table_rows(workspace / "jobs" / "TRACKER.md", ["| Status |"]):
        if len(cells) != 9:
            continue
        folder = cells[8].strip().strip("`").strip()
        rows.setdefault(
            folder,
            {
                "date": cells[0],
                "company": cells[1],
                "role": cells[2],
                "track": cells[3],
                "fit": cells[4],
                "status": cells[5],
                "source": cells[7],
            },
        )
    return rows


def _last_swept(path):
    path = Path(path)
    if not path.exists():
        return None
    for line in path.read_text().splitlines():
        if line.lower().startswith("last swept:"):
            match = re.search(r"\d{4}-\d{2}-\d{2}", line)
            return _iso_date(match.group(0)) if match else None
    return None


def read_shortlist(workspace):
    path = workspace / "jobs" / "SHORTLIST.md"
    rows = []
    for number, cells in enumerate(_table_rows(path, ["| Status |", "| Role |"]), start=1):
        if len(cells) != 8:
            continue
        match = LINK.match(cells[1])
        title = match.group(1).strip() if match else cells[1]
        url = match.group(2).strip() if match else ""
        rows.append(
            {
                "row": number,
                "title": title,
                "company": cells[2],
                "url": url,
                "status": cells[6],
                "reason": cells[7],
            }
        )
    return rows, _last_swept(path)


def read_dismissed(workspace):
    rows = []
    for cells in _table_rows(workspace / "jobs" / "DISMISSED.md", ["| Link |"]):
        if len(cells) != 4:
            continue
        rows.append(
            {
                "date": cells[0],
                "company": cells[1],
                "title": cells[2],
                "url": cells[3],
            }
        )
    return rows


def _default_track(workspace):
    """The Workspace's first track, or None when it configures none."""
    for track in load(workspace).tracks:
        if isinstance(track, str) and track.strip():
            return track
    return None


def _preserved(entry):
    """An existing entry, made SCHEMA-valid for migration.

    A date-only entry is migration-only and must carry ``approx: true``.
    RFC3339 entries keep their own flag.
    """
    approx = entry.approx or bool(DATE.fullmatch(entry.at))
    return (entry.status, entry.at, approx)


def _plan_log(app, status, applied, tracker_date, fallback, details):
    """The migrated status_log, or None to leave the existing one untouched.

    Bulk rule: a single existing entry is a bulk stamp, not history. Keep it
    and prepend ``Applied`` at the legacy ``applied`` date (else the tracker
    Date), then append the mapped tracker Status when it differs and does not
    repeat.
    """
    log = list(app.status_log) if app is not None else []
    applied_stamp = applied or tracker_date or fallback
    status_stamp = tracker_date or applied or fallback
    if not log:
        if status:
            details.append(f"status_log: {status} (approx, {status_stamp})")
            return [(status, status_stamp, True)]
        details.append(f"status_log: Lead (approx, {fallback})")
        return [("Lead", fallback, True)]
    if len(log) > 1:
        details.append(f"status_log: left unchanged ({len(log)} entries)")
        return None
    entry = _preserved(log[0])
    if entry[0] == "Applied":
        result = [entry]
        if status and status != "Applied":
            details.append(f"status_log: {status} appended (approx, {status_stamp})")
            result.append((status, status_stamp, True))
        else:
            details.append("status_log: Applied already present; no duplicate")
        return result
    result = [("Applied", applied_stamp, True), entry]
    details.append(
        f"status_log: Applied prepended (approx, {applied_stamp}); existing entry kept"
    )
    # The mapped tracker Status wins when it differs from the current (kept)
    # Status; a repeat of the current Status is not appended.
    if status and status != result[-1][0]:
        details.append(f"status_log: {status} appended (approx, {status_stamp})")
        result.append((status, status_stamp, True))
    # When the tracker Status is Applied and its date is the legacy applied date,
    # the prepended and the appended Applied entries are the same entry. Keep the
    # later one, so the Application still reads as Applied.
    deduped = meta.dedupe_status_log(result)
    if len(deduped) < len(result):
        details.append("status_log: a repeated entry (same Status and date) written once")
    return deduped


def plan(workspace):
    workspace = Path(workspace)
    changes = []
    reports = []
    tracker = read_tracker(workspace)
    jobs = workspace / "jobs"
    seen_rows = set()

    for track_dir in sorted(jobs.iterdir(), key=lambda entry: os.fsencode(entry.name)):
        if not track_dir.is_dir() or track_dir.name == "_archive":
            continue
        for folder in sorted(track_dir.iterdir(), key=lambda entry: os.fsencode(entry.name)):
            if not folder.is_dir():
                continue
            relative = _rel(workspace, folder)
            row = tracker.get(relative)
            if row is not None:
                seen_rows.add(relative)
            meta_path = folder / "meta.yml"
            jd_path = folder / "jd.md"
            if meta_path.exists() and _has_schema(meta_path):
                continue
            application = meta.rescan(folder) if meta_path.exists() else None
            if application is not None and application.malformed:
                # A pre-migration stub may hold only status_log (the TUI or an
                # earlier partial run). It has no schema and no real fields, so
                # rebuild it from jd.md and TRACKER.md. Anything else is a real
                # malformed file: stop before writing.
                stub = (
                    not _has_schema(meta_path)
                    and not application.title
                    and not application.company
                    and not application.track
                    and not application.unknown
                )
                if not stub:
                    raise meta.MetaError(application.malformed)
                reports.append(
                    Report(
                        f"{relative}: incomplete pre-migration meta.yml ignored; "
                        "rebuilt from jd.md/TRACKER.md"
                    )
                )
                application = None
            if row is None and application is None and not jd_path.exists():
                continue

            front, body, had_front = parse_front_matter(jd_path)
            row = row or {}
            status_raw = str(row.get("status", "")).strip()
            status = map_status(status_raw) if status_raw else None
            if status_raw and status is None:
                reports.append(
                    Report(
                        f"{relative}: unknown TRACKER Status {status_raw!r}; "
                        "folder skipped"
                    )
                )
                continue
            details = []
            conflicts = []

            def choose(key, label):
                from_jd = str(front.get(key, "")).strip()
                from_row = str(row.get(key, "")).strip()
                if from_jd and from_row and from_jd != from_row:
                    conflicts.append(
                        f"conflict {label}: jd.md {from_jd!r} vs TRACKER.md "
                        f"{from_row!r}; using jd.md"
                    )
                return from_jd or from_row

            title = choose("role", "title")
            company = choose("company", "company")
            track = choose("track", "track") or track_dir.name
            url = choose("source", "url")
            closes = str(front.get("closes", "")).strip()
            if url and not url.lower().startswith(("http://", "https://")):
                url = ""
            if closes and not _iso_date(closes):
                closes = ""

            fields = {
                "schema": "1",
                "title": title,
                "company": company,
                "track": track,
            }
            if url:
                fields["url"] = url
            if closes:
                fields["closes"] = closes
            details.extend(conflicts)

            # An empty Fit cell, "—", "-" or "n/a" means "not assessed": leave
            # it unset silently. Only non-empty unmappable text is reported.
            fit_raw = str(row.get("fit", "")).strip()
            if fit_raw.lower() not in ("", "—", "-", "n/a", "na"):
                fit = map_fit(fit_raw)
                if fit is None:
                    reports.append(
                        Report(
                            f"{relative}: Fit {fit_raw!r} is unmappable; not written "
                            "(set it by hand)"
                        )
                    )
                elif fit:
                    fields["fit"] = fit

            if not title or not company:
                reports.append(
                    Report(f"{relative}: missing title/company; folder skipped")
                )
                continue

            tracker_date = _iso_date(row.get("date")) or _iso_date(front.get("captured"))
            applied = ""
            if application is not None:
                applied = _iso_date(application.unknown.get("applied"))
            fallback = tracker_date or _iso_date(front.get("captured")) or _folder_date(folder)
            if fallback is None:
                reports.append(Report(f"{relative}: no usable date; folder skipped"))
                continue
            status_log = _plan_log(
                application, status, applied, tracker_date, fallback, details
            )

            for key in ("schema", "title", "company", "track", "url", "closes", "fit"):
                if key in fields:
                    details.append(f"{key}: {fields[key]}")
            if had_front:
                details.append("jd.md: front matter removed; body bytes preserved")
            changes.append(
                FolderChange(
                    folder=folder,
                    fields=fields,
                    status_log=status_log,
                    jd_body=body if had_front else None,
                    had_meta=application is not None,
                    details=details,
                )
            )

    for relative in sorted(set(tracker) - seen_rows):
        reports.append(Report(f"TRACKER.md row for {relative} has no folder; skipped"))

    reserved = set()
    day = _last_swept(workspace / "jobs" / "SHORTLIST.md") or date.today().isoformat()
    default_track = _default_track(workspace)
    rows, _ = read_shortlist(workspace)
    for row in rows:
        if row["status"] != "New":
            reports.append(
                Report(
                    f'SHORTLIST row {row["row"]} {row["title"]!r} at Status '
                    f'{row["status"]!r} is dropped (not migrated)'
                )
            )
            continue
        track = default_track
        if track is None:
            reports.append(
                Report(
                    f'SHORTLIST row {row["row"]} {row["title"]!r} skipped: '
                    "workspace.json has no tracks"
                )
            )
            continue
        parent = jobs / track
        folder = meta.unique_folder(
            parent, row["company"], row["title"], day, reserved
        )
        jd = _stub(row["title"], row["company"], row["url"], row["reason"])
        changes.append(
            FolderChange(
                folder=folder,
                fields={
                    "title": row["title"],
                    "company": row["company"],
                    "track": track,
                    "url": row["url"],
                },
                status_log=[("Lead", day, True)],
                jd_body=jd,
                had_meta=False,
                details=[
                    f'source: SHORTLIST row {row["row"]} (Status New)',
                    f"status_log: Lead (approx, {day})",
                    "jd.md: stub with URL and triage reason",
                ],
            )
        )

    for row in read_dismissed(workspace):
        row_date = _iso_date(row["date"])
        if row_date is None:
            reports.append(
                Report(f'DISMISSED row {row["title"]!r} has no valid date; skipped')
            )
            continue
        track = default_track
        if track is None:
            reports.append(
                Report(
                    f'DISMISSED row {row["title"]!r} skipped: '
                    "workspace.json has no tracks"
                )
            )
            continue
        parent = jobs / track
        folder = meta.unique_folder(
            parent, row["company"], row["title"], row_date, reserved
        )
        changes.append(
            FolderChange(
                folder=folder,
                fields={
                    "title": row["title"],
                    "company": row["company"],
                    "track": track,
                    "url": row["url"],
                },
                status_log=[("Dismissed", row_date, True)],
                jd_body=_stub(row["title"], row["company"], row["url"], ""),
                had_meta=False,
                details=[
                    "source: DISMISSED row",
                    f"status_log: Dismissed (approx, {row_date})",
                    "jd.md: stub with URL",
                ],
            )
        )

    archive = jobs / "_archive"
    for name in ARCHIVE:
        source = jobs / name
        if not source.exists():
            continue
        destination = archive / name
        number = 2
        while destination.exists():
            stem, suffix = os.path.splitext(name)
            destination = archive / f"{stem}-{number}{suffix}"
            number += 1
        changes.append(Archive(source, destination))
    archived = any(isinstance(change, Archive) for change in changes)
    if (archived or archive.is_dir()) and not (archive / "README.md").exists():
        changes.append(ArchiveNotice(archive / "README.md"))

    return changes + reports


def _stub(title, company, url, reason):
    lines = [f"# {title} — {company}", ""]
    if url:
        lines.extend([url, ""])
    if reason:
        lines.extend([f"Why it's worth applying: {reason}", ""])
    return ("\n".join(lines).rstrip("\n") + "\n").encode("utf-8")


def render(workspace, changes):
    if not changes:
        return "No changes."
    lines = []
    for change in changes:
        if isinstance(change, Report):
            lines.append("report: " + change.text)
        elif isinstance(change, Archive):
            lines.append(
                f"archive {_rel(workspace, change.source)} -> "
                f"{_rel(workspace, change.destination)} (read-only)"
            )
        elif isinstance(change, ArchiveNotice):
            lines.append(f"write {_rel(workspace, change.path)} (read-only archive notice)")
        else:
            lines.append(f"{change.verb} {_rel(workspace, change.folder)}")
            lines.extend("  " + detail for detail in change.details)
    return "\n".join(lines)


def apply(workspace, changes):
    workspace = Path(workspace)
    written = 0
    for change in changes:
        if isinstance(change, Report):
            continue
        if isinstance(change, Archive):
            change.destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            os.replace(change.source, change.destination)
            os.chmod(change.destination, 0o400)
            written += 1
            continue
        if isinstance(change, ArchiveNotice):
            change.path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            meta.atomic_write(change.path, ARCHIVE_NOTICE.encode("utf-8"), 0o400)
            written += 1
            continue
        change.folder.mkdir(parents=True, exist_ok=True, mode=0o700)
        if change.had_meta:
            meta.update(change.folder, change.fields, change.status_log)
        else:
            if not (change.folder / "jd.md").exists():
                meta.atomic_write(change.folder / "jd.md", b"", 0o600)
            meta.create_migrated(change.folder, change.fields, change.status_log)
        if change.jd_body is not None:
            jd_path = change.folder / "jd.md"
            mode = jd_path.stat().st_mode & 0o777 if jd_path.exists() else 0o600
            meta.atomic_write(jd_path, change.jd_body, mode)
        written += 1
    return written


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    add_data_argument(parser)
    parser.add_argument("--dry-run", action="store_true", help="print the plan only")
    args = parser.parse_args(argv)
    workspace = workspace_from_cli(args.data)
    try:
        changes = plan(workspace)
        print(render(workspace, changes))
        if args.dry_run:
            return
        written = apply(workspace, changes)
        if written:
            print(f"Applied {written} change(s).")
    except (meta.MetaError, OSError, ValueError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
