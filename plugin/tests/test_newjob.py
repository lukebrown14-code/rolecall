"""new-job.py captures a job description into a new Application folder.

Capture is a writer of ``meta.yml`` through the shared ``meta.create``, so the
tests drive the CLI and assert on the files: one Lead entry, a plain-Markdown
``jd.md`` with no front matter, and no tracker or prepared-document scaffolding.
"""

import importlib.util
import io
import json
import subprocess
import sys
from datetime import date
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace, run_script

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "plugin" / "scripts" / "new-job.py"
FIXTURE = ROOT / "fixtures" / "workspace"


def load(filename, name):
    spec = importlib.util.spec_from_file_location(name, ROOT / "plugin" / "scripts" / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


nj = load("new-job.py", "newjob_under_test")
signals = load("signals.py", "signals_under_test")


def only_application(workspace):
    folders = sorted(
        folder for folder in (workspace / "jobs" / "SWE").iterdir() if folder.is_dir()
        if (folder / "meta.yml").is_file()
    )
    captured = [
        folder
        for folder in folders
        if folder.name.endswith(f"-{date.today().isoformat()}")
        or f"-{date.today().isoformat()}-" in folder.name
    ]
    assert len(captured) == 1, [folder.name for folder in folders]
    return captured[0]


def labels(text, kind="BLOCKER"):
    return [label for label, signal in nj.scan(text) if signal == kind]


# ------------------------------------------------------------------ blockers

@pytest.mark.parametrize(
    "text,expected",
    [
        ("Applicants must be Australian citizens.", "requires Australian citizenship"),
        ("You must hold an NV1 clearance.", "requires an AU security clearance (citizens only)"),
        ("Baseline clearance is required for this role.", "requires an AU security clearance (citizens only)"),
        ("Must be an Australian permanent resident.", "asks for permanent residency"),
        ("Candidates must be PR holders.", "asks for permanent residency"),
        ("Australian citizen or PR required.", "requires Australian citizenship"),
        ("We are unable to provide visa sponsorship for this role.", "states sponsorship is not available"),
        ("No sponsorship available.", "states sponsorship is not available"),
        (
            "Unrestricted working rights are essential.",
            "asks for unrestricted work rights (short-contract visas have a term limit)",
        ),
    ],
)
def test_real_blockers_detected(text, expected):
    assert expected in labels(text)


@pytest.mark.parametrize(
    "text",
    [
        "You will do PR review and mentor others on code review.",
        "Open a PR, get a review, merge. We love small PRs.",
        "Reviewing pr comments daily.",
        "We encourage applications from Aboriginal and Torres Strait Islander people, Australian citizens and permanent residents.",
        "We are an equal opportunity employer and welcome Australian citizens, residents and visa holders alike.",
        "Australian citizenship is not required.",
        "Citizenship is not a requirement for this role.",
        "You do not need to be an Australian citizen.",
        "No security clearance required.",
        "Permanent residency is not necessary; we support working-holiday visa holders.",
    ],
)
def test_false_positives_not_flagged(text):
    assert labels(text) == []


def test_negation_does_not_hide_a_real_blocker_in_another_sentence():
    text = "Citizenship is not required for the graduate stream. This role requires NV1 clearance."
    assert any("clearance" in label for label in labels(text))


def test_sponsorship_refusal_not_cancelled_by_negation_logic():
    assert labels("We do not offer sponsorship.") == ["states sponsorship is not available"]


def test_favourable_short_contract():
    assert "short contract term — suits a short-contract visa" in labels(
        "This is a 6 month contract.", "in your favour"
    )
    assert labels("12 month contract with extension.", "in your favour") == [
        "contract role — the contract-term limit is a non-issue"
    ]


def test_blockers_interface_for_sweep():
    assert signals.BLOCKERS and all(hasattr(pattern, "search") for pattern, _ in signals.BLOCKERS)
    assert signals.FAVOURABLE


# ---------------------------------------------------------------- page parsing

JSONLD = """<html><head><title>Ignore me | Seek</title>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"JobPosting",
"title":"Senior Platform Engineer","hiringOrganization":{"@type":"Organization","name":"Acme Pty Ltd"},
"description":"&lt;p&gt;Build things.&lt;/p&gt;&lt;ul&gt;&lt;li&gt;Go&lt;/li&gt;&lt;/ul&gt;",
"validThrough":"2026-11-15T00:00","employmentType":["CONTRACTOR"],
"baseSalary":{"@type":"MonetaryAmount","currency":"AUD","value":{"@type":"QuantitativeValue","minValue":120000,"maxValue":140000,"unitText":"YEAR"}}}
</script></head><body><p>x</p></body></html>"""


def test_jsonld_wins():
    meta = nj.page_meta(JSONLD)
    assert meta["role"] == "Senior Platform Engineer"
    assert meta["company"] == "Acme Pty Ltd"
    assert meta["closes"] == "2026-11-15"
    assert meta["type"] == "Contractor"
    assert "120000-140000" in meta["salary"] and "AUD" in meta["salary"]
    assert "Build things." in meta["description"] and "Go" in meta["description"]
    assert "<" not in meta["description"]


def test_og_title_fallback():
    html = '<html><head><meta property="og:title" content="Data Engineer at Globex"><title>x</title></head></html>'
    meta = nj.page_meta(html)
    assert (meta["role"], meta["company"]) == ("Data Engineer", "Globex")


def test_og_title_content_before_property_and_site_name():
    html = '<meta content="Backend Developer" name="og:title"><meta property="og:site_name" content="Initech">'
    meta = nj.page_meta(html)
    assert (meta["role"], meta["company"]) == ("Backend Developer", "Initech")


def test_title_tag_fallback():
    meta = nj.page_meta("<html><head><title>QA Analyst | Hooli Careers</title></head></html>")
    assert (meta["role"], meta["company"]) == ("QA Analyst", "Hooli")


def test_title_with_dash_separator():
    assert nj.split_title("Product Designer - Umbrella Corp") == (
        "Product Designer",
        "Umbrella Corp",
    )


def test_header_content_kept_on_ats_page():
    body = "Responsibilities include building services. " * 15
    html = f"<html><body><header><h1>Engineer</h1><div>{body}</div></header><nav>Menu</nav></body></html>"
    plain, _ = nj.html_to_job(html, ats=False)
    ats, _ = nj.html_to_job(html, ats=True)
    assert "Responsibilities" in plain
    assert "Responsibilities" in ats and "Menu" not in ats


def test_nav_still_stripped_on_normal_pages():
    long_body = "<p>" + "Real ad body text. " * 40 + "</p>"
    text, _ = nj.html_to_job(
        f"<html><body><nav>Contract Recruitment</nav>{long_body}</body></html>"
    )
    assert "Contract Recruitment" not in text


def test_next_data_used_for_js_pages():
    description = "<p>" + "Own the data platform end to end. " * 30 + "</p>"
    blob = json.dumps({"props": {"pageProps": {"job": {"title": "X", "jobDescription": description}}}})
    html = f'<html><body><div id="__next"></div><script id="__NEXT_DATA__" type="application/json">{blob}</script></body></html>'
    text, _ = nj.html_to_job(html)
    assert "Own the data platform" in text and "<p>" not in text


def test_looks_like_html():
    assert nj.looks_like_html("<!DOCTYPE html><html>")
    assert not nj.looks_like_html("Company: Acme\nRole: Dev\nWe use a < b comparisons")


# ------------------------------------------------------------- fact extraction

def test_salary():
    assert nj.extract_salary("Salary: $120,000 - $140,000 per annum plus super") == "$120,000 - $140,000 per annum"
    assert nj.extract_salary("Paying $130k + super") == "$130k"
    assert nj.extract_salary("Day rate $850 per day").startswith("$850")
    assert nj.extract_salary("We manage a $5 million budget") == ""
    assert nj.extract_salary("No pay info here") == ""


def test_closing_date():
    assert nj.extract_closing("Applications close 31 October 2026.") == "2026-10-31"
    assert nj.extract_closing("Closing date: 2026-12-01") == "2026-12-01"
    assert nj.extract_closing("Apply by 5/11/2026") == "2026-11-05"
    assert nj.extract_closing("Closes: Nov 3, 2026") == "2026-11-03"
    assert nj.extract_closing("Open until filled") == ""


def test_employment_type():
    assert nj.extract_type("Employment type: Fixed term") == "Fixed term"
    assert nj.extract_type("This is a 6 month contract role") == "Contract"
    assert nj.extract_type("A permanent full-time position") == "Permanent"
    assert nj.extract_type("nothing relevant") == ""


def test_seniority():
    assert nj.extract_seniority("Senior Software Engineer") == "senior"
    assert nj.extract_seniority("Principal Engineer") == "senior"
    assert nj.extract_seniority("Graduate Developer") == "junior"
    assert nj.extract_seniority("Software Engineer", "Requires 10+ years of experience") == "senior"
    assert nj.extract_seniority("Software Engineer", "3 years experience") == ""


def test_extract_facts_prefers_structured_meta():
    facts = nj.extract_facts(
        "Salary $90k. Contract.", "Engineer", {"salary": "AUD 1-2", "type": "Full Time"}
    )
    assert facts["salary"] == "AUD 1-2" and facts["type"] == "Full Time"


# ----------------------------------------------------------------- end to end

BODY = (
    "We are hiring. "
    + "Build and run reliable backend services for customers. " * 8
    + "\nSalary $120k - $140k. Applications close 31 October 2026. 6 month contract."
)


def test_paste_creates_one_application_with_a_plain_jd(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    html = JSONLD.replace("<p>x</p>", f"<p>{BODY}</p>")

    result = run_script(workspace, SCRIPT, "--track", "SWE", "--paste", stdin=html)

    assert result.returncode == 0, result.stderr
    folder = only_application(workspace)
    assert folder.name == f"acme-pty-ltd-senior-platform-engineer-{date.today().isoformat()}"
    meta = (folder / "meta.yml").read_text()
    assert 'schema: 1' in meta
    assert 'title: "Senior Platform Engineer"' in meta
    assert 'company: "Acme Pty Ltd"' in meta
    assert 'track: "SWE"' in meta
    assert 'closes: "2026-11-15"' in meta
    assert meta.count("status: Lead") == 1
    assert "{status: Lead, at: " in meta and meta.rstrip().endswith("Z}")
    jd = (folder / "jd.md").read_text()
    assert not jd.startswith("---")
    assert "\n---\n" not in jd
    assert "Build and run reliable backend services for customers." in jd
    assert "Salary $120k - $140k." in jd
    assert sorted(entry.name for entry in folder.iterdir()) == ["jd.md", "meta.yml"]
    assert not (folder / "out").exists()
    assert not (workspace / "jobs" / "TRACKER.md").exists()
    for name in ("fit.md", "resume.md", "cover-letter.md", "gaps.md"):
        assert not (folder / name).exists()


def test_capturing_twice_suffixes_the_second_folder(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    text = f"Company: Initech\nRole: Junior Dev\n{BODY}"

    first = run_script(workspace, SCRIPT, "--track", "SWE", "--paste", stdin=text)
    refused = run_script(workspace, SCRIPT, "--track", "SWE", "--paste", stdin=text)
    second = run_script(workspace, SCRIPT, "--track", "SWE", "--paste", "--allow-duplicate",
                        stdin=text)

    assert refused.returncode == 1 and "looks like a saved Application" in refused.stderr
    assert first.returncode == 0, first.stderr
    assert second.returncode == 0, second.stderr
    names = sorted(
        folder.name
        for folder in (workspace / "jobs" / "SWE").iterdir()
        if folder.name.startswith("initech-junior-dev-")
    )
    today = date.today().isoformat()
    assert names == [f"initech-junior-dev-{today}", f"initech-junior-dev-{today}-2"]


def test_explicit_company_role_and_source_fill_the_record(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)

    result = run_script(
        workspace,
        SCRIPT,
        "--track",
        "SWE",
        "--paste",
        "--source",
        "https://x.test/1",
        "--company",
        "Initech",
        "--role",
        "Junior Dev",
        stdin=BODY,
    )

    assert result.returncode == 0, result.stderr
    folder = only_application(workspace)
    meta = (folder / "meta.yml").read_text()
    assert 'title: "Junior Dev"' in meta
    assert 'company: "Initech"' in meta
    assert 'url: "https://x.test/1"' in meta
    assert 'closes: "2026-10-31"' in meta
    # The free-text facts stay out; the structured ad facts (SCHEMA.md) go in.
    assert "\nseniority:" not in meta and "\nsalary:" not in meta and "\ntype:" not in meta
    assert 'pay_min: "120000"' in meta and 'pay_max: "140000"' in meta
    assert 'pay_unit: "year"' in meta and 'currency: "AUD"' in meta
    assert 'employment_type: "contract"' in meta and 'term_months: "6"' in meta
    assert 'added_by: "agent"' in meta


def test_existing_folder_is_left_untouched(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    today = date.today().isoformat()
    existing = workspace / "jobs" / "SWE" / f"acme-dev-{today}"
    existing.mkdir()
    marker = existing / "marker.txt"
    marker.write_text("hand-written\n")

    result = run_script(
        workspace,
        SCRIPT,
        "--track",
        "SWE",
        "--paste",
        "--company",
        "Acme",
        "--role",
        "Dev",
        stdin=BODY,
    )

    assert result.returncode == 0, result.stderr
    assert marker.read_text() == "hand-written\n"
    assert (workspace / "jobs" / "SWE" / f"acme-dev-{today}-2" / "meta.yml").is_file()


def test_file_source_creates_the_application(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    source = tmp_path / "jd.txt"
    source.write_text(f"Company: Globex\nRole: QA Analyst\n{BODY}")

    result = run_script(workspace, SCRIPT, "--track", "SWE", "--file", str(source))

    assert result.returncode == 0, result.stderr
    folder = only_application(workspace)
    meta = (folder / "meta.yml").read_text()
    assert 'title: "QA Analyst"' in meta
    assert 'company: "Globex"' in meta
    assert BODY in (folder / "jd.md").read_text()


def test_fetch_only_writes_nothing(tmp_path):
    workspace = tmp_path / "workspace"
    workspace.mkdir()

    result = run_script(workspace, SCRIPT, "--fetch-only", "--paste", stdin=BODY)

    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["meta"]["salary"] == "$120k - $140k"
    assert payload["meta"]["closes"] == "2026-10-31"
    assert not (workspace / "jobs").exists()


def test_jd_write_failure_is_reported_without_a_traceback(tmp_path, monkeypatch, capsys):
    workspace = tmp_path / "workspace"
    workspace.mkdir()
    folder = workspace / "jobs" / "SWE" / "captured"
    folder.mkdir(parents=True)
    (folder / "jd.md").mkdir()  # writing text to a directory is an OSError

    monkeypatch.setattr(nj.meta_store, "create", lambda *args, **kwargs: folder)
    monkeypatch.setattr(
        sys, "argv", ["new-job.py", "--data", str(workspace), "--track", "SWE", "--paste"]
    )
    monkeypatch.setattr(sys, "stdin", io.StringIO(BODY))

    with pytest.raises(SystemExit) as e:
        nj.main()

    assert e.value.code == 1
    err = capsys.readouterr().err
    assert "rolecall:" in err and "jd.md" in err
    assert "Traceback" not in err


def test_missing_workspace_is_a_clear_error(tmp_path):
    import os

    env = {key: value for key, value in os.environ.items() if key != "ROLECALL_WORKSPACE"}
    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--track", "SWE", "--paste"],
        input=BODY,
        capture_output=True,
        text=True,
        env=env,
        cwd=tmp_path,
    )

    assert result.returncode != 0
    assert "--data" in result.stderr
    assert "ROLECALL_WORKSPACE" in result.stderr
