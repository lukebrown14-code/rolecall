"""set-fit.py writes only the ``fit`` key through the shared meta writer.

Every test drives the CLI as a subprocess against a temp copy of
``fixtures/workspace`` and asserts on bytes: the four Fit values land on one
line, unknown keys and comments survive untouched, and a refusal leaves the
file exactly as it was.
"""

import json
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace, run_script

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "plugin" / "scripts" / "set-fit.py"
META = ROOT / "plugin" / "scripts" / "meta.py"
FIXTURE = ROOT / "fixtures" / "workspace"
PATCH = ROOT / "internal" / "store" / "tests" / "testdata" / "patch"

VALUES = ("Strong", "Good", "Marginal", "Poor")


def read_fit(workspace, folder):
    """The Fit value a reader reports, through the meta.py CLI."""
    result = subprocess.run(
        [sys.executable, str(META), "--data", str(workspace)],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    for application in json.loads(result.stdout):
        if Path(application["Folder"]).resolve() == folder.resolve():
            return application["Fit"]
    raise AssertionError(f"{folder} not found in scan output")


@pytest.mark.parametrize("value", VALUES)
def test_sets_each_fit_value_and_changes_only_the_fit_line(tmp_path, value):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "docs-ready"
    path = folder / "meta.yml"
    before = path.read_bytes()
    expected = before.replace(
        b"status_log:\n",
        f'fit: "{value}"\nstatus_log:\n'.encode("utf-8"),
    )

    result = run_script(workspace, SCRIPT, "--folder", str(folder), "--fit", value)

    assert result.returncode == 0, result.stderr
    assert path.read_bytes() == expected
    text = path.read_text()
    assert 'legacy_key: "kept" # unknown' in text
    assert read_fit(workspace, folder) == value


def test_relative_folder_resolves_against_the_workspace_jobs(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "docs-ready"

    result = run_script(workspace, SCRIPT, "--folder", "SWE/docs-ready", "--fit", "Marginal")

    assert result.returncode == 0, result.stderr
    assert read_fit(workspace, folder) == "Marginal"


def test_relative_folder_accepts_workspace_prefixed_path(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "docs-ready"

    result = run_script(workspace, SCRIPT, "--folder", "jobs/SWE/docs-ready", "--fit", "Good")

    assert result.returncode == 0, result.stderr
    assert read_fit(workspace, folder) == "Good"


def test_bare_folder_name_resolves_when_one_track_holds_it(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "docs-ready"

    result = run_script(workspace, SCRIPT, "--folder", "docs-ready", "--fit", "Strong")

    assert result.returncode == 0, result.stderr
    assert read_fit(workspace, folder) == "Strong"


def test_ambiguous_bare_folder_name_is_refused(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    first = workspace / "jobs" / "SWE" / "docs-ready"
    second = workspace / "jobs" / "Platform" / "docs-ready"
    shutil.copytree(first, second)
    before = [
        first.joinpath("meta.yml").read_bytes(),
        second.joinpath("meta.yml").read_bytes(),
    ]

    result = run_script(workspace, SCRIPT, "--folder", "docs-ready", "--fit", "Poor")

    assert result.returncode != 0
    assert "ambiguous" in result.stderr
    assert "Platform" in result.stderr and "SWE" in result.stderr
    assert [
        first.joinpath("meta.yml").read_bytes(),
        second.joinpath("meta.yml").read_bytes(),
    ] == before


@pytest.mark.parametrize("value", ["strong", "GOOD", "Excellent", "strong ", ""])
def test_invalid_fit_values_are_refused_and_change_nothing(tmp_path, value):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "docs-ready"
    path = folder / "meta.yml"
    before = path.read_bytes()

    result = run_script(workspace, SCRIPT, "--folder", str(folder), "--fit", value)

    assert result.returncode != 0
    assert "invalid choice" in result.stderr
    for allowed in VALUES:
        assert allowed in result.stderr
    assert path.read_bytes() == before


def test_missing_meta_yml_is_a_clear_error(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "not-an-application"
    before = sorted(entry.name for entry in folder.iterdir())

    result = run_script(workspace, SCRIPT, "--folder", str(folder), "--fit", "Good")

    assert result.returncode != 0
    assert "no meta.yml" in result.stderr
    assert "not an Application" in result.stderr
    assert sorted(entry.name for entry in folder.iterdir()) == before


def test_malformed_application_is_refused_without_temp_files(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "malformed"
    path = folder / "meta.yml"
    before = path.read_bytes()
    before_names = sorted(entry.name for entry in folder.iterdir())

    result = run_script(workspace, SCRIPT, "--folder", str(folder), "--fit", "Good")

    assert result.returncode != 0
    assert "meta.yml" in result.stderr
    assert "unexpected nesting" in result.stderr
    assert path.read_bytes() == before
    assert sorted(entry.name for entry in folder.iterdir()) == before_names


def test_other_files_in_the_folder_are_untouched(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "docs-ready"
    (folder / "notes.md").write_text("Owner notes\n")
    (folder / "fit.md").write_text("# Existing fit\n")
    (folder / "sent").mkdir(exist_ok=True)
    (folder / "sent" / "application.md").write_text("sent\n")
    watched = [
        folder / "notes.md",
        folder / "fit.md",
        folder / "sent" / "application.md",
    ]
    before = [path.read_bytes() for path in watched]

    result = run_script(workspace, SCRIPT, "--folder", str(folder), "--fit", "Good")

    assert result.returncode == 0, result.stderr
    assert [path.read_bytes() for path in watched] == before


def test_rewrites_the_fit_line_and_keeps_comments_and_unknown_keys(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    path = folder / "meta.yml"
    path.write_bytes((PATCH / "input.yml").read_bytes())

    result = run_script(workspace, SCRIPT, "--folder", str(folder), "--fit", "Poor")

    assert result.returncode == 0, result.stderr
    assert path.read_bytes() == (PATCH / "fit.golden.yml").read_bytes()
