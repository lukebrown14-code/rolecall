"""Resolve the Workspace for every plugin script.

Order: the ``--data`` option, then ``ROLECALL_WORKSPACE``. There is no
repository-root or dashboard-era fallback: a script that cannot find a real
directory stops with a clear message instead of guessing.

Every script resolves through these functions, so a script behaves the same
whether it is run from a shell or called in process with its own ``argv``.
"""

import argparse
import json
import os
import sys
from pathlib import Path

ENV_VAR = "ROLECALL_WORKSPACE"


class WorkspaceError(Exception):
    """The Workspace argument or environment variable is missing or invalid."""


class Config:
    """The shared workspace.json fields, read the same way by every script.

    A missing or invalid file, or a field of the wrong shape, reads as its default
    (empty), so a script can run against a bare Workspace.
    """

    def __init__(self, raw):
        self.raw = raw if isinstance(raw, dict) else {}

    @property
    def tracks(self):
        tracks = self.raw.get("tracks")
        return tracks if isinstance(tracks, list) else []

    @property
    def search_area(self):
        area = self.raw.get("search_area")
        return area if isinstance(area, dict) else {}

    @property
    def gates(self):
        keys = ("letterWordCap", "letterWordMin", "keySkillsMin", "keySkillsMax")
        return {key: self.raw[key] for key in keys if key in self.raw}

    @property
    def blockers(self):
        blockers = self.raw.get("blockers")
        return blockers if isinstance(blockers, list) else []

    @property
    def sources(self):
        sources = self.raw.get("sources")
        return sources if isinstance(sources, dict) else {}


def load(path):
    """Read a Workspace's workspace.json into the shared fields."""
    try:
        raw = json.loads((Path(path) / "workspace.json").read_text())
    except (OSError, ValueError):
        raw = {}
    return Config(raw)


def add_data_argument(parser):
    """Add the shared ``--data`` option to a script's argument parser."""
    parser.add_argument("--data", help="Workspace path; or set ROLECALL_WORKSPACE")


def data_from_argv(argv=None):
    """Return the ``--data`` value in argv, or None.

    For a script that must resolve the Workspace before it can build its full
    parser (a choice list that depends on the Workspace's tracks).
    """
    parser = argparse.ArgumentParser(add_help=False)
    add_data_argument(parser)
    data, _ = parser.parse_known_args(sys.argv[1:] if argv is None else argv)
    return data.data


def resolve_workspace(data=None):
    """Return the Workspace as an absolute Path, or raise WorkspaceError.

    ``data`` is the parsed ``--data`` value (or None); it falls back to
    ``ROLECALL_WORKSPACE``.
    """
    value = data if data is not None else os.environ.get(ENV_VAR)
    if value is None or not value.strip():
        raise WorkspaceError(f"no Workspace: pass --data <path> or set {ENV_VAR}")
    workspace = Path(value).expanduser().resolve()
    if not workspace.is_dir():
        raise WorkspaceError(f"Workspace is not a directory: {workspace}")
    return workspace


def workspace_from_cli(data=None):
    """Resolve the Workspace or exit non-zero with the reason on stderr."""
    try:
        return resolve_workspace(data)
    except WorkspaceError as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error
