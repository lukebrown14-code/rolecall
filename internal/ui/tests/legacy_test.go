package ui_test

import (
	"path/filepath"
	"testing"
)

// A Harness with old instructions writes jobs/SHORTLIST.md while the app is open:
// nothing reads it, so the footer says so instead of the roles silently not appearing.
func TestLiveLegacyShortlistWarnsInTheFooter(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.write(filepath.Join(h.root, "jobs", "SHORTLIST.md"), "# Shortlist\n\n| Role | Status |\n| --- | --- |\n| Repro | New |\n")
	h.wait("SHORTLIST.md isn't read")
}

func TestLegacyFileAtStartupWarnsInTheFooter(t *testing.T) {
	h := newLiveHarness(t, func(root string) {
		writeFile(t, filepath.Join(root, "SHORTLIST.md"), "# Shortlist\n")
	})
	requireAll(t, h.finalView(), "SHORTLIST.md isn't read by rolecall")
}
