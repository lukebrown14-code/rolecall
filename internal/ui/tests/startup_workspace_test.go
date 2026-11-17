package ui_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// An invalid workspace.json does not lock the Owner out: the app starts with no Track
// filter, says what is wrong, and Settings shows the path and parse error with e.
func TestInvalidWorkspaceFileStillStarts(t *testing.T) {
	for name, bad := range map[string]string{"syntax": `{"tracks": [`, "type": `{"tracks": "SWE"}`} {
		t.Run(name, func(t *testing.T) {
			h := newSettingsHarness(t, 120, bad, func(string) {})
			requireAll(t, squash(h.tm.plain()), "invalid Workspace settings", "6 then e to fix it")
			h.tm.Type("6")
			requireUnwrapped(t, h.tm.plain(), filepath.Join(h.root, "workspace.json"), "Press e to fix it")
		})
	}
}

// The banner names keys that work where it shows (e on a table screen edits the row, so it
// says 6 then e), the Settings tab keeps a ⚠ after the banner is dismissed, and the
// Settings screen has e in its key line.
func TestInvalidWorkspaceFileHintsAndMarker(t *testing.T) {
	h := newSettingsHarness(t, 80, `{"tracks": [`, func(string) {})
	requireAll(t, squash(h.tm.plain()), "6 then e to fix it")
	if strings.Contains(squash(h.tm.plain()), "6 Settings, e to fix") {
		t.Fatal("old hint still shown")
	}
	h.tm.Type("esc")
	if !strings.Contains(firstLine(h.tm.plain()), "⚠") {
		t.Fatalf("no ⚠ on the tab bar after the banner is dismissed:\n%s", h.tm.plain())
	}
	h.tm.Type("6")
	view := h.tm.plain()
	requireAll(t, view, "e fix")
	if !strings.Contains(firstLine(view), "⚠") {
		t.Fatalf("no ⚠ on the tab bar in Settings:\n%s", view)
	}
}

// A long workspace.json path wraps on the Settings error screen rather than being cut.
func TestInvalidWorkspaceFilePathWraps(t *testing.T) {
	h := newSettingsHarness(t, 60, `{"tracks": [`, func(string) {})
	h.tm.Type("6")
	path := filepath.Join(h.root, "workspace.json")
	requireUnwrapped(t, h.tm.plain(), path)
}

func firstLine(s string) string { l, _, _ := strings.Cut(s, "\n"); return l }
