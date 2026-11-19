"""The showcase Workspace's documents still pass the build gates.

examples/workspace ships built PDFs. If a Profile edit or a tightened gate would make
build.py refuse one of its resumes, letters or its CV, the checked-in PDFs no longer
match what a rebuild would do; this catches that without pandoc or typst. Rebuild with
the commands in examples/README.md.
"""

import importlib.util
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
SHOWCASE = ROOT / "examples" / "workspace"
SCRIPT = ROOT / "plugin" / "scripts" / "build.py"


@pytest.fixture(scope="module")
def build():
    spec = importlib.util.spec_from_file_location("build_showcase", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.DATA = str(SHOWCASE)
    module.load_config()
    return module


def gate(name, check):
    """Run one gate; a refusal fails the test with build.py's own message."""
    try:
        check()
    except SystemExit as refusal:
        pytest.fail(f"{name}: build.py would refuse it: {refusal}")


def written_applications(build):
    for application in build.meta_module.scan(SHOWCASE):
        if not application.malformed and application.has_resume:
            yield application


def test_showcase_has_written_documents(build):
    assert len(list(written_applications(build))) >= 8


def test_every_showcase_document_passes_the_gates(build):
    profile = build.Profile.read(SHOWCASE)
    for application in written_applications(build):
        folder = Path(application.folder)
        label = folder.relative_to(SHOWCASE).as_posix()
        company = build.titlecase(application.company)
        subs = {"HEADLINE": build.headline_for(profile.master, application.track)}
        jd = (folder / "jd.md").read_text()
        if application.pdfs:
            letter = build._read(folder / "cover-letter.md")
            gate(label, lambda: build.check_letter_present(application, letter))
        for name in ("resume.md", "cover-letter.md"):
            text = build._read(folder / name)
            if text.strip():
                gate(
                    f"{label}/{name}",
                    lambda: build.run_gates(name, text, company, subs, profile, jd),
                )


def test_showcase_cv_passes_the_gates(build):
    profile = build.Profile.read(SHOWCASE)
    text = (SHOWCASE / "cv" / "general.md").read_text()
    gate("cv/general.md", lambda: build.check_key_skills(text, profile.master, "cv/general.md"))
    gate(
        "cv/general.md",
        lambda: build.check_facts("cv/general.md", text, "resume", profile.master, profile.projects),
    )
