"""The plugin's prose is part of its contract, so it gets checks too.

These tests read the skills, the writing rules and the style guide as files. They
catch two kinds of regression: personal data leaking into the code repo, and the
tone-layering / writer-rules promises drifting while the documents still build.
"""

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PLUGIN = ROOT / "plugin"
STYLE = PLUGIN / "styles" / "au.md"
RULES = PLUGIN / "writing-rules.md"
PREPARE = PLUGIN / "skills" / "prepare-documents" / "SKILL.md"

PROSE = [STYLE, RULES, PREPARE]

# Facts that belonged to the dashboard-era Owner, not to the plugin. None of
# these may appear anywhere the code repo ships.
PERSONAL = [
    r"\bLuke\b",
    r"\bBrown\b",
    r"lukebrown",
    r"\bRoke\b",
    r"AudioConnect",
    r"\bMelbourne\b",
    r"\b417\b",
    r"subclass",
    r"anz-software-engineer",
    r"national-security",
    r"[\w.+-]+@[\w-]+\.[\w.]+",
    r"\+61",
    r"linkedin\.com/in/",
]

# Dashboard-era file names and paths the plugin no longer has.
RETIRED = [
    "tracker.md",
    "shortlist.md",
    "dismissed.md",
    "jd front matter",
    "src/scripts",
    "bun run ui",
    "localhost:4330",
    "templates/style.md",
    "docs/writing-rules.md",
    "docs/writing-preferences.md",
]


def read(path):
    return path.read_text()


def test_prose_carries_no_personal_data():
    for path in PROSE:
        text = read(path)
        for pattern in PERSONAL:
            assert not re.search(pattern, text, re.IGNORECASE), (
                f"{path.relative_to(ROOT)} contains {pattern!r}"
            )


def test_writing_rules_drops_the_dashboard_contract():
    text = read(RULES).lower()
    for retired in RETIRED:
        assert retired not in text, f"writing-rules.md still mentions {retired!r}"


def test_prepare_documents_states_the_tone_layering_order():
    text = read(PREPARE)

    start = text.index("plugin/writing-rules.md")
    ordering = text[start:]

    positions = [
        ordering.index("plugin/writing-rules.md"),
        ordering.index("plugin/styles/au.md"),
        ordering.index("profile/writing-preferences.md"),
    ]
    assert positions == sorted(positions), "tone layering must be stated in order"
    assert "later" in text.lower() and "win" in text.lower()


def test_prepare_documents_keeps_resume_only_explicit():
    text = read(PREPARE)

    assert "resume_only=true" in text
    assert "only when" in text.lower()
    assert "never infer" in text.lower() or "not infer" in text.lower()


def test_every_plugin_skill_has_matching_front_matter():
    skills = sorted((PLUGIN / "skills").glob("*/SKILL.md"))
    assert skills, "expected plugin skills"

    for path in skills:
        lines = read(path).splitlines()
        assert lines[0] == "---", f"{path} has no front matter"

        block = []
        for line in lines[1:]:
            if line == "---":
                break
            block.append(line)
        else:
            raise AssertionError(f"{path} has unterminated front matter")

        fields = {}
        for line in block:
            key, _, value = line.partition(":")
            fields[key.strip()] = value.strip()

        assert fields.get("name") == path.parent.name, f"{path} name mismatch"
        assert fields.get("description"), f"{path} has no description"
