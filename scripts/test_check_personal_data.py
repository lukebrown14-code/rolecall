import importlib.util
import subprocess
import sys
from pathlib import Path


SCRIPT = Path(__file__).with_name("check-personal-data.py")
spec = importlib.util.spec_from_file_location("personal_data", SCRIPT)
scanner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scanner)


def scan_text(tmp_path, text):
    subprocess.run(["git", "init", "--quiet", str(tmp_path)], check=True)
    path = tmp_path / "example.txt"
    path.write_text(text)
    subprocess.run(["git", "-C", str(tmp_path), "add", "example.txt"], check=True)
    return scanner.scan(tmp_path, set())


def scan_paths(tmp_path, files):
    subprocess.run(["git", "init", "--quiet", str(tmp_path)], check=True)
    for name, text in files.items():
        path = tmp_path / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        subprocess.run(["git", "-C", str(tmp_path), "add", "-f", name], check=True)
    return scanner.scan(tmp_path, set())


BLOCKED = [
    "workspace.json",
    "config.toml",
    "jobs/SWE/acme/jd.md",
    "profile/master-profile.md",
    "cv/resume.md",
    "SWE/acme/meta.yml",
    "docs/sent/letter.md",
    "out/Resume.pdf",
    ".rolecall-local",
    "plugin/jobs/roles.json",
]
ALLOWED = [
    "fixtures/workspace/workspace.json",
    "fixtures/config.toml",
    "fixtures/workspace/jobs/SWE/acme/meta.yml",
    "fixtures/workspace/profile/master-profile.md",
    "fixtures/workspace/jobs/SWE/x/out/Resume.pdf",
    "examples/workspace/cv/resume.md",
    "README.md",
    "plugin/templates/profile.md",
]


def test_blocked_workspace_paths_are_reported_with_the_rule(tmp_path):
    for index, name in enumerate(BLOCKED):
        findings = scan_paths(tmp_path / str(index), {name: "x\n"})
        assert len(findings) == 1, name
        assert findings[0].startswith(f"{name}: ")
        assert "tracked outside fixtures/ and examples/" in findings[0]


def test_fixture_and_example_paths_are_allowed(tmp_path):
    for index, name in enumerate(ALLOWED):
        assert scan_paths(tmp_path / str(index), {name: "x\n"}) == [], name


def test_non_fictional_email_in_fixtures_is_reported(tmp_path):
    address = "person" + "@private-domain.com"
    for index, name in enumerate(("fixtures/a.md", "examples/a.md")):
        findings = scan_paths(tmp_path / str(index), {name: f"Mail {address}\n"})
        assert len(findings) == 1
        assert findings[0].startswith(f"{name}:1: email outside a fictional domain")


def test_fictional_emails_in_fixtures_pass(tmp_path):
    text = "a" + "@example.test b" + "@example.com c" + "@mail.example.org\n"
    assert scan_paths(tmp_path, {"fixtures/a.md": text}) == []


def test_phone_in_fixtures_is_still_reported(tmp_path):
    findings = scan_paths(tmp_path, {"fixtures/a.md": "Call +61" + " 412 345 678\n"})
    assert findings == ["fixtures/a.md:1: phone"]


def test_optional_terms_file_still_works(tmp_path):
    findings = scan_paths(tmp_path, {"fixtures/a.md": "has synthetic-term\n"})
    assert findings == []
    assert scanner.scan(tmp_path, {"synthetic-term"}) == ["fixtures/a.md:1: Owner-specific term"]


def test_finds_us_national_phone_numbers(tmp_path):
	findings = scan_text(tmp_path, "Call (415) 555-0132.\n")
	assert findings == ["example.txt:1: phone"]
	findings = scan_text(tmp_path, "Call 212-555-0198.\n")
	assert findings == ["example.txt:1: phone"]


def commit(root, name, text, message):
    (root / name).write_text(text)
    subprocess.run(["git", "-C", str(root), "add", "-A"], check=True)
    subprocess.run(
        ["git", "-C", str(root), "-c", "user.name=T", "-c", "user.email=t" + "@example.test",
         "commit", "--quiet", "-m", message],
        check=True,
    )


def test_history_finds_terms_removed_from_the_tree(tmp_path):
    subprocess.run(["git", "init", "--quiet", str(tmp_path)], check=True)
    commit(tmp_path, "notes.md", "Worked at Initech.\n", "Add notes")
    commit(tmp_path, "notes.md", "Worked somewhere.\n", "Mention Initech again")

    assert scanner.scan(tmp_path, {"initech"}) == []
    findings = scanner.scan(tmp_path, {"initech"}, scanner.history_texts(tmp_path))
    assert len(findings) == 2
    assert any(f.endswith(":notes.md:1: Owner-specific term") for f in findings)
    assert any(f.startswith("commit ") and " message:1: " in f for f in findings)


def test_github_scans_issues_pulls_comments_and_reviews(tmp_path):
    responses = {
        "repos/o/r/issues?state=all&per_page=100": [
            {"number": 1, "title": "Initech bug", "body": None, "pr": False},
            {"number": 2, "title": "Fix", "body": "ok\nfrom Initech", "pr": True},
        ],
        "repos/o/r/issues/comments?per_page=100": [
            {"id": 7, "issue_url": "https://api.github.com/repos/o/r/issues/1", "body": "Initech"},
        ],
        "repos/o/r/pulls/comments?per_page=100": [
            {"id": 8, "pull_request_url": "https://api.github.com/repos/o/r/pulls/2", "body": "x"},
        ],
        "repos/o/r/pulls/2/reviews?per_page=100": [{"id": 9, "body": "Initech"}],
    }
    texts = scanner.github_texts("o/r", lambda path, _jq: responses[path])

    assert scanner.scan(tmp_path, {"initech"}, texts) == [
        "issue #1 title:1: Owner-specific term",
        "pr #2 body:2: Owner-specific term",
        "#1 comment 7:1: Owner-specific term",
        "pr #2 review 9:1: Owner-specific term",
    ]


def test_history_finds_workspace_paths_removed_from_the_tree(tmp_path):
    subprocess.run(["git", "init", "--quiet", str(tmp_path)], check=True)
    (tmp_path / "profile").mkdir()
    commit(tmp_path, "profile/master-profile.md", "x\n", "Add profile")
    subprocess.run(["git", "-C", str(tmp_path), "rm", "-q", "-r", "profile"], check=True)
    commit(tmp_path, "README.md", "x\n", "Remove profile")

    assert scanner.scan(tmp_path, set()) == []
    findings = scanner.scan(tmp_path, set(), scanner.history_texts(tmp_path))
    assert len(findings) == 1
    assert findings[0] == (
        "history:profile/master-profile.md: Workspace profile folder is tracked outside fixtures/ "
        "and examples/ (Workspace data stays in the Workspace)"
    )

REAL_EMAIL = "jane.real" + "@private-domain.com"


def git(root, *args):
    subprocess.run(["git", "-C", str(root), *args], check=True, capture_output=True)


def staged_repo(tmp_path, files):
    """Stage each file's first bytes, then overwrite the working copy with its second."""
    subprocess.run(["git", "init", "--quiet", str(tmp_path)], check=True)
    for name, (staged, working) in files.items():
        path = tmp_path / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(staged)
        git(tmp_path, "add", "-f", name)
        path.write_bytes(working)
    return tmp_path


def test_staged_mode_scans_the_staged_blob_not_the_clean_working_copy(tmp_path):
    root = staged_repo(tmp_path, {"notes.md": (f"contact: {REAL_EMAIL}\n".encode(), b"clean\n")})
    assert scanner.scan(root, set(), staged=True) == ["notes.md:1: email"]
    assert scanner.scan(root, set()) == []  # the working tree alone misses it


def test_staged_mode_passes_a_clean_stage_with_a_dirty_working_copy(tmp_path):
    root = staged_repo(tmp_path, {"notes.md": (b"clean\n", f"contact: {REAL_EMAIL}\n".encode())})
    assert scanner.scan(root, set(), staged=True) == []


def test_staged_mode_cli_exits_non_zero_on_staged_personal_data(tmp_path):
    root = staged_repo(tmp_path, {"notes.md": (f"contact: {REAL_EMAIL}\n".encode(), b"clean\n")})
    staged = subprocess.run(
        [sys.executable, str(SCRIPT), "--staged", str(root)], capture_output=True, text=True
    )
    assert staged.returncode == 1
    assert "notes.md:1: email" in staged.stdout
    working = subprocess.run([sys.executable, str(SCRIPT), str(root)], capture_output=True, text=True)
    assert working.returncode == 0


def test_staged_mode_applies_the_workspace_rule_to_renames_and_deletions(tmp_path):
    root = staged_repo(tmp_path, {"notes.md": (b"x\n", b"x\n"), "workspace.json": (b"{}\n", b"{}\n")})
    (root / "profile").mkdir()
    git(root, "mv", "notes.md", "profile/notes.md")
    git(root, "rm", "--cached", "--quiet", "workspace.json")
    assert scanner.scan(root, set(), staged=True) == [
        "profile/notes.md: Workspace profile folder is tracked outside fixtures/ and examples/ "
        "(Workspace data stays in the Workspace)"
    ]


def test_staged_mode_reads_the_staged_allowlist(tmp_path):
    allowlist = f'["{REAL_EMAIL}"]\n'.encode()
    root = staged_repo(
        tmp_path,
        {
            "notes.md": (f"{REAL_EMAIL}\n".encode(), f"{REAL_EMAIL}\n".encode()),
            "scripts/personal-data-allowlist.json": (b"[]\n", allowlist),
        },
    )
    assert scanner.scan(root, set(), staged=True) == ["notes.md:1: email"]
    assert scanner.scan(root, set()) == []


def test_non_utf8_files_are_scanned_in_both_modes(tmp_path):
    latin1 = f"Caf\xe9 contact {REAL_EMAIL}\n".encode("latin-1")
    root = staged_repo(tmp_path, {"export.csv": (latin1, latin1)})
    assert scanner.scan(root, set()) == ["export.csv:1: email"]
    assert scanner.scan(root, set(), staged=True) == ["export.csv:1: email"]
