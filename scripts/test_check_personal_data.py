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
