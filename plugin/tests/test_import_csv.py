"""import_csv.py turns a CSV export into Application folders through meta.create.

The fixture export (fixtures/import/applications.csv) is fictional. Tests assert on the files
written: a Lead or later Status with an approximate, date-only status_log, notes kept
verbatim, and nothing invented for a row that lacks a fact.
"""

import json
import sys

import pytest

from conftest import ROOT, copy_fixture_workspace, run_script

SCRIPT = ROOT / "plugin" / "scripts" / "import_csv.py"
EXPORT = ROOT / "fixtures" / "import" / "applications.csv"
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))

import import_csv  # noqa: E402
import meta as meta_store  # noqa: E402

STATUS_ARGS = ("--status-map", "ghosted=Rejected", "--status-map", "phone screen=Interviewing")


def applications(workspace, before=frozenset()):
    """The Applications added since ``before`` (a names() snapshot), by "title|company"."""
    return {app.title + "|" + app.company: app for app in meta_store.scan(workspace)
            if app.folder.name not in before}


def names(workspace):
    return {f.name for f in (workspace / "jobs").glob("*/*") if f.is_dir()}


def importer(workspace, *args):
    return run_script(workspace, SCRIPT, "--file", str(EXPORT), *args)


# ------------------------------------------------------------------ mapping


def test_detects_columns_from_their_headers():
    headers = ["Position", "Employer", "Stage", "Applied On", "Link", "Comments", "Other"]
    assert import_csv.detect_mapping(headers) == {
        "title": "Position", "company": "Employer", "status": "Stage",
        "applied": "Applied On", "url": "Link", "notes": "Comments",
    }


def test_a_column_is_never_guessed_from_an_unfamiliar_header():
    assert import_csv.detect_mapping(["What", "Who", "When"]) == {}


def test_map_overrides_a_column_and_refuses_an_unknown_one():
    headers = ["What", "Who"]
    mapping = import_csv.apply_overrides(headers, {}, ["title=What", "company=Who"])
    assert mapping == {"title": "What", "company": "Who"}
    with pytest.raises(import_csv.ImportError_, match="no column named"):
        import_csv.apply_overrides(headers, {}, ["title=Nope"])
    with pytest.raises(import_csv.ImportError_, match="field=Column"):
        import_csv.apply_overrides(headers, {}, ["colour=What"])


def test_inspect_lists_mapping_and_unknown_statuses(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    result = importer(workspace, "--inspect", "--json")
    assert result.returncode == 0, result.stderr
    report = json.loads(result.stdout)
    assert report["rows"] == 9
    assert report["mapping"]["applied"] == "Date Applied"
    assert report["unused_columns"] == ["Salary"]
    assert report["statuses"]["Ghosted"]["status"] is None
    assert report["statuses"]["Phone screen"]["status"] == "Interviewing"
    assert not (workspace / "jobs" / "SWE" / "northwind-traders-backend-engineer-2026-11-17").exists()


# ------------------------------------------------------------------ dates and statuses


@pytest.mark.parametrize("text,order,expected", [
    ("2026-03-12", "dmy", "2026-03-12"),
    ("12/03/2026", "dmy", "2026-03-12"),
    ("12/03/2026", "mdy", "2026-12-03"),
    ("25/03/2026", "mdy", "2026-03-25"),  # only valid day-first
    ("5 Jan 2026", "dmy", "2026-01-05"),
    ("Feb 3, 2026", "dmy", "2026-02-03"),
    ("3rd March 2026", "dmy", "2026-03-03"),
    ("1/2/26", "dmy", "2026-02-01"),
])
def test_parse_date(text, order, expected):
    assert import_csv.parse_date(text, order) == (expected, "")


@pytest.mark.parametrize("text", ["31/02/2026", "soon", "2026-13-01"])
def test_an_unreadable_date_is_left_out_not_guessed(text):
    value, why = import_csv.parse_date(text)
    assert value is None and why


def test_an_empty_date_is_not_a_problem():
    assert import_csv.parse_date("  ") == (None, "")


def test_status_words_and_overrides():
    assert import_csv.map_status("Phone Screen") == "Interviewing"
    assert import_csv.map_status("  REJECTED ") == "Rejected"
    assert import_csv.map_status("Ghosted") is None
    assert import_csv.map_status("Ghosted", {"ghosted": "Rejected"}) == "Rejected"
    with pytest.raises(import_csv.ImportError_):
        import_csv.status_overrides(["ghosted=Banana"])


def test_status_log_dates_come_only_from_the_export():
    assert import_csv.status_log_for("Applied", "2026-03-12", "2026-11-17") == [
        ("Applied", "2026-03-12", True)]
    assert import_csv.status_log_for("Rejected", "2026-01-05", "2026-11-17") == [
        ("Applied", "2026-01-05", True), ("Rejected", "2026-01-05", True)]
    assert import_csv.status_log_for("Lead", "2026-01-05", "2026-11-17") == [
        ("Lead", "2026-11-17", True)]
    assert import_csv.status_log_for("Offer", None, "2026-11-17") == [("Offer", "2026-11-17", True)]


# ------------------------------------------------------------------ dry run and apply


def test_dry_run_writes_nothing(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    before = names(workspace)
    result = importer(workspace, "--track", "SWE", "--dry-run", *STATUS_ARGS)
    assert result.returncode == 0, result.stderr
    assert names(workspace) == before
    assert "7 would be created, 2 skipped" in result.stdout
    assert "Nothing was written" in result.stdout


def test_apply_creates_folders_with_approximate_status_logs(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    before = names(workspace)
    result = importer(workspace, "--track", "SWE", *STATUS_ARGS)
    assert result.returncode == 0, result.stderr
    apps = applications(workspace, before)

    northwind = apps["Backend Engineer|Northwind Traders"]
    assert northwind.status == "Applied"
    assert [(e.status, e.at, e.approx) for e in northwind.status_log] == [
        ("Applied", "2026-03-12", True)]
    assert northwind.url == "https://careers.northwind.example/jobs/101"
    assert (northwind.folder / "notes.md").read_text() == "Referred by a friend\n"

    contoso = apps["Data Analyst|Contoso Ltd"]
    assert contoso.status == "Interviewing"
    assert [e.status for e in contoso.status_log] == ["Applied", "Interviewing"]
    assert "asked about SQL" in (contoso.folder / "notes.md").read_text()

    assert apps["Platform Engineer|Fabrikam Pty Ltd"].status == "Rejected"
    assert apps["Support Engineer|Tailspin Toys"].status == "Rejected"  # mapped by the Owner
    assert apps["QA Lead|Wingtip Group"].status == "Lead"  # no status, no date: a Lead
    assert apps["Developer Advocate|Proseware"].status == "Withdrawn"
    assert all(not app.malformed for app in apps.values())


def test_nothing_is_invented_for_a_row_with_missing_facts(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    before = names(workspace)
    importer(workspace, "--track", "SWE", *STATUS_ARGS)
    apps = applications(workspace, before)
    assert not any("Litware" in key for key in apps)  # no title: skipped, no placeholder
    sre = apps["Site Reliability Engineer|Adventure Works"]
    assert sre.url == ""  # "not a link" is not written as the URL
    assert "Imported link: not a link" in (sre.folder / "notes.md").read_text()
    # Its bad date was dropped, so its entry is the import day, still marked approximate.
    assert [(e.status, e.approx) for e in sre.status_log] == [("Offer", True)]
    for app in apps.values():
        assert (app.folder / "jd.md").read_text() == ""  # no ad text made up
        assert app.fit == "" and app.closes == ""


def test_an_unmapped_status_stops_the_import_before_any_write(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    before = names(workspace)
    result = importer(workspace, "--track", "SWE", "--status-map", "phone screen=Interviewing")
    assert result.returncode == 1
    assert "not mapped" in result.stderr and "--status-map" in result.stderr
    assert names(workspace) == before


def test_duplicates_in_the_file_and_in_the_workspace_are_skipped(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    importer(workspace, "--track", "SWE", *STATUS_ARGS)
    before = names(workspace)
    again = importer(workspace, "--track", "SWE", "--dry-run", *STATUS_ARGS)
    assert "0 would be created" in again.stdout
    assert again.stdout.count("looks like") + again.stdout.count("same URL") >= 6
    rerun = importer(workspace, "--track", "SWE", *STATUS_ARGS)
    assert rerun.returncode == 0
    assert names(workspace) == before


def test_within_file_duplicate_is_named(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    result = importer(workspace, "--track", "SWE", "--dry-run", *STATUS_ARGS)
    assert "skip    row 9: same role as row 2 in this file" in result.stdout


def test_allow_duplicate_creates_them(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    importer(workspace, "--track", "SWE", *STATUS_ARGS)
    result = importer(workspace, "--track", "SWE", "--allow-duplicate", *STATUS_ARGS)
    assert result.returncode == 0
    assert len(list((workspace / "jobs" / "SWE").glob("northwind-traders-backend-engineer-*"))) >= 2


def test_a_track_is_required_and_must_exist(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    result = importer(workspace, "--dry-run", *STATUS_ARGS)
    assert "no track: pass --track" in result.stdout
    bad = importer(workspace, "--track", "Nope", "--dry-run", *STATUS_ARGS)
    assert bad.returncode == 1 and "is not one of" in bad.stderr


def test_a_track_column_is_matched_to_the_workspace_tracks(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    export = tmp_path / "t.csv"
    export.write_text("Title,Company,Track\nEngineer,Acme,platform\nAnalyst,Beta,Marketing\n")
    result = run_script(workspace, SCRIPT, "--file", str(export), "--dry-run")
    assert "create  row 2" in result.stdout
    assert "skip    row 3: track 'Marketing' is not one of" in result.stdout


# ------------------------------------------------------------------ the file itself


def test_semicolon_and_tab_exports_are_read(tmp_path):
    for sep in (";", "\t"):
        export = tmp_path / "x.csv"
        export.write_text(sep.join(["Title", "Company", "Status"]) + "\n"
                          + sep.join(["Engineer", "Acme", "Applied"]) + "\n")
        headers, rows = import_csv.read_table(export)
        assert headers == ["Title", "Company", "Status"] and rows == [["Engineer", "Acme", "Applied"]]


def test_a_byte_order_mark_does_not_hide_the_first_header(tmp_path):
    export = tmp_path / "bom.csv"
    export.write_bytes(b"\xef\xbb\xbfTitle,Company\nEngineer,Acme\n")
    assert import_csv.read_table(export)[0][0] == "Title"


@pytest.mark.parametrize("name,content,message", [
    ("a.xlsx", b"PK", "export it as CSV"),
    ("empty.csv", b"Title,Company\n", "no data rows"),
    ("missing.csv", None, "cannot read"),
])
def test_unusable_files_are_one_line_errors(tmp_path, name, content, message):
    workspace = copy_fixture_workspace(tmp_path)
    path = tmp_path / name
    if content is not None:
        path.write_bytes(content)
    result = run_script(workspace, SCRIPT, "--file", str(path), "--track", "SWE", "--dry-run")
    assert result.returncode == 1
    assert message in result.stderr
    assert "Traceback" not in result.stderr


def test_missing_title_column_asks_for_a_mapping(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    export = tmp_path / "odd.csv"
    export.write_text("What,Who\nEngineer,Acme\n")
    result = run_script(workspace, SCRIPT, "--file", str(export), "--track", "SWE")
    assert result.returncode == 1 and "pass --map title=Column" in result.stderr
    ok = run_script(workspace, SCRIPT, "--file", str(export), "--track", "SWE",
                    "--map", "title=What", "--map", "company=Who")
    assert ok.returncode == 0, ok.stderr
