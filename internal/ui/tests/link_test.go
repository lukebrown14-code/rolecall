package ui_test

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/process"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

const adURL = "https://boards.example.test/acme/backend"

// linkWorkspace is a fresh Workspace with one Lead whose meta.yml has url set to link.
func linkWorkspace(t *testing.T, link string) string {
	t.Helper()
	root := freshWorkspace(t)
	meta := "schema: 1\ntitle: Backend Engineer\ncompany: Acme\ntrack: SWE\n"
	if link != "" {
		meta += "url: " + link + "\n"
	}
	meta += "status_log:\n  - {status: Lead, at: 2026-11-01T01:00:00Z}\n"
	writeFile(t, filepath.Join(root, "jobs", "SWE", "acme-backend", "meta.yml"), meta)
	return root
}

func TestLinkOpensTheAdInTheBrowser(t *testing.T) {
	h := &fakeHarness{}
	d := newDriver(t, linkWorkspace(t, adURL), setupNow, 120, 40, ui.WithExternalRunner(h.runner))
	requireAll(t, d.plain(), "w open link", "c copy link")
	d.Type("w")
	if len(h.calls) != 1 {
		t.Fatalf("w ran %d commands, want 1", len(h.calls))
	}
	want := "xdg-open"
	if runtime.GOOS == "darwin" {
		want = "open"
	}
	if got := h.calls[0]; got[0] != want || got[len(got)-1] != adURL {
		t.Fatalf("w ran %v, want %s %s", got, want, adURL)
	}
}

func TestLinkCopiesTheURL(t *testing.T) {
	var copied string
	d := newDriver(t, linkWorkspace(t, adURL), setupNow, 120, 40,
		ui.WithClipboard(func(s string) error { copied = s; return nil }))
	d.Type("c")
	if copied != adURL {
		t.Fatalf("copied %q, want %q", copied, adURL)
	}
	requireAll(t, d.plain(), "Copied link")
}

func TestLinkCopyWithoutAClipboardProgramUsesTheTerminal(t *testing.T) {
	d := newDriver(t, linkWorkspace(t, adURL), setupNow, 120, 40,
		ui.WithClipboard(func(string) error { return process.ErrNoClipboard }))
	d.Type("c")
	requireAll(t, d.plain(), "Copied link through the terminal")
}

func TestLinkCopyFailureIsReported(t *testing.T) {
	d := newDriver(t, linkWorkspace(t, adURL), setupNow, 120, 40,
		ui.WithClipboard(func(string) error { return errors.New("pbcopy failed") }))
	d.Type("c")
	requireAll(t, d.plain(), "pbcopy failed")
}

// A url that is not an http or https link is never handed to the desktop launcher,
// which would open a local file or an app just the same.
func TestLinkKeysOnlyForWebLinks(t *testing.T) {
	for _, link := range []string{"", "file:///etc/passwd", "javascript:alert(1)", "boards.example.test/no-scheme"} {
		h := &fakeHarness{}
		copied := false
		d := newDriver(t, linkWorkspace(t, link), setupNow, 120, 40, ui.WithExternalRunner(h.runner),
			ui.WithClipboard(func(string) error { copied = true; return nil }))
		if v := d.plain(); strings.Contains(v, "open link") || strings.Contains(v, "copy link") {
			t.Errorf("url %q: link keys shown:\n%s", link, v)
		}
		d.Type("w")
		d.Type("c")
		if len(h.calls) != 0 || copied {
			t.Errorf("url %q: w ran %v, c copied %v; want nothing", link, h.calls, copied)
		}
	}
}
