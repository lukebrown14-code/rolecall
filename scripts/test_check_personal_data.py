import importlib.util
import subprocess
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
