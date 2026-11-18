"""Shared test helpers: a fixture Workspace copy and a subprocess runner."""

import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "fixtures" / "workspace"


@pytest.fixture(autouse=True, scope="session")
def utc_zone():
    """Run the suite, and the scripts it starts, in UTC whatever the developer's TZ.

    A timestamp counts on its date in the Owner's local zone (SCHEMA status_log), so
    expectations hold only in a known zone; a test about another zone sets its own.
    """
    before = os.environ.get("TZ")
    os.environ["TZ"] = "UTC"
    if hasattr(time, "tzset"):
        time.tzset()
    yield
    if before is None:
        os.environ.pop("TZ", None)
    else:
        os.environ["TZ"] = before
    if hasattr(time, "tzset"):
        time.tzset()


@pytest.fixture
def owner_offset(monkeypatch):
    """Set the process zone, and so the scripts', to a fixed UTC offset in hours."""
    if not hasattr(time, "tzset"):
        pytest.skip("time.tzset is not available on this platform")

    def set_offset(hours):
        sign = "-" if hours >= 0 else "+"  # POSIX TZ offsets are west-positive
        monkeypatch.setenv("TZ", f"RCT{sign}{abs(hours):02d}:00")
        time.tzset()

    yield set_offset
    monkeypatch.undo()
    time.tzset()


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
