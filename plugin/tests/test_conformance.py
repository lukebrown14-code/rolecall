"""meta.yml rules shared with the Go store. Both sides assert ``fixtures/conformance.json``
(``internal/store/tests/conformance_test.go`` and ``internal/schedule/tests/conformance_test.go``),
so a rule changed on one side fails here or there until the other agrees.
"""

import json
import re
import sys
import time
from datetime import datetime
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))

import adfacts  # noqa: E402
import meta  # noqa: E402
import rules  # noqa: E402
import workspaceio  # noqa: E402

CONFORMANCE = json.loads((ROOT / "fixtures" / "conformance.json").read_text())

# The shared vectors the Python Plugin does not meet yet, by the audit finding that tracks
# each (reports/code-audit-2026-11-18). A listed vector is skipped while it fails and fails
# once it passes, so whoever fixes the finding deletes its entry here. The Go lists are
# knownDrift in internal/store/tests/conformance_test.go and internal/schedule/tests/conformance_test.go.
KNOWN_DRIFT = {}


def _drift(vector):
    for finding, entry in KNOWN_DRIFT.items():
        if vector in entry["vectors"]:
            return finding
    return None


def _conform(vector, problems):
    """Assert a shared vector: skip a known drift while it fails, fail once it passes."""
    finding = _drift(vector)
    if finding is None:
        assert not problems, problems
    elif problems:
        pytest.skip(f"known drift {finding} ({KNOWN_DRIFT[finding]['reason']}): {problems}")
    else:
        pytest.fail(f"{vector} now conforms: remove it from KNOWN_DRIFT[{finding!r}]")


def _ids(case):
    return case["name"]


def test_known_drift_names_shared_vectors():
    names = {f"{section}/{case['name']}" for section in ("malformed", "slugs", "follow_up", "posting_age")
             for case in CONFORMANCE[section]}
    for finding, entry in KNOWN_DRIFT.items():
        assert entry["vectors"] <= names, (finding, entry["vectors"] - names)


@pytest.mark.parametrize("case", CONFORMANCE["lines"], ids=lambda c: c["line"])
def test_meta_lines_match_the_shared_conformance_fixture(tmp_path, case):
    folder = tmp_path / "jobs" / "SWE" / "lead"
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_text(CONFORMANCE["base"] + case["line"] + "\n")
    app = meta.rescan(folder)
    assert (app.malformed is None) == case["valid"], app.malformed


@pytest.mark.parametrize("case", CONFORMANCE["malformed"], ids=_ids)
def test_malformed_texts_match_the_shared_conformance_fixture(tmp_path, case):
    folder = tmp_path / "jobs" / "SWE" / "lead"
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_bytes(case["text"].encode("utf-8"))
    app = meta.rescan(folder)
    problems = []
    if (app.malformed is None) != case["valid"]:
        problems.append(f"valid = {app.malformed is None}, want {case['valid']} ({app.malformed})")
    elif not case["valid"]:
        match = re.search(r"meta\.yml:(\d+): ", app.malformed)
        if match is None or int(match.group(1)) != case["line"]:
            problems.append(f"error {app.malformed!r}, want line {case['line']}")
        if case.get("reason", "") not in app.malformed:
            problems.append(f"error {app.malformed!r}, want it to say {case['reason']!r}")
    _conform("malformed/" + case["name"], problems)


@pytest.mark.parametrize("case", CONFORMANCE["slugs"], ids=_ids)
def test_folder_slugs_match_the_shared_conformance_fixture(tmp_path, case):
    try:
        got = workspaceio.unique_folder(tmp_path, case["company"], case["title"], case["date"]).name
        problems = [] if got == case["want"] else [f"folder {got!r}, want {case['want']!r}"]
    except OSError as error:
        problems = [f"unique_folder raised {error!r}"]
    _conform("slugs/" + case["name"], problems)


def _meta_text(case):
    text = CONFORMANCE["base"]
    if case.get("followed_up"):
        text += f"followed_up: {case['followed_up']}\n"
    if case.get("next_step"):
        text += f"next_step: {{at: {case['next_step']}}}\n"
    text += "status_log:\n"
    for entry in case["status_log"]:
        approx = ", approx: true" if entry.get("approx") else ""
        text += f"  - {{status: {entry['status']}, at: {entry['at']}{approx}}}\n"
    return text


@pytest.fixture
def owner_zone(monkeypatch):
    """Set the process zone to a fixed UTC offset, as the Owner's machine would be."""
    if not hasattr(time, "tzset"):
        pytest.skip("time.tzset is not available on this platform")

    def set_zone(moment):
        minutes = int(moment.utcoffset().total_seconds() // 60)
        sign = "-" if minutes >= 0 else "+"  # POSIX TZ offsets are west-positive
        monkeypatch.setenv("TZ", f"RCT{sign}{abs(minutes) // 60:02d}:{abs(minutes) % 60:02d}")
        time.tzset()

    yield set_zone
    monkeypatch.undo()
    time.tzset()


@pytest.mark.parametrize("case", CONFORMANCE["follow_up"], ids=_ids)
def test_follow_up_matches_the_shared_conformance_fixture(tmp_path, owner_zone, case):
    now = datetime.fromisoformat(case["now"])
    owner_zone(now)
    folder = tmp_path / "jobs" / "SWE" / "lead"
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_text(_meta_text(case))
    app = meta.rescan(folder)
    assert app.malformed is None, app.malformed
    result = meta.follow_up_status(app, today=now.date(), days=case["days"])
    want = case["want"]
    got = {
        "from": result["From"],
        "since": result["Since"],
        "due": result["Due"],
        # DaysPast is the count past due_on, null when the Application cannot be due.
        "days_past": result["DaysPast"] if result["Status"] in meta.FOLLOW_UP_STATUSES else None,
    }
    problems = [f"{key} {got[key]!r}, want {want[key]!r}" for key in want if got[key] != want[key]]
    _conform("follow_up/" + case["name"], problems)


@pytest.mark.parametrize("case", CONFORMANCE["posting_age"], ids=_ids)
def test_posting_age_matches_the_shared_conformance_fixture(case):
    raw = {"expiry_warning_days": case["expiry_warning_days"]}
    if case["max_posting_age_days"] is not None:
        raw["max_posting_age_days"] = case["max_posting_age_days"]
    found = rules.rules_from(raw, case.get("sources", ()))
    source = case.get("source", "")
    age = rules.age_of(case["posted"], today=datetime.strptime(case["today"], "%Y-%m-%d").date())
    finding = rules.check_age(age, found, source)
    want = case["want"]
    got = {
        "limit": found.age_limit(source),
        "age_days": age.min_days,
        "expired": finding is not None and finding.reason == rules.EXPIRED,
    }
    problems = [f"{key} {got[key]!r}, want {want[key]!r}" for key in got if got[key] != want[key]]
    _conform("posting_age/" + case["name"], problems)


def test_ad_fact_tables_match_the_shared_conformance_fixture():
    for key, values in CONFORMANCE["enums"].items():
        assert meta.FACTS[key][2] == set(values), key
    enums = {k for k, (_, _, kind) in meta.FACTS.items() if isinstance(kind, set)}
    assert enums == set(CONFORMANCE["enums"])
    assert adfacts.ANNUAL == CONFORMANCE["payPerYear"]
    assert adfacts.FX_AUD == CONFORMANCE["payFX"]
