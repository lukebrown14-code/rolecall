"""Resolve the Workspace for every plugin script.

Order: ``--data <path>``, ``--data=<path>``, then ``ROLECALL_WORKSPACE``.
There is no repository-root or dashboard-era fallback: a script that cannot
find a real directory stops with a clear message instead of guessing.
"""

import os
import sys
from pathlib import Path

ENV_VAR = "ROLECALL_WORKSPACE"


class WorkspaceError(Exception):
    """The Workspace argument or environment variable is missing or invalid."""


def _argument(argv):
    if "--data" in argv:
        index = argv.index("--data")
        if index + 1 >= len(argv):
            raise WorkspaceError("--data needs a path")
        return argv[index + 1]
    for arg in argv:
        if arg.startswith("--data="):
            return arg.split("=", 1)[1]
    return None


def resolve_workspace(argv=None):
    """Return the Workspace as an absolute Path, or raise WorkspaceError."""
    argv = sys.argv[1:] if argv is None else argv
    value = _argument(argv)
    if value is None:
        value = os.environ.get(ENV_VAR)
    if value is None or not value.strip():
        raise WorkspaceError(
            f"no Workspace: pass --data <path> or set {ENV_VAR}"
        )
    workspace = Path(value).expanduser().resolve()
    if not workspace.is_dir():
        raise WorkspaceError(f"Workspace is not a directory: {workspace}")
    return workspace


def workspace_from_cli(argv=None):
    """Resolve the Workspace or exit non-zero with the reason on stderr."""
    try:
        return resolve_workspace(argv)
    except WorkspaceError as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error
