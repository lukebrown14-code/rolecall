"""Tests for plugin/scripts/build.py.

The unit cases drive the module directly with Typst and PDFs stubbed out; the
CLI cases run the script as a subprocess against a temp copy of
``fixtures/workspace`` with fake ``pandoc``/``typst``/``pdftotext`` binaries, so
nothing here needs the real toolchain or the network.
"""

import importlib.util
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "plugin" / "scripts" / "build.py"
FIXTURE = ROOT / "fixtures" / "workspace"

spec = importlib.util.spec_from_file_location("build", SCRIPT)
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


PROFILE = """# Master Profile

## Headlines (approved per track)

| Track | Headline |
| :-- | :-- |
| SWE | Full Stack Software Engineer |
| SA | Software Engineer \\| Solutions Architecture |

## Experience

### Acme — Sydney, AU

**Dates:** 2022 – 2025
**Canonical title:** Software Engineer

- Lifted unit coverage from **70% to 80%**

## Key Skills keyword pool

Python, Go, Docker

## Metrics register

| Metric | Value |
| :-- | :-- |
| Coverage | 70% → 80% |
| Years | 4+ |

## Facts that are NOT available

- No cloud certifications
- No Kubernetes production experience
- No SOAR or security incident management platform experience
"""


@pytest.fixture
def data(tmp_path, monkeypatch):
    monkeypatch.setattr(build, "DATA", tmp_path)
    (tmp_path / "profile").mkdir()
    (tmp_path / "profile" / "master-profile.md").write_text(PROFILE)
    return tmp_path


# A Profile with a Contact table. The fields are fictional: the build must
# take name, contact line and filenames from here, never from constants.
CONTACT_PROFILE = """# Master Profile — Alex Quinn

## Contact

| Field | Value |
| :-- | :-- |
| Name | Alex Quinn |
| Email | alex.quinn@example.test |
| Phone | +61 400 000 000 |
| Location | Brisbane, QLD |
| LinkedIn | linkedin.com/in/alexquinn |
| GitHub | github.com/alexquinn |
| Work rights | Full working rights in Australia |

## Headlines (approved per track)

| Track | Headline |
| :-- | :-- |
| SWE | Full Stack Software Engineer |

## Experience

### Acme — Brisbane, AU

**Dates:** Ongoing
**Canonical title:** Developer

- Built services in Python with Docker, SQL, Git, REST, CI/CD, Testing and Linux.

## Key Skills keyword pool

Python, Docker, SQL, Git, REST, CI/CD, Testing, Linux

## Facts that are NOT available

- No Kubernetes production experience
"""


def meta_text(resume_only=None, status="Lead", extra="legacy_key: \"kept\" # unknown\n"):
    text = (
        "schema: 1\n"
        "title: Developer\n"
        "company: Acme\n"
        "track: SWE\n"
        "url: https://example.test/acme\n"
        + extra
    )
    if resume_only is not None:
        text += f"resume_only: {resume_only}\n"
    text += f"status_log:\n  - {{status: {status}, at: 2026-09-24T01:00:00Z}}\n"
    return text


# --- resume_only gate ----------------------------------------------------------


@pytest.mark.parametrize(
    "value,allowed",
    [(None, False), ('""', False), ('"false"', False), ('"true"', True), ("true", True)],
)
def test_empty_letter_requires_explicit_resume_only(tmp_path, value, allowed):
    (tmp_path / "cover-letter.md").write_text("  \n")
    (tmp_path / "meta.yml").write_text(meta_text(resume_only=value))
    if allowed:
        build.check_letter_present(tmp_path)
    else:
        with pytest.raises(SystemExit):
            build.check_letter_present(tmp_path)


def test_resume_only_builds_resume_with_empty_letter(tmp_path, monkeypatch):
    job = tmp_path / "jobs" / "SWE" / "acme"
    job.mkdir(parents=True)
    (job / "jd.md").write_text("Acme needs a developer.\n")
    (job / "resume.md").write_text("Resume text")
    (job / "cover-letter.md").write_text(" \n")
    (job / "meta.yml").write_text(meta_text(resume_only='"true"'))
    (tmp_path / "profile").mkdir()
    (tmp_path / "profile" / "master-profile.md").write_text(CONTACT_PROFILE)
    monkeypatch.setattr(build, "need", lambda *args: None)
    monkeypatch.setattr(build, "run_gates", lambda *args: None)
    monkeypatch.setattr(build, "ats_check", lambda *args: None)
    monkeypatch.setattr(sys, "argv", ["build.py", str(job), "--data", str(tmp_path)])

    def make_pdf(md, template, output, subs):
        output.write_text("pdf")
        return output

    monkeypatch.setattr(build, "build", make_pdf)
    build.main()
    # The filename carries the Profile's name, not a hardcoded one.
    assert [p.name for p in (job / "out").glob("*.pdf")] == [
        "Alex Quinn Resume Acme Developer.pdf"
    ]
    # Lead -> Draft after PDFs.
    assert "status: Draft" in (job / "meta.yml").read_text()


# --- identity gate -------------------------------------------------------------


def test_identity_failure_accepts_matching_text():
    text = "Alex Quinn\nalex.quinn@example.test\n"
    assert build.identity_failure(text, "Alex Quinn", "alex.quinn@example.test") is None


def test_identity_failure_matches_the_name_case_insensitively():
    assert (
        build.identity_failure(
            "ALEX QUINN\nalex.quinn@example.test\n", "Alex Quinn", "alex.quinn@example.test"
        )
        is None
    )


def test_identity_failure_reports_a_wrong_name():
    reason = build.identity_failure(
        "Sam Rivers\nalex.quinn@example.test\n", "Alex Quinn", "alex.quinn@example.test"
    )
    assert "Alex Quinn" in reason


def test_identity_failure_reports_a_missing_email():
    reason = build.identity_failure(
        "Alex Quinn\n", "Alex Quinn", "alex.quinn@example.test"
    )
    assert "alex.quinn@example.test" in reason


def test_build_fails_and_names_the_missing_identity_field(tmp_path, monkeypatch):
    monkeypatch.setattr(build, "need", lambda *args: None)
    (tmp_path / "profile").mkdir()
    (tmp_path / "profile" / "master-profile.md").write_text(
        "# Master Profile\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n"
        "| Name | Alex Quinn |\n"
    )
    monkeypatch.setattr(
        sys, "argv", ["build.py", str(tmp_path / "jobs" / "SWE" / "acme"), "--data", str(tmp_path)]
    )
    with pytest.raises(SystemExit) as e:
        build.main()
    assert "Email" in str(e.value)


# --- document fixtures ---------------------------------------------------------

CHECK_JD = "Acme needs a developer to own Python services.\n"
CHECK_RESUME = (
    "# Alex Quinn\n\n## Professional Summary\n\n"
    "Backend developer who ships Python services.\n\n"
    "## Key Skills\n\nPython, Docker, SQL, Git, REST, CI/CD, Testing, Linux\n\n"
    "## Professional Experience\n\n"
    "### Developer — Acme | Ongoing\n\n"
    "- Built services in Python with Docker, SQL, Git and REST, owning CI/CD and Testing on Linux.\n"
)
CHECK_LETTER = (
    "# Alex Quinn\n\nAcme — Developer\n\nDear Acme team,\n\n"
    "Acme needs a developer to own Python services, and that is the shape of my recent work.\n\n"
    "One practical note: I hold full working rights in Australia.\n\n"
    "Kind regards,\nAlex Quinn\n"
)


# --- titlecase -----------------------------------------------------------------


@pytest.mark.parametrize(
    "raw,want",
    [
        ("software engineer", "Software Engineer"),
        ("head of the platform", "Head of the Platform"),
        ("UpGuard cyber-security", "UpGuard Cyber Security"),
        ("(cyber analyst)", "(Cyber Analyst)"),
        ("AWS_engineer", "AWS Engineer"),
    ],
)
def test_titlecase(raw, want):
    assert build.titlecase(raw) == want


# --- fit_verdict ---------------------------------------------------------------


def test_fit_verdict_shapes(tmp_path):
    (tmp_path / "fit.md").write_text("**Verdict: Strong — apply**\n")
    assert build.fit_verdict(tmp_path) == "Strong — apply"
    (tmp_path / "fit.md").write_text("## Verdict: **Worth applying.**\n")
    assert build.fit_verdict(tmp_path) == "Worth applying"


def test_fit_verdict_missing_or_empty(tmp_path):
    assert build.fit_verdict(tmp_path) == "—"
    (tmp_path / "fit.md").write_text("  \n")
    assert build.fit_verdict(tmp_path) == "—"


# --- key skills ----------------------------------------------------------------


def resume(skills, body="Built services in Python with Docker and tests. Went going gone."):
    return (
        f"# Alex Quinn\n\n## Professional Summary\n\nText.\n\n## Key Skills\n\n{skills}\n\n"
        f"## Professional Experience\n\n{body}\n"
    )


EIGHT = "Python, Docker, SQL, Git, REST, CI/CD, Java, Linux"


def test_kw_in_word_boundaries():
    assert not build.kw_in("Go", "going to the gold")
    assert build.kw_in("Go", "services written in Go.")
    assert not build.kw_in("Java", "JavaScript")
    assert build.kw_in("CI/CD", "own the CI/CD pipeline")


def test_key_skills_ok(data):
    f = data / "r.md"
    f.write_text(resume(EIGHT, "Python Docker SQL Git REST CI/CD Java Linux"))
    build.check_key_skills(f)


def test_key_skills_go_not_evidenced_by_going(data):
    f = data / "r.md"
    f.write_text(
        resume(
            "Python, Docker, SQL, Git, REST, CI/CD, Java, Rust",
            "Python Docker SQL Git REST CI/CD Java Rusty",
        )
    )
    with pytest.raises(SystemExit) as e:
        build.check_key_skills(f)
    assert "Rust" in str(e.value)


def test_key_skills_count_range(data):
    f = data / "r.md"
    f.write_text(resume("Python, Docker, Git", "Python Docker Git"))
    with pytest.raises(SystemExit) as e:
        build.check_key_skills(f)
    assert "3 keywords" in str(e.value)


def test_key_skills_block_missing_is_error(data):
    f = data / "r.md"
    f.write_text("# Alex Quinn\n\n## Professional Summary\n\nText.\n")
    with pytest.raises(SystemExit) as e:
        build.check_key_skills(f)
    assert "Key Skills" in str(e.value)


def test_key_skills_not_evidenced_by_gap_list(data):
    f = data / "r.md"
    f.write_text(
        resume(
            "Python, Docker, SQL, Git, REST, CI/CD, Java, Kubernetes",
            "Python Docker SQL Git REST CI/CD Java",
        )
    )
    with pytest.raises(SystemExit):
        build.check_key_skills(f)


# --- word count ----------------------------------------------------------------


def test_letter_word_count_and_cap(tmp_path):
    f = tmp_path / "cover-letter.md"
    f.write_text("# Alex Quinn\n\n" + " ".join(["word"] * 100) + "\n")
    assert build.letter_word_count(f) == 100
    f.write_text("# Alex Quinn\n\n" + " ".join(["word"] * (build.LETTER_MAX_WORDS + 1)) + "\n")
    with pytest.raises(SystemExit):
        build.check_letter_length(f)


# --- fact gate -----------------------------------------------------------------


def test_fact_gate_numbers():
    ok = "Lifted coverage from 70% to 80% over 4+ years at Acme, 2022 – 2025."
    assert build.fact_errors(ok, "resume", PROFILE) == []
    bad = build.fact_errors("Cut costs by 45% and led 12 engineers.", "resume", PROFILE)
    assert any("45%" in e for e in bad) and any("'12'" in e for e in bad)


def test_fact_gate_allows_a_manual_project_written_without_github():
    projects = "## Atlas\n\n**Dates:** 2024 – 2025\n\n- Scaled the service to 1,240 users.\n"
    assert build.fact_errors("Scaled the service to 1,240 users.", "resume", PROFILE, projects) == []
    assert build.fact_errors("Scaled the service to 9,999 users.", "resume", PROFILE, projects) != []


def test_fact_gate_not_available_and_placeholders():
    errs = build.fact_errors("Ran Kubernetes in production. TODO fix [Company]", "resume", PROFILE)
    assert any("Kubernetes" in e for e in errs)
    assert any("TODO" in e for e in errs) and any("[Company]" in e for e in errs)


def test_fact_gate_letter_may_own_a_gap():
    txt = "One gap to state directly: I have not run services on Kubernetes."
    assert build.fact_errors(txt, "letter", PROFILE) == []
    assert build.fact_errors(txt, "resume", PROFILE) != []


def test_fact_gate_experience_headings():
    good = (
        "## Professional Experience\n\n"
        "### Software Engineer — Acme, Sydney, AU | 2022 – 2025\n"
    )
    assert build.fact_errors(good, "resume", PROFILE) == []
    bad = "## Professional Experience\n\n### Software Engineer — Initech | 2022 – 2025\n"
    assert any("Initech" in e for e in build.fact_errors(bad, "resume", PROFILE))
    wrong = good.replace("2022 – 2025", "2021 – 2025")
    assert any("dates" in e for e in build.fact_errors(wrong, "resume", PROFILE))


def test_headline_gate():
    assert build.check_headline("Full Stack Software Engineer", "", PROFILE) == []
    assert build.check_headline("Staff Wizard | Magic", "Software Engineer", PROFILE)
    assert build.check_headline("Platform Engineer | Cloud", "We need a Platform Engineer", PROFILE) == []


# --- cover letter checks -------------------------------------------------------

LETTER = (
    "Acme Pty Ltd — Developer (Ref 123)\n\nDear Sam,\n\nBody about Acme.\n\n"
    "One practical note: working rights run six months.\n\nKind regards,\nAlex Quinn\n"
)


def test_letter_checks_pass():
    assert build.letter_errors(LETTER, "Acme Pty Ltd") == []


def test_letter_checks_fail():
    errs = build.letter_errors(LETTER.replace("Acme", "Zed"), "Acme")
    assert any("not mentioned" in e for e in errs)
    errs = build.letter_errors(LETTER + "\nOne practical note: again.", "Acme")
    assert any("2 times" in e for e in errs)
    errs = build.letter_errors(LETTER.replace("Body", "I am passionate about code. Body"), "Acme")
    assert any("passionate" in e for e in errs)
    assert any("practical note" in e for e in build.letter_errors("Dear Sam,\n\nAcme.\n", "Acme"))


# --- CLI seam: fixtures copy, fake toolchain -----------------------------------


def fake_tools(tmp_path):
    """Fake pandoc/typst/pdftotext so the CLI runs without the real toolchain."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    (bin_dir / "pandoc").write_text(
        '#!/bin/sh\nfor last; do :; done\ncat "$last"\n'
    )
    (bin_dir / "typst").write_text(
        '#!/bin/sh\nfor last; do :; done\nprintf "pdf" > "$last"\n'
    )
    (bin_dir / "pdftotext").write_text(
        "#!/bin/sh\nprintf 'Alex Quinn\\nalex.quinn@example.test\\n\\f'\n"
    )
    for name in ("pandoc", "typst", "pdftotext"):
        (bin_dir / name).chmod(0o755)
    return bin_dir


def cli_workspace(tmp_path, status="Lead"):
    workspace = tmp_path / "workspace"
    shutil.copytree(FIXTURE, workspace)
    (workspace / "profile").mkdir()
    (workspace / "profile" / "master-profile.md").write_text(CONTACT_PROFILE)
    folder = workspace / "jobs" / "SWE" / "acme-developer"
    folder.mkdir()
    (folder / "meta.yml").write_text(meta_text(status=status))
    (folder / "jd.md").write_text(CHECK_JD)
    (folder / "resume.md").write_text(CHECK_RESUME)
    (folder / "cover-letter.md").write_text(CHECK_LETTER)
    return workspace, folder


def run_cli(tmp_path, workspace, folder, *args):
    bin_dir = fake_tools(tmp_path)
    env = dict(os.environ)
    env["PATH"] = f"{bin_dir}:{env['PATH']}"
    return subprocess.run(
        [sys.executable, str(SCRIPT), str(folder), "--data", str(workspace), *args],
        capture_output=True,
        text=True,
        env=env,
    )


def draft_entries(text):
    return re.findall(r"status: Draft", text)


def without_appended_draft(text):
    return re.sub(r"^  - \{status: Draft, at: [^}]+\}\n", "", text, flags=re.M)


def test_cli_lead_build_appends_one_draft_and_names_pdfs(tmp_path):
    workspace, folder = cli_workspace(tmp_path)
    before = (folder / "meta.yml").read_text()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode == 0, result.stderr + result.stdout
    assert sorted(p.name for p in (folder / "out").glob("*.pdf")) == [
        "Alex Quinn Acme Developer Cover Letter.pdf",
        "Alex Quinn Resume Acme Developer.pdf",
    ]
    after = (folder / "meta.yml").read_text()
    assert draft_entries(after) == ["status: Draft"]
    # Unknown keys and inline comments survive byte for byte; only the appended
    # status entry is new.
    assert without_appended_draft(after) == before
    assert "legacy_key: \"kept\" # unknown" in after


@pytest.mark.parametrize("status", ["Draft", "Applied"])
def test_cli_non_lead_build_writes_pdfs_and_no_status_entry(tmp_path, status):
    workspace, folder = cli_workspace(tmp_path, status=status)
    before = (folder / "meta.yml").read_text()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode == 0, result.stderr + result.stdout
    assert (folder / "out" / "Alex Quinn Resume Acme Developer.pdf").exists()
    after = (folder / "meta.yml").read_text()
    assert after == before
    assert after.count("status:") == before.count("status:")


def test_cli_gate_failure_writes_no_pdf_and_no_status(tmp_path):
    workspace, folder = cli_workspace(tmp_path)
    (folder / "resume.md").write_text("# Alex Quinn\n\n## Professional Summary\n\nText.\n")
    before = (folder / "meta.yml").read_text()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode != 0
    assert "Key Skills" in result.stderr
    assert not list((folder / "out").glob("*.pdf"))
    assert (folder / "meta.yml").read_text() == before


def test_cli_later_letter_gate_failure_leaves_no_out_and_no_status(tmp_path):
    workspace, folder = cli_workspace(tmp_path)
    # The resume is valid; only the letter breaks a gate. The resume must not
    # be left behind as a PDF, and out/ must not even be created.
    (folder / "cover-letter.md").write_text(
        CHECK_LETTER.replace(
            "Acme needs a developer to own Python services, and that is the shape of my recent work.",
            "I am passionate about Python services.",
        )
    )
    before = (folder / "meta.yml").read_text()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode != 0
    assert "banned phrase" in result.stderr
    assert not (folder / "out").exists()
    assert not list(folder.glob(".build-*"))
    assert (folder / "meta.yml").read_text() == before


def test_cli_later_resume_skill_gate_failure_leaves_no_out(tmp_path):
    workspace, folder = cli_workspace(tmp_path)
    # The letter is valid; the resume has no Key Skills block.
    (folder / "resume.md").write_text("# Alex Quinn\n\n## Professional Summary\n\nText.\n")
    before = (folder / "meta.yml").read_text()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode != 0
    assert "Key Skills" in result.stderr
    assert not (folder / "out").exists()
    assert (folder / "meta.yml").read_text() == before


def test_cli_creates_no_tracker_no_sent_and_no_legacy_keys(tmp_path):
    workspace, folder = cli_workspace(tmp_path)
    remnant = workspace / "jobs" / "SWE" / "TRACKER.md"
    remnant_before = remnant.read_bytes()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode == 0, result.stderr + result.stdout
    assert not (workspace / "jobs" / "TRACKER.md").exists()
    assert remnant.read_bytes() == remnant_before
    assert not (folder / "sent").exists()
    after = (folder / "meta.yml").read_text()
    for key in ("applied", "followup", "contact", "salary", "outcome", "outcome_date",
                "rejection_reason"):
        assert f"{key}:" not in after


def test_cli_malformed_meta_is_refused_before_any_output(tmp_path):
    workspace, folder = cli_workspace(tmp_path)
    (folder / "meta.yml").write_text("schema: 1\ntitle: Developer\nstatus_log:\n")
    before = (folder / "meta.yml").read_bytes()

    result = run_cli(tmp_path, workspace, folder)

    assert result.returncode != 0
    assert "malformed" in result.stderr
    assert not (folder / "out").exists()
    assert (folder / "meta.yml").read_bytes() == before
