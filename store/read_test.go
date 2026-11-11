package store_test

import (
	"github.com/lukebrown14-code/rolecall/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMeta(t *testing.T, text string) store.Application {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "jobs", "SWE", "fern")
	if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range apps {
		if a.Folder == dir {
			return a
		}
	}
	t.Fatal("missing Application")
	return store.Application{}
}
func TestReadMetadata(t *testing.T) {
	a := readMeta(t, "title: \"Engineer #1\" # note\ncompany: Acme\ntrack: Unknown\nurl: https://example.com\ncloses: 2026-12-01\nfit: Strong\nresume_only: true\n")
	if a.Title != "Engineer #1" || a.URL != "https://example.com" || a.Closes != "2026-12-01" || a.Fit != "Strong" || !a.ResumeOnly || a.Schema != 1 || a.Track != "Unknown" {
		t.Fatalf("%+v", a)
	}
}

func TestStatusHistoryAndMalformedFiles(t *testing.T) {
	base := "title: Engineer\ncompany: Acme\ntrack: SWE\n"
	for _, status := range []string{"Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed"} {
		t.Run(status, func(t *testing.T) {
			a := readMeta(t, base+"status_log:\n  - {status: Applied, at: 2025-01-01, approx: true}\n  - {status: "+status+", at: 2026-01-01T01:00:00Z}\n")
			if a.Malformed != nil || a.Status != status || a.AppliedDate.At != "2025-01-01" || !a.AppliedDate.Approx || a.LastChange.At != "2026-01-01T01:00:00Z" || a.LastChange.Approx {
				t.Fatalf("%+v", a)
			}
		})
	}
	for _, suffix := range []string{"status_log: []\n", ""} {
		a := readMeta(t, base+suffix)
		if a.Malformed != nil || a.Status != "Lead" || a.ResumeOnly || a.Fit != "" || a.AppliedDate != nil || a.LastChange != nil {
			t.Fatalf("%+v", a)
		}
	}
	for _, suffix := range []string{"bad key: x\n", "nested:\n  child: value\n", "status_log:\n  - {status: lead, at: 2026-01-01}\n", "status_log:\n  - nope\n", "title: duplicate\n", "fit: Excellent\n", "resume_only: yes\n", "status_log:\n  - {status: Lead, at: yesterday}\n"} {
		t.Run(suffix, func(t *testing.T) {
			a := readMeta(t, base+suffix)
			if a.Malformed == nil {
				t.Fatalf("expected malformed: %+v", a)
			}
			if !strings.Contains(a.Malformed.Error(), filepath.Join(a.Folder, "meta.yml")) {
				t.Fatal(a.Malformed)
			}
		})
	}
}

func TestMalformedMetaErrorNamesPathWithoutValues(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "jobs", "SWE", "bad")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	const sentinel = "PrivateMetaSentinel"
	body := "title: Engineer\ncompany: Acme\ntrack: SWE\nstatus_log:\n  - {status: " + sentinel + ", at: 2026-01-01}\n"
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil || len(apps) != 1 {
		t.Fatalf("scan: %v, %v", apps, err)
	}
	message := apps[0].Malformed.Error()
	if !strings.Contains(message, filepath.Join(folder, "meta.yml")) || strings.Contains(message, sentinel) {
		t.Fatalf("malformed metadata error should name path without quoting values: %s", message)
	}
}
