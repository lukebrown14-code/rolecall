"""capture-job follows an aggregator listing to the employer's own ad (ticket #220)."""

import http.server
import importlib.util
import json
import sys
import threading
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
sys.path.insert(0, str(SCRIPTS))

import aggregator  # noqa: E402
import meta as meta_store  # noqa: E402


def load_newjob():
    spec = importlib.util.spec_from_file_location("nj_aggregator", SCRIPTS / "new-job.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


BASE = "https://au.jora.com/job/abc"


def page(*anchors):
    return "<html><body>" + "".join(anchors) + "</body></html>"


def test_picks_an_apply_link_to_the_employer():
    html = page('<a href="/about">About</a>',
                '<a href="https://careers.acme.test/jobs/42">Apply on company site</a>')
    assert aggregator.original_link(html, BASE) == "https://careers.acme.test/jobs/42"


def test_an_ats_link_wins_over_a_plain_text_match():
    html = page('<a href="https://acme.test/careers">Company page</a>',
                '<a href="https://boards.greenhouse.io/acme/jobs/7">Job</a>')
    assert aggregator.original_link(html, BASE) == "https://boards.greenhouse.io/acme/jobs/7"


def test_unwraps_the_aggregators_own_redirect_link():
    html = page('<a href="/jobapply?url=https%3A%2F%2Fjobs.lever.co%2Facme%2F9">Apply now</a>')
    assert aggregator.original_link(html, BASE) == "https://jobs.lever.co/acme/9"


def test_a_relative_link_is_resolved_only_against_the_page():
    html = page('<a href="/apply/3">Apply</a>')
    assert aggregator.original_link(html, BASE) == ""  # same host: not the employer's


@pytest.mark.parametrize("href", [
    "https://www.facebook.com/sharer?u=x",
    "https://twitter.com/intent/tweet",
    "https://play.google.com/store/apps/x",
    "mailto:jobs@acme.test",
    "javascript:void(0)",
    "https://au.jora.com/other",
    "https://www.adzuna.com/job/1",
])
def test_ignores_links_that_are_not_the_employers_ad(href):
    assert aggregator.original_link(page(f'<a href="{href}">Apply</a>'), BASE) == ""


def test_never_invents_a_link_when_the_page_has_none():
    assert aggregator.original_link(page("<p>No links, just text</p>"), BASE) == ""
    assert aggregator.original_link("", BASE) == ""


def test_is_aggregator_matches_subdomains_only():
    assert aggregator.is_aggregator("https://au.jora.com/x")
    assert not aggregator.is_aggregator("https://notjora.com/x")
    assert not aggregator.is_aggregator("https://acme.test/jora.com")


# ------------------------------------------------------------------ end to end

AD = ("Senior Engineer at Acme. " + "You will build and run services for our customers. " * 20
      + " Pay is $140k. Applications close 2031-01-31.")
CUT_DOWN = "Senior Engineer. " + "Build services. " * 30


class _Handler(http.server.BaseHTTPRequestHandler):
    original_url = ""

    def do_GET(self):  # noqa: N802
        if self.path == "/job/1":  # the aggregator, served as localhost
            body = (f"<html><head><title>Senior Engineer - Acme</title></head><body><p>{CUT_DOWN}</p>"
                    f'<a href="{self.original_url}">Apply on company site</a></body></html>')
        elif self.path == "/job/2":  # an aggregator whose employer link is dead
            body = (f"<html><body><p>{CUT_DOWN}</p>"
                    f'<a href="{self.original_url.replace("/ad", "/gone")}">Apply</a></body></html>')
        elif self.path == "/ad":
            body = f"<html><head><title>Senior Engineer at Acme</title></head><body><p>{AD}</p></body></html>"
        else:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(body.encode())

    def log_message(self, *args):
        pass


@pytest.fixture
def web(monkeypatch):
    httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
    port = httpd.server_port
    _Handler.original_url = f"http://127.0.0.1:{port}/ad"
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    # localhost plays the aggregator; 127.0.0.1 (the same server) plays the employer.
    monkeypatch.setattr(aggregator, "AGGREGATORS", ("localhost",))
    yield f"http://localhost:{port}"
    httpd.shutdown()


def test_fetches_the_original_ad_and_records_both_urls(web, tmp_path, capsys):
    workspace = copy_fixture_workspace(tmp_path)
    load_newjob().main(["--data", str(workspace), "--track", "SWE", "--url", web + "/job/1"])
    folder = next(f for f in (workspace / "jobs" / "SWE").iterdir() if "acme" in f.name)
    meta_text = (folder / "meta.yml").read_text().replace('"', "")
    assert f"\nurl: {_Handler.original_url}\n" in meta_text
    assert f"aggregator_url: {web}/job/1" in meta_text
    assert "build and run services" in (folder / "jd.md").read_text().lower()
    assert "jd_completeness" not in meta_text  # a full ad, not a summary
    assert meta_store.rescan(folder).malformed is None


def test_falls_back_to_the_listing_marked_summary_when_the_link_is_dead(web, tmp_path, capsys):
    workspace = copy_fixture_workspace(tmp_path)
    load_newjob().main(["--data", str(workspace), "--track", "SWE", "--url", web + "/job/2",
                        "--company", "Acme", "--role", "Senior Engineer"])
    folder = next(f for f in (workspace / "jobs" / "SWE").iterdir() if "acme" in f.name)
    meta_text = (folder / "meta.yml").read_text()
    assert "jd_completeness: summary" in meta_text.replace('"', "")
    assert "aggregator_url" not in meta_text
    assert f"url: {web}/job/2" in meta_text.replace('"', "")
    assert "may be partial" in capsys.readouterr().out


def test_no_follow_keeps_the_listing(web, tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    load_newjob().main(["--data", str(workspace), "--track", "SWE", "--url", web + "/job/1",
                        "--no-follow", "--company", "Acme", "--role", "Senior Engineer"])
    folder = next(f for f in (workspace / "jobs" / "SWE").iterdir() if "acme" in f.name)
    assert "aggregator_url" not in (folder / "meta.yml").read_text()


def test_listing_url_flag_records_the_listing_for_a_direct_capture(web, tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    load_newjob().main(["--data", str(workspace), "--track", "SWE", "--url",
                        _Handler.original_url, "--listing-url", "https://au.jora.com/job/zz"])
    folder = next(f for f in (workspace / "jobs" / "SWE").iterdir() if "acme" in f.name)
    assert "aggregator_url: https://au.jora.com/job/zz" in (folder / "meta.yml").read_text().replace('"', "")


def test_recapturing_the_same_listing_is_refused_as_a_duplicate(web, tmp_path):
    workspace = copy_fixture_workspace(tmp_path)
    nj = load_newjob()
    nj.main(["--data", str(workspace), "--track", "SWE", "--url", web + "/job/1"])
    with pytest.raises(SystemExit) as caught:
        nj.main(["--data", str(workspace), "--track", "SWE", "--url", web + "/job/1",
                 "--company", "Other Co", "--role", "Other Role"])
    assert "looks like a saved Application" in str(caught.value)


def test_fetch_only_reports_both_urls(web, tmp_path, capsys):
    workspace = copy_fixture_workspace(tmp_path)
    load_newjob().main(["--data", str(workspace), "--fetch-only", "--url", web + "/job/1"])
    payload = json.loads(capsys.readouterr().out)
    assert payload["meta"]["url"] == _Handler.original_url
    assert payload["meta"]["aggregator_url"] == web + "/job/1"
    assert "build and run services" in payload["jd"].lower()
