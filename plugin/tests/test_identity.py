"""identity.py parses the Owner's identity from the Profile's `## Contact` table.

The table is the single source of truth: the build, the filenames and the PDF
checks all read it. Every fixture here is fictional.
"""

import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))
import identity  # noqa: E402

FICTIONAL = """# Master Profile — Alex Quinn

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
"""


def test_parse_contact_reads_every_row_and_ignores_scaffolding():
    fields = identity.parse_contact(FICTIONAL)
    assert fields["Name"] == "Alex Quinn"
    assert fields["Email"] == "alex.quinn@example.test"
    assert fields["Work rights"] == "Full working rights in Australia"
    assert "Field" not in fields and ":--" not in fields


def test_contact_line_joins_the_profile_fields_and_github_is_optional():
    ident = identity.Identity(identity.parse_contact(FICTIONAL))
    line = "alex.quinn@example.test | +61 400 000 000 | Brisbane, QLD | linkedin.com/in/alexquinn"
    assert ident.contact_line(github=True) == line + " | github.com/alexquinn"
    # Analyst track omits GitHub.
    assert ident.contact_line(github=False) == line


def test_trailing_editor_note_on_a_cell_is_dropped():
    text = FICTIONAL.replace(
        "| Work rights | Full working rights in Australia |",
        "| Work rights | Full working rights in Australia *(the subclass never appears)* |",
    )
    assert identity.parse_contact(text)["Work rights"] == "Full working rights in Australia"


def test_missing_contact_section_names_the_required_fields():
    ident = identity.read("/nonexistent")
    assert ident.name == "" and ident.email == ""
    assert ident.missing() == ["Name", "Email"]


def test_missing_one_required_field_names_only_that_one():
    text = FICTIONAL.replace("| Email | alex.quinn@example.test |\n", "")
    assert identity.Identity(identity.parse_contact(text)).missing() == ["Email"]


@pytest.mark.parametrize("github,expected_has_github", [(True, True), (False, False)])
def test_contact_line_github_flag(github, expected_has_github):
    ident = identity.Identity(identity.parse_contact(FICTIONAL))
    assert ("github.com/alexquinn" in ident.contact_line(github=github)) is expected_has_github


def test_read_returns_empty_identity_when_the_profile_is_absent(tmp_path):
    ident = identity.read(tmp_path)
    assert ident.get("GitHub") == ""
    assert "github.com/" not in ident.contact_line(github=True)
