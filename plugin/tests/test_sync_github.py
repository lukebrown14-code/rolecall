"""sync-github.py regenerates only the fact table in profile/projects.md.

An Owner who lists projects by hand writes them beside (or instead of) the
generated table. A refresh must not lose that prose: it replaces the table
between the marker and the rule, and nothing else.
"""

import importlib.util
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "plugin" / "scripts" / "sync-github.py"


def load():
    spec = importlib.util.spec_from_file_location("sync_under_test", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def fake_fetch(name):
    return {
        "name": name,
        "description": "",
        "private": not name.endswith("Delta"),
        "url": f"https://github.com/example/{name}",
        "homepage": "",
        "created": "2026-01-01",
        "pushed": "2026-02-01",
        "languages": ["Python"],
        "commits": 5,
    }


def test_refresh_replaces_the_table_and_keeps_a_hand_written_project(
    tmp_path, monkeypatch
):
    sync = load()
    profile = tmp_path / "profile"
    profile.mkdir()
    out = profile / "projects.md"
    out.write_text(
        "# Projects\n\n"
        f"{sync.MARKER}\n\n"
        "*Synced 2026-01-01 from github.com/example.*\n\n"
        "| Project | Visibility | Stack | Commits | Active | Link |\n"
        "| :-- | :-- | :-- | --: | :-- | :-- |\n"
        "| Old | public | Python | 1 | 2020-01-01 | — |\n\n"
        "---\n\n"
        "## Atlas (manual)\n\nHand-written facts about 1,240 users.\n"
    )
    monkeypatch.setattr(sync, "workspace_from_cli", lambda: tmp_path)
    monkeypatch.setattr(sync, "OWNER", "example")
    monkeypatch.setattr(sync, "REPOS", ["Delta"])
    monkeypatch.setattr(sync, "fetch", fake_fetch)

    sync.main()

    text = out.read_text()
    assert "| Delta | public | Python | 5 |" in text
    assert "| Old |" not in text
    assert text.index(sync.MARKER) < text.index("## Atlas (manual)")
    assert "1,240 users" in text


def test_missing_workspace_stops_before_writing(tmp_path):
    env = {
        key: value
        for key, value in os.environ.items()
        if key not in ("ROLECALL_WORKSPACE", "ROLECALL_GITHUB_OWNER")
    }
    result = subprocess.run(
        [sys.executable, str(SCRIPT)],
        capture_output=True,
        text=True,
        env=env,
        cwd=tmp_path,
    )

    assert result.returncode != 0
    assert "--data" in result.stderr
    assert "ROLECALL_WORKSPACE" in result.stderr
