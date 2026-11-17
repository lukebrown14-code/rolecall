"""drift.py lists unsent drafts that break the fact rules against the current Profile.

The URL check takes an injected fetcher, so nothing here uses the network.
"""

import importlib.util
import json
import sys
from pathlib import Path

import pytest
from conftest import run_script

SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"
sys.path.insert(0, str(SCRIPTS))
import httpfetch  # noqa: E402
import urlcheck  # noqa: E402

spec = importlib.util.spec_from_file_location("drift_under_test", SCRIPTS / "drift.py")
drift = importlib.util.module_from_spec(spec)
spec.loader.exec_module(drift)

PROFILE = """# Master Profile

## Experience

### Acme — Sydney, AU

**Dates:** 2022 – 2025
**Canonical title:** Software Engineer

- Cut build time by 40%
- Built the payments service

## Metrics register

| Metric | Value |
| :-- | :-- |
| Build time | 40% |

## Facts that are NOT available

- No cloud certifications
"""


def application(workspace, slug, status, resume="", letter=""):
    folder = workspace / "jobs" / "SWE" / slug
    folder.mkdir(parents=True)
    (folder / "jd.md").write_text("A job.\n")
    (folder / "meta.yml").write_text(
        'title: "Engineer"\ncompany: Acme\ntrack: SWE\nurl: https://example.test/job\n'
        f"status_log:\n  - {{status: {status}, at: 2026-01-01}}\n"
    )
    if resume:
        (folder / "resume.md").write_text(resume)
    if letter:
        (folder / "cover-letter.md").write_text(letter)
    return folder


@pytest.fixture
def workspace(tmp_path):
    (tmp_path / "profile").mkdir()
    (tmp_path / "profile" / "master-profile.md").write_text(PROFILE)
    return tmp_path


def snapshot(root):
    return sorted((str(p), p.read_bytes()) for p in root.rglob("*") if p.is_file())


def test_clean_drafts_are_not_stale(workspace):
    application(workspace, "ok", "Draft", resume="- Cut build time by 40%\n")
    assert drift.find_drift(workspace) == ([], [])


def test_a_removed_metric_makes_the_draft_stale(workspace):
    folder = application(workspace, "old", "Lead", resume="- Cut build time by 40%\n")
    profile = workspace / "profile" / "master-profile.md"
    profile.write_text(PROFILE.replace("40%", "").replace("by \n", "\n"))
    stale, _ = drift.find_drift(workspace)
    assert [(e["folder"], e["document"]) for e in stale] == [(str(folder), "resume.md")]
    assert "40%" in stale[0]["reasons"][0]


def test_a_fact_moved_to_not_available_makes_the_letter_stale(workspace):
    application(workspace, "rust", "Draft", letter="I have shipped Rust services.\n")
    assert drift.find_drift(workspace) == ([], [])
    profile = workspace / "profile" / "master-profile.md"
    profile.write_text(PROFILE.replace("- No cloud certifications", "- No Rust experience"))
    stale, _ = drift.find_drift(workspace)
    assert stale and "Rust" in stale[0]["reasons"][0]


def test_sent_applications_are_ignored(workspace):
    application(workspace, "sent", "Applied", resume="- Grew revenue by 900%\n")
    assert drift.find_drift(workspace) == ([], [])


def test_cli_exit_codes_json_and_read_only(workspace):
    folder = application(workspace, "bad", "Draft", resume="- Grew revenue by 900%\n")
    before = snapshot(workspace)
    result = run_script(workspace, SCRIPTS / "drift.py")
    assert result.returncode == 1
    assert "resume.md" in result.stdout and "900%" in result.stdout
    assert "1 stale document(s) in 1 Application(s)." in result.stdout
    data = json.loads(run_script(workspace, SCRIPTS / "drift.py", "--json").stdout)
    assert data["stale"][0]["document"] == "resume.md"
    assert before == snapshot(workspace)

    (folder / "resume.md").write_text("- Cut build time by 40%\n")
    clean = run_script(workspace, SCRIPTS / "drift.py")
    assert clean.returncode == 0 and "No drift" in clean.stdout


def fake(table):
    """A fetcher: None answers, a status raises that HTTP error, 0 is a connection failure."""

    def fetcher(url, timeout=0, max_bytes=0):
        outcome = table[url]
        if outcome is not None:
            raise httpfetch.FetchError(f"{url} -- failed", status=outcome or None)

    return fetcher


def test_urls_in_trims_markdown_punctuation():
    text = "See [site](https://a.test/p) and https://b.test/x_(y). Also https://c.test/z, ok."
    assert urlcheck.urls_in(text) == [
        "https://a.test/p",
        "https://b.test/x_(y)",
        "https://c.test/z",
    ]


def test_dead_blocked_and_unreachable_are_told_apart():
    table = {
        "https://ok.test": None,
        "https://gone.test": 404,
        "https://old.test": 410,
        "https://bot.test": 999,
        "https://down.test": 0,
    }
    results = urlcheck.check(list(table), fetcher=fake(table))
    assert results["https://ok.test"][0] == "ok"
    assert results["https://gone.test"][0] == "dead"
    assert results["https://old.test"][0] == "dead"
    assert results["https://bot.test"][0] == "unverified"
    assert results["https://down.test"][0] == "unreachable"
    assert sorted(urlcheck.failures(results)) == [
        "https://down.test",
        "https://gone.test",
        "https://old.test",
    ]
    assert urlcheck.unverified(results) == ["https://bot.test"]


def test_everything_unreachable_is_offline_not_failure():
    table = {"https://a.test": 0, "https://b.test": 0}
    results = urlcheck.check(list(table), fetcher=fake(table))
    assert urlcheck.offline(results) and urlcheck.failures(results) == []


def test_drift_urls_reports_dead_links(workspace):
    application(workspace, "links", "Draft", resume="- Cut build time by 40% https://gone.test/p\n")
    stale, warnings = drift.find_drift(
        workspace, check_urls=True, fetcher=fake({"https://gone.test/p": 404})
    )
    assert "dead link" in stale[0]["reasons"][0] and "gone.test" in stale[0]["reasons"][0]
    assert warnings == []
