import importlib.util
import subprocess
from pathlib import Path


SCRIPT = Path(__file__).with_name("check-personal-data.py")
spec = importlib.util.spec_from_file_location("personal_data", SCRIPT)
scanner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scanner)


def scan_text(tmp_path, text):
    subprocess.run(["git", "init", "--quiet", str(tmp_path)], check=True)
    path = tmp_path / "example.txt"
    path.write_text(text)
    subprocess.run(["git", "-C", str(tmp_path), "add", "example.txt"], check=True)
    return scanner.scan(tmp_path, set())


def test_finds_us_national_phone_numbers(tmp_path):
	findings = scan_text(tmp_path, "Call (415) 555-0132.\n")
	assert findings == ["example.txt:1: phone"]
	findings = scan_text(tmp_path, "Call 212-555-0198.\n")
	assert findings == ["example.txt:1: phone"]
