"""Every plugin script resolves the Workspace from its own argv, so an in-process call
behaves like a shelled-out one.

The bug this guards: ``main(argv)`` parsed ``argv`` but then resolved the Workspace from
``sys.argv``, so ``main(["--data", ws, ...])`` failed with "no Workspace".
"""

import importlib.util
import io
import sys
from pathlib import Path

import pytest

from conftest import copy_fixture_workspace

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "plugin" / "scripts"
sys.path.insert(0, str(SCRIPTS))

import workspace  # noqa: E402


def load(filename):
    """Load a plugin script as a module, the way the plugin runs it."""
    name = "under_test_" + filename.replace("-", "_").removesuffix(".py")
    spec = importlib.util.spec_from_file_location(name, SCRIPTS / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_resolve_workspace_honours_data(tmp_path, monkeypatch):
    ws = copy_fixture_workspace(tmp_path)
    monkeypatch.delenv(workspace.ENV_VAR, raising=False)
    assert workspace.resolve_workspace(str(ws)) == ws.resolve()


def test_resolve_workspace_falls_back_to_env(tmp_path, monkeypatch):
    ws = copy_fixture_workspace(tmp_path)
    monkeypatch.setenv(workspace.ENV_VAR, str(ws))
    assert workspace.resolve_workspace() == ws.resolve()


def test_data_from_argv_reads_both_forms():
    assert workspace.data_from_argv(["--data", "/x"]) == "/x"
    assert workspace.data_from_argv(["--data=/x"]) == "/x"
    assert workspace.data_from_argv([]) is None


def test_meta_resolves_data_from_argv(tmp_path, capsys):
    ws = copy_fixture_workspace(tmp_path)
    load("meta.py").main(["--data", str(ws)])
    assert "orbit" in capsys.readouterr().out


def test_migrate_resolves_data_from_argv(tmp_path, capsys):
    ws = copy_fixture_workspace(tmp_path, ROOT / "fixtures" / "dashboard-era")
    load("migrate.py").main(["--data", str(ws), "--dry-run"])
    assert capsys.readouterr().out != ""


def test_sweep_resolves_data_from_argv(tmp_path, capsys):
    ws = copy_fixture_workspace(tmp_path)
    load("sweep.py").main(["--data", str(ws), "--platforms"])
    assert "greenhouse" in capsys.readouterr().out.lower()


def test_sync_github_resolves_data_from_argv(tmp_path, monkeypatch):
    ws = copy_fixture_workspace(tmp_path)
    sync = load("sync-github.py")
    monkeypatch.setattr(sync, "OWNER", "")
    monkeypatch.setattr(sync, "REPOS", [])
    with pytest.raises(SystemExit) as exc:
        sync.main(["--data", str(ws)])
    # Resolved the Workspace, then refused the missing GitHub owner.
    assert "ROLECALL_GITHUB_OWNER" in str(exc.value)


def test_set_fit_resolves_data_from_argv(tmp_path):
    ws = copy_fixture_workspace(tmp_path)
    load("set-fit.py").main(["--data", str(ws), "--folder", "orbit", "--fit", "Good"])
    assert 'fit: "Good"' in (ws / "jobs" / "SWE" / "orbit" / "meta.yml").read_text()


def test_new_job_resolves_data_from_argv(tmp_path, monkeypatch):
    ws = copy_fixture_workspace(tmp_path)
    new_job = load("new-job.py")
    monkeypatch.setattr(sys, "stdin", io.StringIO(""))
    with pytest.raises(SystemExit) as exc:
        new_job.main(["--data", str(ws), "--fetch-only", "--paste"])
    # Resolved the Workspace, then refused the too-short job description.
    assert "Job description" in str(exc.value)
