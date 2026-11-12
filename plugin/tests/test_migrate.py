"""Migration from the dashboard-era layout, tested at the CLI seam.

Every test copies ``fixtures/dashboard-era`` to a temporary Workspace and runs
``plugin/scripts/migrate.py`` as a subprocess. Assertions are on files, bytes,
exit codes and stdout; nothing imports the script.
"""

import importlib.util
import re
from collections import Counter
from pathlib import Path

from conftest import copy_fixture_workspace, run_script

ROOT = Path(__file__).resolve().parents[2]
MIGRATE = ROOT / "plugin" / "scripts" / "migrate.py"
FIXTURE = ROOT / "fixtures" / "dashboard-era"


def load_migrate():
    """Load migrate.py in process, the way the plugin runs it."""
    spec = importlib.util.spec_from_file_location("migrate_under_test", MIGRATE)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

ARCHIVE_FILES = [
    "TRACKER.md",
    "SHORTLIST.md",
    "DISMISSED.md",
    "INBOX.md",
    "sweep-candidates.md",
]


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
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    before = snapshot(workspace)

    result = run_script(workspace, MIGRATE, "--dry-run")

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
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)

    result = run_script(workspace, MIGRATE)

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
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    # A single non-Applied bulk stamp is kept, and Applied is prepended at the
    # legacy `applied` date (preferred over the tracker Date). The mapped
    # tracker Status (Applied) differs from the kept Draft, so it is appended
    # and the store reads the Application on its tracker Status. The preserved
    # Draft entry gains approx for its bare date.
    initech = meta(workspace, "jobs/SWE/initech-analyst-2026-03-01")
    assert statuses(initech) == [
        ("Applied", "2026-02-20", ", approx: true"),
        ("Draft", "2026-03-05", ", approx: true"),
        ("Applied", "2026-03-01", ", approx: true"),
    ]

    # A single Applied stamp is kept, and the mapped Status is appended.
    globex = meta(workspace, "jobs/SWE/globex-engineer-2026-02-01")
    assert statuses(globex) == [
        ("Applied", "2026-01-20", ", approx: true"),
        ("Interviewing", "2026-02-01", ", approx: true"),
    ]


def test_legacy_keys_and_comments_survive(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)

    result = run_script(workspace, MIGRATE)

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
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    original = (workspace / "jobs/SWE/acme-dev-2026-01-01/jd.md").read_bytes()
    body = original.split(b"---\n", 2)[2]

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    migrated = (workspace / "jobs/SWE/acme-dev-2026-01-01/jd.md").read_bytes()
    assert migrated == body
    assert not migrated.startswith(b"---")
    assert b"role:" not in migrated


def test_shortlist_new_rows_become_leads_and_passed_rows_are_dropped(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    first = workspace / "jobs/SWE/northwind-platform-engineer-2026-09-01"
    second = workspace / "jobs/SWE/northwind-platform-engineer-2026-09-01-2"
    assert first.is_dir() and second.is_dir()
    text = (first / "meta.yml").read_text()
    assert 'title: "Platform Engineer"' in text
    assert 'company: "Northwind"' in text
    assert 'track: "SWE"' in text
    assert "https://example.test/jobs/short-new" in text
    assert statuses(text) == [("Lead", "2026-09-01", ", approx: true")]
    assert "https://example.test/jobs/short-new" in (first / "jd.md").read_text()
    assert "Strong team, good stack" in (first / "jd.md").read_text()
    assert (first / "jd.md").read_text().startswith("# ")

    assert not (workspace / "jobs/SWE/contoso-data-analyst-2026-09-01").exists()
    assert not (workspace / "jobs/SWE/initech-backend-engineer-2026-09-01").exists()


def test_dismissed_rows_become_one_approximate_dismissed_entry(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    folder = workspace / "jobs/SWE/initech-site-reliability-engineer-2026-08-15"
    assert folder.is_dir()
    text = (folder / "meta.yml").read_text()
    assert statuses(text) == [("Dismissed", "2026-08-15", ", approx: true")]
    assert "https://example.test/jobs/dismissed-sre" in text
    assert "https://example.test/jobs/dismissed-sre" in (folder / "jd.md").read_text()


def test_unmappable_fit_is_reported_and_not_written(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)

    dry = run_script(workspace, MIGRATE, "--dry-run")
    assert dry.returncode == 0, dry.stderr
    assert "Maybe later" in dry.stdout
    assert "unmappable" in dry.stdout.lower()

    result = run_script(workspace, MIGRATE)
    assert result.returncode == 0, result.stderr
    text = meta(workspace, "jobs/SWE/cyberdyne-ml-2026-08-01")
    assert "fit:" not in text


def test_archive_files_move_and_nothing_is_deleted(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    before = snapshot(workspace)

    result = run_script(workspace, MIGRATE)

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
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    bad = workspace / "jobs/SWE/globex-engineer-2026-02-01/meta.yml"
    bad.write_text("title: Only a title\n")
    before = snapshot(workspace)

    result = run_script(workspace, MIGRATE)

    assert result.returncode != 0
    assert result.stderr.strip()
    assert snapshot(workspace) == before


def test_incomplete_pre_migration_meta_is_rebuilt(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    stub = workspace / "jobs/SWE/globex-engineer-2026-02-01/meta.yml"
    stub.write_text("status_log:\n  - {status: Applied, at: 2026-01-01}\n")

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    assert "incomplete pre-migration meta.yml ignored" in result.stdout
    text = meta(workspace, "jobs/SWE/globex-engineer-2026-02-01")
    assert "schema: 1" in text
    assert "Globex" in text
    assert entry(text, "Interviewing")[0] == "Interviewing"


def test_apply_writes_each_application_meta_once(tmp_path, monkeypatch):
    """Each Application is one read-modify-validate-write, not a patch then a rewrite."""
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    migrate = load_migrate()
    writes = Counter()
    real = migrate.meta.atomic_write

    def counting(path, data, mode):
        writes[Path(path)] += 1
        return real(path, data, mode)

    monkeypatch.setattr(migrate.meta, "atomic_write", counting)
    changes = migrate.plan(workspace)
    migrate.apply(workspace, changes)

    folders = [
        change.folder for change in changes if isinstance(change, migrate.FolderChange)
    ]
    assert folders
    for folder in folders:
        assert writes[folder / "meta.yml"] == 1, folder


def test_second_run_changes_nothing(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    first = run_script(workspace, MIGRATE)
    assert first.returncode == 0, first.stderr
    once = snapshot(workspace)

    second = run_script(workspace, MIGRATE)

    assert second.returncode == 0, second.stderr
    assert snapshot(workspace) == once
    assert "no changes" in second.stdout.lower()
    assert not list((workspace / "jobs").glob("*/.meta-*"))


def test_every_date_only_migrated_entry_carries_approx(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    result = run_script(workspace, MIGRATE)
    assert result.returncode == 0, result.stderr

    for path in (workspace / "jobs").glob("*/*/meta.yml"):
        for status, at, approx in statuses(path.read_text()):
            if re.fullmatch(r"\d{4}-\d{2}-\d{2}", at):
                assert approx == ", approx: true", f"{path}: {status} {at} has no approx"


def test_lead_maps_and_unknown_status_is_reported_and_skipped(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    ghosted = workspace / "jobs/SWE/fictional-ghosted-2026-09-02"
    ghosted.mkdir()
    (ghosted / "jd.md").write_text(
        "---\nrole: Ghosted Role\ncompany: Fictional\n---\nGhosted body.\n"
    )
    tracker = workspace / "jobs" / "TRACKER.md"
    tracker.write_text(
        tracker.read_text()
        + "| 2026-09-02 | Fictional | Ghosted Role | SWE | Strong | Ghosted | 0 | "
        "https://example.test/jobs/fictional-ghosted | "
        "`jobs/SWE/fictional-ghosted-2026-09-02` |\n"
    )
    before_jd = (ghosted / "jd.md").read_bytes()

    dry = run_script(workspace, MIGRATE, "--dry-run")
    assert dry.returncode == 0, dry.stderr
    assert "Ghosted" in dry.stdout
    assert "unknown" in dry.stdout.lower()

    result = run_script(workspace, MIGRATE)
    assert result.returncode == 0, result.stderr

    lead = meta(workspace, "jobs/SWE/fictional-lead-2026-09-03")
    assert statuses(lead) == [("Lead", "2026-09-03", ", approx: true")]

    # The unknown Status skips the whole folder: no meta.yml, jd front matter
    # left in place.
    assert not (ghosted / "meta.yml").exists()
    assert (ghosted / "jd.md").read_bytes() == before_jd


def test_folder_without_a_tracker_row_defaults_to_an_approximate_lead(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    orphan = workspace / "jobs" / "SWE" / "orphan-2026-09-10"
    orphan.mkdir()
    (orphan / "jd.md").write_text(
        "---\nrole: Orphan Role\ncompany: Fictional\n---\nOrphan role body.\n"
    )

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    text = (orphan / "meta.yml").read_text()
    assert statuses(text) == [("Lead", "2026-09-10", ", approx: true")]


def test_empty_fit_cell_is_silently_not_assessed(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    dry = run_script(workspace, MIGRATE, "--dry-run")
    assert dry.returncode == 0, dry.stderr
    # Only the free-text "Maybe later" (Cyberdyne) is unmappable; the "—" Fit
    # on the Fictional Lead row means "not assessed" and is not reported.
    assert dry.stdout.lower().count("unmappable") == 1
    assert "cyberdyne" in dry.stdout.lower()

    result = run_script(workspace, MIGRATE)
    assert result.returncode == 0, result.stderr
    assert "fit:" not in meta(workspace, "jobs/SWE/fictional-lead-2026-09-03")


def test_no_configured_tracks_reports_and_skips_new_rows(tmp_path):
    workspace = copy_fixture_workspace(tmp_path, FIXTURE)
    (workspace / "workspace.json").write_text("{}")

    result = run_script(workspace, MIGRATE)

    assert result.returncode == 0, result.stderr
    assert "no tracks" in result.stdout
    assert not (workspace / "jobs/SWE/northwind-platform-engineer-2026-09-01").exists()
    assert not (workspace / "jobs/SWE/initech-site-reliability-engineer-2026-08-15").exists()
