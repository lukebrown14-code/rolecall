package ui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// snapshotStates are the screens and pop-ups a design review looks at, named by what
// was pressed.
var snapshotStates = []struct {
	name string
	keys []string
	env  map[string]string // set for this state only, e.g. the light theme
}{
	{"01-leads", []string{"1"}, nil},
	{"02-drafts", []string{"2"}, nil},
	{"03-applied-all", []string{"3"}, nil},
	{"04-applied-interviewing", []string{"3", "tab", "tab"}, nil},
	{"05-detail", []string{"3", "enter"}, nil},
	{"06-detail-docs-tab", []string{"3", "enter", "left"}, nil}, // left wraps to the last tab, Docs, at every size
	{"07-move-popup", []string{"1", "m"}, nil},
	{"08-add-form", []string{"a"}, nil},
	{"09-add-form-error", []string{"a", "enter"}, nil},
	{"10-help", []string{"?"}, nil},
	{"11-search-typing", []string{"/", "p", "l"}, nil},
	{"12-search-no-match", []string{"/", "z", "z", "q", "enter"}, nil},
	{"13-error", []string{"G", "m", "d"}, nil},
	{"14-track-cycled", []string{"1", "t"}, nil},
	{"15-stats", []string{"4"}, nil},
	{"16-resume-reader", []string{"3", "/docs-written", "enter", "enter", "left", "left", "left", "left", "left"}, nil}, // wraps to Docs, then back to Resume, at every size
	{"17-profile", []string{"5"}, nil},
	{"18-settings-search", []string{"6"}, nil},
	{"19-settings-documents", []string{"6", "]"}, nil},
	{"20-settings-tracking", []string{"6", "]", "]"}, nil},
	{"21-settings-machine", []string{"6", "]", "]", "]"}, nil},
	{"22-settings-search-popup", []string{"6", "enter"}, nil},
	{"23-settings-gate-popup", []string{"6", "]", "enter"}, nil},
	{"24-settings-track-add", []string{"6", "]", "]", "a"}, nil},
	{"25-settings-track-remove-refused", []string{"6", "]", "]", "x"}, nil},
	{"26-settings-track-rename", []string{"6", "]", "]", "r"}, nil},
	{"27-settings-machine-edit", []string{"6", "]", "]", "]", "enter"}, nil},
	{"28-light-applied-all", []string{"3"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"29-light-move-popup", []string{"1", "m"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"30-light-detail", []string{"3", "enter"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"31-light-profile", []string{"5"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"32-ascii-applied-all", []string{"3"}, map[string]string{"ROLECALL_ASCII": "1"}},
	{"33-no-color-applied-all", []string{"3"}, map[string]string{"NO_COLOR": "1"}},
	{"34-toast-after-move", []string{"3", "m", "i"}, nil},
	{"35-command-palette", []string{"1", ":", "mo"}, nil},
	{"36-search-chip-detail", []string{"3", "/docs-written", "enter", "enter"}, nil},
	{"37-error-fix-hint", []string{"G", "enter", "m", "d"}, nil},
	{"41-add-form-save-errors", []string{"a", "ctrl+s"}, nil},
	{"42-add-form-discard", []string{"a", "tab", "Engineer", "esc"}, nil},
	{"43-settings-tracks-rows", []string{"6", "]", "]"}, nil},
	{"44-settings-machine-edit-editor", []string{"6", "]", "]", "]", "enter"}, nil},
	{"45-settings-gate-refused", []string{"6", "]", "enter", "backspace", "backspace", "backspace", "100", "enter"}, nil},
	{"46-settings-saved-undo", []string{"6", "]", "enter", "backspace", "backspace", "backspace", "300", "enter"}, nil},
	{"47-offer-celebration", []string{"3", "tab", "m", "o"}, nil},
	{"48-detail-next-draft", []string{"2", "enter"}, nil},
	{"49-leads-sorted-newest", []string{"1", "o"}, nil},
	{"51-workspace-menu", []string{"3", " "}, nil},
	{"52-strip-focus", []string{"3", "S", "right"}, nil},
	{"53-settings-dirty", []string{"6", "]", "]", "j", "j", "j", "enter", "backspace", "backspace", "7", "enter"}, nil},
	{"54-status-filter-applied", []string{"3", "s"}, nil},
	{"55-follow-up-detail", []string{"3", "/Engineer", "enter", "j"}, nil},
	{"56-dismiss-toast", []string{"1", "x"}, nil},
}

// editedSnapshotStates start from an edited copy of the fixture: the first-run screen
// needs a Workspace with no Applications.
var editedSnapshotStates = []struct {
	name string
	edit func(root string)
	keys []string
}{
	{"50-first-run", func(root string) { _ = os.RemoveAll(filepath.Join(root, "jobs")) }, []string{"1"}},
}

// sizedSnapshotStates are states that only exist at one size, written at that size
// whatever $ROLECALL_SNAP_SIZE says: the too-small notice and the detail view's strip.
var sizedSnapshotStates = []struct {
	name string
	w, h int
	keys []string
}{
	{"38-too-small", 48, 12, []string{"1"}},
	{"39-detail-strip", 80, 24, []string{"3", "enter"}},
	{"40-detail-strip-lead", 60, 16, []string{"1", "j", "enter"}},
}

// TestSnapshots writes every snapshotState as .txt (plain) and .ansi (with colour) to
// $ROLECALL_SNAP_DIR, at $ROLECALL_SNAP_SIZE (default 120x36). Run it with
// scripts/snap.sh; it skips in a normal test run.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("ROLECALL_SNAP_DIR")
	if dir == "" {
		t.Skip("set ROLECALL_SNAP_DIR, or run scripts/snap.sh")
	}
	w, h := 120, 36
	if size := os.Getenv("ROLECALL_SNAP_SIZE"); size != "" {
		if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil {
			t.Fatalf("ROLECALL_SNAP_SIZE %q: want WIDTHxHEIGHT", size)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, w, h int, env map[string]string, keys []string, edit ...func(string)) {
		t.Run(name, func(t *testing.T) {
			for k, v := range env {
				t.Setenv(k, v)
			}
			var change func(string)
			if len(edit) > 0 {
				change = edit[0]
			}
			view := runKeysSized(t, fixtureCopy(t, change), w, h, keys...)
			base := filepath.Join(dir, fmt.Sprintf("%s-%dx%d", name, w, h))
			for ext, body := range map[string]string{".txt": ansi.Strip(view), ".ansi": view} {
				if err := os.WriteFile(base+ext, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	for _, s := range snapshotStates {
		write(s.name, w, h, s.env, s.keys)
	}
	for _, s := range sizedSnapshotStates {
		write(s.name, s.w, s.h, nil, s.keys)
	}
	for _, s := range editedSnapshotStates {
		write(s.name, w, h, nil, s.keys, s.edit)
	}
}
