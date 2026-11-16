"""httpfetch.fetch is the one guarded web fetch new-job.py, sweep.py and rules.py share.

A local HTTP server stands in for the web: only http and https are read, a body is
capped, and every failure is a one-line FetchError carrying any HTTP status.
"""

import http.server
import sys
import threading
from pathlib import Path

import pytest

from conftest import FIXTURE

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
sys.path.insert(0, str(SCRIPTS))

import httpfetch  # noqa: E402


class _Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802
        if self.path == "/big":
            body = b"x" * 5000
        elif self.path == "/gone":
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"no such job")
            return
        else:
            body = b"<h1>Data Engineer</h1>"
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


@pytest.fixture(scope="module")
def server():
    httpd = http.server.HTTPServer(("127.0.0.1", 0), _Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    yield f"http://127.0.0.1:{httpd.server_port}"
    httpd.shutdown()


def test_fetch_reads_an_http_page(server):
    assert httpfetch.fetch(server + "/ok") == "<h1>Data Engineer</h1>"


def test_fetch_caps_the_body(server):
    assert len(httpfetch.fetch(server + "/big", max_bytes=1000)) == 1000


def test_fetch_reports_http_status_and_body(server):
    with pytest.raises(httpfetch.FetchError) as caught:
        httpfetch.fetch(server + "/gone")
    assert caught.value.status == 404
    assert "no such job" in caught.value.body


@pytest.mark.parametrize("url", ["example.com", "file:///etc/hostname", "ftp://x.test/a", ""])
def test_fetch_rejects_non_web_urls_before_reading(url, monkeypatch):
    def boom(*args, **kwargs):
        raise AssertionError("urlopen called")

    monkeypatch.setattr(httpfetch.urllib.request, "urlopen", boom)
    with pytest.raises(httpfetch.FetchError) as caught:
        httpfetch.fetch(url)
    assert caught.value.status is None
    assert "\n" not in str(caught.value)


@pytest.mark.parametrize("url", ["example.com", "file:///etc/os-release"])
def test_new_job_fetch_only_fails_in_one_clean_line(url):
    import subprocess

    result = subprocess.run(
        [sys.executable, str(SCRIPTS / "new-job.py"), "--fetch-only", "--url", url,
         "--data", str(FIXTURE)],
        capture_output=True, text=True,
    )
    assert result.returncode == 2
    assert "Traceback" not in result.stderr
    assert result.stdout == ""
    assert f"Could not fetch {url} -- not a web address" in result.stderr
