"""Shared test helpers: a fixture Workspace copy and a subprocess runner."""

import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "fixtures" / "workspace"


def copy_fixture_workspace(tmp_path, fixture=FIXTURE):
    """A writable copy of a fixture Workspace under ``tmp_path``."""
    workspace = tmp_path / "workspace"
    shutil.copytree(fixture, workspace)
    return workspace


def run_script(workspace, script, *args, stdin=""):
    """Run a plugin script as a subprocess against ``workspace``."""
    return subprocess.run(
        [sys.executable, str(script), "--data", str(workspace), *args],
        input=stdin,
        capture_output=True,
        text=True,
    )
