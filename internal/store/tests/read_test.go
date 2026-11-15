package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func readMeta(t *testing.T, text string) store.Application {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
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

func TestNewerMetaSchemaIsRejectedWithUpgradeGuidance(t *testing.T) {
	a := readMeta(t, "schema: 2\ntitle: Engineer\ncompany: Acme\ntrack: SWE\n")
	if a.Malformed == nil || !strings.Contains(a.Malformed.Error(), "upgrade rolecall") {
		t.Fatalf("newer schema should be rejected with upgrade guidance: %+v", a)
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

// A scraped title can carry escapes a quoted scalar decodes (an OSC that retitles the
// window, a CSI that clears the screen). They never reach the UI, and line breaks and
// tabs read as spaces, so a row stays one line.
func TestReadCleansControlCharactersFromDisplayedStrings(t *testing.T) {
	a := readMeta(t, `title: "Evil\u001b]0;PWNED\u0007 title\there\nnext"`+"\n"+
		`company: "Co\e[2J\u009b"`+"\n"+`track: "SWE\r"`+"\n"+`url: "https://x.test/\u0000a"`+"\n")
	if a.Malformed != nil {
		t.Fatal(a.Malformed)
	}
	for _, got := range []string{a.Title, a.Company, a.Track, a.URL} {
		if strings.ContainsFunc(got, func(r rune) bool { return r < 0x20 || (r >= 0x7f && r <= 0x9f) }) {
			t.Fatalf("control character kept: %q", got)
		}
	}
	if a.Title != "Evil]0;PWNED title here next" || a.Company != "Co[2J" || a.Track != "SWE" || a.URL != "https://x.test/a" {
		t.Fatalf("%q %q %q %q", a.Title, a.Company, a.Track, a.URL)
	}
}

// Bidi controls and zero-width characters can reorder or hide a row, so they go too; the
// joiners that shape emoji and scripts stay.
func TestDisplayTextDropsBidiAndZeroWidth(t *testing.T) {
	in := "👩\u200d💻 a\u202eRTL\u2066x\u2069\u200bZW\u200c\ufeff\x9b"
	if got, want := store.DisplayText(in), "👩\u200d💻 aRTLxZW\u200c\ufffd"; got != want {
		t.Fatalf("DisplayText(%q) = %q, want %q", in, got, want)
	}
}

// A document keeps its lines and tabs so the Markdown still renders; every escape goes.
func TestDocumentTextKeepsLinesAndDropsEscapes(t *testing.T) {
	in := "# JD\r\n\n\tEvil \x1b]0;PWNED\x07 and\x1b]52;c;SGVsbG8=\x07 \u202eclip\u2028end"
	if got, want := store.DocumentText(in), "# JD\n\n\tEvil ]0;PWNED and]52;c;SGVsbG8= clip\nend"; got != want {
		t.Fatalf("DocumentText(%q) = %q, want %q", in, got, want)
	}
	if plain := "# Café\n\n- 東京 🚀\n"; store.DocumentText(plain) != plain {
		t.Fatalf("DocumentText changed plain Markdown")
	}
}

func TestDisplayTextLeavesPlainTextAlone(t *testing.T) {
	for in, want := range map[string]string{"Café 東京 🚀": "Café 東京 🚀", "a\u2028b": "a b", "\x1b[31mred": "[31mred"} {
		if got := store.DisplayText(in); got != want {
			t.Fatalf("DisplayText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMalformedErrorTextHasNoControlCharacters(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "jobs", "SWE", "bad\x1b]0;x\x07")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil || len(apps) != 1 || apps[0].Malformed == nil {
		t.Fatalf("scan: %v, %v", apps, err)
	}
	if msg := apps[0].Malformed.Error(); strings.ContainsAny(msg, "\x1b\x07") {
		t.Fatalf("%q", msg)
	}
}
