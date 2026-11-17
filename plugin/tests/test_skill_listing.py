"""Every skill is well formed and listed, and the new ones keep the locked-fact rule."""

import re
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
SKILLS = sorted((ROOT / "plugin" / "skills").glob("*/SKILL.md"))
NEW = ("import-applications", "interview-prep", "answer-questions")


def front_matter(path):
    match = re.match(r"---\n(.*?)\n---\n", path.read_text(), re.S)
    assert match, f"{path} has no front matter"
    return dict(line.split(": ", 1) for line in match.group(1).splitlines())


@pytest.mark.parametrize("path", SKILLS, ids=lambda p: p.parent.name)
def test_skill_name_matches_its_folder_and_has_a_description(path):
    meta = front_matter(path)
    assert meta["name"] == path.parent.name
    assert len(meta["description"]) > 40


def test_the_new_skills_exist():
    assert {p.parent.name for p in SKILLS} >= set(NEW)


def test_the_readme_lists_every_skill():
    readme = (ROOT / "plugin" / "README.md").read_text()
    for path in SKILLS:
        assert f"`{path.parent.name}`" in readme, path.parent.name


@pytest.mark.parametrize("name", ["interview-prep", "answer-questions"])
def test_writing_skills_keep_the_profile_only_rule_and_one_file(name):
    text = (ROOT / "plugin" / "skills" / name / "SKILL.md").read_text()
    assert "Profile" in text and "evidence" in text.lower()
    assert "Never touch `meta.yml`" in text
    assert "${CLAUDE_PLUGIN_ROOT}" in text


def test_schema_names_the_additive_files():
    schema = (ROOT / "SCHEMA.md").read_text()
    assert "| `prep.md` | — |" in schema
    assert "| `answers.md` | — |" in schema
    assert "| `aggregator_url` | string | — |" in schema


def test_capture_job_says_to_fetch_the_raw_page():
    text = (ROOT / "plugin" / "skills" / "capture-job" / "SKILL.md").read_text()
    assert "Fetch the raw page, always" in text
    assert "aggregator_url" in text
    assert "--listing-url" in text
