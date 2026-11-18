"""Follow-up due in the Python reader mirrors the Go TUI.

The TUI's ``l`` writes ``followed_up: YYYY-MM-DD`` to ``meta.yml`` when the Owner has
followed up without a Status change. Follow-up due then counts ``follow_up_days`` from the
later of the last Status change and that day. A value that is not a date reads as unset,
as the Go store reads it, and every writer keeps the line.
"""

import json
import sys
from datetime import date
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace, run_script

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
META = SCRIPTS / "meta.py"
sys.path.insert(0, str(SCRIPTS))

import meta  # noqa: E402

TODAY = date(2026, 11, 16)


def write_app(workspace, status_log, followed_up=None):
    folder = workspace / "jobs" / "SWE" / "followco"
    folder.mkdir(parents=True)
    lines = ["schema: 1", "title: Engineer", "company: Followco", "track: SWE"]
    if followed_up is not None:
        lines.append(f"followed_up: {followed_up}")
    lines.append("status_log:")
    lines += [f"  - {{status: {s}, at: {at}}}" for s, at in status_log]
    (folder / "meta.yml").write_text("\n".join(lines) + "\n")
    return folder


def applied(workspace, changed, followed_up=None, status="Applied"):
    log = [("Lead", "2026-08-01T09:00:00Z"), ("Applied", changed)]
    if status == "Interviewing":
        log.append(("Interviewing", changed))
    return meta.rescan(write_app(workspace, log, followed_up))


def test_reader_parses_followed_up(tmp_path):
    app = applied(tmp_path, "2026-09-01T10:00:00Z", '"2026-11-01"')
    assert app.malformed is None
    assert app.followed_up == "2026-11-01"
    assert app.to_dict()["FollowedUp"] == "2026-11-01"
    assert "followed_up" not in app.unknown


@pytest.mark.parametrize("value", ["soon", "2026-13-01", "2026-02-30", "16/11/2026", ""])
def test_a_followed_up_that_is_not_a_date_reads_as_unset(tmp_path, value):
    app = applied(tmp_path, "2026-09-01T10:00:00Z", value)
    assert app.malformed is None
    assert app.followed_up == ""


def test_no_followed_up_counts_from_the_last_change(tmp_path):
    app = applied(tmp_path, "2026-10-01T10:00:00Z")
    got = meta.follow_up_status(app, TODAY, 14)
    assert (got["From"], got["Since"], got["DueOn"]) == ("2026-10-01", "last change", "2026-10-15")
    assert got["Due"] and got["DaysPast"] == 32


def test_a_recent_follow_up_holds_the_reminder_off(tmp_path):
    app = applied(tmp_path, "2026-09-01T10:00:00Z", "2026-11-13")
    got = meta.follow_up_status(app, TODAY, 14)
    assert (got["From"], got["Since"]) == ("2026-11-13", "followed_up")
    assert not got["Due"] and got["DaysPast"] == -11


def test_a_lapsed_follow_up_is_due_again_from_its_own_day(tmp_path):
    app = applied(tmp_path, "2026-08-20T10:00:00Z", "2026-11-01")
    got = meta.follow_up_status(app, TODAY, 14)
    assert got["Since"] == "followed_up" and got["Due"] and got["DaysPast"] == 1


def test_a_follow_up_before_a_later_status_change_counts_for_nothing(tmp_path):
    app = applied(tmp_path, "2026-10-20T10:00:00Z", "2026-10-01", status="Interviewing")
    got = meta.follow_up_status(app, TODAY, 14)
    assert (got["From"], got["Since"]) == ("2026-10-20", "last change")
    assert got["Due"] and got["DaysPast"] == 13


def test_a_follow_up_on_the_day_of_the_change_is_the_change(tmp_path):
    app = applied(tmp_path, "2026-10-20T10:00:00Z", "2026-10-20")
    assert meta.follow_up_from(app) == (date(2026, 10, 20), "last change")


def test_due_on_the_day_the_period_ends(tmp_path):
    app = applied(tmp_path, "2026-09-01T10:00:00Z", "2026-11-02")
    got = meta.follow_up_status(app, TODAY, 14)
    assert got["Due"] and got["DaysPast"] == 0


def test_only_sent_applications_are_due(tmp_path):
    folder = write_app(tmp_path, [("Lead", "2026-01-01T10:00:00Z")])
    got = meta.follow_up_status(meta.rescan(folder), TODAY, 14)
    assert not got["Due"] and got["DueOn"] == "" and got["DaysPast"] is None


def test_a_booked_next_step_holds_it_and_says_so(tmp_path):
    folder = write_app(tmp_path, [("Applied", "2026-09-01T10:00:00Z")])
    text = (folder / "meta.yml").read_text().replace(
        "status_log:", "next_step: {at: 2026-11-16T15:00, what: Screen}\nstatus_log:"
    )
    (folder / "meta.yml").write_text(text)
    got = meta.follow_up_status(meta.rescan(folder), TODAY, 14)
    assert got["HeldBy"] == "2026-11-16T15:00"
    assert not got["Due"] and got["DaysPast"] is None and got["DueOn"] == ""
    lapsed = meta.follow_up_status(meta.rescan(folder), date(2026, 11, 17), 14)
    assert lapsed["HeldBy"] == "" and lapsed["Due"]


def test_an_undone_move_does_not_restart_the_count(tmp_path):
    folder = write_app(
        tmp_path,
        [
            ("Applied", "2026-10-01T10:00:00Z"),
            ("Interviewing", "2026-11-10T10:00:00Z"),
            ("Applied", "2026-11-10T10:15:00Z"),
        ],
    )
    got = meta.follow_up_status(meta.rescan(folder), TODAY, 14)
    assert got["From"] == "2026-10-01" and got["Due"]


def test_follow_up_command_prints_the_reckoning(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = write_app(
        workspace, [("Applied", "2026-01-05T10:00:00Z")], followed_up=date.today().isoformat()
    )
    result = run_script(workspace, META, "follow-up", "--folder", str(folder), "--days", "10")
    assert result.returncode == 0, result.stderr
    got = json.loads(result.stdout)
    assert got["Since"] == "followed_up" and got["Days"] == 10
    assert got["DaysPast"] == -10 and not got["Due"]


def test_follow_up_command_refuses_days_out_of_range(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    folder = write_app(workspace, [("Applied", "2026-01-05T10:00:00Z")])
    result = run_script(workspace, META, "follow-up", "--folder", str(folder), "--days", "0")
    assert result.returncode == 1
    assert "1 to 90" in result.stderr


@pytest.mark.parametrize("value", ['"2026-11-01" # by l', "not-a-date"])
def test_writers_keep_the_followed_up_line(tmp_path, value):
    folder = write_app(tmp_path, [("Applied", "2026-09-01T10:00:00Z")], followed_up=value)
    line = f"followed_up: {value}\n"
    meta.patch(folder, {"fit": "Good"})
    assert meta.append_status(folder, "Interviewing", "2026-11-16T10:00:00Z")
    text = (folder / "meta.yml").read_text()
    assert line in text
    assert 'fit: "Good"' in text and "{status: Interviewing" in text
