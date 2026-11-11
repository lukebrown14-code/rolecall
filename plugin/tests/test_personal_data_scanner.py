import os
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SCANNER = ROOT / "scripts" / "check-personal-data.py"


def run_scanner(
    root: Path, *, env: dict[str, str] | None = None
) -> subprocess.CompletedProcess[str]:
    clean_env = os.environ.copy()
    clean_env.pop("ROLECALL_PERSONAL_DATA_TERMS", None)
    clean_env.pop("ROLECALL_PERSONAL_DATA_TERMS_FILE", None)
    if env:
        clean_env.update(env)
    return subprocess.run(
        [sys.executable, str(SCANNER), str(root)],
        check=False,
        capture_output=True,
        text=True,
        env=clean_env,
    )


def tracked_tree(tmp_path: Path, content: str) -> Path:
    root = tmp_path / "repo"
    root.mkdir(parents=True)
    (root / "note.md").write_text(content)
    scripts = root / "scripts"
    scripts.mkdir()
    (scripts / "personal-data-allowlist.json").write_text("[]\n")
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    subprocess.run(["git", "-C", str(root), "add", "note.md"], check=True)
    return root


def test_email_finding_reports_path_line_and_pattern(tmp_path: Path) -> None:
    address = "person" + "@private-domain.com"
    result = run_scanner(tracked_tree(tmp_path, f"safe\nContact {address}\n"))

    assert result.returncode == 1
    assert result.stdout == "note.md:2: email\n"


def test_phone_numbers_international_and_local_are_detected(tmp_path: Path) -> None:
    phones = ("+61" + " 412 345 678", "+1" + " (415) 555-0132")
    for index, phone in enumerate(phones):
        root = tracked_tree(tmp_path / str(index), f"Call {phone}\n")
        result = run_scanner(root)
        assert result.returncode == 1
        assert result.stdout == "note.md:1: phone\n"


def test_linkedin_profile_url_is_detected(tmp_path: Path) -> None:
    result = run_scanner(tracked_tree(tmp_path, "https://www.linkedin.com" + "/in/private-person\n"))

    assert result.returncode == 1
    assert result.stdout == "note.md:1: LinkedIn profile URL\n"


def test_ssh_git_remote_is_not_reported_as_an_email(tmp_path: Path) -> None:
    result = run_scanner(tracked_tree(tmp_path, "git@github.com:owner/repo.git\n"))

    assert result.returncode == 0
    assert result.stdout == ""


def test_street_address_is_detected(tmp_path: Path) -> None:
    result = run_scanner(tracked_tree(tmp_path, "42 Example" + " Street, Sampletown\n"))

    assert result.returncode == 1
    assert result.stdout == "note.md:1: street address\n"


def test_allowlisted_values_pass(tmp_path: Path) -> None:
    content = "alex.quinn" + "@example.test +61 400 000 000 linkedin.com/in/alexquinn\n"
    root = tracked_tree(tmp_path, content)
    (root / "scripts" / "personal-data-allowlist.json").write_text(
        '["alex.quinn\\u0040example.test", "+61 400 000 000", "linkedin.com/in/alexquinn"]\n'
    )
    result = run_scanner(root)

    assert result.returncode == 0
    assert result.stdout == ""
    assert "generic patterns only" in result.stderr


def test_only_tracked_files_are_scanned(tmp_path: Path) -> None:
    root = tracked_tree(tmp_path, "clean\n")
    (root / "untracked.md").write_text("someone" + "@private-domain.com\n")

    result = run_scanner(root)

    assert result.returncode == 0
    assert result.stdout == ""


def test_local_terms_file_finds_terms_without_printing_them(tmp_path: Path) -> None:
    term = "synthetic-private-term-75"
    root = tracked_tree(tmp_path, f"A note contains {term}.\n")
    terms = tmp_path / "terms.txt"
    terms.write_text(f"# local-only terms\n\n{term}\n")

    result = run_scanner(root, env={"ROLECALL_PERSONAL_DATA_TERMS_FILE": str(terms)})

    assert result.returncode == 1
    assert result.stdout == "note.md:1: Owner-specific term\n"
    assert term not in result.stdout + result.stderr


def test_default_terms_file_is_used(tmp_path: Path) -> None:
    term = "synthetic-default-term-75"
    root = tracked_tree(tmp_path, f"Contains {term}.\n")
    (root / ".rolecall-personal-data-terms").write_text(f"{term}\n")

    result = run_scanner(root)

    assert result.returncode == 1
    assert result.stdout == "note.md:1: Owner-specific term\n"
    assert term not in result.stdout + result.stderr


def test_environment_terms_find_terms_without_printing_them(tmp_path: Path) -> None:
    term = "synthetic-env-term-75"
    result = run_scanner(
        tracked_tree(tmp_path, f"Contains {term}.\n"),
        env={"ROLECALL_PERSONAL_DATA_TERMS": term},
    )

    assert result.returncode == 1
    assert result.stdout == "note.md:1: Owner-specific term\n"
    assert term not in result.stdout + result.stderr


def test_no_owner_terms_reports_generic_only_mode(tmp_path: Path) -> None:
    result = run_scanner(tracked_tree(tmp_path, "safe\n"))

    assert result.returncode == 0
    assert "generic patterns only" in result.stderr
