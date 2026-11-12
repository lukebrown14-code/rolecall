"""The shared plugin modules: one Workspace loader, one signal vocabulary, one writer."""

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))

import signals  # noqa: E402
import workspace  # noqa: E402
import workspaceio  # noqa: E402


def test_workspace_load_reads_the_shared_fields(tmp_path):
    (tmp_path / "workspace.json").write_text(
        json.dumps(
            {
                "tracks": ["SWE", "Platform"],
                "search_area": {"places": ["Australia"], "remote": "in-places"},
                "letterWordCap": 300,
                "blockers": ["x"],
                "sources": {"greenhouse": ["a"]},
            }
        )
    )
    cfg = workspace.load(tmp_path)
    assert cfg.tracks == ["SWE", "Platform"]
    assert cfg.search_area == {"places": ["Australia"], "remote": "in-places"}
    assert cfg.gates == {"letterWordCap": 300}
    assert cfg.blockers == ["x"]
    assert cfg.sources == {"greenhouse": ["a"]}


def test_workspace_load_defaults_on_missing_or_broken_file(tmp_path):
    assert workspace.load(tmp_path).tracks == []
    (tmp_path / "workspace.json").write_text("{not json")
    cfg = workspace.load(tmp_path)
    assert cfg.tracks == [] and cfg.gates == {} and cfg.sources == {}


def test_signals_blockers_for_uses_configured_or_defaults():
    assert signals.blockers_for(None) == signals.BLOCKERS
    custom = signals.blockers_for([("secret clearance", "custom")])
    assert custom[0][1] == "custom"
    assert signals.blocker_hits("Nothing of the sort here.", custom) == []
    assert signals.blocker_hits("A secret clearance is required.", custom) == ["custom"]


def test_atomic_write_publishes_bytes_with_the_mode(tmp_path):
    path = tmp_path / "out" / "f.txt"
    path.parent.mkdir()
    workspaceio.atomic_write(path, b"hello", 0o600)
    assert path.read_bytes() == b"hello"
    assert (path.stat().st_mode & 0o777) == 0o600
