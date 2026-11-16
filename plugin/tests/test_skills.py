"""The plugin's prose is part of its contract, so it gets checks too.

These tests read the skills, the writing rules and the style guide as files. They
catch two kinds of regression: personal data leaking into the code repo, and the
tone-layering / writer-rules promises drifting while the documents still build.
"""

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PLUGIN = ROOT / "plugin"
STYLE = PLUGIN / "styles" / "au.md"
RULES = PLUGIN / "writing-rules.md"
PREPARE = PLUGIN / "skills" / "prepare-documents" / "SKILL.md"

PROSE = [STYLE, RULES, PREPARE]

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


def test_writing_rules_drops_the_dashboard_contract():
    text = read(RULES).lower()
    for retired in RETIRED:
        assert retired not in text, f"writing-rules.md still mentions {retired!r}"


def test_prepare_documents_states_the_tone_layering_order():
    text = read(PREPARE)

    start = text.index("/writing-rules.md")
    ordering = text[start:]

    positions = [
        ordering.index("/writing-rules.md"),
        ordering.index("/styles/au.md"),
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


HELP = PLUGIN / "skills" / "help"
# Hyphenated words in the help guide that are plain English, not skill names.
HELP_WORDS = {"re-read", "re-reads"}


def test_help_names_only_skills_that_exist():
    import re

    text = read(HELP / "app.md")
    skills = {p.parent.name for p in (PLUGIN / "skills").glob("*/SKILL.md")}
    # A hyphenated word that is not a file name (no extension or slash after it).
    named = set(re.findall(r"\b([a-z]+(?:-[a-z]+)+)\b(?![./\w-])", text)) - HELP_WORDS
    assert named, "app.md names no skills"
    assert named <= skills, f"app.md names skills that do not exist: {sorted(named - skills)}"


def test_help_lists_every_machine_setting():
    import re

    keys = set(re.findall(r'toml:"([a-z_]+)"', read(ROOT / "internal" / "config" / "config.go")))
    app = read(HELP / "app.md")
    missing = sorted(k for k in keys if f"`{k}`" not in app)
    assert not missing, f"app.md is missing config.toml keys: {missing}"


def test_help_is_read_only():
    text = read(HELP / "SKILL.md").lower()
    assert "read only" in text and "changes nothing" in text


def test_skills_never_run_a_script_by_a_workspace_relative_path():
    # The Harness runs skills from inside the Workspace, where plugin/scripts/ does not
    # exist: a bare relative path made the agent hand-write meta.yml instead.
    for path in sorted((PLUGIN / "skills").rglob("*.md")):
        text = read(path)
        assert "python3 plugin/" not in text, f"{path} runs a script by a relative path"
        assert "`plugin/" not in text, f"{path} names a plugin file by a relative path"
        if "${CLAUDE_PLUGIN_ROOT}" in text and path.name == "SKILL.md":
            assert "**Plugin paths.**" in text, f"{path} uses the plugin root without the note"
