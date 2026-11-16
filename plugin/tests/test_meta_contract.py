"""The Python reader and the Go store agree on the shared fixture Workspace."""

import json
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
META = ROOT / "plugin" / "scripts" / "meta.py"
FIXTURE = ROOT / "fixtures" / "workspace"

DEFAULTS = {
    "Folder": "",
    "Title": "",
    "Company": "",
    "Track": "",
    "Status": "",
    "Documents": "",
    "PDFs": [],
    "HasJD": False,
    "HasNotes": False,
    "HasFit": False,
    "HasGaps": False,
    "HasResume": False,
    "HasCoverLetter": False,
    "HasSent": False,
    "Schema": 0,
    "URL": "",
    "Closes": "",
    "Fit": "",
    "ResumeOnly": False,
    "StatusLog": [],
    "AppliedDate": None,
    "LastChange": None,
    "Posted": "",
    "PostedLabel": "",
    "Applicants": None,
    "Source": "",
    "FilterReason": "",
    "FilterNote": "",
    "FilterOverruled": False,
}


def normalise(application):
    """Mirror the Go test: zero values for absent keys, then drop Folder."""
    result = {}
    for key, default in DEFAULTS.items():
        value = "" if key == "Folder" else application.get(key, default)
        if key == "StatusLog":
            value = [
                {
                    "Status": entry.get("Status", ""),
                    "At": entry.get("At", ""),
                    "Approx": entry.get("Approx", False),
                }
                for entry in value
            ]
        elif key in ("AppliedDate", "LastChange") and value is not None:
            value = {
                "Status": value.get("Status", ""),
                "At": value.get("At", ""),
                "Approx": value.get("Approx", False),
            }
        result[key] = value
    return result


def scan(workspace):
    result = subprocess.run(
        [sys.executable, str(META), "--data", str(workspace)],
        capture_output=True,
        text=True,
        env={"PATH": "/usr/bin:/bin"},
    )
    assert result.returncode == 0, result.stderr
    return json.loads(result.stdout)


def test_every_fixture_application_matches_the_go_expectations(tmp_path):
    workspace = tmp_path / "workspace"
    shutil.copytree(FIXTURE, workspace)
    expected = json.loads((ROOT / "fixtures" / "expected.json").read_text())
    got = scan(workspace)

    assert len(got) == len(expected)
    malformed_seen = False
    for application, want in zip(got, expected):
        assert application["Malformed"] == want.get("MalformedExpected", False)
        if application["Malformed"]:
            malformed_seen = True
            assert application["Error"].startswith(
                f"{application['Folder']}{Path('/')}meta.yml:"
            )
        assert normalise(application) == normalise(want)
    assert malformed_seen, "the fixture Workspace should contain a malformed meta.yml"


def test_scan_order_follows_then_sorted_folder_names(tmp_path):
    workspace = tmp_path / "workspace"
    shutil.copytree(FIXTURE, workspace)
    expected = json.loads((ROOT / "fixtures" / "expected.json").read_text())

    assert [app["Title"] for app in scan(workspace)] == [
        app["Title"] for app in expected
    ]


def test_newer_meta_schema_is_rejected_with_upgrade_guidance(tmp_path):
    workspace = tmp_path / "workspace"
    shutil.copytree(FIXTURE, workspace)
    folder = workspace / "jobs" / "SWE" / "fern"
    (folder / "meta.yml").write_text(
        "schema: 2\ntitle: Engineer\ncompany: Acme\ntrack: SWE\n"
    )

    app = next(item for item in scan(workspace) if item["Folder"] == str(folder))

    assert app["Malformed"] is True
    assert "upgrade rolecall" in app["Error"]


def test_malformed_meta_schema_number_is_refused(tmp_path):
    workspace = tmp_path / "workspace"
    shutil.copytree(FIXTURE, workspace)
    folder = workspace / "jobs" / "SWE" / "fern"
    (folder / "meta.yml").write_text(
        "schema: 2_0\ntitle: Engineer\ncompany: Acme\ntrack: SWE\n"
    )

    app = next(item for item in scan(workspace) if item["Folder"] == str(folder))

    assert app["Malformed"] is True
    assert "unsupported schema" in app["Error"]


@pytest.mark.parametrize(
    "broken",
    [
        "title: Broken\r\ncompany: Example\r\ntrack: SWE\r\n",
        "title: Broken\ncompany: Example\ntrack: SWE",
        " title: Broken\ncompany: Example\ntrack: SWE\n",
        "title: Broken\ntitle: Twice\ncompany: Example\ntrack: SWE\n",
        "title: Broken\ncompany: Example\ntrack: SWE\nfit: Great\n",
        "title: Broken\ncompany: Example\ntrack: SWE\nstatus_log:\n  - {status: Nope, at: 2026-01-01}\n",
        "title: Broken\ncompany: Example\ntrack: SWE\nstatus_log:\n  - {status: Lead, at: not-a-date}\n",
    ],
)
def test_malformed_meta_is_reported_not_raised(tmp_path, broken):
    workspace = tmp_path / "workspace"
    folder = workspace / "jobs" / "SWE" / "broken"
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_text(broken)

    got = scan(workspace)

    assert len(got) == 1
    assert got[0]["Malformed"] is True
    assert got[0]["Error"].startswith(f"{folder}{Path('/')}meta.yml:")
    assert isinstance(got[0]["Title"], str)
