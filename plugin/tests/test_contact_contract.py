"""The Python reader and the Go store agree on the fixture Profile's Contact."""

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))
import contact  # noqa: E402

PROFILE = ROOT / "fixtures" / "workspace" / "profile" / "master-profile.md"
EXPECTED = ROOT / "fixtures" / "profile-contact.json"


def test_fixture_profile_contact_matches_the_shared_expectations():
    expected = json.loads(EXPECTED.read_text())
    assert contact.parse_contact(PROFILE.read_text()) == expected
