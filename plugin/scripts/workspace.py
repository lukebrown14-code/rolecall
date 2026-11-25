"""Resolve the Workspace for every plugin script.

Order: the ``--data`` option, then ``ROLECALL_WORKSPACE``. There is no
repository-root fallback: a script that cannot find a real directory stops with
a clear message instead of guessing.

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
    def contact_omit(self):
        omit = self.raw.get("contactOmit")
        if isinstance(omit, dict) and all(
            isinstance(fields, list) and all(isinstance(field, str) for field in fields)
            for fields in omit.values()
        ):
            return omit
        return {}

    @property
    def blockers(self):
        blockers = self.raw.get("blockers")
        return blockers if isinstance(blockers, list) else []

    @property
    def sources(self):
        """The ATS plan as platform -> targets, from either form of ``sources``.

        The list form (the Source registry) contributes its enabled ``api`` entries; the
        legacy object form is returned as it is."""
        sources = self.raw.get("sources")
        if isinstance(sources, dict):
            return sources
        plan = {}
        for entry in self.source_registry:
            if entry["type"] == "api" and entry["enabled"] and entry.get("platform"):
                plan.setdefault(entry["platform"], []).extend(entry.get("targets") or [])
        return plan

    @property
    def source_registry(self):
        """Every Source as a registry entry (SCHEMA.md), whichever form the file uses.

        A legacy object becomes one enabled ``api`` entry per platform; an entry of the
        wrong shape is skipped here and reported by the Sweep's validation."""
        sources = self.raw.get("sources")
        if isinstance(sources, dict):
            return [
                {"name": name, "type": "api", "platform": name, "targets": list(targets),
                 "access": "unknown", "enabled": True}
                for name, targets in sources.items() if isinstance(targets, list)
            ]
        out = []
        for entry in sources if isinstance(sources, list) else []:
            if not isinstance(entry, dict) or not isinstance(entry.get("name"), str):
                continue
            item = dict(entry)
            item.setdefault("type", "manual")
            item.setdefault("access", "unknown")
            item["enabled"] = entry.get("enabled", True) is not False
            out.append(item)
        return out

    @property
    def search_rules(self):
        rules = self.raw.get("search_rules")
        return rules if isinstance(rules, dict) else {}


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
