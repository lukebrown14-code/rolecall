"""The Workspace comes from --data or ROLECALL_WORKSPACE, and nowhere else."""

import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
META = ROOT / "plugin" / "scripts" / "meta.py"


def run(args, workspace=None):
    env = {k: v for k, v in os.environ.items() if k != "ROLECALL_WORKSPACE"}
    if workspace is not None:
        env["ROLECALL_WORKSPACE"] = str(workspace)
    return subprocess.run(
        [sys.executable, str(META), *args],
        capture_output=True,
        text=True,
        env=env,
    )


def make_workspace(root, title):
    """A one-Application Workspace so resolution is visible in the output."""
    folder = root / "jobs" / "SWE" / title.lower().replace(" ", "-")
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_text(
        f"title: {title}\ncompany: Example\ntrack: SWE\nstatus_log: []\n"
    )
    return root


def test_missing_workspace_is_a_clear_error():
    result = run([])

    assert result.returncode != 0
    assert result.stdout == ""
    assert "--data" in result.stderr
    assert "ROLECALL_WORKSPACE" in result.stderr


def test_data_must_be_a_directory(tmp_path):
    not_a_dir = tmp_path / "workspace.json"
    not_a_dir.write_text("{}")

    result = run(["--data", str(not_a_dir)])

    assert result.returncode != 0
    assert result.stderr.strip()
    assert str(not_a_dir) in result.stderr


def test_data_equals_form_resolves_the_workspace(tmp_path):
    root = make_workspace(tmp_path / "ws", "Dash Form")

    result = run([f"--data={root}"])

    assert result.returncode == 0, result.stderr
    assert [app["Title"] for app in json.loads(result.stdout)] == ["Dash Form"]


def test_environment_variable_resolves_the_workspace(tmp_path):
    root = make_workspace(tmp_path / "ws", "Env Form")

    result = run([], workspace=root)

    assert result.returncode == 0, result.stderr
    assert [app["Title"] for app in json.loads(result.stdout)] == ["Env Form"]


def test_data_wins_over_the_environment_variable(tmp_path):
    chosen = make_workspace(tmp_path / "chosen", "Chosen")
    ignored = make_workspace(tmp_path / "ignored", "Ignored")

    result = run(["--data", str(chosen)], workspace=ignored)

    assert result.returncode == 0, result.stderr
    assert [app["Title"] for app in json.loads(result.stdout)] == ["Chosen"]


def test_empty_workspace_scans_to_no_applications(tmp_path):
    root = tmp_path / "ws"
    (root / "jobs").mkdir(parents=True)

    result = run(["--data", str(root)])

    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) == []
