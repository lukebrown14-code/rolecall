"""The Application JSON shape is the contract between the Go store and the Python
reader. Both sides assert ``fixtures/application-keys.json``, so a key added on one
side without the other fails here or in ``internal/store/tests``.
"""

import importlib.util
import json
from pathlib import Path

from test_meta_contract import DEFAULTS

ROOT = Path(__file__).resolve().parents[2]
META = ROOT / "plugin" / "scripts" / "meta.py"


def contract():
    return json.loads((ROOT / "fixtures" / "application-keys.json").read_text())


def load_meta():
    spec = importlib.util.spec_from_file_location("meta_contract_keys", META)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_scan_shape_matches_the_shared_contract():
    keys = contract()
    got = set(load_meta().Application(Path("/tmp/none")).to_dict())
    assert got == set(keys["data"]) | set(keys["scanOnly"])


def test_expected_json_defaults_match_the_shared_contract():
    assert set(DEFAULTS) == set(contract()["data"])
