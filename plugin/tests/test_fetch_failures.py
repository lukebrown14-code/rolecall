"""`new-job.py --fetch-only` ends a failure with one JSON line the TUI reads (ticket #210).

Each kind is driven against a local HTTP server, and the exact lines the Go side parses
(internal/ui/fetch_internal_test.go) are pinned here so the two cannot drift apart.
"""

import http.server
import json
import socket
import subprocess
import sys
import threading
import time
from pathlib import Path

import pytest

from conftest import FIXTURE

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
sys.path.insert(0, str(SCRIPTS))

import httpfetch  # noqa: E402

PAGES = {
    "/missing": (404, "text/html", b"<h1>gone</h1>"),
    "/denied": (403, "text/html", b"<h1>denied</h1>"),
    "/limited": (429, "text/html", b"slow down"),
    "/doc.pdf": (200, "application/pdf", b"%PDF-1.4 not a page"),
    "/thin": (200, "text/html", b"<html><body><p>Loading...</p></body></html>"),
}


class _Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802
        if self.path == "/slow":
            time.sleep(3)
            return
        status, ctype, body = PAGES[self.path]
        self.send_response(status)
        self.send_header("Content-Type", ctype)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


@pytest.fixture(scope="module")
def server():
    httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    yield f"http://127.0.0.1:{httpd.server_port}"
    httpd.shutdown()


def fetch_only(*args, stdin=""):
    result = subprocess.run(
        [sys.executable, str(SCRIPTS / "new-job.py"), "--fetch-only", *args, "--data", str(FIXTURE)],
        input=stdin, capture_output=True, text=True,
    )
    assert result.stdout == ""
    assert "Traceback" not in result.stderr
    return result


def failure(result):
    """The JSON failure line: the last non-blank line of stderr."""
    last = [line for line in result.stderr.splitlines() if line.strip()][-1]
    return json.loads(last)


@pytest.mark.parametrize(
    "path,kind",
    [("/missing", "not_found"), ("/denied", "blocked"), ("/limited", "blocked"),
     ("/doc.pdf", "non_html"), ("/thin", "thin_page")],
)
def test_each_http_failure_kind(server, path, kind):
    result = fetch_only("--url", server + path)
    assert result.returncode == 2
    line = failure(result)
    assert line["kind"] == kind
    assert line["error"]
    assert "\n" not in line["error"]


def test_a_refused_connection_is_unreachable():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]  # closed again on exit: nothing listens here
    result = fetch_only("--url", f"http://127.0.0.1:{port}/")
    assert failure(result)["kind"] == "unreachable"


@pytest.mark.parametrize("url", ["example.com", "file:///etc/hostname"])
def test_a_bad_address_is_bad_url(url):
    result = fetch_only("--url", url)
    assert result.returncode == 2
    assert failure(result)["kind"] == "bad_url"
    assert f"Could not fetch {url} -- not a web address" in result.stderr


def test_a_login_walled_site_is_login_wall():
    result = fetch_only("--url", "https://www.seek.com.au/job/1")
    assert failure(result)["kind"] == "login_wall"


def test_too_short_text_is_too_short():
    result = fetch_only("--paste", stdin="tiny")
    assert result.returncode == 1
    assert failure(result)["kind"] == "too_short"


def test_a_timeout_is_a_timeout(server):
    with pytest.raises(httpfetch.FetchError) as caught:
        httpfetch.fetch(server + "/slow", timeout=1)
    assert caught.value.kind == "timeout"


def test_the_human_text_stays_above_the_json_line(server):
    result = fetch_only("--url", server + "/missing")
    assert "WARNING" in result.stderr
    assert "--paste" in result.stderr
    assert result.stderr.strip().splitlines()[-1].startswith('{"error"')


def test_without_fetch_only_there_is_no_json_line(server):
    result = subprocess.run(
        [sys.executable, str(SCRIPTS / "new-job.py"), "--track", "SWE", "--url",
         server + "/missing", "--data", str(FIXTURE)],
        capture_output=True, text=True,
    )
    assert result.returncode == 2
    assert '{"error"' not in result.stderr


def test_the_lines_the_go_parser_is_tested_with():
    """internal/ui/fetch_internal_test.go carries these exact lines."""
    import importlib.util

    spec = importlib.util.spec_from_file_location("nj_fail", SCRIPTS / "new-job.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    assert module.fail_line("not_found", "Could not fetch https://x.test/a -- HTTP Error 404: Not Found") == (
        '{"error": "Could not fetch https://x.test/a -- HTTP Error 404: Not Found", "kind": "not_found"}'
    )
