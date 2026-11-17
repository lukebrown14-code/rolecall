"""The Python meta writers mirror the Go store: patch, append-status, create.

Tests drive the script as a subprocess and compare bytes with the shared Go
goldens in ``internal/store/tests/testdata/patch``. Reading the goldens from their Go home
means the two implementations cannot drift apart silently.
"""

import json
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace, run_script

ROOT = Path(__file__).resolve().parents[2]
META = ROOT / "plugin" / "scripts" / "meta.py"
FIXTURE = ROOT / "fixtures" / "workspace"
PATCH = ROOT / "internal" / "store" / "tests" / "testdata" / "patch"

VALUES = {
    "schema": "1",
    "title": "Senior Engineer",
    "company": "Other Co",
    "track": "Other",
    "url": "https://example.org/new",
    "closes": "2027-01-01",
    "fit": "Poor",
    "resume_only": "true",
    "legacy": "updated",
}


@pytest.mark.parametrize("key", sorted(VALUES))
def test_patch_matches_the_go_golden(tmp_path, key):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    path = folder / "meta.yml"
    path.write_bytes((PATCH / "input.yml").read_bytes())
    path.chmod(0o640)

    result = run_script(workspace, META, "patch", "--folder", str(folder), "--set", f"{key}={VALUES[key]}")

    assert result.returncode == 0, result.stderr
    assert path.read_bytes() == (PATCH / f"{key}.golden.yml").read_bytes()
    assert path.stat().st_mode & 0o777 == 0o640


def test_patch_changes_only_the_target_lines(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    path = folder / "meta.yml"
    path.write_text(
        "schema: 1\n"
        "title: Engineer # keep title\n"
        "company: Changed externally\n"
        "track: SWE\n"
        "legacy: untouched # keep unknown\n"
        "status_log: []\n"
    )

    result = run_script(
        workspace,
        META,
        "patch",
        "--folder",
        str(folder),
        "--set",
        "title=Senior Engineer",
        "--set",
        "fit=Good",
    )

    assert result.returncode == 0, result.stderr
    assert path.read_text() == (
        'schema: 1\n'
        'title: "Senior Engineer" # keep title\n'
        'company: Changed externally\n'
        'track: SWE\n'
        'legacy: untouched # keep unknown\n'
        'fit: "Good"\n'
        'status_log: []\n'
    )


def test_patch_quotes_like_the_go_encoder(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    value = 'A "B" \\ C\t'

    result = run_script(workspace, META, "patch", "--folder", str(folder), "--set", f"title={value}")

    assert result.returncode == 0, result.stderr
    line = next(
        line
        for line in (folder / "meta.yml").read_text().splitlines()
        if line.startswith("title:")
    )
    assert line == 'title: "A \\"B\\" \\\\ C\\u0009"'


def test_patch_rejects_status_log_and_invalid_keys(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    before = (folder / "meta.yml").read_bytes()

    for setting in ("status_log=[]", "not a key=1"):
        result = run_script(workspace, META, "patch", "--folder", str(folder), "--set", setting)
        assert result.returncode != 0
        assert result.stderr.strip()
        assert (folder / "meta.yml").read_bytes() == before


def test_appending_the_current_status_is_byte_identical(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    before = (folder / "meta.yml").read_bytes()

    result = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "Applied",
        "--at",
        "2030-01-01T00:00:00Z",
    )

    assert result.returncode == 0, result.stderr
    assert (folder / "meta.yml").read_bytes() == before


@pytest.mark.parametrize("folder_name", ["status-draft", "orbit"])
def test_append_only_if_current_skips_a_non_lead_status(tmp_path, folder_name):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / folder_name
    before = (folder / "meta.yml").read_bytes()

    result = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "Draft",
        "--at",
        "2030-01-01T00:00:00Z",
        "--only-if-current",
        "Lead",
    )

    assert result.returncode == 0, result.stderr
    assert (folder / "meta.yml").read_bytes() == before


def test_append_only_if_current_appends_when_the_status_matches(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "fern"

    result = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "Draft",
        "--at",
        "2030-01-01T00:00:00Z",
        "--only-if-current",
        "Lead",
    )

    assert result.returncode == 0, result.stderr
    text = (folder / "meta.yml").read_text()
    assert text.endswith("  - {status: Draft, at: 2030-01-01T00:00:00Z}\n")
    assert text.count("status: Draft") == 1


def test_append_status_adds_one_utc_entry(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"

    result = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "Offer",
        "--at",
        "2026-11-11T13:30:00+11:00",
    )

    assert result.returncode == 0, result.stderr
    text = (folder / "meta.yml").read_text()
    assert text.endswith("  - {status: Offer, at: 2026-11-11T02:30:00Z}\n")
    assert text.count("status: Offer") == 1


def test_append_status_converts_an_empty_log(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    (folder / "meta.yml").write_text(
        "schema: 1\ntitle: Engineer\ncompany: Acme\ntrack: SWE\nstatus_log: []\n"
    )

    result = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "Draft",
        "--at",
        "2026-01-02",
    )

    assert result.returncode == 0, result.stderr
    assert (folder / "meta.yml").read_text() == (
        "schema: 1\ntitle: Engineer\ncompany: Acme\ntrack: SWE\n"
        "status_log:\n  - {status: Draft, at: 2026-01-02T00:00:00Z}\n"
    )


def test_invalid_status_is_rejected(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "orbit"
    before = (folder / "meta.yml").read_bytes()

    result = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "invalid",
        "--at",
        "2026-01-02",
    )

    assert result.returncode != 0
    assert "invalid Status" in result.stderr
    assert (folder / "meta.yml").read_bytes() == before


def test_writers_refuse_malformed_files_and_leave_no_temp(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "malformed"
    path = folder / "meta.yml"
    before = path.read_bytes()

    patch = run_script(workspace, META, "patch", "--folder", str(folder), "--set", "fit=Good")
    append = run_script(
        workspace,
        META,
        "append-status",
        "--folder",
        str(folder),
        "--status",
        "Offer",
        "--at",
        "2026-01-02",
    )

    assert patch.returncode != 0
    assert append.returncode != 0
    assert path.read_bytes() == before
    assert [entry.name for entry in folder.iterdir() if entry.name.startswith(".meta-")] == []


@pytest.mark.parametrize(
    "company,title,want",
    [
        ("Acme, Inc.", "Senior Engineer!", "acme-inc-senior-engineer-2026-11-11"),
        ("  Acme  ", "-- Engineer --", "acme-engineer-2026-11-11"),
        ("Café 東京", "Déveloper", "caf-d-veloper-2026-11-11"),
    ],
)
def test_create_slug_and_collision_suffixes(tmp_path, company, title, want):
    workspace = tmp_path / "workspace"
    workspace.mkdir()

    folders = []
    for _ in range(3):
        result = run_script(
            workspace,
            META,
            "create",
            "--track",
            "SWE",
            "--company",
            company,
            "--title",
            title,
            "--at",
            "2026-11-11T00:00:00Z",
            "--allow-duplicate",  # the same role three times is the point here
        )
        assert result.returncode == 0, result.stderr
        folders.append(Path(result.stdout.strip()))

    assert [folder.name for folder in folders] == [want, f"{want}-2", f"{want}-3"]


def test_create_writes_only_supplied_keys_and_one_lead(tmp_path):
    workspace = tmp_path / "workspace"
    workspace.mkdir()

    result = run_script(
        workspace,
        META,
        "create",
        "--track",
        "SWE",
        "--company",
        "Acme",
        "--title",
        "Engineer #1",
        "--at",
        "2026-11-11T00:00:00Z",
    )

    assert result.returncode == 0, result.stderr
    folder = Path(result.stdout.strip())
    text = (folder / "meta.yml").read_text()
    assert text == (
        'schema: 1\n'
        'title: "Engineer #1"\n'
        'company: "Acme"\n'
        'track: "SWE"\n'
        'status_log:\n'
        '  - {status: Lead, at: 2026-11-11T00:00:00Z}\n'
    )
    for key in ("url:", "closes:", "fit:"):
        assert key not in text
    assert (folder / "jd.md").read_bytes() == b""
    assert sorted(entry.name for entry in folder.iterdir()) == ["jd.md", "meta.yml"]


def test_create_records_the_optional_fields(tmp_path):
    workspace = tmp_path / "workspace"
    workspace.mkdir()

    result = run_script(
        workspace,
        META,
        "create",
        "--track",
        "SWE",
        "--company",
        "Acme",
        "--title",
        "Engineer",
        "--url",
        "https://example.test/1",
        "--closes",
        "2026-12-01",
        "--fit",
        "Good",
        "--at",
        "2026-11-11T00:00:00Z",
    )

    assert result.returncode == 0, result.stderr
    text = (Path(result.stdout.strip()) / "meta.yml").read_text()
    assert 'url: "https://example.test/1"' in text
    assert 'closes: "2026-12-01"' in text
    assert 'fit: "Good"' in text


def test_create_rejects_an_unsafe_track(tmp_path):
    workspace = tmp_path / "workspace"
    workspace.mkdir()

    result = run_script(
        workspace,
        META,
        "create",
        "--track",
        "..",
        "--company",
        "Acme",
        "--title",
        "Engineer",
        "--at",
        "2026-11-11T00:00:00Z",
    )

    assert result.returncode != 0
    assert not (workspace / "jobs").exists()


def test_find_by_url_matches_every_status(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)

    result = run_script(workspace, META, "find-url", "https://example.test/shared")

    assert result.returncode == 0, result.stderr
    matches = json.loads(result.stdout)
    assert len(matches) == 6
    assert {match["Status"] for match in matches} >= {"Dismissed", "Offer", "Rejected"}


def test_find_by_url_is_exact(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)

    result = run_script(workspace, META, "find-url", "https://example.test/shar")

    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) == []


def _load_meta():
    import importlib.util

    spec = importlib.util.spec_from_file_location("meta_under_test", META)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_dedupe_status_log_keeps_the_last_of_each_repeat():
    meta = _load_meta()
    log = [
        ("Applied", "2026-03-01", True),
        ("Lead", "2026-03-05", True),
        ("Applied", "2026-03-01", True),
        ("Interviewing", "2026-04-01T00:00:00Z", False),
        ("Interviewing", "2026-04-01T00:00:00Z", False),
    ]

    assert meta.dedupe_status_log(log) == [
        ("Lead", "2026-03-05", True),
        ("Applied", "2026-03-01", True),
        ("Interviewing", "2026-04-01T00:00:00Z", False),
    ]


@pytest.mark.parametrize("writer", ["set_status_log", "update"])
def test_whole_log_writes_drop_exact_repeats(tmp_path, writer):
    meta = _load_meta()
    workspace = copy_fixture_workspace(tmp_path)
    folder = workspace / "jobs" / "SWE" / "status-draft"
    log = [
        ("Lead", "2026-01-01T00:00:00Z", False),
        ("Applied", "2026-01-02T00:00:00Z", False),
        ("Applied", "2026-01-02T00:00:00Z", False),
    ]

    if writer == "set_status_log":
        meta.set_status_log(folder, log)
    else:
        meta.update(folder, {}, log)

    application = meta.rescan(folder)
    assert [(e.status, e.at) for e in application.status_log] == [
        ("Lead", "2026-01-01T00:00:00Z"),
        ("Applied", "2026-01-02T00:00:00Z"),
    ]
    assert application.status == "Applied"


def test_duplicate_status_entries_names_each_repeat_once(tmp_path):
    meta = _load_meta()
    folder = tmp_path / "app"
    folder.mkdir()
    (folder / "meta.yml").write_text(
        "schema: 1\ntitle: T\ncompany: C\ntrack: SWE\nstatus_log:\n"
        "  - {status: Applied, at: 2026-01-02T00:00:00Z}\n"
        "  - {status: Applied, at: 2026-01-02T00:00:00Z}\n"
        "  - {status: Applied, at: 2026-01-02T00:00:00Z}\n"
    )

    application = meta.rescan(folder)

    assert meta.duplicate_status_entries(application.status_log) == [
        ("Applied", "2026-01-02T00:00:00Z")
    ]
