"""The default Templates, built with the real toolchain and read back as text.

An ATS reads a PDF the way ``pdftotext`` does, so these build a resume and a
cover letter through ``build.py`` and check the extracted text: the reading
order of the header, dates on their role's line, Key Skills in order, no word
split at a hyphen, and line breaks the Markdown asked for. Skipped when typst,
pandoc or pdftotext is not installed.
"""

import re
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "plugin" / "scripts" / "build.py"
FIXTURE = ROOT / "fixtures" / "workspace"

pytestmark = pytest.mark.skipif(
    not all(shutil.which(tool) for tool in ("typst", "pandoc", "pdftotext")),
    reason="needs typst, pandoc and pdftotext",
)

SKILLS = [
    "Python", "Docker", "SQL", "Git", "REST APIs", "CI/CD", "Testing", "Linux",
    "Event-driven architecture", "Observability", "Infrastructure as Code", "PostgreSQL",
]

PROFILE = f"""# Master Profile — Alex Quinn

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

**Dates:** Mar 2021 – Present
**Canonical title:** Developer

- Built full-platform services with {", ".join(SKILLS)}.

## Key Skills keyword pool

{", ".join(SKILLS)}
"""

# Bullets one character apart in width, so the line breaks at the hyphen in
# "full-platform" for at least one of them unless the Template prevents it.
BULLETS = "".join(
    f"- Moved the {'w' * width} team onto a single full-platform deployment model for every service.\n"
    for width in range(1, 25)
)

RESUME = f"""# Alex Quinn

## Professional Summary

Developer who builds full-platform services end to end, owning the infrastructure and the pipeline.

## Key Skills

{", ".join(SKILLS)}

## Technical Skills

**Languages:** Python, SQL
**Platforms:** Docker, Linux

## Experience

### Developer — Acme | Mar 2021 – Present

#### Brisbane product studio

{BULLETS}- Built services in Python with Docker, SQL and Linux.
"""

LETTER = (
    "Acme — Developer\n\nDear Hiring Manager,\n\n"
    + "I build Python services with Docker and SQL, and I own them in production. " * 12
    + "\n\nOne practical note: I hold full working rights in Australia.\n\n"
    "Kind regards,\nAlex Quinn\n"
)


@pytest.fixture(scope="module")
def built(tmp_path_factory):
    """(resume text, letter text, resume PDF bytes) from one real build."""
    workspace = tmp_path_factory.mktemp("templates") / "workspace"
    shutil.copytree(FIXTURE, workspace)
    (workspace / "profile" / "master-profile.md").write_text(PROFILE)
    folder = workspace / "jobs" / "SWE" / "acme-developer"
    folder.mkdir()
    (folder / "meta.yml").write_text(
        "schema: 1\ntitle: Developer\ncompany: Acme\ntrack: SWE\n"
        "status_log:\n  - {status: Lead, at: 2026-09-24T01:00:00Z}\n"
    )
    (folder / "jd.md").write_text("Acme needs " + ", ".join(SKILLS) + ".\n")
    (folder / "resume.md").write_text(RESUME)
    (folder / "cover-letter.md").write_text(LETTER)
    result = subprocess.run(
        [sys.executable, str(SCRIPT), str(folder), "--data", str(workspace)],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    out = folder / "out"
    resume = out / "Alex Quinn Resume Acme Developer.pdf"
    letter = out / "Alex Quinn Acme Developer Cover Letter.pdf"
    return text_of(resume), text_of(letter), resume.read_bytes()


def text_of(pdf):
    """Non-blank lines of the PDF's text in pdftotext's reading order."""
    out = subprocess.run(["pdftotext", str(pdf), "-"], capture_output=True, text=True).stdout
    return [line.strip() for line in out.splitlines() if line.strip()]


@pytest.mark.parametrize("which", [0, 1])
def test_header_reads_name_headline_rights_then_contact(built, which):
    lines = built[which]
    header = lines[:4]
    assert header[:3] == [
        "Alex Quinn",
        "Full Stack Software Engineer",
        "Full working rights in Australia",
    ]
    assert header[3] == (
        "alex.quinn@example.test | +61 400 000 000 | Brisbane, QLD"
        " | linkedin.com/in/alexquinn | github.com/alexquinn"
    )


def test_contact_items_are_links(built):
    uris = set(re.findall(rb"/URI ?\(([^)]*)\)", built[2]))
    # Built from the Profile's own values, so the personal-data scan sees only the
    # allowlisted fictional ones.
    phone = "+61 400 000 000".replace(" ", "")
    linkedin = "linkedin.com/in/alexquinn"
    assert {
        b"mailto:alex.quinn@example.test",
        ("tel:" + phone).encode(),
        ("https://" + linkedin).encode(),
        b"https://github.com/alexquinn",
    } <= uris


def test_dates_share_their_role_line(built):
    assert "Developer — Acme Mar 2021 – Present" in built[0]


def test_key_skills_read_in_order(built):
    lines = built[0]
    start = lines.index("KEY SKILLS") + 1
    end = lines.index("TECHNICAL SKILLS")
    run = " ".join(lines[start:end])
    assert [s.strip() for s in run.split("·")] == SKILLS


def test_no_word_is_split_at_a_hyphen(built):
    text = "\n".join(built[0] + built[1])
    assert "fullplatform" not in text
    assert text.count("full-platform") == 25


def test_bold_lines_and_sign_off_keep_their_line_breaks(built):
    assert "Languages: Python, SQL" in built[0]
    assert "Platforms: Docker, Linux" in built[0]
    assert built[1][-2:] == ["Kind regards,", "Alex Quinn"]
