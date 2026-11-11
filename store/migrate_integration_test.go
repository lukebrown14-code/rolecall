package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lukebrown14-code/rolecall/store"
)

// TestMigrateDashboardEra runs the one-time Python migration against a copy of
// the dashboard-era fixture, then proves the Go store reads every migrated
// Application on the expected Status. This is the cross-language acceptance
// check: Python writes, Go reads.
func TestMigrateDashboardEra(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/dashboard-era")); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../plugin/scripts/migrate.py")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, script, "--data", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("migrate.py: %v\n%s", err, out)
	}

	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"acme-dev-2026-01-01":                          "Draft",
		"globex-engineer-2026-02-01":                   "Interviewing",
		"initech-analyst-2026-03-01":                   "Applied",
		"umbrella-senior-2026-04-01":                   "Interviewing",
		"hooli-pm-2026-05-01":                          "Offer",
		"wayne-qa-2026-06-01":                          "Rejected",
		"stark-ops-2026-07-01":                         "Withdrawn",
		"cyberdyne-ml-2026-08-01":                      "Applied",
		"northwind-platform-engineer-2026-09-01":       "Lead",
		"northwind-platform-engineer-2026-09-01-2":     "Lead",
		"initech-site-reliability-engineer-2026-08-15": "Dismissed",
		"contoso-data-engineer-2026-07-20":             "Dismissed",
		"fictional-lead-2026-09-03":                    "Lead",
	}
	if len(apps) != len(want) {
		t.Fatalf("got %d Applications, want %d", len(apps), len(want))
	}
	for _, app := range apps {
		if app.Malformed != nil {
			t.Fatalf("%s: malformed meta.yml: %v", app.Folder, app.Malformed)
		}
		name := filepath.Base(app.Folder)
		status, ok := want[name]
		if !ok {
			t.Fatalf("unexpected Application %s", name)
		}
		if app.Status != status {
			t.Errorf("%s: Status = %q, want %q", name, app.Status, status)
		}
		// A date-only entry is migration-only and must carry approx: true.
		for _, entry := range app.StatusLog {
			if len(entry.At) == len("2006-01-02") && !entry.Approx {
				t.Errorf("%s: date-only entry %+v has no approx flag", name, entry)
			}
		}
	}

	initech, err := s.Rescan(filepath.Join(root, "jobs", "SWE", "initech-analyst-2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	if initech == nil || initech.AppliedDate == nil {
		t.Fatal("Initech has no Applied entry")
	}
	if !initech.AppliedDate.Approx || initech.AppliedDate.At != "2026-02-20" {
		t.Errorf("Initech Applied = %+v, want 2026-02-20 approx", *initech.AppliedDate)
	}
}
