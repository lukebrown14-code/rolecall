"""The code repo ships no Owner personal data.

The prose guard lives in ``test_skills.py``; this one also covers the Python
scripts and their tests, where a ported default or comment could leak the old
Workspace's identity, city, visa subclass or dated wording. The guard file
itself necessarily contains the patterns, so it is skipped when scanning.
"""

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PLUGIN = ROOT / "plugin"
GUARD = Path(__file__).resolve()

# Owner-identifying terms from the dashboard era. The prose guard uses these.
PERSONAL = [
    r"\bLuke\b",
    r"\bBrown\b",
    r"lukebrown",
    r"\bRoke\b",
    "Audio" + "Connect",
    r"\bMelbourne\b",
    r"\b417\b",
    r"subclass",
    r"anz-software-engineer",
    r"national-security",
    r"[\w.+-]+@[\w-]+\.[\w.]+",
    r"\+61",
    r"linkedin\.com/in/",
]

# Scripts and tests may not carry the Owner's identity, home city, visa number
# or the old six-month cap wording. Generic visa gate wording (subclass /
# working holiday) is allowed in build.py.
SCRIPT_PERSONAL = [
    r"\bLuke\b",
    r"\bBrown\b",
    r"lukebrown",
    r"\bRoke\b",
    "Audio" + "Connect",
    r"\bMelbourne\b",
    r"\b417\b",
    r"six-month cap",
]


def code_files():
    return sorted([*PLUGIN.glob("scripts/*.py"), *PLUGIN.glob("tests/*.py")])


def test_scripts_and_tests_carry_no_owner_data():
    assert code_files(), "expected plugin scripts and tests"
    for path in code_files():
        if path == GUARD:
            continue
        text = path.read_text()
        for pattern in SCRIPT_PERSONAL:
            assert not re.search(pattern, text, re.IGNORECASE), (
                f"{path.relative_to(ROOT)} contains {pattern!r}"
            )
