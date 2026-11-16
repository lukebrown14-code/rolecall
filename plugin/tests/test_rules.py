"""Search rules: posting-age labels, the create gate, the dry run and dismissal by rule.

Rules live in workspace.json ``search_rules`` (SCHEMA.md). ``rules.py check`` lists what
a rule would remove before anything is written; ``--apply`` dismisses with a reason code
and never moves a folder. An Application the Owner restored is never dismissed again.
"""

import importlib.util
import json
import sys
from datetime import date
from pathlib import Path

import pytest

from conftest import run_script

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
RULES = SCRIPTS / "rules.py"
META = SCRIPTS / "meta.py"
SWEEP = SCRIPTS / "sweep.py"
TODAY = date(2026, 11, 16)

sys.path.insert(0, str(SCRIPTS))


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


rules = load("rules_under_test", RULES)
meta = load("meta_rules_under_test", META)


def workspace_with(tmp_path, search_rules=None, sources=None):
    workspace = tmp_path / "ws"
    (workspace / "jobs").mkdir(parents=True)
    config = {"tracks": ["SWE"]}
    if search_rules is not None:
        config["search_rules"] = search_rules
    if sources is not None:
        config["sources"] = sources
    (workspace / "workspace.json").write_text(json.dumps(config))
    return workspace


def lead(workspace, company, **keys):
    fields = {"title": "Engineer", "company": company, "track": "SWE", **keys}
    return meta.create(workspace, fields)


# ------------------------------------------------------------------- labels


@pytest.mark.parametrize(
    "label, low, high",
    [
        ("14d ago", 14, 14),
        ("Posted 30+ Days Ago", 30, None),
        ("1mo ago", 28, 61),
        ("2w ago", 14, 20),
        ("3 hours ago", 0, 0),
        ("Just posted", 0, 0),
        ("yesterday", 1, 1),
    ],
)
def test_age_labels_read_as_day_ranges(label, low, high):
    age = rules.parse_age_label(label)
    assert (age.min_days, age.max_days) == (low, high)


def test_an_unreadable_label_is_unknown_not_fresh():
    assert rules.parse_age_label("Featured") is None
    assert rules.check_age(None, rules.Rules(max_posting_age_days=30)) is None


def test_age_at_or_past_the_limit_is_expired_and_a_range_across_it_is_borderline():
    limit = rules.Rules(max_posting_age_days=30)
    assert rules.check_age(rules.parse_age_label("Posted 30+ Days Ago"), limit).certain
    borderline = rules.check_age(rules.parse_age_label("1mo ago"), limit)
    assert borderline.reason == "expired" and not borderline.certain
    assert rules.check_age(rules.parse_age_label("14d ago"), limit) is None


def test_ambiguous_age_labels_decide_the_borderline():
    discard = rules.rules_from({"max_posting_age_days": 30,
                                "ambiguous_age_labels": {"1MO  ago": "discard"}})
    keep = rules.rules_from({"max_posting_age_days": 30,
                             "ambiguous_age_labels": {"1mo ago": "keep"}})
    age = rules.parse_age_label("1mo ago")
    assert rules.check_age(age, discard).certain
    assert rules.check_age(age, keep) is None


def test_a_posted_date_wins_over_the_label():
    age = rules.age_of("2026-11-10", "1mo ago", TODAY)
    assert (age.min_days, age.max_days) == (6, 6)


def test_applicant_caps_are_per_source_and_registry_rules_add_to_them():
    found = rules.rules_from(
        {"max_applicants": {"LinkedIn": 100}},
        [{"name": "seek", "type": "browser", "rules": {"max_applicants": 50}}],
    )
    assert found.applicant_cap("linkedin") == 100 and found.applicant_cap("seek") == 50
    hits = rules.evaluate(found, source="linkedin", applicants=140)
    assert [h.reason for h in hits] == ["applicant_cap"]
    assert rules.evaluate(found, source="greenhouse:acme", applicants=500) == []


def test_contract_length_is_advisory_only():
    found = rules.rules_from({"max_contract_months": 6})
    hits = rules.evaluate(found, text="This is a 12 month contract role.")
    assert hits[0].reason == "contract_length" and hits[0].advisory
    assert rules.deciding(hits) is None


@pytest.mark.parametrize(
    "bad",
    [{"max_posting_age_days": 0}, {"max_applicants": {"linkedin": -1}},
     {"ambiguous_age_labels": {"1mo ago": "maybe"}}, ["30"]],
)
def test_a_bad_rule_is_refused_with_its_key(bad):
    with pytest.raises(rules.RulesError):
        rules.rules_from(bad)


# --------------------------------------------------------------------- gate


def test_create_refuses_a_lead_a_rule_rules_out(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30,
                                          "max_applicants": {"linkedin": 100}})
    old = run_script(workspace, META, "create", "--track", "SWE", "--title", "Engineer",
                     "--company", "Old Co", "--posted-label", "Posted 30+ Days Ago")
    assert old.returncode == 1 and "expired" in old.stderr
    busy = run_script(workspace, META, "create", "--track", "SWE", "--title", "Engineer",
                      "--company", "Busy Co", "--source", "linkedin", "--applicants", "250")
    assert busy.returncode == 1 and "applicant_cap" in busy.stderr
    assert not any((workspace / "jobs").rglob("meta.yml"))


def test_create_keeps_a_borderline_lead_and_says_so(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30})
    result = run_script(workspace, META, "create", "--track", "SWE", "--title", "Engineer",
                        "--company", "Edge Co", "--posted-label", "1mo ago", "--source", "jora")
    assert result.returncode == 0, result.stderr
    assert "borderline" in result.stderr
    text = (Path(result.stdout.strip()) / "meta.yml").read_text()
    assert 'posted_label: "1mo ago"' in text and 'source: "jora"' in text


def test_create_stores_posted_and_applicants_and_override_saves_anyway(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30})
    result = run_script(workspace, META, "create", "--track", "SWE", "--title", "Engineer",
                        "--company", "Asked Co", "--posted", "2026-01-01", "--applicants", "12",
                        "--override-rules")
    assert result.returncode == 0, result.stderr
    app = meta.rescan(Path(result.stdout.strip()))
    assert app.posted == "2026-01-01" and app.applicants == 12 and not app.malformed


def test_create_rejects_a_bad_posted_date(tmp_path):
    workspace = workspace_with(tmp_path)
    result = run_script(workspace, META, "create", "--track", "SWE", "--title", "E",
                        "--company", "C", "--posted", "16/11/2026")
    assert result.returncode == 1 and "YYYY-MM-DD" in result.stderr


@pytest.mark.parametrize("line", ["posted: 2026-13-01", "applicants: many",
                                  "filter_reason: bored", "filter_overruled: yes"])
def test_bad_new_keys_are_malformed(tmp_path, line):
    folder = tmp_path / "app"
    folder.mkdir()
    text = f"schema: 1\ntitle: T\ncompany: C\ntrack: SWE\n{line}\nstatus_log: []\n"
    (folder / "meta.yml").write_text(text)
    assert meta.rescan(folder).malformed


# --------------------------------------------------------------- dry run


def test_check_is_a_dry_run_that_separates_leads_borderline_and_drafts(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30})
    accenture = lead(workspace, "Accenture", posted_label="Posted 30+ Days Ago")
    heart = lead(workspace, "Heart Foundation", posted_label="1mo ago")
    leaders = lead(workspace, "Leaders IT", posted_label="Posted 30+ Days Ago")
    meta.append_status(leaders, "Draft", "2026-11-01T00:00:00Z")
    fresh = lead(workspace, "Fresh Co", posted="2026-11-10")
    before = {p: p.read_bytes() for p in (workspace / "jobs").rglob("meta.yml")}

    result = run_script(workspace, RULES, "check", "--today", "2026-11-16")

    assert result.returncode == 0, result.stderr
    out = result.stdout
    assert "Would dismiss 1 Lead(s)" in out and "Accenture" in out
    assert "Borderline" in out and "Heart Foundation" in out
    assert "Drafts a rule would remove (1)" in out and "Leaders IT" in out
    assert "Fresh Co" not in out and "Dry run" in out
    assert {p: p.read_bytes() for p in (workspace / "jobs").rglob("meta.yml")} == before
    assert accenture.exists() and heart.exists() and fresh.exists()


def test_apply_dismisses_leads_with_a_reason_and_keeps_folders_in_place(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30})
    old = lead(workspace, "Accenture", posted_label="Posted 30+ Days Ago")
    draft = lead(workspace, "Leaders IT", posted_label="Posted 30+ Days Ago")
    meta.append_status(draft, "Draft", "2026-11-01T00:00:00Z")

    result = run_script(workspace, RULES, "check", "--apply", "--today", "2026-11-16")

    assert result.returncode == 0, result.stderr
    app = meta.rescan(old)
    assert app.status == "Dismissed" and app.filter_reason == "expired"
    assert "limit 30 days" in app.filter_note
    assert meta.rescan(draft).status == "Draft"

    result = run_script(workspace, RULES, "check", "--apply", "--include-drafts",
                        "--today", "2026-11-16")
    assert meta.rescan(draft).status == "Dismissed"


def test_an_overruled_application_is_never_dismissed_by_a_rule(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30})
    kept = lead(workspace, "Kept Co", posted_label="Posted 30+ Days Ago")
    meta.patch(kept, {"filter_overruled": "true"})
    result = run_script(workspace, RULES, "check", "--apply")
    assert "No saved Lead or Draft breaks a Search rule." in result.stdout
    assert not meta.dismiss_by_rule(kept, "expired", "x")
    assert meta.rescan(kept).status == "Lead"


def test_blockers_in_the_ad_text_are_a_rule_too(tmp_path):
    workspace = workspace_with(tmp_path, {})
    folder = lead(workspace, "Agency")
    (folder / "jd.md").write_text("Applicants must hold Australian citizenship.\n")
    result = run_script(workspace, RULES, "check")
    assert "blocker" in result.stdout and "Agency" in result.stdout


def test_label_command_explains_a_verdict(tmp_path):
    workspace = workspace_with(tmp_path, {"max_posting_age_days": 30})
    result = run_script(workspace, RULES, "label", "1mo ago")
    assert json.loads(result.stdout)["verdict"] == "borderline"


# --------------------------------------------------------------- registry


REGISTRY = [
    {"name": "greenhouse", "type": "api", "platform": "greenhouse", "targets": ["acme"]},
    {"name": "Old board", "type": "api", "platform": "lever", "targets": ["gone"],
     "enabled": False},
    {"name": "LinkedIn", "type": "browser", "url": "https://www.linkedin.example/jobs",
     "access": "login wall", "age_format": "2 weeks ago", "applicants_format": "Over 100 applicants",
     "rules": {"max_applicants": 100}},
    {"name": "Jora", "type": "manual", "notes": "search each Place"},
]


def test_registry_feeds_the_sweep_plan_with_enabled_api_sources_only(tmp_path):
    workspace = workspace_with(tmp_path, sources=REGISTRY)
    result = run_script(workspace, SWEEP, "--sources")
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) == {"greenhouse": ["acme"]}


@pytest.mark.parametrize(
    "bad, why",
    [([{"name": "x", "type": "rss"}], "type"),
     ([{"name": "x", "type": "api", "platform": "nope", "targets": []}], "platform"),
     ([{"name": "x", "type": "manual"}, {"name": "X", "type": "manual"}], "twice"),
     ([{"name": "x", "type": "manual", "access": "blocked"}], "access")],
)
def test_a_bad_registry_entry_stops_the_sweep(tmp_path, bad, why):
    workspace = workspace_with(tmp_path, sources=bad)
    result = run_script(workspace, SWEEP, "--sources")
    assert result.returncode != 0 and why in result.stderr


def test_manual_sources_are_listed_with_their_hints_and_rules(tmp_path):
    sweep = load("sweep_rules_under_test", SWEEP)
    workspace = workspace_with(tmp_path, sources=REGISTRY)
    entries = sweep.manual_sources(workspace)
    text = "\n".join(sweep.manual_lines(entries, rules.load_rules(workspace)))
    assert "**LinkedIn** — browser · access: login wall" in text
    assert "Over 100 applicants" in text and "at most 100 applicants" in text
    assert "**Jora** — manual" in text and "search each Place" in text


def test_manual_source_cannot_be_tested(tmp_path):
    workspace = workspace_with(tmp_path, sources=REGISTRY)
    result = run_script(workspace, SWEEP, "--test-source", "jora")
    assert result.returncode == 1
    assert json.loads(result.stdout)["access"] == "manual"


def test_unknown_source_names_the_listed_ones(tmp_path):
    workspace = workspace_with(tmp_path, sources=REGISTRY)
    result = run_script(workspace, SWEEP, "--test-source", "seek")
    assert result.returncode != 0 and "LinkedIn" in result.stderr


@pytest.mark.parametrize(
    "status, body, access",
    [(403, "<title>Just a moment...</title>", "cloudflare"),
     (403, "Forbidden", "403"),
     (999, "", "login wall"),
     (200, "<title>Sign In | Board</title>", "login wall"),
     (200, "<title>Jobs</title>", "works")],
)
def test_access_is_classified_from_the_response(status, body, access):
    sweep = load("sweep_access_under_test", SWEEP)
    assert sweep.classify_access(status, body) == access


@pytest.mark.parametrize(
    "value, want",
    [("2026-11-02T10:00:00Z", "2026-11-02"), (1762041600000, "2025-11-02"), ("", ""), (None, "")],
)
def test_api_posting_times_become_dates(value, want):
    sweep = load("sweep_dates_under_test", SWEEP)
    assert sweep.posted_date(value) == want
