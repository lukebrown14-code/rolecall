"""The Owner's identity, read from the Profile's `## Contact` table.

The Profile (`profile/master-profile.md`) is the one source of truth for who
the Owner is: nothing in the build, the filenames or the PDF checks may hardcode
a name, email or link. This module parses the Contact table's `| Field | Value |`
rows.

Required fields are Name and Email. A Profile missing either fails the build,
naming the field (see build.py's identity gate).
"""

import re
from pathlib import Path

PROFILE_FILE = "master-profile.md"
REQUIRED = ("Name", "Email")

# A trailing italic aside on a cell is a note to the human, not the value --
# the real Work rights cell carries "*(the subclass never appears ...)*".
_NOTE = re.compile(r"\s*\*\([^)\n]*\)\*\s*$")
_EMPHASIS = re.compile(r"\*\*([^*\n]+)\*\*|\*([^*\n]+)\*|`([^`\n]+)`")
_ROW = re.compile(r"^\|\s*([^|]+?)\s*\|\s*(.*?)\s*\|\s*$")


def _cell(value):
    v = _NOTE.sub("", value.strip())
    return _EMPHASIS.sub(lambda m: next(g for g in m.groups() if g is not None), v).strip()


def parse_contact(text):
    """{Field: value} from the `## Contact` table; {} when absent."""
    m = re.search(r"^## Contact\s*\n(.*?)(?=\n## |\Z)", text, re.S | re.M)
    fields = {}
    for line in (m.group(1) if m else "").splitlines():
        row = _ROW.match(line)
        if not row:
            continue
        key = row.group(1)
        if key == "Field" or set(key) <= set(":- "):
            continue
        fields[key] = _cell(row.group(2))
    return fields


class Identity:
    def __init__(self, fields):
        self.fields = fields

    def get(self, field, default=""):
        return self.fields.get(field, default)

    @property
    def name(self):
        return self.get("Name")

    @property
    def email(self):
        return self.get("Email")

    @property
    def work_rights(self):
        return self.get("Work rights")

    def missing(self):
        """Required identity fields with no value, in REQUIRED order."""
        return [f for f in REQUIRED if not self.get(f)]

    def contact_line(self, github=True):
        """Line one of the header: email, phone, location, LinkedIn, and GitHub."""
        parts = [self.get("Email"), self.get("Phone"), self.get("Location"), self.get("LinkedIn")]
        if github and self.get("GitHub"):
            parts.append(self.get("GitHub"))
        return " | ".join(p for p in parts if p)


def read(data_dir):
    """The Identity in <data_dir>/profile/master-profile.md (empty fields if absent).

    Reads the file at call time: the Workspace changes per --data run, so the
    path must not be captured at import.
    """
    f = Path(data_dir) / "profile" / PROFILE_FILE
    return Identity(parse_contact(f.read_text() if f.exists() else ""))
