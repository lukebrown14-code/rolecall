"""next_step, contacts and close_reason read as the Go store reads them (SCHEMA.md)."""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))

import meta  # noqa: E402

BASE = "schema: 1\ntitle: T\ncompany: C\ntrack: SWE\n"


def read(tmp_path, extra):
    folder = tmp_path / "jobs" / "SWE" / "x"
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_text(BASE + extra + "status_log:\n  - {status: Applied, at: 2026-01-01}\n")
    return meta.rescan(folder)


def test_next_step_contacts_and_close_reason_read(tmp_path):
    app = read(
        tmp_path,
        'next_step: {at: 2026-12-03T10:30, what: "Phone screen, with Sam"} # keep\n'
        'contacts:\n  - {name: "Sam Lee", role: Recruiter, email: sam@example.test}\n'
        '  - {url: "https://example.test/in/kim"}\n'
        "close_reason: after-screen\n",
    )
    assert app.malformed is None, app.malformed
    assert app.next_step == {"At": "2026-12-03T10:30", "What": "Phone screen, with Sam"}
    assert [c["Name"] for c in app.contacts] == ["Sam Lee", ""]
    assert app.contacts[1]["URL"] == "https://example.test/in/kim"
    assert app.close_reason == "after-screen"
    assert app.to_dict()["NextStep"]["At"] == "2026-12-03T10:30"


def test_legacy_contact_reads_as_the_first_contact(tmp_path):
    app = read(tmp_path, "contact: Jo Park\n")
    assert app.contacts == [{"Name": "Jo Park", "Role": "", "Email": "", "URL": ""}]
    assert read_email(tmp_path / "e", "jo@example.test").contacts[0]["Email"] == "jo@example.test"


def read_email(tmp_path, value):
    tmp_path.mkdir()
    return read(tmp_path, f"contact: {value}\n")


def test_contacts_list_wins_over_legacy_contact(tmp_path):
    app = read(tmp_path, "contact: Old Name\ncontacts:\n  - {name: New}\n")
    assert [c["Name"] for c in app.contacts] == ["New"]


def test_patch_refuses_the_structured_keys(tmp_path):
    read(tmp_path, "")
    folder = tmp_path / "jobs" / "SWE" / "x"
    for key in ("contacts", "next_step"):
        try:
            meta.patch(folder, {key: "x"})
        except meta.MetaError:
            continue
        raise AssertionError(key)
