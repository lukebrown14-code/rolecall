"""Migration from the dashboard-era layout, tested at the CLI seam.

Every test copies ``fixtures/dashboard-era`` to a temporary Workspace and runs
``plugin/scripts/migrate.py`` as a subprocess. Assertions are on files, bytes,
exit codes and stdout; nothing imports the script.
"""

import re
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MIGRATE = ROOT / "plugin" / "scripts" / "migrate.py"
FIXTURE = ROOT / "fixtures" / "dashboard-era"

ARCHIVE_FILES = [
    "TRACKER.md",
    "SHORTLIST.md",
    "DISMISSED.md",
    "INBOX.md",
    "sweep-candidates.md",
]


def copy_workspace(tmp_path):
    workspace = tmp_path / "workspace"
    shutil.copytree(FIXTURE, workspace)
    return workspace


def run(workspace, *args):
    return subprocess.run(
        [sys.executable, str(MIGRATE), "--data", str(workspace), *args],
        capture_output=True,
        text=True,
    )


def snapshot(root):
    return {
        str(path.relative_to(root)): path.read_bytes()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }


def meta(workspace, relative):
    return (workspace / relative / "meta.yml").read_text()


def statuses(text):
    found = re.findall(
        r"\{status: ([A-Za-z]+), at: ([^,}]+)(, approx: true)?\}", text
    )
    return [(status, at, approx or None) for status, at, approx in found]


def entry(text, status):
    return next(item for item in statuses(text) if item[0] == status)


def test_dry_run_writes_nothing_and_lists_the_whole_plan(tmp_path):
    workspace = copy_workspace(tmp_path)
    before = snapshot(workspace)

    result = run(workspace, "--dry-run")

    assert result.returncode == 0, result.stderr
    assert snapshot(workspace) == before
    out = result.stdout
    # every existing folder and the keys/Statuses it gains
    assert "jobs/SWE/acme-dev-2026-01-01" in out
    assert "jobs/SWE/globex-engineer-2026-02-01" in out
    assert "jobs/SWE/initech-analyst-2026-03-01" in out
    assert "jobs/Platform/umbrella-senior-2026-04-01" in out
    assert "jobs/Platform/hooli-pm-2026-05-01" in out
    assert "jobs/SWE/wayne-qa-2026-06-01" in out
    assert "jobs/SWE/stark-ops-2026-07-01" in out
    assert "jobs/SWE/cyberdyne-ml-2026-08-01" in out
    assert "schema" in out
    assert "title" in out and "company" in out and "track" in out and "url" in out
    # Status mapping and rebuilt dates
    assert "Draft" in out
    assert "Interviewing" in out
    assert "Offer" in out
    assert "Rejected" in out
    assert "Withdrawn" in out
    assert "approx" in out
    # Fit mapping branches and the one that cannot be mapped
    assert "Strong" in out and "Good" in out and "Marginal" in out and "Poor" in out
    assert "Maybe later" in out and "unmappable" in out.lower()
    # conflicts are reported
    assert "conflict" in out.lower()
    # jd front matter removal
    assert "front matter" in out.lower()
    # new Lead and Dismissed folders
    assert "jobs/SWE/northwind-platform-engineer-2026-09-01" in out
    assert "jobs/SWE/northwind-platform-engineer-2026-09-01-2" in out
    assert "jobs/SWE/initech-site-reliability-engineer-2026-08-15" in out
    # dropped shortlist rows and every archived file
    assert "Data Analyst" in out
    for name in ARCHIVE_FILES:
        assert f"jobs/{name}" in out
    assert "jobs/_archive/" in out
    assert "roles.json" not in out


def test_apply_maps_tracker_statuses_and_writes_schema_one(tmp_path):
    workspace = copy_workspace(tmp_path)

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    acme = meta(workspace, "jobs/SWE/acme-dev-2026-01-01")
    assert "schema: 1" in acme
    assert 'title: "Developer"' in acme
    assert 'company: "Acme"' in acme
    assert 'track: "SWE"' in acme
    assert "https://example.test/jobs/acme-dev" in acme
    assert 'fit: "Strong"' in acme
    assert statuses(acme) == [("Draft", "2026-01-01", ", approx: true")]

    umbrella = meta(workspace, "jobs/Platform/umbrella-senior-2026-04-01")
    assert 'track: "Platform"' in umbrella
    assert statuses(umbrella) == [("Interviewing", "2026-04-01", ", approx: true")]
    assert 'fit: "Marginal"' in umbrella

    assert statuses(meta(workspace, "jobs/Platform/hooli-pm-2026-05-01")) == [
        ("Offer", "2026-05-01", ", approx: true")
    ]
    assert statuses(meta(workspace, "jobs/SWE/wayne-qa-2026-06-01")) == [
        ("Rejected", "2026-06-01", ", approx: true")
    ]
    assert statuses(meta(workspace, "jobs/SWE/stark-ops-2026-07-01")) == [
        ("Withdrawn", "2026-07-01", ", approx: true")
    ]
    assert statuses(meta(workspace, "jobs/SWE/cyberdyne-ml-2026-08-01")) == [
        ("Applied", "2026-08-01", ", approx: true")
    ]


def test_bulk_stamped_logs_get_an_approximate_applied_entry(tmp_path):
    workspace = copy_workspace(tmp_path)

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    # A single non-Applied bulk stamp is replaced by Applied at the legacy
    # `applied` date (preferred over the tracker Date), then the mapped Status.
    initech = meta(workspace, "jobs/SWE/initech-analyst-2026-03-01")
    assert statuses(initech) == [("Applied", "2026-02-20", ", approx: true")]

    # A single Applied stamp is kept, and the mapped Status is appended.
    globex = meta(workspace, "jobs/SWE/globex-engineer-2026-02-01")
    assert statuses(globex) == [
        ("Applied", "2026-01-20", None),
        ("Interviewing", "2026-02-01", ", approx: true"),
    ]


def test_legacy_keys_and_comments_survive(tmp_path):
    workspace = copy_workspace(tmp_path)

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    text = meta(workspace, "jobs/SWE/initech-analyst-2026-03-01")
    for line in (
        "# hand note kept",
        "applied: 2026-02-20",
        "applied_approx: true",
        "followup: 2026-03-06",
        'contact: "Sam"',
        "salary:",
        "outcome:",
        "outcome_date:",
        "rejection_reason:",
        "custom_key: keep me # inline comment",
    ):
        assert line in text


def test_jd_front_matter_is_removed_and_the_body_is_byte_identical(tmp_path):
    workspace = copy_workspace(tmp_path)
    original = (workspace / "jobs/SWE/acme-dev-2026-01-01/jd.md").read_bytes()
    body = original.split(b"---\n", 2)[2]

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    migrated = (workspace / "jobs/SWE/acme-dev-2026-01-01/jd.md").read_bytes()
    assert migrated == body
    assert not migrated.startswith(b"---")
    assert b"role:" not in migrated


def test_shortlist_new_rows_become_leads_and_passed_rows_are_dropped(tmp_path):
    workspace = copy_workspace(tmp_path)

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    first = workspace / "jobs/SWE/northwind-platform-engineer-2026-09-01"
    second = workspace / "jobs/SWE/northwind-platform-engineer-2026-09-01-2"
    assert first.is_dir() and second.is_dir()
    text = (first / "meta.yml").read_text()
    assert 'title: "Platform Engineer"' in text
    assert 'company: "Northwind"' in text
    assert 'track: "SWE"' in text
    assert "https://example.test/jobs/short-new" in text
    assert statuses(text) == [("Lead", "2026-09-01", None)]
    assert "https://example.test/jobs/short-new" in (first / "jd.md").read_text()
    assert "Strong team, good stack" in (first / "jd.md").read_text()
    assert (first / "jd.md").read_text().startswith("# ")

    assert not (workspace / "jobs/SWE/contoso-data-analyst-2026-09-01").exists()
    assert not (workspace / "jobs/SWE/initech-backend-engineer-2026-09-01").exists()


def test_dismissed_rows_become_one_approximate_dismissed_entry(tmp_path):
    workspace = copy_workspace(tmp_path)

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    folder = workspace / "jobs/SWE/initech-site-reliability-engineer-2026-08-15"
    assert folder.is_dir()
    text = (folder / "meta.yml").read_text()
    assert statuses(text) == [("Dismissed", "2026-08-15", ", approx: true")]
    assert "https://example.test/jobs/dismissed-sre" in text
    assert "https://example.test/jobs/dismissed-sre" in (folder / "jd.md").read_text()


def test_unmappable_fit_is_reported_and_not_written(tmp_path):
    workspace = copy_workspace(tmp_path)

    dry = run(workspace, "--dry-run")
    assert dry.returncode == 0, dry.stderr
    assert "Maybe later" in dry.stdout
    assert "unmappable" in dry.stdout.lower()

    result = run(workspace)
    assert result.returncode == 0, result.stderr
    text = meta(workspace, "jobs/SWE/cyberdyne-ml-2026-08-01")
    assert "fit:" not in text


def test_archive_files_move_and_nothing_is_deleted(tmp_path):
    workspace = copy_workspace(tmp_path)
    before = snapshot(workspace)

    result = run(workspace)

    assert result.returncode == 0, result.stderr
    archive = workspace / "jobs" / "_archive"
    for name in ARCHIVE_FILES:
        assert not (workspace / "jobs" / name).exists()
        assert (archive / name).exists()
        assert (archive / name).read_bytes() == before[f"jobs/{name}"]
    # roles.json is workspace configuration, not a migration artefact.
    assert (workspace / "jobs" / "roles.json").exists()
    # Nothing was deleted: every original file still exists somewhere.
    after = snapshot(workspace)
    for relative, data in before.items():
        name = Path(relative).name
        if relative.startswith("jobs/") and "/" not in relative[5:] and name in ARCHIVE_FILES:
            assert after[f"jobs/_archive/{name}"] == data
        elif relative.endswith("jd.md"):
            assert relative in after
        else:
            assert after.get(relative) == data or relative in after


def test_malformed_meta_aborts_before_any_write(tmp_path):
    workspace = copy_workspace(tmp_path)
    bad = workspace / "jobs/SWE/globex-engineer-2026-02-01/meta.yml"
    bad.write_text("title: Only a title\n")
    before = snapshot(workspace)

    result = run(workspace)

    assert result.returncode != 0
    assert result.stderr.strip()
    assert snapshot(workspace) == before


def test_second_run_changes_nothing(tmp_path):
    workspace = copy_workspace(tmp_path)
    first = run(workspace)
    assert first.returncode == 0, first.stderr
    once = snapshot(workspace)

    second = run(workspace)

    assert second.returncode == 0, second.stderr
    assert snapshot(workspace) == once
    assert "no changes" in second.stdout.lower()
    assert not list((workspace / "jobs").glob("*/.meta-*"))
