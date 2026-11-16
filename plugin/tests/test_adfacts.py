"""Ad facts: pay, contract terms and likely duplicates, mirrored by the Go store."""

import json
import sys
from pathlib import Path

import pytest

from conftest import run_script

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
META = SCRIPTS / "meta.py"
RULES = SCRIPTS / "rules.py"
sys.path.insert(0, str(SCRIPTS))

import adfacts  # noqa: E402
import meta  # noqa: E402
import rules  # noqa: E402

LONG_AD = "Design and run data pipelines for a large retailer. " * 10


def workspace(tmp_path, config=None):
    ws = tmp_path / "ws"
    (ws / "jobs").mkdir(parents=True)
    (ws / "workspace.json").write_text(json.dumps(config or {"tracks": ["SWE"]}))
    return ws


# ------------------------------------------------------------------------ pay


def test_annual_estimate_uses_the_unit_and_the_aud_rate():
    low, high = adfacts.annual_aud(900, 1100, "day", "AUD")
    assert (round(low), round(high)) == (207000, 253000)
    assert adfacts.pay_text(100, None, "hour", "USD") == "~A$306k/yr"
    assert adfacts.annual_aud(100, None, "hour", "XYZ") is None


@pytest.mark.parametrize(
    "low, high, unit, why",
    [(900, 1100, "hour", "looks like a day rate"),
     (120000, None, "day", "looks like an annual figure"),
     (85, 95, "year", "looks like an hourly or daily rate"),
     (85, 95, "hour", "")],
)
def test_figures_that_look_mislabelled_are_flagged(low, high, unit, why):
    assert adfacts.mislabel(low, high, unit) == why


def test_pay_fx_in_workspace_json_overrides_the_defaults(tmp_path):
    ws = workspace(tmp_path, {"pay_fx": {"USD": 2, "JPY": 0.01}})
    table = adfacts.fx_table(ws)
    assert table["USD"] == 2 and table["JPY"] == 0.01 and table["AUD"] == 1


@pytest.mark.parametrize(
    "text, want",
    [("Salary $120k - $140k + super", {"pay_min": 120000, "pay_max": 140000, "pay_unit": "year", "currency": "AUD"}),
     ("$900–$1,100 an hour", {"pay_min": 900, "pay_max": 1100, "pay_unit": "hour", "currency": "AUD"}),
     ("USD 80 per hour", {"pay_min": 80, "pay_unit": "hour", "currency": "USD"}),
     ("$750 per day", {"pay_min": 750, "pay_unit": "day", "currency": "AUD"}),
     ("Join 5 teams", {})],
)
def test_pay_is_read_as_figure_unit_and_currency(text, want):
    assert adfacts.read_pay(text) == want


# ---------------------------------------------------------------------- terms


@pytest.mark.parametrize(
    "text, want",
    [("This is a 6 month contract with possible extension.",
      {"term_months": "6", "employment_type": "contract", "extension": "possible"}),
     ("12-month fixed term role. No extension.",
      {"term_months": "12", "employment_type": "fixed-term", "extension": "no"}),
     ("Permanent full time. We are unable to offer visa sponsorship.",
      {"employment_type": "permanent", "sponsorship": "not-offered"}),
     ("Visa sponsorship available.", {"sponsorship": "offered"})],
)
def test_terms_are_read_only_as_stated(text, want):
    assert adfacts.read_terms(text) == want


# ----------------------------------------------------------------- duplicates


def test_title_key_drops_level_words_and_trailing_brackets():
    assert adfacts.title_key("Mid to Senior Level Data Engineer (Hybrid)") == \
        adfacts.title_key("Mid to Senior Data Engineer")
    assert adfacts.company_key("MSL Group Pty Ltd") == adfacts.company_key("MSL")


def test_create_refuses_a_likely_duplicate_and_names_the_folder(tmp_path):
    ws = workspace(tmp_path)
    first = meta.create(ws, {"title": "Mid to Senior Data Engineer", "company": "MSL", "track": "SWE"})
    result = run_script(ws, META, "create", "--track", "SWE", "--company", "MSL Pty Ltd",
                        "--title", "Mid to Senior Level Data Engineer")
    assert result.returncode == 1 and str(first) in result.stderr
    assert "same employer and title" in result.stderr


def test_the_same_ad_text_under_another_url_and_title_is_a_duplicate(tmp_path):
    ws = workspace(tmp_path)
    first = meta.create(ws, {"title": "Data Engineer", "company": "AGL", "track": "SWE",
                             "url": "https://jora.example/1"})
    (first / "jd.md").write_text(LONG_AD)
    ad = tmp_path / "ad.txt"
    ad.write_text("  " + LONG_AD.upper())
    result = run_script(ws, META, "find-duplicates", "--company", "Chandler Macleod",
                        "--title", "Pipeline Engineer", "--jd-file", str(ad))
    assert json.loads(result.stdout)[0]["Why"] == "same ad text"


def test_a_recruiter_ad_matches_the_client_it_names(tmp_path):
    ws = workspace(tmp_path)
    meta.create(ws, {"title": "Analyst", "company": "AGL", "recruiter": "Chandler Macleod",
                     "track": "SWE"})
    result = run_script(ws, META, "find-duplicates", "--company", "Chandler Macleod",
                        "--title", "Analyst")
    assert len(json.loads(result.stdout)) == 1


def test_allow_duplicate_saves_anyway(tmp_path):
    ws = workspace(tmp_path)
    meta.create(ws, {"title": "Analyst", "company": "AGL", "track": "SWE"})
    result = run_script(ws, META, "create", "--track", "SWE", "--company", "AGL",
                        "--title", "Analyst", "--allow-duplicate")
    assert result.returncode == 0, result.stderr


# --------------------------------------------------------------- meta keys


def test_create_writes_the_ad_facts_and_show_reads_them(tmp_path):
    ws = workspace(tmp_path)
    result = run_script(
        ws, META, "create", "--track", "SWE", "--company", "AGL", "--title", "Analyst",
        "--recruiter", "Chandler Macleod", "--employment-type", "contract", "--term-months", "6",
        "--extension", "possible", "--pay-min", "900", "--pay-max", "1100", "--pay-unit", "day",
        "--currency", "AUD", "--closes-unclear", "true", "--jd-completeness", "blurb",
        "--added-by", "sweep", "--search-query", "analyst melbourne")
    assert result.returncode == 0, result.stderr
    shown = json.loads(run_script(ws, META, "show", "--folder", result.stdout.strip()).stdout)
    assert shown["Recruiter"] == "Chandler Macleod" and shown["TermMonths"] == 6
    assert shown["PayMin"] == 900 and shown["PayUnit"] == "day" and shown["ClosesUnclear"] is True
    assert shown["JDCompleteness"] == "blurb" and shown["AddedBy"] == "sweep"


@pytest.mark.parametrize("flag, value", [("--term-months", "0"), ("--pay-unit", "fortnight"),
                                         ("--currency", "aud"), ("--employment-type", "gig")])
def test_create_refuses_a_bad_fact(tmp_path, flag, value):
    ws = workspace(tmp_path)
    result = run_script(ws, META, "create", "--track", "SWE", "--company", "C", "--title", "T",
                        flag, value)
    assert result.returncode == 1 and not any((ws / "jobs").rglob("meta.yml"))


def test_closes_and_closes_unclear_are_exclusive(tmp_path):
    ws = workspace(tmp_path)
    result = run_script(ws, META, "create", "--track", "SWE", "--company", "C", "--title", "T",
                        "--closes", "2026-12-01", "--closes-unclear", "true")
    assert result.returncode == 1 and "not both" in result.stderr


# ----------------------------------------------------------- rules: dates, live


def test_a_lead_past_its_closing_date_is_closed(tmp_path):
    ws = workspace(tmp_path)
    folder = meta.create(ws, {"title": "Engineer", "company": "Metro Trains", "track": "SWE",
                              "closes": "2026-11-10"})
    result = run_script(ws, RULES, "check", "--apply", "--today", "2026-11-16")
    assert result.returncode == 0, result.stderr
    app = meta.rescan(folder)
    assert app.status == "Dismissed" and app.filter_reason == "closed"


def test_the_later_of_two_duplicate_leads_is_the_one_dismissed(tmp_path):
    ws = workspace(tmp_path)
    older = meta.create(ws, {"title": "Data Engineer", "company": "Hassett", "track": "SWE"},
                        now=meta._parse_moment("2026-11-01"))
    newer = meta.create(ws, {"title": "Data Engineer", "company": "Hassett", "track": "SWE"},
                        now=meta._parse_moment("2026-11-05"))
    run_script(ws, RULES, "check", "--apply", "--today", "2026-11-16")
    assert meta.rescan(older).status == "Lead"
    assert meta.rescan(newer).filter_reason == "duplicate"


@pytest.mark.parametrize(
    "body, error, want",
    [("<h1>Data Engineer</h1>", None, "live"),
     ("Sorry, this job is no longer available.", None, "closed"),
     ("", 404, "gone"),
     ("", 403, None)],
)
def test_listing_state_reads_the_ad_page(body, error, want):
    from httpfetch import FetchError

    def fetch(url):
        if error:
            raise FetchError(f"{url} -- HTTP Error {error}", status=error)
        return body

    assert rules.listing_state("https://x.test/1", fetch) == want


def test_recheck_records_listing_and_check_dismisses_pulled_ads(tmp_path):
    ws = workspace(tmp_path)
    gone = meta.create(ws, {"title": "A", "company": "ACER", "track": "SWE", "url": "https://x.test/gone"})
    live = meta.create(ws, {"title": "B", "company": "Co", "track": "SWE", "url": "https://x.test/live"})
    pages = {"https://x.test/gone": "This position has been filled.", "https://x.test/live": "Apply now"}
    rules.recheck(ws, fetch=pages.__getitem__, today=rules.date(2026, 11, 16))
    assert meta.rescan(gone).listing == "closed" and meta.rescan(live).listing == "live"
    assert meta.rescan(gone).listing_checked == "2026-11-16"
    run_script(ws, RULES, "check", "--apply", "--today", "2026-11-16")
    assert meta.rescan(gone).filter_reason == "pulled" and meta.rescan(live).status == "Lead"


def test_two_employers_sharing_only_a_recruiter_are_not_duplicates(tmp_path):
    ws = workspace(tmp_path)
    meta.create(ws, {"title": "Analyst", "company": "AGL", "recruiter": "Chandler Macleod",
                     "track": "SWE"})
    result = run_script(ws, META, "find-duplicates", "--company", "Origin",
                        "--recruiter", "Chandler Macleod", "--title", "Analyst")
    assert json.loads(result.stdout) == []
    # With no company stated, the recruiter stands in for it.
    result = run_script(ws, META, "find-duplicates", "--company", "",
                        "--recruiter", "Chandler Macleod", "--title", "Analyst")
    assert len(json.loads(result.stdout)) == 1


def test_a_symlinked_jd_is_not_read_for_the_text_match(tmp_path):
    ws = workspace(tmp_path)
    first = meta.create(ws, {"title": "Data Engineer", "company": "AGL", "track": "SWE"})
    outside = tmp_path / "outside.md"
    outside.write_text(LONG_AD)
    (first / "jd.md").unlink(missing_ok=True)
    (first / "jd.md").symlink_to(outside)
    assert adfacts.likely_duplicates(meta.scan(ws), "Other", "Other", LONG_AD) == []
