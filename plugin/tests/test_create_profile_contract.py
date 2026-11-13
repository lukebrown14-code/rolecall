"""The create-profile skill writes a Profile the Contact reader can consume."""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))
import contact  # noqa: E402

SKILL = ROOT / "plugin" / "skills" / "create-profile" / "SKILL.md"
WORKSPACE = ROOT / "fixtures" / "workspace"


def test_skill_documents_resume_and_questionnaire_paths_and_final_approval():
    text = SKILL.read_text()

    assert "From a resume" in text
    assert "From scratch" in text
    assert "cv/" in text
    assert "approve" in text.lower()


def test_documented_profile_shape_has_contact_readable_by_plugin():
    text = (WORKSPACE / "profile" / "master-profile.md").read_text()
    fields = contact.parse_contact(text)

    assert fields == {
        "Name": "Alex Quinn",
        "Email": "alex.quinn@example.test",
        "Phone": "+61 400 000 000",
        "Location": "Brisbane, QLD",
        "LinkedIn": "linkedin.com/in/alexquinn",
        "GitHub": "github.com/alexquinn",
        "Work rights": "Full working rights in Australia",
    }
    assert "## Experience" in text
    assert "### Acme — Brisbane, AU" in text
    assert "**Dates:** 2023 – Ongoing" in text
    assert "**Canonical title:** Developer" in text
    assert "Built services in Python" in text
    assert "Lifted unit coverage" in text
    assert "## Key Skills" in text
