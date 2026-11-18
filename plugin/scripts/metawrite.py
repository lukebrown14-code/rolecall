"""Write meta.yml the way the Go store does: re-read, validate, change, validate, publish.

Every writer keeps every other byte and inline comment, refuses a change that would
leave the file malformed, and publishes through ``workspaceio.atomic_write``.
"""

import os
import shutil
from datetime import datetime, timezone
from pathlib import Path

from metaread import FACTS, FILTER_REASONS, STATUSES, read_meta
from metayaml import KEY, MetaError, parse_moment, scalar_parts, valid_timestamp
from workspaceio import atomic_write, quote_scalar, unique_folder


def _utc_stamp(value):
    moment = parse_moment(value)
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _update(folder, change):
    """Re-read, validate, apply ``change`` to the lines, validate, write."""
    path = Path(folder) / "meta.yml"
    data = path.read_bytes()
    application = read_meta(Path(folder), data)
    if application.malformed:
        raise MetaError(application.malformed)
    lines = data.decode("utf-8")[:-1].split("\n")
    lines = change(lines)
    out = ("\n".join(lines) + "\n").encode("utf-8")
    checked = read_meta(Path(folder), out)
    if checked.malformed:
        raise MetaError(checked.malformed)
    if out == data:
        return
    atomic_write(path, out, path.stat().st_mode & 0o777)


def _patch_lines(lines, keys):
    """Set scalar keys in place, keeping every other byte and inline comment."""
    for name in sorted(keys):
        if not KEY.fullmatch(name) or name in ("status_log", "contacts", "next_step"):
            raise MetaError(f"invalid scalar key {name!r}")
        value = quote_scalar(keys[name])
        found = False
        for index, line in enumerate(lines):
            if line.startswith(name + ":"):
                _, suffix = scalar_parts(line[len(name) + 1 :])
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
        current = read_meta(Path(folder), ("\n".join(lines) + "\n").encode("utf-8"))
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
        current = read_meta(Path(folder), ("\n".join(lines) + "\n").encode("utf-8"))
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
    if not valid_timestamp(str(at)):
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
    checked = read_meta("", text.encode("utf-8"))
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
    checked = read_meta("", text.encode("utf-8"))
    if checked.malformed:
        raise MetaError(checked.malformed)

    parent = Path(workspace) / "jobs" / track
    parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    # The creation date in the Owner's zone, as the TUI names its folders.
    day = moment.astimezone().strftime("%Y-%m-%d")
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
