"""sweep.py collects candidates and writes jobs/sweep-candidates.md.

The collectors are pure functions over a fetch, so the network is always
stubbed: ``sw.get`` is replaced for the platform tests and the platform
registry is replaced with a local collector for whole-run tests. Dedupe is
built from every Application's ``meta.yml`` URL, whatever its Status.
"""

import importlib.util
import json
import sys
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace, run_script

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "plugin" / "scripts" / "sweep.py"
FIXTURE = ROOT / "fixtures" / "workspace"


def load():
    spec = importlib.util.spec_from_file_location("sweep_under_test", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


sw = load()


def write_workspace(workspace, config):
    (workspace / "jobs").mkdir(parents=True, exist_ok=True)
    (workspace / "workspace.json").write_text(json.dumps(config))


@pytest.fixture
def catalogue():
    return {
        "version": 1,
        "roles": [
            {
                "id": "implementation",
                "category": "Consulting",
                "track": "Analyst",
                "priority": "primary",
                "titles": ["Implementation Consultant"],
                "search_terms": ["implementation consultant", "sdet"],
                "evidence": "Client discovery",
                "gaps": "Formal onboarding",
            }
        ],
        "additional_sources": {"greenhouse": ["smartsheet", "cultureamp"]},
    }


def write_catalogue(workspace, catalogue):
    (workspace / "jobs").mkdir(exist_ok=True)
    (workspace / "jobs" / "roles.json").write_text(json.dumps(catalogue))


# ------------------------------------------------------------------ catalogue


def test_workspace_catalogue_controls_matching(tmp_path, monkeypatch, catalogue):
    write_catalogue(tmp_path, catalogue)
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", sw.load_role_catalogue(tmp_path))
    assert sw.matches_track("Senior Implementation-Consultant", ["SWE", "Analyst"]) == ["Analyst"]
    assert sw.matches_track("SDET", ["Analyst"]) == ["Analyst"]
    assert sw.matches_track("SDETastic", ["Analyst"]) == []
    assert sw.matches_track("Software Engineer", ["SWE"]) == []
    assert sw.matches_track("Implementation Consultant", ["SWE"]) == []


def test_missing_catalogue_keeps_legacy_titles(tmp_path, monkeypatch):
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", sw.load_role_catalogue(tmp_path))
    assert sw.matches_track("Senior Software Engineer", ["SWE"]) == ["SWE"]


@pytest.mark.parametrize(
    "change",
    [
        lambda c: c.update(version=2),
        lambda c: c.update(roles=[]),
        lambda c: c["roles"][0].update(search_terms="consultant"),
        lambda c: c["roles"][0].update(search_terms=[""]),
        lambda c: c["roles"][0].update(track="Unknown"),
        lambda c: c["roles"].append(c["roles"][0].copy()),
        lambda c: c.update(additional_sources={"unsupported": ["board"]}),
    ],
)
def test_invalid_catalogue_fails_clearly(tmp_path, catalogue, change):
    change(catalogue)
    write_catalogue(tmp_path, catalogue)
    with pytest.raises(SystemExit, match="roles.json"):
        sw.load_role_catalogue(tmp_path)


def test_catalogue_sources_extend_defaults_but_respect_override(tmp_path, monkeypatch, catalogue):
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", catalogue)
    write_workspace(tmp_path, {"tracks": ["SWE", "Analyst"]})
    sources = dict(sw.resolve_sources(tmp_path))
    assert "smartsheet" in sources["greenhouse"]
    assert sources["greenhouse"].count("cultureamp") == 1
    write_workspace(tmp_path, {"tracks": ["SWE", "Analyst"], "sources": {"lever": []}})
    assert sw.resolve_sources(tmp_path) == [("lever", [])]


def test_catalogue_supports_workspace_defined_tracks(tmp_path, monkeypatch, catalogue):
    catalogue["roles"][0]["track"] = "Consulting"
    write_workspace(tmp_path, {"tracks": ["Consulting"]})
    write_catalogue(tmp_path, catalogue)
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", sw.load_role_catalogue(tmp_path))
    assert sw.matches_track("Implementation Consultant", ["Consulting"]) == ["Consulting"]


def test_malformed_json_stops_sweep(tmp_path):
    (tmp_path / "jobs").mkdir()
    (tmp_path / "jobs" / "roles.json").write_text("{broken")
    with pytest.raises(SystemExit, match="roles.json"):
        sw.load_role_catalogue(tmp_path)


def test_catalogue_cli_is_workspace_local_and_read_only(tmp_path, catalogue):
    write_catalogue(tmp_path, catalogue)
    result = run_script(tmp_path, SCRIPT, "--roles")
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) == catalogue
    assert not (tmp_path / "jobs" / "sweep-candidates.md").exists()
    (tmp_path / "jobs" / "roles.json").unlink()
    result = run_script(tmp_path, SCRIPT, "--roles")
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) is None


def test_sources_cli_uses_same_catalogue_and_overrides_as_sweep(tmp_path, catalogue):
    write_catalogue(tmp_path, catalogue)
    sources = json.loads(run_script(tmp_path, SCRIPT, "--sources").stdout)
    assert sources["greenhouse"].count("cultureamp") == 1
    assert "smartsheet" in sources["greenhouse"]
    (tmp_path / "workspace.json").write_text(json.dumps({"sources": {}}))
    sources = json.loads(run_script(tmp_path, SCRIPT, "--sources").stdout)
    assert sources == {}
    assert not (tmp_path / "jobs" / "sweep-candidates.md").exists()


def test_platforms_cli_is_read_only(tmp_path):
    result = run_script(tmp_path, SCRIPT, "--platforms")
    assert result.returncode == 0, result.stderr
    platforms = json.loads(result.stdout)
    assert "greenhouse" in platforms and "lever" in platforms
    assert not (tmp_path / "jobs" / "sweep-candidates.md").exists()


def test_matches_explain_all_families_and_choose_specific_phrase(monkeypatch, catalogue):
    catalogue["roles"][0]["search_terms"] = ["consultant", "implementation consultant"]
    other = {
        **catalogue["roles"][0],
        "id": "consulting",
        "track": "SA",
        "priority": "secondary",
        "search_terms": ["consultant"],
    }
    catalogue["roles"].append(other)
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", catalogue)
    matches = sw.role_matches("Senior Implementation-Consultant", ["Analyst", "SA"])
    assert [m["id"] for m in matches] == ["implementation", "consulting"]
    assert matches[0]["term"] == "implementation consultant"
    assert matches[0]["priority"] == "primary"
    assert matches[1]["track"] == "SA"
    assert sw.matches_track("Senior Implementation-Consultant", ["SA", "Analyst"]) == ["SA", "Analyst"]


# ---------------------------------------------------------------- whole sweep


def sweep_workspace(tmp_path, catalogue=None, tracks=("Analyst",), search_area=None):
    workspace = tmp_path / "workspace"
    config = {"tracks": list(tracks)}
    if search_area is not None:
        config["search_area"] = search_area
    write_workspace(workspace, config)
    if catalogue is not None:
        write_catalogue(workspace, catalogue)
    return workspace


def run_sweep(monkeypatch, workspace, collector, *args, targets=("fixture",)):
    monkeypatch.setattr(sw, "PLATFORMS", {**sw.PLATFORMS, "lever": (collector, [])})
    monkeypatch.setattr(
        sw, "resolve_sources", lambda _workspace: [("lever", list(targets))]
    )
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", sw.load_role_catalogue(workspace))
    monkeypatch.setattr(
        sys, "argv", ["sweep.py", "--data", str(workspace), *args]
    )
    sw.main()
    return (workspace / "jobs" / "sweep-candidates.md").read_text()


@pytest.mark.parametrize("partial", [False, True])
def test_sweep_explains_candidates_and_accounts_for_each_family_outcome(
    tmp_path, monkeypatch, catalogue, partial
):
    catalogue["roles"].append({**catalogue["roles"][0], "id": "excluded", "track": "AI"})
    workspace = sweep_workspace(
        tmp_path, catalogue, tracks=("SWE", "Analyst", "AI"), search_area=AU
    )

    def job(slug, location="Brisbane, Australia", content=""):
        return dict(
            title=f"Implementation Consultant {slug}",
            company="Acme",
            location=location,
            url=f"https://example.com/{slug}",
            content=content,
            type_hint="",
            closes="",
            source="lever",
        )

    def collector(target):
        if target == "broken":
            raise sw.SourceError("fixture failure")
        return [
            job("new"),
            job("known"),
            job("outside", "London"),
            job("blocked", content="Australian citizenship is required."),
            job("new"),
        ]

    monkeypatch.setattr(sw, "seen_urls", lambda _workspace: {"https://example.com/known"})
    targets = ("fixture", "broken") if partial else ("fixture",)
    output = run_sweep(
        monkeypatch, workspace, collector, "--track", "Analyst", targets=targets
    )
    state = "Partial" if partial else "Fetched boards"
    assert (
        f"| implementation | Implementation Consultant | Analyst | primary | 4 | 1 | 1 | 1 | 1 | {state} |"
        in output
    )
    assert (
        "| excluded | Implementation Consultant | AI | primary | 0 | 0 | 0 | 0 | 0 | Not searched |"
        in output
    )
    assert (
        "- match: `implementation` · Implementation Consultant · Analyst · primary · "
        "title contains `implementation consultant`" in output
    )


@pytest.mark.parametrize("has_source", [False, True])
def test_empty_results_distinguish_no_sources_from_no_matches(
    tmp_path, monkeypatch, catalogue, has_source
):
    workspace = sweep_workspace(tmp_path, catalogue, tracks=("SWE", "Analyst"))
    if has_source:
        monkeypatch.setattr(
            sw, "resolve_sources", lambda _workspace: [("lever", ["fixture"])]
        )
    else:
        monkeypatch.setattr(sw, "resolve_sources", lambda _workspace: [])
    monkeypatch.setattr(sw, "PLATFORMS", {**sw.PLATFORMS, "lever": (lambda target: [], [])})
    monkeypatch.setattr(sw, "ROLE_CATALOGUE", sw.load_role_catalogue(workspace))
    monkeypatch.setattr(sys, "argv", ["sweep.py", "--data", str(workspace)])
    sw.main()
    status = "Fetched boards" if has_source else "Not searched"
    output = (workspace / "jobs" / "sweep-candidates.md").read_text()
    assert (
        f"| implementation | Implementation Consultant | Analyst | primary | "
        f"0 | 0 | 0 | 0 | 0 | {status} |" in output
    )


def test_sweep_collects_non_swe_roles_from_catalogue(tmp_path, monkeypatch, catalogue):
    workspace = sweep_workspace(tmp_path, catalogue, tracks=("SWE", "Analyst"))

    def collector(target):
        return [
            dict(
                title="Implementation Consultant",
                company="Acme",
                location="Brisbane, Australia",
                url="https://example.com/implementation",
                content="",
                type_hint="Contract",
                closes="",
                source="lever:fixture",
            )
        ]

    output = run_sweep(monkeypatch, workspace, collector)
    assert "Implementation Consultant — Acme" in output
    assert "Analyst" in output
    assert "[Master role list](roles.json)" in output


# ---------------------------------------------------------------------- scan


def blockers_in(text):
    blockers, favourable = sw.load_blockers(Path("/nonexistent-workspace"))
    return [label for label, kind in sw.scan(text, blockers, favourable) if kind == "BLOCKER"]


def test_scan_flags_citizenship_and_clearance():
    assert blockers_in("Must be an Australian citizen with NV1 clearance.") == [
        "requires Australian citizenship",
        "requires an AU security clearance (citizens only)",
    ]


@pytest.mark.parametrize(
    "text",
    [
        "Expect PR review and code review as part of the job.",
        "We welcome applications from Australian citizens, permanent residents and visa holders. Equal opportunity employer.",
        "Australian citizenship is not required.",
    ],
)
def test_scan_false_positives(text):
    assert blockers_in(text) == []


def test_scan_favourable_only_short_terms():
    blockers, favourable = sw.load_blockers(Path())
    hits = sw.scan("A 6 month contract.", blockers, favourable)
    assert [kind for _, kind in hits] == ["favourable"]
    assert sw.scan("Contract Recruitment | Permanent Recruitment", blockers, favourable) == []


# --------------------------------------------------------- Search area filter

AU = {"places": ["Australia"], "remote": "in-places"}


def area(*places, remote="in-places"):
    return {"places": list(places), "remote": remote}


@pytest.mark.parametrize(
    "loc, search_area",
    [
        ("Brisbane, QLD", AU),  # country covers its city and state
        ("Sydney, NSW", AU),
        ("Brisbane, Australia or Singapore", AU),  # one Place is enough
        ("Sydney", area("sydney")),  # direct match, any case
        ("Remote, APAC", area("APAC")),  # group named in Places
        ("Wellington", area("APAC")),  # group covers a country's city
        ("Brisbane", area("Brisbane", "Singapore")),
        ("", AU),  # empty location is kept
        (None, AU),
        ("Atlantis", AU),  # unknown place is kept
        ("Hybrid", AU),
    ],
)
def test_search_area_keeps_roles_in_places(loc, search_area):
    assert sw.location_ok(loc, search_area)


@pytest.mark.parametrize(
    "loc, search_area",
    [
        ("London, UK", AU),
        ("London", AU),
        ("Singapore", AU),
        ("Victoria, BC, Canada", AU),  # an explicit country decides over a shared name
        ("Brisbane", area("Sydney")),  # a known city outside the Places
        ("Australia", area("Sydney")),  # a wider place is not covered by a narrower Place
        ("Paris", area("APAC")),
    ],
)
def test_search_area_drops_known_places_outside(loc, search_area):
    assert not sw.location_ok(loc, search_area)


def test_country_name_is_matched_as_a_whole_word():
    assert sw.location_ok("Fusion Centre", AU)  # "us" inside a word is not the US
    assert not sw.location_ok("Remote - US", AU)


@pytest.mark.parametrize(
    "loc, remote, expected",
    [
        ("Remote", "anywhere", True),
        ("Remote - US", "anywhere", True),
        ("Remote - US", "in-places", False),
        ("Remote", "in-places", True),  # names no known place
        ("Remote - Australia", "in-places", True),
        ("Remote, APAC", "in-places", True),
        ("Remote - Australia", "no", False),
        ("Remote", "no", False),
        ("Brisbane, Australia", "no", True),  # not a remote role
        ("Brisbane, Australia", "anywhere", True),
    ],
)
def test_remote_rules(loc, remote, expected):
    assert sw.location_ok(loc, area("Australia", remote=remote)) is expected


def test_remote_defaults_to_in_places():
    assert sw.location_ok("Remote", {"places": ["Australia"]})
    assert not sw.location_ok("Remote - US", {"places": ["Australia"]})


@pytest.mark.parametrize("search_area", [None, "Australia", ["Australia"]])
def test_no_search_area_keeps_everything(search_area):
    assert sw.location_ok("London", search_area)


def sweep_one(monkeypatch, workspace, location, *args):
    def collector(target):
        return [
            dict(
                title="Implementation Consultant",
                company="Acme",
                location=location,
                url="https://example.com/role",
                content="",
                type_hint="",
                closes="",
                source="lever:fixture",
            )
        ]

    return run_sweep(monkeypatch, workspace, collector, *args)


def test_sweep_filters_by_workspace_search_area(tmp_path, monkeypatch, catalogue, capsys):
    workspace = sweep_workspace(tmp_path, catalogue, search_area=AU)
    output = sweep_one(monkeypatch, workspace, "London")
    assert "Implementation Consultant — Acme" not in output
    assert "1 wrong location" in capsys.readouterr().out
    workspace = sweep_workspace(tmp_path / "b", catalogue, search_area=AU)
    output = sweep_one(monkeypatch, workspace, "Brisbane, QLD")
    assert "Implementation Consultant — Acme" in output


def test_sweep_without_search_area_keeps_all_and_says_so(
    tmp_path, monkeypatch, catalogue, capsys
):
    workspace = sweep_workspace(tmp_path, catalogue)
    output = sweep_one(monkeypatch, workspace, "London")
    assert "Implementation Consultant — Acme" in output
    assert "no Search area set: all locations kept" in capsys.readouterr().out
    assert "no Search area set: all locations kept" in output


@pytest.mark.parametrize("search_area", [{}, {"places": []}, {"remote": "anywhere"}, {"places": [" "]}])
def test_search_area_without_places_keeps_all_and_says_so(
    tmp_path, monkeypatch, catalogue, capsys, search_area
):
    workspace = sweep_workspace(tmp_path, catalogue, search_area=search_area)
    output = sweep_one(monkeypatch, workspace, "London")
    assert "Implementation Consultant — Acme" in output
    assert "no Search area set: all locations kept" in capsys.readouterr().out
    assert "no Search area set: all locations kept" in output


def test_sweep_with_search_area_does_not_report_none(tmp_path, monkeypatch, catalogue, capsys):
    workspace = sweep_workspace(tmp_path, catalogue, search_area=AU)
    output = sweep_one(monkeypatch, workspace, "Brisbane, QLD")
    assert "no Search area set" not in capsys.readouterr().out + output


def test_all_locations_bypasses_search_area(tmp_path, monkeypatch, catalogue):
    workspace = sweep_workspace(tmp_path, catalogue, search_area=AU)
    output = sweep_one(monkeypatch, workspace, "London", "--all-locations")
    assert "Implementation Consultant — Acme" in output


def test_fixture_workspace_has_search_area_and_track_references():
    assert sw.config(FIXTURE)["search_area"] == {"places": ["Australia"], "remote": "in-places"}
    catalogue = sw.load_role_catalogue(FIXTURE)
    assert {role["track"] for role in catalogue["roles"]} == {"SWE", "Platform"}
    # The shared fixture also exercises the documented sources/blockers shape.
    assert sw.resolve_sources(FIXTURE) == [("greenhouse", ["cultureamp", "buildkite"])]
    blockers, _ = sw.load_blockers(FIXTURE)
    assert blockers


def test_gazetteer_is_neutral_data():
    gazetteer = json.loads((ROOT / "plugin" / "gazetteer.json").read_text())
    assert {"countries", "groups"} <= gazetteer.keys()
    assert "Australia" in gazetteer["countries"]


# ------------------------------------------------------------------- dedupe


def test_norm_url_strips_query_fragment_slash():
    a = sw.norm_url(
        "https://Boards.greenhouse.io/acme/jobs/123/?gh_src=abc&utm_source=x#apply"
    )
    assert a == "https://boards.greenhouse.io/acme/jobs/123"
    assert sw.norm_url("https://x.com/a/b),") == "https://x.com/a/b"


def test_norm_key_ignores_noise():
    assert sw.norm_key("Acme Pty Ltd", "Senior Engineer!") == sw.norm_key(
        "ACME", "senior engineer"
    )
    assert sw.norm_key("EdTech via Michael Page", "Dev") == sw.norm_key("EdTech", "Dev")


def candidate(title, url):
    return dict(
        title=title,
        company="Fictional Labs",
        location="Brisbane, Australia",
        url=url,
        content="",
        type_hint="",
        closes="",
        source="lever:fixture",
    )


def test_seen_urls_include_every_status(tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    seen = sw.seen_urls(workspace)
    assert "https://example.com/status-dismissed" in seen
    assert "https://example.com/status-offer" in seen
    assert "https://example.test/shared" in seen
    assert "" not in seen


def test_sweep_drops_candidates_already_in_any_application(tmp_path, monkeypatch):
    workspace = copy_fixture_workspace(tmp_path)
    candidates = [
        candidate("Software Engineer (new)", "https://example.test/jobs/new?utm_source=x"),
        candidate("Software Engineer (dismissed)", "https://example.com/status-dismissed"),
        candidate("Software Engineer (docs dismissed)", "https://example.test/shared"),
    ]
    output = run_sweep(monkeypatch, workspace, lambda target: candidates, "--track", "SWE")
    assert "### Software Engineer (new) — Fictional Labs" in output
    assert "dismissed" not in output
    assert "3 fetched, 1 matched, 2 already known" in output


def test_sweep_drops_within_run_duplicates(tmp_path, monkeypatch):
    workspace = copy_fixture_workspace(tmp_path)
    duplicates = [
        candidate("Software Engineer One", "https://example.test/jobs/dupe?utm_source=a"),
        candidate("Software Engineer One", "https://example.test/jobs/dupe?utm_source=b"),
    ]
    output = run_sweep(monkeypatch, workspace, lambda target: duplicates, "--track", "SWE")
    assert output.count("### Software Engineer One — Fictional Labs") == 1


def test_sweep_writes_the_expected_sections(tmp_path, monkeypatch):
    workspace = copy_fixture_workspace(tmp_path)
    candidates = [
        candidate("Software Engineer New", "https://example.test/jobs/new"),
        candidate(
            "Software Engineer Blocked",
            "https://example.test/jobs/blocked",
        ),
    ]
    candidates[1]["content"] = "Australian citizenship is required."
    output = run_sweep(monkeypatch, workspace, lambda target: candidates, "--track", "SWE")
    for heading in (
        "# Sweep candidates",
        "## Source health",
        "## Role coverage",
        "## To assess (1)",
        "## Blocked (1)",
        "## Not reachable by this script",
    ):
        assert heading in output


def test_sweep_has_no_legacy_markdown_read_paths():
    source = SCRIPT.read_text()
    for legacy in ("SHORTLIST.md", "DISMISSED.md", "TRACKER.md"):
        assert legacy not in source


# ------------------------------------------------------------- source health


def test_dead_board_is_failed_not_zero(monkeypatch):
    def boom(url, timeout=25):
        raise sw.SourceError(f"{url} -- 404")

    monkeypatch.setattr(sw, "get", boom)
    jobs, health = sw.collect("greenhouse", "nope", sw.from_greenhouse)
    assert jobs == [] and health["ok"] is False and "404" in health["note"]


def test_bad_json_is_failed(monkeypatch):
    monkeypatch.setattr(sw, "get", lambda url, timeout=25: "<html>not json")
    _jobs, health = sw.collect("ashby", "x", sw.from_ashby)
    assert not health["ok"] and "bad JSON" in health["note"]


def test_empty_board_is_ok_zero(monkeypatch):
    monkeypatch.setattr(
        sw, "get", lambda url, timeout=25: json.dumps({"jobs": []})
    )
    jobs, health = sw.collect("greenhouse", "quiet", sw.from_greenhouse)
    assert jobs == [] and health["ok"] is True and health["count"] == 0
    assert "ok (0 posted)" in "\n".join(sw.health_lines([health]))


def test_health_lines_flag_failures():
    text = "\n".join(
        sw.health_lines(
            [
                {"source": "lever", "target": "x", "ok": False, "count": 0, "note": "boom"},
                {"source": "ashby", "target": "y", "ok": True, "count": 3, "note": ""},
            ]
        )
    )
    assert "1 of 2 sources FAILED" in text and "**FAILED**" in text


def test_lever_error_object_is_failure(monkeypatch):
    monkeypatch.setattr(
        sw, "get", lambda url, timeout=25: '{"ok":false,"error":"Document not found"}'
    )
    _jobs, health = sw.collect("lever", "ghost", sw.from_lever)
    assert not health["ok"] and "not found" in health["note"]


# ------------------------------------------------------------- company names


def test_ashby_company_is_readable(monkeypatch):
    payload = {
        "jobs": [
            {
                "title": "Dev",
                "location": "Sydney",
                "jobUrl": "u",
                "descriptionHtml": "<p>x</p>",
            }
        ]
    }
    monkeypatch.setattr(sw, "get", lambda url, timeout=25: json.dumps(payload))
    assert sw.from_ashby("relevance-ai")[0]["company"] == "Relevance Ai"


def test_greenhouse_company_fallback(monkeypatch):
    payload = {
        "jobs": [
            {
                "title": "Dev",
                "location": {"name": "Brisbane, Australia"},
                "absolute_url": "u",
                "content": "",
            }
        ]
    }
    monkeypatch.setattr(sw, "get", lambda url, timeout=25: json.dumps(payload))
    assert sw.from_greenhouse("octopusdeploy")[0]["company"] == "Octopusdeploy"


def test_smartrecruiters_gets_content_for_blocker_scan(monkeypatch):
    listing = {
        "content": [
            {
                "id": "1",
                "name": "Software Engineer",
                "company": {"name": "Wise Co"},
                "location": {"city": "Brisbane", "country": "au"},
            },
            {"id": "2", "name": "Chef", "location": {}},
        ]
    }
    detail = {
        "jobAd": {
            "sections": {"jobDescription": {"text": "<p>Must hold NV1 clearance.</p>"}}
        }
    }
    calls = []

    def fake(url, timeout=25):
        calls.append(url)
        return json.dumps(detail if url.endswith("/1") else listing)

    monkeypatch.setattr(sw, "get", fake)
    monkeypatch.setattr(sw, "ACTIVE_TRACKS", ["SWE"])
    jobs = sw.from_smartrecruiters("wise")
    assert jobs[0]["company"] == "Wise Co" and jobs[0]["location"] == "Brisbane, AU"
    assert "NV1" in jobs[0]["content"]
    assert jobs[1]["content"] == ""
    assert not any(url.endswith("/2") for url in calls)
    assert blockers_in(jobs[0]["content"])


def test_michael_page_company_and_cap(monkeypatch):
    paths = "".join(
        f'<a href="/job-detail/dev-{i:02d}/ref/jn-{i:02d}">x</a>' for i in range(30)
    )
    fetched = []

    def fake(url, timeout=25):
        fetched.append(url)
        return paths if "/jobs/" in url else "<p>Permanent role</p>"

    monkeypatch.setattr(sw, "get", fake)
    monkeypatch.setattr(sw, "MP_DELAY", 0)
    jobs = sw.from_michael_page(
        "https://www.michaelpage.com.au/jobs/technology"
    )
    assert len(jobs) == 30
    assert len(fetched) == 1 + sw.MP_MAX_ADS
    assert all("via Michael Page" not in job["company"] for job in jobs)
    assert jobs[0]["location"] == "Technology"
    assert jobs[-1]["content"] == ""
    _jobs, health = sw.collect("michaelpage", "u", lambda target: jobs)
    assert "cap" in health["note"]
