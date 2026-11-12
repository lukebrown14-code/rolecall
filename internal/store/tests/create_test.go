package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func TestCreateRefusesReservedTrackNames(t *testing.T) {
	s := fixtureProfileStore(t)
	for _, track := range []string{"", ".", "..", "_archive", "a/b", `a\b`} {
		if _, err := s.Create(store.CreateFields{Title: "T", Company: "C", Track: track}, time.Now()); err == nil {
			t.Fatalf("Create accepted Track %q", track)
		}
	}
}

func TestCreateApplication(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	now := time.Date(2026, 11, 12, 0, 30, 0, 0, time.FixedZone("east", 11*3600))
	fields := store.CreateFields{Title: "Engineer #1", Company: "Acme", Track: "New", URL: "https://example.test", Closes: "2026-12-01", Fit: "Good"}
	folder, err := s.Create(fields, now)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range apps {
		if a.Folder == folder {
			found = true
			if a.Malformed != nil || a.Title != fields.Title || a.Company != fields.Company || a.Track != fields.Track || a.URL != fields.URL || a.Closes != fields.Closes || a.Fit != fields.Fit || a.Status != "Lead" || len(a.StatusLog) != 1 || a.StatusLog[0].At != "2026-11-11T13:30:00Z" {
				t.Fatalf("%+v", a)
			}
		}
	}
	if !found {
		t.Fatal("missing Application")
	}
	b, err := os.ReadFile(filepath.Join(folder, "jd.md"))
	if err != nil || len(b) != 0 {
		t.Fatalf("jd: %q %v", b, err)
	}
	entries, _ := os.ReadDir(folder)
	if len(entries) != 2 {
		t.Fatal(entries)
	}
}

func TestCreateSlugAndCollisions(t *testing.T) {
	for _, tc := range []struct{ company, title, want string }{
		{"Acme, Inc.", "Senior Engineer!", "acme-inc-senior-engineer-2026-11-11"},
		{"  Acme  ", "-- Engineer --", "acme-engineer-2026-11-11"},
		{"Café 東京", "Déveloper", "caf-d-veloper-2026-11-11"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
				t.Fatal(err)
			}
			s, _ := store.Open(root)
			for _, suffix := range []string{"", "-2", "-3"} {
				folder, err := s.Create(store.CreateFields{Company: tc.company, Title: tc.title, Track: "SWE"}, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC))
				if err != nil {
					t.Fatal(err)
				}
				if filepath.Base(folder) != tc.want+suffix {
					t.Fatal(folder)
				}
				b, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
				if err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"url:", "closes:", "fit:"} {
					if strings.Contains(string(b), key) {
						t.Fatal(string(b))
					}
				}
			}
		})
	}
}
