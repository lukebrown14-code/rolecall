"""Workspace file primitives shared by every plugin script that writes.

``atomic_write`` publishes bytes through a same-folder temporary file, so a crash
never leaves a half-written Workspace file. ``unique_folder`` mirrors the Go
store's slug and collision rule. ``quote_scalar`` encodes a scalar the way the Go
store does, so the two writers agree byte for byte.
"""

import os
import re
import tempfile
from pathlib import Path


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


def atomic_write(path, data, mode):
    """Publish bytes through a same-folder temporary file, then rename."""
    path = Path(path)
    descriptor, name = tempfile.mkstemp(dir=str(path.parent), prefix=".rolecall-")
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


# The most of company-title a folder name keeps (SCHEMA "Folder name"), so a pasted
# headline never runs past the file system's name limit.
MAX_SLUG_WORDS = 60

# The name for a company and title with no [a-z0-9] at all (a CJK or symbol-only name).
SLUG_FALLBACK = "application"


def folder_slug(company, title, day):
    """SCHEMA's folder name before any -N suffix, as the Go store's folderSlug.

    Each character is lowercased on its own, as Go's strings.ToLower does: str.lower()
    maps dotted capital I to "i" plus a combining dot, so only its first character is kept.
    """
    lowered = "".join(character.lower()[:1] for character in company + "-" + title)
    words = re.sub(r"[^a-z0-9]+", "-", lowered).strip("-")
    if len(words) > MAX_SLUG_WORDS:
        cut = words[: MAX_SLUG_WORDS + 1]
        dash = cut.rfind("-")
        words = cut[:dash] if dash > 0 else words[:MAX_SLUG_WORDS]
    return (words or SLUG_FALLBACK) + "-" + day


def unique_folder(parent, company, title, day, reserved=None):
    """The first free ``<slug>-<day>`` / ``-2`` / ``-3`` path under ``parent``.

    Mirrors the Go store's slug and collision rule. ``reserved`` lets a planner
    claim names before they exist on disk.
    """
    if reserved is None:
        reserved = set()
    parent = Path(parent)
    slug = folder_slug(company, title, day)
    number = 1
    while True:
        name = slug if number == 1 else f"{slug}-{number}"
        if name not in reserved and not (parent / name).exists():
            reserved.add(name)
            return parent / name
        number += 1
